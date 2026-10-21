package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"WinAutoStartManager/internal/appdata"
)

const version = 1

// Item describes one program to start.
type Item struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Args         string `json:"args"`
	WorkDir      string `json:"workDir"`
	Elevated     bool   `json:"elevated"`
	DelaySeconds int    `json:"delaySeconds"`
	Enabled      bool   `json:"enabled"`
}

// Config is the persisted startup list.
type Config struct {
	Version int    `json:"version"`
	Items   []Item `json:"items"`
}

// Dir returns the active data directory: %LOCALAPPDATA%\AutoStartManager by
// default, or the location chosen in settings.
func Dir() (string, error) {
	return appdata.Root()
}

// Path returns the config file path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config from the default location.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads the config from path. A missing file is not an error.
func LoadFrom(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{Version: version, Items: []Item{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.Version == 0 {
		cfg.Version = version
	}
	if cfg.Items == nil {
		cfg.Items = []Item{}
	}
	return &cfg, nil
}

// Save writes the config to the default location.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the config atomically to path.
func (c *Config) SaveTo(path string) error {
	if c.Version == 0 {
		c.Version = version
	}
	if c.Items == nil {
		c.Items = []Item{}
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}
	}

	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存配置失败: %w", err)
	}
	return nil
}

// NewItem builds a new enabled item with a random ID.
func NewItem(name, path string) Item {
	return Item{
		ID:      newID(),
		Name:    name,
		Path:    path,
		Enabled: true,
	}
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%016x", os.Getpid())
	}
	return hex.EncodeToString(b[:])
}

var envPattern = regexp.MustCompile(`%([^%]+)%`)

// ExpandPath expands %VAR% references in p. Unknown variables are left as-is.
func ExpandPath(p string) string {
	return envPattern.ReplaceAllStringFunc(p, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return m
	})
}

// Validate reports configuration problems in user-facing language.
func (c *Config) Validate() []string {
	var errs []string
	seen := make(map[string]bool, len(c.Items))
	for i, it := range c.Items {
		n := i + 1
		if strings.TrimSpace(it.Name) == "" {
			errs = append(errs, fmt.Sprintf("第 %d 项：名称不能为空", n))
		}
		if strings.TrimSpace(it.Path) == "" {
			errs = append(errs, fmt.Sprintf("第 %d 项：程序路径不能为空", n))
		}
		if it.DelaySeconds < 0 || it.DelaySeconds > 300 {
			errs = append(errs, fmt.Sprintf("第 %d 项：延迟必须在 0–300 秒之间", n))
		}
		if strings.TrimSpace(it.ID) == "" {
			errs = append(errs, fmt.Sprintf("第 %d 项：ID 不能为空", n))
			continue
		}
		if seen[it.ID] {
			errs = append(errs, fmt.Sprintf("第 %d 项：ID 重复", n))
			continue
		}
		seen[it.ID] = true
	}
	return errs
}

// ValidateForSave checks only structural problems (ID and delay). Missing
// name or path is allowed: an unfinished entry is a draft that gets saved but
// never launched, so it must not block editing or deleting other entries.
func (c *Config) ValidateForSave() []string {
	var errs []string
	seen := make(map[string]bool, len(c.Items))
	for i, it := range c.Items {
		n := i + 1
		if it.DelaySeconds < 0 || it.DelaySeconds > 300 {
			errs = append(errs, fmt.Sprintf("第 %d 项：延迟必须在 0–300 秒之间", n))
		}
		if strings.TrimSpace(it.ID) == "" {
			errs = append(errs, fmt.Sprintf("第 %d 项：ID 不能为空", n))
			continue
		}
		if seen[it.ID] {
			errs = append(errs, fmt.Sprintf("第 %d 项：ID 重复", n))
			continue
		}
		seen[it.ID] = true
	}
	return errs
}

// Runnable reports whether the item can actually be launched.
func (it Item) Runnable() bool {
	return strings.TrimSpace(it.Path) != ""
}
