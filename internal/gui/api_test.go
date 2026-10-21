package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/schedule"
	"WinAutoStartManager/internal/state"
)

func TestRunItemUsesProvidedItem(t *testing.T) {
	a := &App{}
	res, err := a.runItem(config.Item{ID: "unsaved", Path: filepath.Join(t.TempDir(), "missing.exe")})
	if err != nil {
		t.Fatalf("runItem: %v", err)
	}
	if res.OK || !strings.Contains(res.Error, "程序不存在") {
		t.Fatalf("result = %+v, want missing executable error", res)
	}
}

func TestDirtyFlag(t *testing.T) {
	a := &App{}
	if a.hasUnsaved() {
		t.Fatal("new app should be clean")
	}
	a.setDirty(true)
	if !a.hasUnsaved() {
		t.Fatal("dirty flag not set")
	}
	a.setDirty(false)
	if a.hasUnsaved() {
		t.Fatal("dirty flag not cleared")
	}
}

func TestSaveItemsAllowsDraft(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := &App{dataDir: dir, exePath: exe, logDir: filepath.Join(dir, "logs")}

	items := []config.Item{
		{ID: "draft", Enabled: true},
		{ID: "real", Name: "real", Path: exe, Enabled: true},
	}
	if _, err := a.saveItems(items); err != nil {
		t.Fatalf("saveItems: %v", err)
	}

	cfg, err := config.LoadFrom(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(cfg.Items))
	}
	if cfg.Items[0].Path != "" {
		t.Fatalf("draft path = %q, want empty", cfg.Items[0].Path)
	}
}

func TestBuildBootstrapStatuses(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.exe")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.exe")

	cfg := &config.Config{Items: []config.Item{
		{ID: "a", Name: "AHK", Path: existing, Elevated: true, Enabled: true},
		{ID: "b", Name: "bad", Path: missing, Enabled: true},
		{ID: "c", Name: "never", Path: existing, Enabled: true},
	}}
	now := time.Date(2026, 10, 21, 8, 31, 2, 0, time.Local)
	runs := map[string]*state.Run{
		"admin": {
			Tier:       "admin",
			FinishedAt: now.Add(2 * time.Second),
			Results: []state.ItemResult{
				{ID: "a", Name: "AHK", OK: true, PID: 11, StartedAt: now},
				{ID: "b", Name: "bad", OK: false, Error: "file not found", StartedAt: now},
			},
		},
	}

	got := buildBootstrap(cfg, schedule.StatusOK, runs)
	if len(got.Items) != 3 {
		t.Fatalf("items = %d", len(got.Items))
	}
	if !got.Items[0].Exists || !got.Items[0].LastOK || got.Items[0].LastError != "" {
		t.Fatalf("item a = %+v", got.Items[0])
	}
	if got.Items[1].Exists {
		t.Fatal("item b should not exist")
	}
	if got.Items[1].LastOK || got.Items[1].LastError != "file not found" {
		t.Fatalf("item b = %+v", got.Items[1])
	}
	if got.Items[2].LastOK || !got.Items[2].LastRunAt.IsZero() {
		t.Fatalf("item c = %+v", got.Items[2])
	}
	if got.TaskStatus != "ok" {
		t.Fatalf("task status = %q", got.TaskStatus)
	}
	if got.LastRun == "" {
		t.Fatal("last run summary should not be empty")
	}
}

func TestBuildBootstrapEmpty(t *testing.T) {
	got := buildBootstrap(&config.Config{}, schedule.StatusMissing, nil)
	if len(got.Items) != 0 {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.Items == nil {
		t.Fatal("items must be [] not null")
	}
	if got.TaskStatus != "missing" {
		t.Fatalf("task status = %q", got.TaskStatus)
	}
}

func TestBuildBootstrapPicksLatestRun(t *testing.T) {
	old := time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local)
	recent := old.Add(24 * time.Hour)
	cfg := &config.Config{Items: []config.Item{{ID: "a", Name: "a", Path: `C:\x.exe`}}}
	runs := map[string]*state.Run{
		"admin": {Tier: "admin", FinishedAt: old, Results: []state.ItemResult{{ID: "a", OK: true, StartedAt: old}}},
		"user":  {Tier: "user", FinishedAt: recent, Results: []state.ItemResult{{ID: "a", OK: false, Error: "x", StartedAt: recent}}},
	}
	got := buildBootstrap(cfg, schedule.StatusOK, runs)
	if got.LastRun == "" {
		t.Fatal("expected summary")
	}
	if got.Items[0].LastOK || got.Items[0].LastError != "x" {
		t.Fatalf("newest result should win: %+v", got.Items[0])
	}
	if !got.Items[0].LastRunAt.Equal(recent) {
		t.Fatalf("last run at = %v, want %v", got.Items[0].LastRunAt, recent)
	}
}

func TestCombineTaskStatus(t *testing.T) {
	tests := []struct {
		name        string
		admin, user schedule.Status
		want        schedule.Status
	}{
		{"both ok", schedule.StatusOK, schedule.StatusOK, schedule.StatusOK},
		{"stale beats ok", schedule.StatusStale, schedule.StatusOK, schedule.StatusStale},
		{"missing beats stale", schedule.StatusMissing, schedule.StatusStale, schedule.StatusMissing},
		{"missing beats unknown", schedule.StatusMissing, schedule.StatusUnknown, schedule.StatusMissing},
		{"unknown beats ok", schedule.StatusUnknown, schedule.StatusOK, schedule.StatusUnknown},
		{"unknown beats stale", schedule.StatusStale, schedule.StatusUnknown, schedule.StatusUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := combineTaskStatus(tc.admin, tc.user); got != tc.want {
				t.Fatalf("combineTaskStatus(%q, %q) = %q, want %q", tc.admin, tc.user, got, tc.want)
			}
		})
	}
}
