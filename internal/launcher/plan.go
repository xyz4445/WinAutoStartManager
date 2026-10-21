package launcher

import (
	"strings"
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/schedule"
)

// TierOf returns the privilege tier an item belongs to.
func TierOf(it config.Item) schedule.Tier {
	if it.Elevated {
		return schedule.TierAdmin
	}
	return schedule.TierUser
}

// PlanItem is one item plus its absolute offset from login.
type PlanItem struct {
	Item config.Item
	At   time.Duration
}

// BuildPlan selects enabled items for a tier, preserving order.
func BuildPlan(cfg *config.Config, t schedule.Tier) []PlanItem {
	var plan []PlanItem
	for _, it := range cfg.Items {
		if !it.Enabled || TierOf(it) != t || !it.Runnable() {
			continue
		}
		plan = append(plan, PlanItem{
			Item: it,
			At:   time.Duration(it.DelaySeconds) * time.Second,
		})
	}
	return plan
}

// BuildCommandLine quotes the executable path and appends raw arguments.
func BuildCommandLine(path, args string) string {
	line := `"` + path + `"`
	if strings.TrimSpace(args) != "" {
		line += " " + args
	}
	return line
}
