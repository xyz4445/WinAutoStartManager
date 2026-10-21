// Command autostart-manager is a GUI startup-list manager with two
// scheduled tasks (admin / normal) that launch programs at sign-in.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"WinAutoStartManager/internal/apply"
	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/gui"
	"WinAutoStartManager/internal/launcher"
	"WinAutoStartManager/internal/logging"
	"WinAutoStartManager/internal/notify"
	"WinAutoStartManager/internal/schedule"
	"WinAutoStartManager/internal/settings"
	"WinAutoStartManager/internal/state"
)

func main() {
	launch := flag.Bool("launch", false, "内部参数：登录时启动指定权限层")
	tier := flag.String("tier", "", "内部参数：admin 或 user")
	applyTasks := flag.String("apply-tasks", "", "内部参数：写入计划任务并把结果写到此文件")
	removeTasks := flag.Bool("remove-tasks", false, "内部参数：删除两个计划任务")
	flag.Parse()

	switch {
	case *applyTasks != "":
		runApply(*applyTasks, *removeTasks)
	case *launch:
		runLaunch(*tier)
	default:
		if err := gui.Run(); err != nil {
			_ = notify.Show("开机启动管理器", err.Error())
			os.Exit(1)
		}
	}
}

// runApply handles the elevated helper mode.
func runApply(resultPath string, remove bool) {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	tmpDir, err := config.Dir()
	if err != nil || tmpDir == "" {
		tmpDir = os.TempDir()
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		_ = apply.WriteResult(resultPath, fmt.Errorf("创建数据目录失败: %w", err))
		os.Exit(1)
	}
	code := apply.ApplyAndReport(resultPath, exe, func() error {
		if remove {
			return schedule.Remove(schedule.ExecRunner)
		}
		return schedule.Apply(exe, tmpDir, schedule.ExecRunner)
	})
	os.Exit(code)
}

// runLaunch handles one scheduled task run.
func runLaunch(tierName string) {
	tier := schedule.Tier(tierName)
	if !tier.Valid() {
		fmt.Fprintln(os.Stderr, "无效的 --tier，必须是 admin 或 user")
		os.Exit(2)
	}

	startedAt := time.Now()
	dataDir, err := config.Dir()
	if err != nil {
		os.Exit(1)
	}

	logger, err := logging.Open(dataDir, tierName, startedAt)
	if err != nil {
		_ = notify.Show("开机启动管理器", "无法写入日志："+err.Error())
		os.Exit(1)
	}
	defer logger.Close()
	logger.Printf("=== 启动开始 tier=%s ===", tierName)

	retention := settings.DefaultLogRetentionDays
	if st, err := settings.Load(dataDir); err == nil {
		retention = st.LogRetentionDays
	} else {
		logger.Printf("读取设置失败，使用默认日志保留天数: %v", err)
	}

	exe, err := os.Executable()
	if err != nil {
		logger.Printf("无法确定自身路径: %v", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Printf("读取配置失败: %v", err)
		_ = notify.Show("开机启动管理器", "读取配置失败："+err.Error())
		os.Exit(1)
	}
	if errs := cfg.ValidateForSave(); len(errs) > 0 {
		msg := strings.Join(errs, "；")
		logger.Printf("配置校验失败: %s", msg)
		_ = notify.Show("开机启动管理器", "配置有误："+msg)
		os.Exit(1)
	}
	drafts := 0
	for _, it := range cfg.Items {
		if it.Enabled && !it.Runnable() {
			drafts++
		}
	}
	if drafts > 0 {
		logger.Printf("跳过 %d 个未填写路径的草稿条目", drafts)
	}

	other := schedule.TierUser
	if tier == schedule.TierUser {
		other = schedule.TierAdmin
	}
	otherStatus, _ := schedule.Check(exe, other, schedule.ExecRunner)
	logger.Printf("另一权限层任务状态: %s", otherStatus)

	results := launcher.RunTier(tier, cfg, otherStatus, launcher.Deps{
		Start: launcher.StartProcess,
		Wait:  time.Sleep,
		Now:   time.Now,
		Logf:  logger.Printf,
	})

	run := state.Run{
		Tier:       tierName,
		StartedAt:  startedAt,
		FinishedAt: time.Now(),
		Results:    results,
	}
	if err := state.Save(dataDir, tierName, run); err != nil {
		logger.Printf("保存运行状态失败: %v", err)
	}

	failures := launcher.Failures(results)
	if len(failures) > 0 {
		logger.Printf("=== 启动结束：%d/%d 失败 ===", len(failures), len(results))
		msg := fmt.Sprintf("%d 个程序启动失败：%s\n详见日志：%s",
			len(failures), strings.Join(failures, "、"),
			logging.PathFor(dataDir, tierName, startedAt))
		_ = notify.Show("开机启动管理器", msg)
	} else {
		logger.Printf("=== 启动结束：全部成功（%d 项） ===", len(results))
	}

	if err := logging.Prune(dataDir, startedAt, retention); err != nil {
		logger.Printf("清理旧日志失败: %v", err)
	}
}
