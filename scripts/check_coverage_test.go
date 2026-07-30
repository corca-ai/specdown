package main

import (
	"os"
	"strings"
	"testing"
)

func TestCoveragePolicyFixtures(t *testing.T) {
	policy, err := readCoveragePolicy("testdata/coverage-policy.json")
	if err != nil {
		t.Fatalf("read fixture policy: %v", err)
	}

	t.Run("baseline passes", func(t *testing.T) {
		report := readFixtureProfile(t, "testdata/coverage-baseline.out")
		if err := checkCoverage(report, policy); err != nil {
			t.Fatalf("baseline coverage: %v", err)
		}
	})

	t.Run("material regression fails", func(t *testing.T) {
		report := readFixtureProfile(t, "testdata/coverage-regression.out")
		err := checkCoverage(report, policy)
		if err == nil || !strings.Contains(err.Error(), "coverage regression") ||
			!strings.Contains(err.Error(), "example.com/specdown/critical") {
			t.Fatalf("regression error = %v, want package coverage failure", err)
		}
	})
}

func TestParseCoverageProfileMergesDuplicateBlocks(t *testing.T) {
	report, err := parseCoverageProfile(strings.NewReader(`mode: set
example.com/specdown/critical/a.go:1.1,1.2 2 0
example.com/specdown/critical/a.go:1.1,1.2 2 1
`))
	if err != nil {
		t.Fatalf("parse profile: %v", err)
	}
	value := report.Packages["example.com/specdown/critical"]
	if value.Statements != 2 || value.Covered != 2 {
		t.Fatalf("merged package coverage = %+v, want one covered block", value)
	}
}

func TestCoveragePolicyRejectsMissingRequiredPackage(t *testing.T) {
	report := coverageReport{
		Total: coverageValue{Covered: 1, Statements: 1},
		Packages: map[string]coverageValue{
			"example.com/specdown/other": {Covered: 1, Statements: 1},
		},
	}
	policy := coveragePolicy{
		MinimumPackages: map[string]float64{"example.com/specdown/critical": 50},
	}

	err := checkCoverage(report, policy)

	if err == nil || !strings.Contains(err.Error(), "missing from the coverage profile") {
		t.Fatalf("missing-package error = %v", err)
	}
}

func TestReportedCoverageMustMatchProfile(t *testing.T) {
	report := readFixtureProfile(t, "testdata/coverage-baseline.out")
	policy, err := readCoveragePolicy("testdata/coverage-policy.json")
	if err != nil {
		t.Fatalf("read fixture policy: %v", err)
	}

	t.Run("matching output passes", func(t *testing.T) {
		reported, parseErr := parseGoTestCoverage(strings.NewReader(
			"ok  \texample.com/specdown/critical\t0.123s\tcoverage: 75.0% of statements\n",
		))
		if parseErr != nil {
			t.Fatalf("parse go test output: %v", parseErr)
		}
		if err := checkReportedCoverage(report, policy, reported); err != nil {
			t.Fatalf("reported coverage: %v", err)
		}
	})

	t.Run("helper output mismatch fails", func(t *testing.T) {
		reported, parseErr := parseGoTestCoverage(strings.NewReader(
			"ok  \texample.com/specdown/critical\t0.123s\tcoverage: 0.0% of statements\n",
		))
		if parseErr != nil {
			t.Fatalf("parse go test output: %v", parseErr)
		}
		err := checkReportedCoverage(report, policy, reported)
		if err == nil || !strings.Contains(err.Error(), "reports 0.0% but its profile calculates 75.0%") {
			t.Fatalf("mismatch error = %v", err)
		}
	})

	t.Run("uses Go coverage output rounding", func(t *testing.T) {
		halfwayReport := coverageReport{
			Packages: map[string]coverageValue{
				"example.com/specdown/critical": {Covered: 49, Statements: 400},
			},
		}
		reported, parseErr := parseGoTestCoverage(strings.NewReader(
			"ok  \texample.com/specdown/critical\t0.123s\tcoverage: 12.2% of statements\n",
		))
		if parseErr != nil {
			t.Fatalf("parse go test output: %v", parseErr)
		}
		if err := checkReportedCoverage(halfwayReport, policy, reported); err != nil {
			t.Fatalf("reported coverage: %v", err)
		}
	})
}

func readFixtureProfile(t *testing.T, profilePath string) coverageReport {
	t.Helper()
	file, err := os.Open(profilePath)
	if err != nil {
		t.Fatalf("open fixture profile: %v", err)
	}
	defer func() { _ = file.Close() }()
	report, err := parseCoverageProfile(file)
	if err != nil {
		t.Fatalf("parse fixture profile: %v", err)
	}
	return report
}
