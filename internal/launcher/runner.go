package launcher

import (
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/schedule"
	"WinAutoStartManager/internal/state"
)

// RunTier launches one tier and folds in the other tier's task health.
func RunTier(t schedule.Tier, cfg *config.Config, other schedule.Status, d Deps) []state.ItemResult {
	results := []state.ItemResult{}
	if other != schedule.StatusOK {
		now := time.Now()
		if d.Now != nil {
			now = d.Now()
		}
		results = append(results, state.ItemResult{
			ID:        "__other_tier__",
			Name:      "另一权限层的计划任务",
			OK:        false,
			Error:     "另一权限层的计划任务缺失或指向了旧路径，请打开管理器重新“应用自动启动”。",
			StartedAt: now,
		})
		if d.Logf != nil {
			d.Logf("CHECK other-tier status=%s", other)
		}
	}
	return append(results, Execute(BuildPlan(cfg, t), d)...)
}
