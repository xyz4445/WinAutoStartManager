// Package settings persists program preferences that are not part of the
// startup list.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultLogRetentionDays = 14
	MinLogRetentionDays     = 1
	MaxLogRetentionDays     = 365
)

// Settings holds user-adjustable preferences.
type Settings struct {
	LogRetentionDays int `json:"logRetentionDays"`
}

// Path returns the settings file inside root.
func Path(root string) string {
	return filepath.Join(root, "settings.json")
}

// Load reads settings from root. A missing file yields defaults.
func Load(root string) (*Settings, error) {
	s := &Settings{LogRetentionDays: DefaultLogRetentionDays}
	raw, err := os.ReadFile(Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("读取设置失败: %w", err)
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("解析设置失败: %w", err)
	}
	s.Normalize()
	return s, nil
}

// Normalize fills in defaults and clamps the retention window.
func (s *Settings) Normalize() {
	if s.LogRetentionDays == 0 {
		s.LogRetentionDays = DefaultLogRetentionDays
	}
	if s.LogRetentionDays < MinLogRetentionDays {
		s.LogRetentionDays = MinLogRetentionDays
	}
	if s.LogRetentionDays > MaxLogRetentionDays {
		s.LogRetentionDays = MaxLogRetentionDays
	}
}

// Save writes settings atomically into root.
func Save(root string, s *Settings) error {
	s.Normalize()
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化设置失败: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("创建设置目录失败: %w", err)
	}
	path := Path(root)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("写入设置失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存设置失败: %w", err)
	}
	return nil
}
