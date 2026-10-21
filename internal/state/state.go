package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ItemResult records the outcome of one launch item.
type ItemResult struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OK        bool      `json:"ok"`
	PID       uint32    `json:"pid"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

// Run is one launch pass for a tier.
type Run struct {
	Tier       string       `json:"tier"`
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt time.Time    `json:"finishedAt"`
	Results    []ItemResult `json:"results"`
}

// PathFor returns the state file path for a tier.
func PathFor(dir, tier string) string {
	return filepath.Join(dir, "state-"+tier+".json")
}

// mu serializes access within this process; Windows allows only one writer at a time.
var mu sync.Mutex

// Save writes the run atomically.
func Save(dir, tier string, r Run) error {
	mu.Lock()
	defer mu.Unlock()

	if r.Results == nil {
		r.Results = []ItemResult{}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}
	path := PathFor(dir, tier)
	tmpf, err := os.CreateTemp(dir, "state-*.tmp")
	if err != nil {
		return fmt.Errorf("写入状态失败: %w", err)
	}
	tmp := tmpf.Name()
	defer os.Remove(tmp)
	if _, err := tmpf.Write(raw); err != nil {
		tmpf.Close()
		return fmt.Errorf("写入状态失败: %w", err)
	}
	if err := tmpf.Close(); err != nil {
		return fmt.Errorf("写入状态失败: %w", err)
	}
	for attempt := 0; ; attempt++ {
		err = os.Rename(tmp, path)
		if err == nil {
			return nil
		}
		if attempt >= 20 {
			return fmt.Errorf("保存状态失败: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Load reads the run for a tier. A missing file returns (nil, nil).
func Load(dir, tier string) (*Run, error) {
	mu.Lock()
	defer mu.Unlock()

	path := PathFor(dir, tier)
	var raw []byte
	var err error
	for attempt := 0; ; attempt++ {
		raw, err = os.ReadFile(path)
		if err == nil || os.IsNotExist(err) {
			break
		}
		if attempt >= 20 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取状态失败: %w", err)
	}
	var r Run
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("解析状态失败: %w", err)
	}
	return &r, nil
}
