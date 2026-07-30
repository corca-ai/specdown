package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type coveragePolicy struct {
	MinimumTotal    float64            `json:"minimumTotal"`
	MinimumPackages map[string]float64 `json:"minimumPackages"`
}

type coverageValue struct {
	Covered    int
	Statements int
}

func (value coverageValue) percent() float64 {
	if value.Statements == 0 {
		return 0
	}
	return 100 * float64(value.Covered) / float64(value.Statements)
}

type coverageReport struct {
	Total    coverageValue
	Packages map[string]coverageValue
}

type profileBlock struct {
	packagePath string
	statements  int
	covered     bool
}

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "", "Go coverage profile to evaluate")
	policyPath := flag.String("policy", "", "JSON coverage policy")
	testOutputPath := flag.String("test-output", "", "go test output to cross-check against the profile")
	flag.Parse()
	if *profilePath == "" || *policyPath == "" {
		return errors.New("usage: go run ./scripts/check_coverage.go -profile <cover.out> -policy <policy.json>")
	}

	policy, err := readCoveragePolicy(*policyPath)
	if err != nil {
		return err
	}
	report, err := readCoverageProfile(*profilePath)
	if err != nil {
		return err
	}
	if *testOutputPath != "" {
		if err := checkTestOutput(*testOutputPath, report, policy); err != nil {
			return err
		}
	}
	if err := printCoverageReport(os.Stdout, report, policy); err != nil {
		return err
	}
	if err := checkCoverage(report, policy); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, "coverage policy satisfied"); err != nil {
		return fmt.Errorf("write coverage result: %w", err)
	}
	return nil
}

func readCoverageProfile(profilePath string) (coverageReport, error) {
	profile, err := os.Open(profilePath)
	if err != nil {
		return coverageReport{}, fmt.Errorf("open coverage profile: %w", err)
	}
	defer func() { _ = profile.Close() }()
	return parseCoverageProfile(profile)
}

func checkTestOutput(testOutputPath string, report coverageReport, policy coveragePolicy) error {
	testOutput, err := os.Open(testOutputPath)
	if err != nil {
		return fmt.Errorf("open go test output: %w", err)
	}
	reported, parseErr := parseGoTestCoverage(testOutput)
	closeErr := testOutput.Close()
	if parseErr != nil {
		return parseErr
	}
	if closeErr != nil {
		return fmt.Errorf("close go test output: %w", closeErr)
	}
	return checkReportedCoverage(report, policy, reported)
}

func readCoveragePolicy(policyPath string) (coveragePolicy, error) {
	file, err := os.Open(policyPath)
	if err != nil {
		return coveragePolicy{}, fmt.Errorf("open coverage policy: %w", err)
	}
	defer func() { _ = file.Close() }()

	var policy coveragePolicy
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return coveragePolicy{}, fmt.Errorf("decode coverage policy: %w", err)
	}
	if policy.MinimumPackages == nil {
		return coveragePolicy{}, errors.New("coverage policy must define minimumPackages")
	}
	if err := validateMinimum("total", policy.MinimumTotal); err != nil {
		return coveragePolicy{}, err
	}
	for packagePath, minimum := range policy.MinimumPackages {
		if err := validateMinimum(packagePath, minimum); err != nil {
			return coveragePolicy{}, err
		}
	}
	return policy, nil
}

func validateMinimum(name string, minimum float64) error {
	if minimum < 0 || minimum > 100 {
		return fmt.Errorf("coverage minimum for %s must be between 0 and 100, got %.2f", name, minimum)
	}
	return nil
}

func parseCoverageProfile(reader io.Reader) (coverageReport, error) {
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		return coverageReport{}, errors.New("coverage profile is empty")
	}
	if !strings.HasPrefix(scanner.Text(), "mode: ") {
		return coverageReport{}, errors.New("coverage profile is missing its mode header")
	}

	blocks := make(map[string]profileBlock)
	for scanner.Scan() {
		key, block, err := parseProfileBlock(scanner.Text())
		if err != nil {
			return coverageReport{}, err
		}
		if existing, ok := blocks[key]; ok {
			if existing.packagePath != block.packagePath || existing.statements != block.statements {
				return coverageReport{}, fmt.Errorf("conflicting duplicate coverage block %q", key)
			}
			block.covered = block.covered || existing.covered
		}
		blocks[key] = block
	}
	if err := scanner.Err(); err != nil {
		return coverageReport{}, fmt.Errorf("read coverage profile: %w", err)
	}
	if len(blocks) == 0 {
		return coverageReport{}, errors.New("coverage profile contains no statements")
	}

	return aggregateCoverageBlocks(blocks), nil
}

func parseProfileBlock(line string) (string, profileBlock, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", profileBlock{}, fmt.Errorf("invalid coverage profile line %q", line)
	}
	statements, err := strconv.Atoi(fields[1])
	if err != nil || statements < 0 {
		return "", profileBlock{}, fmt.Errorf("invalid statement count in coverage profile line %q", line)
	}
	count, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return "", profileBlock{}, fmt.Errorf("invalid execution count in coverage profile line %q", line)
	}
	fileEnd := strings.LastIndex(fields[0], ":")
	if fileEnd < 0 {
		return "", profileBlock{}, fmt.Errorf("invalid source range in coverage profile line %q", line)
	}
	return fields[0], profileBlock{
		packagePath: path.Dir(fields[0][:fileEnd]),
		statements:  statements,
		covered:     count > 0,
	}, nil
}

func aggregateCoverageBlocks(blocks map[string]profileBlock) coverageReport {
	report := coverageReport{Packages: make(map[string]coverageValue)}
	for _, block := range blocks {
		value := report.Packages[block.packagePath]
		value.Statements += block.statements
		report.Total.Statements += block.statements
		if block.covered {
			value.Covered += block.statements
			report.Total.Covered += block.statements
		}
		report.Packages[block.packagePath] = value
	}
	return report
}

func checkCoverage(report coverageReport, policy coveragePolicy) error {
	var regressions []string
	if actual := report.Total.percent(); actual < policy.MinimumTotal {
		regressions = append(regressions, fmt.Sprintf(
			"total coverage %.2f%% is below %.2f%%",
			actual,
			policy.MinimumTotal,
		))
	}
	for packagePath, minimum := range policy.MinimumPackages {
		value, ok := report.Packages[packagePath]
		if !ok || value.Statements == 0 {
			regressions = append(regressions, fmt.Sprintf("required package %s is missing from the coverage profile", packagePath))
			continue
		}
		if actual := value.percent(); actual < minimum {
			regressions = append(regressions, fmt.Sprintf(
				"%s coverage %.2f%% is below %.2f%%",
				packagePath,
				actual,
				minimum,
			))
		}
	}
	if len(regressions) == 0 {
		return nil
	}
	sort.Strings(regressions)
	return fmt.Errorf("coverage regression:\n- %s", strings.Join(regressions, "\n- "))
}

func parseGoTestCoverage(reader io.Reader) (map[string]float64, error) {
	reported := make(map[string]float64)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		for index, field := range fields {
			if field != "coverage:" || index == 0 || index+1 >= len(fields) || !strings.HasSuffix(fields[index+1], "%") {
				continue
			}
			packageIndex := 0
			if fields[0] == "ok" {
				packageIndex = 1
			}
			percentage, err := strconv.ParseFloat(strings.TrimSuffix(fields[index+1], "%"), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid coverage percentage in go test line %q", scanner.Text())
			}
			reported[fields[packageIndex]] = percentage
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read go test output: %w", err)
	}
	return reported, nil
}

func checkReportedCoverage(
	report coverageReport,
	policy coveragePolicy,
	reported map[string]float64,
) error {
	var mismatches []string
	for packagePath := range policy.MinimumPackages {
		value, ok := report.Packages[packagePath]
		if !ok {
			continue
		}
		actual, ok := reported[packagePath]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("%s is missing from go test coverage output", packagePath))
			continue
		}
		expected := fmt.Sprintf("%.1f", value.percent())
		if fmt.Sprintf("%.1f", actual) != expected {
			mismatches = append(mismatches, fmt.Sprintf(
				"%s reports %.1f%% but its profile calculates %s%%",
				packagePath,
				actual,
				expected,
			))
		}
	}
	if len(mismatches) == 0 {
		return nil
	}
	sort.Strings(mismatches)
	return fmt.Errorf("coverage output mismatch:\n- %s", strings.Join(mismatches, "\n- "))
}

func printCoverageReport(writer io.Writer, report coverageReport, policy coveragePolicy) error {
	if _, err := fmt.Fprintf(
		writer,
		"coverage total: %.2f%% (minimum %.2f%%)\n",
		report.Total.percent(),
		policy.MinimumTotal,
	); err != nil {
		return fmt.Errorf("write coverage report: %w", err)
	}
	packagePaths := make([]string, 0, len(policy.MinimumPackages))
	for packagePath := range policy.MinimumPackages {
		packagePaths = append(packagePaths, packagePath)
	}
	sort.Strings(packagePaths)
	for _, packagePath := range packagePaths {
		value := report.Packages[packagePath]
		if _, err := fmt.Fprintf(
			writer,
			"coverage %s: %.2f%% (minimum %.2f%%)\n",
			packagePath,
			value.percent(),
			policy.MinimumPackages[packagePath],
		); err != nil {
			return fmt.Errorf("write coverage report: %w", err)
		}
	}
	return nil
}
