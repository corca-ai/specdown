package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
	"github.com/corca-ai/specdown/internal/specdown/shelladapter"
)

var fixturesDir string

func main() {
	flag.StringVar(&fixturesDir, "fixtures-dir", "./fixtures", "Directory containing fixture scripts")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		var request adapterprotocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			fmt.Fprintf(os.Stderr, "shell-adapter: decode request: %v\n", err)
			os.Exit(1)
		}

		switch request.Type {
		case "setup", "teardown":
			continue
		case "runCase":
			result := shelladapter.RunCase(request.ID, request.Case, fixturesDir)
			if err := encoder.Encode(result); err != nil {
				fmt.Fprintf(os.Stderr, "shell-adapter: encode response: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "shell-adapter: unknown request type %q\n", request.Type)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "shell-adapter: read stdin: %v\n", err)
		os.Exit(1)
	}
}
