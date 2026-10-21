package launcher

import (
	"time"

	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/state"
)

// Deps are the side effects Execute needs; tests inject fakes.
type Deps struct {
	Start func(config.Item) (uint32, error)
	Wait  func(time.Duration)
	Now   func() time.Time
	Logf  func(format string, args ...any)
}

// Execute starts plan items at their absolute offsets and records results.
func Execute(plan []PlanItem, d Deps) []state.ItemResult {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Wait == nil {
		d.Wait = time.Sleep
	}
	if d.Logf == nil {
		d.Logf = func(string, ...any) {}
	}

	start := d.Now()
	results := make([]state.ItemResult, 0, len(plan))
	for _, pi := range plan {
		deadline := start.Add(pi.At)
		if now := d.Now(); now.Before(deadline) {
			d.Wait(deadline.Sub(now))
		}

		ts := d.Now()
		pid, err := d.Start(pi.Item)
		res := state.ItemResult{
			ID:        pi.Item.ID,
			Name:      pi.Item.Name,
			OK:        err == nil,
			PID:       pid,
			StartedAt: ts,
		}
		if err != nil {
			res.Error = err.Error()
			d.Logf("START fail name=%s error=%v", pi.Item.Name, err)
		} else {
			d.Logf("START ok pid=%d name=%s", pid, pi.Item.Name)
		}
		results = append(results, res)
	}
	return results
}

// Failures returns the names of failed results.
func Failures(results []state.ItemResult) []string {
	var names []string
	for _, r := range results {
		if !r.OK {
			names = append(names, r.Name)
		}
	}
	return names
}
