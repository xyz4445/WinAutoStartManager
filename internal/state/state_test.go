package state

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func sampleRun(tier string) Run {
	now := time.Date(2026, 10, 21, 8, 31, 2, 0, time.Local)
	return Run{
		Tier:       tier,
		StartedAt:  now,
		FinishedAt: now.Add(3 * time.Second),
		Results: []ItemResult{{
			ID:        "a1",
			Name:      "AutoHotkey",
			OK:        true,
			PID:       1234,
			StartedAt: now.Add(time.Second),
		}},
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	want := sampleRun("admin")
	if err := Save(dir, "admin", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir, "admin")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("expected run, got nil")
	}
	if got.Tier != want.Tier || len(got.Results) != 1 || got.Results[0].PID != 1234 || !got.Results[0].OK {
		t.Fatalf("unexpected run: %+v", got)
	}
	if !got.StartedAt.Equal(want.StartedAt) {
		t.Fatalf("StartedAt = %v, want %v", got.StartedAt, want.StartedAt)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := Load(t.TempDir(), "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestTiersAreIndependent(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "admin", sampleRun("admin")); err != nil {
		t.Fatal(err)
	}
	userRun := sampleRun("user")
	userRun.Results[0].Name = "rclone"
	if err := Save(dir, "user", userRun); err != nil {
		t.Fatal(err)
	}

	admin, _ := Load(dir, "admin")
	user, _ := Load(dir, "user")
	if admin == nil || user == nil {
		t.Fatal("both tiers should exist")
	}
	if admin.Results[0].Name != "AutoHotkey" {
		t.Fatalf("admin overwritten: %+v", admin)
	}
	if user.Results[0].Name != "rclone" {
		t.Fatalf("user overwritten: %+v", user)
	}
	if _, err := filepath.Glob(filepath.Join(dir, "state-*.json")); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentSaveKeepsFileValid(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r := sampleRun("admin")
				r.Results[0].PID = uint32(n*1000 + j)
				if err := Save(dir, "admin", r); err != nil {
					t.Errorf("Save: %v", err)
					return
				}
				if _, err := Load(dir, "admin"); err != nil {
					t.Errorf("Load: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
