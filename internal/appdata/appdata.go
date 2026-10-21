// Package appdata resolves the directory that holds all program data and
// migrates it when the user picks a different location.
package appdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	appDirName        = "AutoStartManager"
	registryKeyPath   = `Software\AutoStartManager`
	registryValueName = "DataDir"
)

// keyPath is a variable so tests can use a throwaway key.
var keyPath = registryKeyPath

// DefaultRoot returns %LOCALAPPDATA%\AutoStartManager.
func DefaultRoot() (string, error) {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("无法确定用户数据目录: %w", err)
		}
	}
	return filepath.Join(base, appDirName), nil
}

// Override returns the user-configured data directory, or "" when the
// default location is in use.
func Override() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("读取数据目录设置失败: %w", err)
	}
	defer k.Close()

	v, _, err := k.GetStringValue(registryValueName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("读取数据目录设置失败: %w", err)
	}
	return strings.TrimSpace(v), nil
}

// Root returns the active data directory.
func Root() (string, error) {
	v, err := Override()
	if err != nil {
		return "", err
	}
	if v != "" {
		return filepath.Clean(v), nil
	}
	return DefaultRoot()
}

// ValidateDir normalizes a user-supplied directory and rejects unusable paths.
func ValidateDir(dir string) (string, error) {
	dir = strings.TrimSpace(strings.Trim(dir, `"`))
	if dir == "" {
		return "", errors.New("数据目录不能为空")
	}
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		return "", errors.New("数据目录必须是绝对路径，例如 D:\\AutoStartManager")
	}
	if filepath.Dir(dir) == dir {
		return "", errors.New("不能直接使用盘符根目录，请新建一个文件夹")
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		return "", errors.New("该路径是一个文件，请选择文件夹")
	}
	return dir, nil
}

// SetOverride persists the custom data directory in HKCU.
func SetOverride(dir string) error {
	dir, err := ValidateDir(dir)
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("保存数据目录设置失败: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue(registryValueName, dir); err != nil {
		return fmt.Errorf("保存数据目录设置失败: %w", err)
	}
	return nil
}

// ClearOverride removes the custom data directory setting.
func ClearOverride() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("清除数据目录设置失败: %w", err)
	}
	defer k.Close()
	if err := k.DeleteValue(registryValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("清除数据目录设置失败: %w", err)
	}
	return nil
}

// Migrate copies the managed files from oldRoot to newRoot. The browser cache
// is intentionally not copied; WebView2 recreates it at the new location.
// Existing files in oldRoot are left untouched. When overwrite is false an
// existing config.json in newRoot aborts the migration; the default location
// is migrated back with overwrite=true.
func Migrate(oldRoot, newRoot string, overwrite bool) error {
	oldRoot = filepath.Clean(oldRoot)
	newRoot = filepath.Clean(newRoot)
	if strings.EqualFold(oldRoot, newRoot) {
		return errors.New("新旧数据目录相同")
	}
	if within(newRoot, oldRoot) {
		return errors.New("新数据目录不能位于旧目录内部")
	}
	if !overwrite {
		cfgDst := filepath.Join(newRoot, "config.json")
		if _, err := os.Stat(cfgDst); err == nil {
			return errors.New("新数据目录里已有配置文件，请换一个空目录，或取消勾选迁移")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("检查新数据目录失败: %w", err)
		}
	}
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		return fmt.Errorf("创建新数据目录失败: %w", err)
	}

	for _, name := range []string{
		"config.json",
		"settings.json",
		"state-admin.json",
		"state-user.json",
	} {
		if err := copyFile(filepath.Join(oldRoot, name), filepath.Join(newRoot, name)); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(filepath.Join(oldRoot, "logs"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("读取旧日志目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(newRoot, "logs"), 0o755); err != nil {
		return fmt.Errorf("创建新日志目录失败: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(
			filepath.Join(oldRoot, "logs", e.Name()),
			filepath.Join(newRoot, "logs", e.Name()),
		); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("读取 %s 失败: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", dst, err)
	}
	return nil
}

func within(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
