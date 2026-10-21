package launcher

import (
	"testing"
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/schedule"
)

func runnerDeps(started *[]string) Deps {
	return Deps{
		Now:  func() time.Time { return time.Date(2026, 10, 21, 8, 31, 2, 0, time.Local) },
		Wait: func(time.Duration) {},
		Start: func(it config.Item) (uint32, error) {
			*started = append(*started, it.Name)
			return 7, nil
		},
	}
}

func TestRunTierFlagsBrokenOtherTier(t *testing.T) {
	cfg := &config.Config{Items: []config.Item{
		{ID: "1", Name: "AHK", Path: `C:\a.exe`, Enabled: true, Elevated: true},
	}}
	var started []string
	results := RunTier(schedule.TierAdmin, cfg, schedule.StatusStale, runnerDeps(&started))

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].OK || results[0].Error == "" {
		t.Fatalf("first result should be the other-tier failure: %+v", results[0])
	}
	if len(started) != 1 || started[0] != "AHK" {
		t.Fatalf("started = %v", started)
	}
}

func TestRunTierHealthyOtherTierAddsNothing(t *testing.T) {
	cfg := &config.Config{Items: []config.Item{
		{ID: "1", Name: "rclone", Path: `C:\b.exe`, Enabled: true},
	}}
	var started []string
	results := RunTier(schedule.TierUser, cfg, schedule.StatusOK, runnerDeps(&started))

	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	if !results[0].OK || results[0].Name != "rclone" {
		t.Fatalf("result = %+v", results[0])
	}
	if len(started) != 1 {
		t.Fatalf("started = %v", started)
	}
}

func TestRunTierMissingOtherTierAlsoFlags(t *testing.T) {
	var started []string
	results := RunTier(schedule.TierUser, &config.Config{}, schedule.StatusMissing, runnerDeps(&started))
	if len(results) != 1 || results[0].OK {
		t.Fatalf("results = %+v", results)
	}
}
