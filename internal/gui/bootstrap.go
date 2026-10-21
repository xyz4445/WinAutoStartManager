package gui

import (
	"fmt"
	"os"
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/schedule"
	"WinAutoStartManager/internal/state"
)

// ItemView is one row sent to the frontend.
type ItemView struct {
	config.Item
	Exists    bool      `json:"exists"`
	LastOK    bool      `json:"lastOK"`
	LastError string    `json:"lastError,omitempty"`
	LastRunAt time.Time `json:"lastRunAt"`
}

// Bootstrap is the full view state for the UI.
type Bootstrap struct {
	Items            []ItemView `json:"items"`
	TaskStatus       string     `json:"taskStatus"`
	LastRun          string     `json:"lastRun"`
	ConfigPath       string     `json:"configPath"`
	LogDir           string     `json:"logDir"`
	ExePath          string     `json:"exePath"`
	DataDir          string     `json:"dataDir"`
	DefaultDataDir   string     `json:"defaultDataDir"`
	LogRetentionDays int        `json:"logRetentionDays"`
}

// RunResult is returned after a manual test run.
type RunResult struct {
	OK    bool   `json:"ok"`
	PID   uint32 `json:"pid"`
	Error string `json:"error,omitempty"`
}

// buildBootstrap merges config, task status and the latest per-tier runs.
func buildBootstrap(cfg *config.Config, taskStatus schedule.Status, runs map[string]*state.Run) Bootstrap {
	b := Bootstrap{
		Items:      []ItemView{},
		TaskStatus: string(taskStatus),
	}

	byID := map[string]state.ItemResult{}
	for _, r := range runs {
		if r == nil {
			continue
		}
		for _, res := range r.Results {
			if prev, ok := byID[res.ID]; !ok || res.StartedAt.After(prev.StartedAt) {
				byID[res.ID] = res
			}
		}
	}

	var latestAt time.Time
	attributed, okCount := 0, 0
	for _, it := range cfg.Items {
		v := ItemView{Item: it}
		if _, err := os.Stat(config.ExpandPath(it.Path)); err == nil {
			v.Exists = true
		}
		if res, ok := byID[it.ID]; ok {
			v.LastOK = res.OK
			v.LastError = res.Error
			v.LastRunAt = res.StartedAt
			attributed++
			if res.OK {
				okCount++
			}
			if res.StartedAt.After(latestAt) {
				latestAt = res.StartedAt
			}
		}
		b.Items = append(b.Items, v)
	}

	if attributed > 0 {
		b.LastRun = fmt.Sprintf("%s · %d/%d 成功",
			latestAt.Format("01-02 15:04"), okCount, attributed)
	}
	return b
}
