package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxFileBytes = 2 << 20
	keepDays     = 14
)

// Logger appends timestamped lines to a per-tier daily file.
type Logger struct {
	mu      sync.Mutex
	f       *os.File
	tier    string
	base    time.Time
	started time.Time
}

// PathFor returns the log path for a tier and day.
func PathFor(dir, tier string, now time.Time) string {
	name := fmt.Sprintf("launcher-%s-%s.log", now.Format("20060102"), tier)
	return filepath.Join(dir, "logs", name)
}

// Open opens (and rotates if needed) the log file for the given tier.
func Open(dir, tier string, now time.Time) (*Logger, error) {
	path := PathFor(dir, tier, now)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	if info, err := os.Stat(path); err == nil && info.Size() >= maxFileBytes {
		rotated := path + ".1"
		_ = os.Remove(rotated)
		if err := os.Rename(path, rotated); err != nil {
			return nil, fmt.Errorf("轮转日志失败: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志失败: %w", err)
	}

	return &Logger{f: f, tier: tier, base: now, started: time.Now()}, nil
}

// Printf writes one timestamped line.
func (l *Logger) Printf(format string, args ...any) {
	if l == nil || l.f == nil {
		return
	}
	ts := l.base.Add(time.Since(l.started)).Format("2006-01-02 15:04:05")
	line := fmt.Sprintf(format, args...)
	l.mu.Lock()
	_, _ = fmt.Fprintf(l.f, "%s [%s] %s\n", ts, l.tier, line)
	l.mu.Unlock()
}

// Close closes the underlying file.
func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.f.Close()
	l.f = nil
	return err
}

// Prune deletes log files older than keepDays.
func Prune(dir string, now time.Time, days int) error {
	logs := filepath.Join(dir, "logs")
	entries, err := os.ReadDir(logs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(logs, e.Name()))
		}
	}
	return nil
}
