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

var checksDir string

func main() {
	flag.StringVar(&checksDir, "checks-dir", "./checks", "Directory containing check scripts")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		if err := handleMessage(scanner.Bytes(), encoder); err != nil {
			fmt.Fprintf(os.Stderr, "shell-adapter: %v\n", err)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "shell-adapter: read stdin: %v\n", err)
		os.Exit(1)
	}
}

func handleMessage(raw []byte, encoder *json.Encoder) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	var msgType string
	if err := json.Unmarshal(fields["type"], &msgType); err != nil {
		return fmt.Errorf("decode type: %w", err)
	}

	switch msgType {
	case "exec":
		var req adapterprotocol.ExecRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return fmt.Errorf("decode exec: %w", err)
		}
		return encoder.Encode(shelladapter.Exec(req.ID, req.Source))
	case "assert":
		var req adapterprotocol.AssertRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return fmt.Errorf("decode assert: %w", err)
		}
		return encoder.Encode(shelladapter.Assert(req.ID, &req, checksDir))
	default:
		return fmt.Errorf("unknown request type %q", msgType)
	}
}
