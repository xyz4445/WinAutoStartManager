package appdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestDefaultRootUsesLocalAppData(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)

	got, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}
	want := filepath.Join(base, appDirName)
	if got != want {
		t.Fatalf("DefaultRoot = %q, want %q", got, want)
	}
}

func TestValidateDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty", "  ", true},
		{"relative", `AutoStartManager`, true},
		{"file", file, true},
		{"drive root", `C:\`, true},
		{"existing dir", dir, false},
		{"new absolute dir", filepath.Join(dir, "new"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDir(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateDir(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateDir(%q): %v", tt.in, err)
			}
			if !filepath.IsAbs(got) {
				t.Fatalf("ValidateDir(%q) = %q, want absolute", tt.in, got)
			}
		})
	}
}

func TestOverrideRoundTrip(t *testing.T) {
	oldKey := keyPath
	keyPath = `Software\AutoStartManager-test-` + strings.ReplaceAll(t.Name(), "/", "-")
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, keyPath)
		keyPath = oldKey
	})

	dir := t.TempDir()
	if err := SetOverride(dir); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}
	got, err := Override()
	if err != nil {
		t.Fatalf("Override: %v", err)
	}
	if got != dir {
		t.Fatalf("Override = %q, want %q", got, dir)
	}

	if err := ClearOverride(); err != nil {
		t.Fatalf("ClearOverride: %v", err)
	}
	got, err = Override()
	if err != nil {
		t.Fatalf("Override after clear: %v", err)
	}
	if got != "" {
		t.Fatalf("Override after clear = %q, want empty", got)
	}
}

func TestMigrateCopiesManagedFiles(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := filepath.Join(t.TempDir(), "data")

	write := func(name, body string) {
		p := filepath.Join(oldRoot, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.json", `{"version":1,"items":[]}`)
	write("settings.json", `{"logRetentionDays":30}`)
	write("state-admin.json", `{"tier":"admin"}`)
	write(filepath.Join("logs", "launcher-admin-20261021.log"), "log line")

	if err := Migrate(oldRoot, newRoot, false); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, name := range []string{
		"config.json",
		"settings.json",
		"state-admin.json",
		filepath.Join("logs", "launcher-admin-20261021.log"),
	} {
		got, err := os.ReadFile(filepath.Join(newRoot, name))
		if err != nil {
			t.Fatalf("read migrated %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(oldRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}

	// Migration copies; it must not destroy the original data.
	if _, err := os.Stat(filepath.Join(oldRoot, "config.json")); err != nil {
		t.Fatalf("old config removed: %v", err)
	}
}

func TestMigrateRejectsExistingConfig(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(oldRoot, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(oldRoot, newRoot, false); err == nil {
		t.Fatal("expected error when target already has config.json")
	}
}

func TestMigrateRejectsNestedTarget(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := filepath.Join(oldRoot, "nested")
	if err := Migrate(oldRoot, newRoot, false); err == nil {
		t.Fatal("expected error for target inside source")
	}
}

func TestMigrateOverwriteReplacesExistingConfig(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(oldRoot, "config.json"), []byte(`{"version":1,"items":[{"id":"new"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "config.json"), []byte(`{"version":1,"items":[{"id":"stale"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(oldRoot, newRoot, true); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(newRoot, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"new"`) {
		t.Fatalf("config = %s, want overwritten content", raw)
	}
}
