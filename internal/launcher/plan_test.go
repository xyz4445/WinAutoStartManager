package launcher

import (
	"testing"
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/schedule"
)

func cfgWith(items ...config.Item) *config.Config {
	return &config.Config{Version: 1, Items: items}
}

func TestTierOf(t *testing.T) {
	if TierOf(config.Item{Elevated: true}) != schedule.TierAdmin {
		t.Fatal("elevated item should be admin tier")
	}
	if TierOf(config.Item{Elevated: false}) != schedule.TierUser {
		t.Fatal("normal item should be user tier")
	}
}

func TestBuildPlanFiltersAndOrders(t *testing.T) {
	cfg := cfgWith(
		config.Item{ID: "1", Name: "AHK", Path: `C:\a.exe`, Elevated: true, Enabled: true, DelaySeconds: 3},
		config.Item{ID: "2", Name: "rclone", Path: `C:\b.exe`, Elevated: false, Enabled: true, DelaySeconds: 1},
		config.Item{ID: "3", Name: "disabled", Path: `C:\c.exe`, Elevated: true, Enabled: false},
		config.Item{ID: "4", Name: "second-admin", Path: `C:\d.exe`, Elevated: true, Enabled: true, DelaySeconds: 5},
	)

	admin := BuildPlan(cfg, schedule.TierAdmin)
	if len(admin) != 2 {
		t.Fatalf("admin plan len = %d, want 2", len(admin))
	}
	if admin[0].Item.Name != "AHK" || admin[1].Item.Name != "second-admin" {
		t.Fatalf("order = %q, %q", admin[0].Item.Name, admin[1].Item.Name)
	}
	if admin[0].At != 3*time.Second || admin[1].At != 5*time.Second {
		t.Fatalf("offsets = %v, %v", admin[0].At, admin[1].At)
	}

	user := BuildPlan(cfg, schedule.TierUser)
	if len(user) != 1 || user[0].Item.Name != "rclone" {
		t.Fatalf("user plan = %+v", user)
	}
}

func TestBuildPlanEmpty(t *testing.T) {
	if got := BuildPlan(cfgWith(), schedule.TierAdmin); len(got) != 0 {
		t.Fatalf("expected empty plan, got %+v", got)
	}
}

func TestBuildCommandLineQuotesPath(t *testing.T) {
	got := BuildCommandLine(`C:\Program Files\A&B\x.exe`, "--flag")
	want := `"C:\Program Files\A&B\x.exe" --flag`
	if got != want {
		t.Fatalf("command line = %q, want %q", got, want)
	}
}

func TestBuildCommandLineKeepsChineseAndSpaces(t *testing.T) {
	got := BuildCommandLine(`C:\工具\rclone.exe`, `mount "G:" remote: --vfs-cache-mode full`)
	want := `"C:\工具\rclone.exe" mount "G:" remote: --vfs-cache-mode full`
	if got != want {
		t.Fatalf("command line = %q, want %q", got, want)
	}
}

func TestBuildCommandLineWithoutArgs(t *testing.T) {
	got := BuildCommandLine(`C:\x.exe`, "   ")
	if got != `"C:\x.exe"` {
		t.Fatalf("command line = %q", got)
	}
}

func TestBuildPlanSkipsDrafts(t *testing.T) {
	cfg := cfgWith(
		config.Item{ID: "1", Name: "ok", Path: `C:\a.exe`, Enabled: true},
		config.Item{ID: "2", Name: "draft", Path: "", Enabled: true},
		config.Item{ID: "3", Name: "", Path: `C:\b.exe`, Enabled: true},
		config.Item{ID: "4", Name: "disabled", Path: `C:\c.exe`, Enabled: false},
	)
	plan := BuildPlan(cfg, schedule.TierUser)
	if len(plan) != 2 {
		t.Fatalf("plan = %+v, want 2 runnable items", plan)
	}
	if plan[0].Item.ID != "1" || plan[1].Item.ID != "3" {
		t.Fatalf("plan ids = %q, %q", plan[0].Item.ID, plan[1].Item.ID)
	}
}
