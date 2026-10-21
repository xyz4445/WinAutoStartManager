package launcher

import (
	"errors"
	"testing"
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/state"
)

func TestExecuteUsesAbsoluteOffsets(t *testing.T) {
	start := time.Date(2026, 10, 21, 8, 31, 2, 0, time.Local)
	now := start
	var waits []time.Duration

	plan := []PlanItem{
		{Item: config.Item{ID: "1", Name: "a"}, At: 3 * time.Second},
		{Item: config.Item{ID: "2", Name: "b"}, At: 1 * time.Second},
		{Item: config.Item{ID: "3", Name: "c"}, At: 5 * time.Second},
	}

	deps := Deps{
		Now:  func() time.Time { return now },
		Wait: func(d time.Duration) { waits = append(waits, d); now = now.Add(d) },
		Start: func(config.Item) (uint32, error) {
			return 42, nil
		},
		Logf: func(string, ...any) {},
	}

	results := Execute(plan, deps)
	if len(results) != 3 {
		t.Fatalf("results = %d", len(results))
	}
	want := []time.Duration{3 * time.Second, 0, 2 * time.Second}
	if len(waits) != 2 {
		t.Fatalf("waits = %v, want two waits (3s, 2s)", waits)
	}
	if waits[0] != want[0] || waits[1] != want[2] {
		t.Fatalf("waits = %v, want [3s 2s]", waits)
	}
}

func TestExecuteSkipsExpiredOffsets(t *testing.T) {
	start := time.Date(2026, 10, 21, 8, 31, 2, 0, time.Local)
	now := start
	waits := 0

	plan := []PlanItem{
		{Item: config.Item{ID: "1", Name: "a"}, At: 0},
		{Item: config.Item{ID: "2", Name: "b"}, At: 2 * time.Second},
		{Item: config.Item{ID: "3", Name: "c"}, At: 1 * time.Second},
	}
	deps := Deps{
		Now:  func() time.Time { return now },
		Wait: func(d time.Duration) { waits++; now = now.Add(d) },
		Start: func(config.Item) (uint32, error) {
			return 1, nil
		},
	}
	Execute(plan, deps)
	if waits != 1 {
		t.Fatalf("waits = %d, want 1 (only the 2s item)", waits)
	}
}

func TestExecuteRecordsFailure(t *testing.T) {
	now := time.Now()
	fail := errors.New("file not found")

	plan := []PlanItem{
		{Item: config.Item{ID: "1", Name: "bad"}, At: 0},
		{Item: config.Item{ID: "2", Name: "good"}, At: 0},
	}
	started := 0
	deps := Deps{
		Now:  func() time.Time { return now },
		Wait: func(time.Duration) {},
		Start: func(it config.Item) (uint32, error) {
			started++
			if it.Name == "bad" {
				return 0, fail
			}
			return 99, nil
		},
	}
	results := Execute(plan, deps)
	if started != 2 {
		t.Fatalf("later items must still run, started = %d", started)
	}
	if results[0].OK || results[0].Error != "file not found" {
		t.Fatalf("first result = %+v", results[0])
	}
	if !results[1].OK || results[1].PID != 99 {
		t.Fatalf("second result = %+v", results[1])
	}
}

func TestFailuresListsNames(t *testing.T) {
	results := []state.ItemResult{
		{Name: "AHK", OK: false},
		{Name: "rclone", OK: true},
		{Name: "other", OK: false},
	}
	got := Failures(results)
	if len(got) != 2 || got[0] != "AHK" || got[1] != "other" {
		t.Fatalf("failures = %v", got)
	}
}

func TestExecuteLogsEachItem(t *testing.T) {
	now := time.Now()
	var logged []string
	deps := Deps{
		Now:  func() time.Time { return now },
		Wait: func(time.Duration) {},
		Start: func(config.Item) (uint32, error) {
			return 10, nil
		},
		Logf: func(format string, args ...any) {
			logged = append(logged, format)
		},
	}
	Execute([]PlanItem{{Item: config.Item{ID: "1", Name: "a"}}}, deps)
	if len(logged) == 0 {
		t.Fatal("expected at least one log line per item")
	}
}
