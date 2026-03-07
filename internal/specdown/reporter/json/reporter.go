package json

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"specdown/internal/specdown/core"
)

func Write(report core.Report, outPath string) (err error) {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close report: %w", cerr)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
