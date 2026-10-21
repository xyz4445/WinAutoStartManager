package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	want := &Config{
		Version: 1,
		Items: []Item{{
			ID:           "abc12345",
			Name:         "AutoHotkey",
			Path:         `C:\Program Files\AutoHotkey\v2\AutoHotkey64.exe`,
			Args:         `"C:\Scripts\main.ahk"`,
			WorkDir:      `C:\Scripts`,
			Elevated:     true,
			DelaySeconds: 3,
			Enabled:      true,
		}},
	}
	if err := want.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Version != 1 {
		t.Fatalf("version = %d, want 1", cfg.Version)
	}
	if len(cfg.Items) != 0 {
		t.Fatalf("items = %v, want empty", cfg.Items)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("AUTOSTART_TEST_DIR", `C:\Tools`)
	got := ExpandPath(`%AUTOSTART_TEST_DIR%\rclone\rclone.exe`)
	want := `C:\Tools\rclone\rclone.exe`
	if got != want {
		t.Fatalf("ExpandPath = %q, want %q", got, want)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		item Item
		want string
	}{
		{"empty name", Item{ID: "a", Name: "  ", Path: `C:\x.exe`}, "名称不能为空"},
		{"empty path", Item{ID: "b", Name: "x", Path: ""}, "路径不能为空"},
		{"negative delay", Item{ID: "c", Name: "x", Path: `C:\x.exe`, DelaySeconds: -1}, "延迟"},
		{"delay too large", Item{ID: "d", Name: "x", Path: `C:\x.exe`, DelaySeconds: 301}, "延迟"},
		{"empty id", Item{ID: "", Name: "x", Path: `C:\x.exe`}, "ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Version: 1, Items: []Item{tt.item}}
			errs := cfg.Validate()
			if len(errs) != 1 {
				t.Fatalf("errors = %v, want exactly 1", errs)
			}
			if !strings.Contains(errs[0], tt.want) {
				t.Fatalf("error %q does not contain %q", errs[0], tt.want)
			}
		})
	}
}

func TestValidateRejectsDuplicateIDs(t *testing.T) {
	cfg := &Config{Version: 1, Items: []Item{
		{ID: "same", Name: "a", Path: `C:\a.exe`},
		{ID: "same", Name: "b", Path: `C:\b.exe`},
	}}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e, "重复") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate id error, got %v", errs)
	}
}

func TestNewItem(t *testing.T) {
	it := NewItem("rclone", `C:\Tools\rclone.exe`)
	if len(it.ID) != 16 {
		t.Fatalf("ID = %q, want 16 hex chars", it.ID)
	}
	if !it.Enabled {
		t.Fatal("new item should be enabled")
	}
	if it.Name != "rclone" || it.Path != `C:\Tools\rclone.exe` {
		t.Fatalf("unexpected item: %+v", it)
	}
}

func TestNewItemProducesUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := NewItem("x", "y").ID
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := &Config{Version: 1, Items: []Item{}}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("leftover files: %v", names)
	}
}

func TestSaveNilItemsAsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := &Config{Version: 1}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if string(probe.Items) != "[]" {
		t.Fatalf("items = %s, want []", probe.Items)
	}
}

func TestValidateForSaveAllowsDrafts(t *testing.T) {
	cfg := &Config{Items: []Item{
		{ID: "a"}, // completely blank draft
		{ID: "b", Name: "only name"},
		{ID: "c", Path: `C:\Tools\x.exe`},
	}}
	if errs := cfg.ValidateForSave(); len(errs) != 0 {
		t.Fatalf("draft save should be allowed, got %v", errs)
	}
	if errs := cfg.Validate(); len(errs) == 0 {
		t.Fatal("strict validation should still flag drafts")
	}
}

func TestValidateForSaveKeepsStructuralChecks(t *testing.T) {
	cfg := &Config{Items: []Item{
		{ID: "a", DelaySeconds: -1},
		{ID: "a"},
		{ID: ""},
		{ID: "d", DelaySeconds: 301},
	}}
	errs := cfg.ValidateForSave()
	if len(errs) != 4 {
		t.Fatalf("errors = %v, want 4", errs)
	}
}
