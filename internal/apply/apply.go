// Package apply writes the outcome of the elevated task registration.
package apply

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Result is the helper process's report.
type Result struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// WriteResult writes the outcome for the parent process.
func WriteResult(path string, applyErr error) error {
	res := Result{OK: applyErr == nil}
	if applyErr != nil {
		res.Error = applyErr.Error()
	}
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, raw, 0o644)
}

// ReadResult reads the helper's report.
func ReadResult(path string) (Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("读取应用结果失败: %w", err)
	}
	var res Result
	if err := json.Unmarshal(raw, &res); err != nil {
		return Result{}, fmt.Errorf("解析应用结果失败: %w", err)
	}
	return res, nil
}

// ApplyAndReport runs apply, writes the result, and returns an exit code.
func ApplyAndReport(resultPath string, exePath string, apply func() error) int {
	err := apply()
	if writeErr := WriteResult(resultPath, err); writeErr != nil && err == nil {
		err = writeErr
	}
	if err != nil {
		return 1
	}
	return 0
}
