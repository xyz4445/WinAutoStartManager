package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingUsesDefaults(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.LogRetentionDays != DefaultLogRetentionDays {
		t.Fatalf("days = %d, want %d", s.LogRetentionDays, DefaultLogRetentionDays)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := &Settings{LogRetentionDays: 30}
	if err := Save(root, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogRetentionDays != 30 {
		t.Fatalf("days = %d, want 30", got.LogRetentionDays)
	}
}

func TestNormalizeClamps(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, DefaultLogRetentionDays},
		{-5, MinLogRetentionDays},
		{1, 1},
		{365, 365},
		{9999, MaxLogRetentionDays},
	}
	for _, tt := range tests {
		s := &Settings{LogRetentionDays: tt.in}
		s.Normalize()
		if s.LogRetentionDays != tt.want {
			t.Fatalf("Normalize(%d) = %d, want %d", tt.in, s.LogRetentionDays, tt.want)
		}
	}
}

func TestLoadCorruptReturnsError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("expected error for corrupt settings")
	}
}
