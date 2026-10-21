package schedule

import (
	"encoding/binary"
	"encoding/xml"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"unicode/utf16"
)

// Tier is the privilege layer a launch item belongs to.
type Tier string

const (
	TierAdmin Tier = "admin"
	TierUser  Tier = "user"

	AdminTaskName = "AutoStartManager-Admin"
	UserTaskName  = "AutoStartManager-User"
)

// Valid reports whether t is a known tier.
func (t Tier) Valid() bool {
	return t == TierAdmin || t == TierUser
}

// TaskName returns the scheduled task name for a tier.
func TaskName(t Tier) string {
	if t == TierAdmin {
		return AdminTaskName
	}
	if t == TierUser {
		return UserTaskName
	}
	return ""
}

type taskDefinition struct {
	XMLName xml.Name `xml:"Task"`
	Version string   `xml:"version,attr"`
	Xmlns   string   `xml:"xmlns,attr"`

	RegistrationInfo struct {
		Author      string `xml:"Author"`
		Description string `xml:"Description"`
	} `xml:"RegistrationInfo"`

	Triggers struct {
		LogonTrigger struct {
			Enabled bool   `xml:"Enabled"`
			UserID  string `xml:"UserId"`
			Delay   string `xml:"Delay"`
		} `xml:"LogonTrigger"`
	} `xml:"Triggers"`

	Principals struct {
		Principal struct {
			ID        string `xml:"id,attr"`
			UserID    string `xml:"UserId"`
			LogonType string `xml:"LogonType"`
			RunLevel  string `xml:"RunLevel"`
		} `xml:"Principal"`
	} `xml:"Principals"`

	Settings struct {
		MultipleInstancesPolicy    string `xml:"MultipleInstancesPolicy"`
		DisallowStartIfOnBatteries bool   `xml:"DisallowStartIfOnBatteries"`
		StopIfGoingOnBatteries     bool   `xml:"StopIfGoingOnBatteries"`
		AllowHardTerminate         bool   `xml:"AllowHardTerminate"`
		StartWhenAvailable         bool   `xml:"StartWhenAvailable"`
		RunOnlyIfNetworkAvailable  bool   `xml:"RunOnlyIfNetworkAvailable"`
		AllowStartOnDemand         bool   `xml:"AllowStartOnDemand"`
		Enabled                    bool   `xml:"Enabled"`
		Hidden                     bool   `xml:"Hidden"`
		RunOnlyIfIdle              bool   `xml:"RunOnlyIfIdle"`
		WakeToRun                  bool   `xml:"WakeToRun"`
		ExecutionTimeLimit         string `xml:"ExecutionTimeLimit"`
		Priority                   int    `xml:"Priority"`
		IdleSettings               struct {
			StopOnIdleEnd bool `xml:"StopOnIdleEnd"`
			RestartOnIdle bool `xml:"RestartOnIdle"`
		} `xml:"IdleSettings"`
	} `xml:"Settings"`

	Actions struct {
		Context string `xml:"Context,attr"`
		Exec    struct {
			Command          string `xml:"Command"`
			Arguments        string `xml:"Arguments"`
			WorkingDirectory string `xml:"WorkingDirectory"`
		} `xml:"Exec"`
	} `xml:"Actions"`
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	domain := os.Getenv("USERDOMAIN")
	name := os.Getenv("USERNAME")
	if domain != "" {
		return domain + `\` + name
	}
	return name
}

// XML renders the scheduled task definition for a tier.
func XML(exePath string, t Tier) (string, error) {
	if !t.Valid() {
		return "", errors.New("无效的权限层")
	}
	if exePath == "" {
		return "", errors.New("exe 路径不能为空")
	}

	runLevel := "LeastPrivilege"
	args := "--launch --tier=user"
	if t == TierAdmin {
		runLevel = "HighestAvailable"
		args = "--launch --tier=admin"
	}

	var task taskDefinition
	task.Version = "1.2"
	task.Xmlns = "http://schemas.microsoft.com/windows/2004/02/mit/task"
	task.RegistrationInfo.Author = "AutoStartManager"
	task.RegistrationInfo.Description = "开机启动管理器：" + string(t) + " 层"

	task.Triggers.LogonTrigger.Enabled = true
	task.Triggers.LogonTrigger.UserID = currentUser()
	task.Triggers.LogonTrigger.Delay = "PT2S"

	task.Principals.Principal.ID = "Author"
	task.Principals.Principal.UserID = currentUser()
	task.Principals.Principal.LogonType = "InteractiveToken"
	task.Principals.Principal.RunLevel = runLevel

	task.Settings.MultipleInstancesPolicy = "IgnoreNew"
	task.Settings.DisallowStartIfOnBatteries = false
	task.Settings.StopIfGoingOnBatteries = false
	task.Settings.AllowHardTerminate = true
	task.Settings.StartWhenAvailable = false
	task.Settings.RunOnlyIfNetworkAvailable = false
	task.Settings.AllowStartOnDemand = true
	task.Settings.Enabled = true
	task.Settings.Hidden = true
	task.Settings.RunOnlyIfIdle = false
	task.Settings.WakeToRun = false
	task.Settings.ExecutionTimeLimit = "PT5M"
	task.Settings.Priority = 7
	task.Settings.IdleSettings.StopOnIdleEnd = false
	task.Settings.IdleSettings.RestartOnIdle = false

	task.Actions.Context = "Author"
	task.Actions.Exec.Command = exePath
	task.Actions.Exec.Arguments = args
	task.Actions.Exec.WorkingDirectory = filepath.Dir(exePath)

	body, err := xml.MarshalIndent(task, "", "  ")
	if err != nil {
		return "", err
	}
	return `<?xml version="1.0" encoding="UTF-16"?>` + "\n" + string(body), nil
}

// UTF16LE encodes s as UTF-16 little endian with a BOM.
func UTF16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 2+len(units)*2)
	out[0] = 0xFF
	out[1] = 0xFE
	for i, u := range units {
		binary.LittleEndian.PutUint16(out[2+i*2:], u)
	}
	return out
}
