package schedule

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func queryXML(command string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>开机启动管理器：admin 层</Description>
  </RegistrationInfo>
  <Actions Context="Author">
    <Exec>
      <Command>` + command + `</Command>
      <Arguments>--launch --tier=admin</Arguments>
    </Exec>
  </Actions>
</Task>`
}

func fakeQueryXML(command string) []byte {
	return UTF16LE(queryXML(command))
}

// queryXMLWithRawDescription builds a query response whose Description bytes
// are supplied by the caller. It is used to model schtasks writing its XML in
// the system ANSI code page instead of UTF-8.
func queryXMLWithRawDescription(desc []byte) []byte {
	head := `<?xml version="1.0" encoding="UTF-16"?>` + "\r\r\n" +
		`<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">` +
		`<RegistrationInfo><Description>`
	tail := `</Description></RegistrationInfo><Actions Context="Author"><Exec>` +
		`<Command>C:\Tools\App.exe</Command>` +
		`<Arguments>--launch --tier=admin</Arguments>` +
		`</Exec></Actions></Task>`
	return append(append([]byte(head), desc...), []byte(tail)...)
}

func listedTasks(names ...string) []byte {
	var b strings.Builder
	b.WriteString("\"TaskName\",\"Next Run Time\",\"Status\"\r\n")
	for _, n := range names {
		b.WriteString("\"" + n + "\",\"N/A\",\"Ready\"\r\n")
	}
	return []byte(b.String())
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestCheckOK(t *testing.T) {
	run := func(name string, args ...string) ([]byte, error) {
		if name != "schtasks" || len(args) != 4 || args[0] != "/Query" || args[1] != "/TN" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		if args[2] != AdminTaskName {
			t.Fatalf("task name = %q", args[2])
		}
		return fakeQueryXML(`C:\Tools\App.exe`), nil
	}
	got, err := Check(`c:\tools\app.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok", got)
	}
}

func TestCheckStale(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return fakeQueryXML(`D:\Old\app.exe`), nil
	}
	got, err := Check(`C:\Tools\app.exe`, TierAdmin, run)
	if err != nil {
		t.Fatal(err)
	}
	if got != StatusStale {
		t.Fatalf("status = %q, want stale", got)
	}
}

func TestCheckMissing(t *testing.T) {
	run := func(_ string, args ...string) ([]byte, error) {
		if hasArg(args, "/XML") {
			return nil, errors.New("ERROR: The system cannot find the file specified.")
		}
		return listedTasks(), nil
	}
	got, err := Check(`C:\Tools\app.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("missing task should not be an error: %v", err)
	}
	if got != StatusMissing {
		t.Fatalf("status = %q, want missing", got)
	}
}

func TestCheckHandlesGBKOutput(t *testing.T) {
	// "开机启动管理器：admin 层" encoded as GBK, which is what schtasks emits
	// on a Chinese Windows when its output is redirected to a pipe.
	gbk := []byte{
		0xBF, 0xAA, 0xBB, 0xFA, 0xC6, 0xF4, 0xB6, 0xAF, 0xB9, 0xDC, 0xC0, 0xED,
		0xC6, 0xF7, 0xA3, 0xBA, 0x61, 0x64, 0x6D, 0x69, 0x6E, 0x20, 0xB2, 0xE3,
	}
	run := func(string, ...string) ([]byte, error) {
		return queryXMLWithRawDescription(gbk), nil
	}
	got, err := Check(`c:\tools\app.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok (ANSI/GBK output must decode)", got)
	}
}

func TestCheckHandlesUTF8WithoutBOM(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return []byte(queryXML(`C:\Tools\App.exe`)), nil
	}
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok", got)
	}
}

func TestCheckHandlesUTF8WithBOM(t *testing.T) {
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte(queryXML(`C:\Tools\App.exe`))...)
	run := func(string, ...string) ([]byte, error) { return body, nil }
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok", got)
	}
}

func TestCheckHandlesUTF16WithoutBOM(t *testing.T) {
	body := UTF16LE(queryXML(`C:\Tools\App.exe`))[2:]
	run := func(string, ...string) ([]byte, error) { return body, nil }
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok", got)
	}
}

func TestCheckQueryFailureIsUnknownWhenTaskExists(t *testing.T) {
	run := func(_ string, args ...string) ([]byte, error) {
		if hasArg(args, "/XML") {
			return nil, errors.New("exit status 1")
		}
		return listedTasks(AdminTaskName), nil
	}
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if got != StatusUnknown {
		t.Fatalf("status = %q, want unknown", got)
	}
	if err == nil {
		t.Fatal("query failure should be reported as an error")
	}
}

func TestCheckParseFailureIsUnknown(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return []byte("<Task><Actions>"), nil
	}
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if got != StatusUnknown {
		t.Fatalf("status = %q, want unknown", got)
	}
	if err == nil {
		t.Fatal("parse failure should be reported as an error")
	}
}

func TestCheckEmptyCommandIsUnknown(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return []byte(queryXML("")), nil
	}
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if got != StatusUnknown {
		t.Fatalf("status = %q, want unknown", got)
	}
	if err == nil {
		t.Fatal("empty command should be reported as an error")
	}
}

func TestCheckRetriesTransientFailure(t *testing.T) {
	old := checkRetryDelay
	checkRetryDelay = 0
	defer func() { checkRetryDelay = old }()

	xmlCalls := 0
	run := func(_ string, args ...string) ([]byte, error) {
		if hasArg(args, "/XML") {
			xmlCalls++
			if xmlCalls == 1 {
				return nil, errors.New("transient failure")
			}
			return fakeQueryXML(`C:\Tools\App.exe`), nil
		}
		return listedTasks(AdminTaskName), nil
	}
	got, err := Check(`C:\Tools\App.exe`, TierAdmin, run)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok after retry", got)
	}
}

func TestExecRunnerKeepsStderrOutOfSuccessfulStdout(t *testing.T) {
	out, err := ExecRunner("cmd", "/c", "echo out & echo err 1>&2")
	if err != nil {
		t.Fatalf("ExecRunner: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "out" {
		t.Fatalf("stdout = %q, want %q", got, "out")
	}
}

func TestCheckHandlesChinesePath(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return fakeQueryXML(`C:\Program Files\工具\rclone.exe`), nil
	}
	got, err := Check(`C:\Program Files\工具\rclone.exe`, TierUser, run)
	if err != nil {
		t.Fatal(err)
	}
	if got != StatusOK {
		t.Fatalf("status = %q, want ok (UTF-16 output must decode)", got)
	}
}

func TestApplyCreatesBothTasks(t *testing.T) {
	dir := t.TempDir()
	type call struct {
		task string
		xml  string
	}
	var calls []call

	run := func(name string, args ...string) ([]byte, error) {
		if name != "schtasks" {
			t.Fatalf("unexpected binary %q", name)
		}
		if len(args) != 6 || args[0] != "/Create" || args[1] != "/TN" || args[3] != "/XML" || args[5] != "/F" {
			t.Fatalf("unexpected args: %v", args)
		}
		raw, err := os.ReadFile(args[4])
		if err != nil {
			t.Fatalf("temp xml missing: %v", err)
		}
		if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xFE {
			t.Fatalf("task xml is not UTF-16LE with BOM")
		}
		calls = append(calls, call{task: args[2], xml: decodeOutput(raw)})
		return nil, nil
	}

	if err := Apply(`C:\Tools\app.exe`, dir, run); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 schtasks calls, got %d", len(calls))
	}
	if calls[0].task != AdminTaskName || calls[1].task != UserTaskName {
		t.Fatalf("task order/names = %q, %q", calls[0].task, calls[1].task)
	}

	joined := calls[0].xml + calls[1].xml
	if !strings.Contains(joined, "HighestAvailable") || !strings.Contains(joined, "LeastPrivilege") {
		t.Fatal("both run levels should be present")
	}
}

func TestApplyCleansTempFiles(t *testing.T) {
	dir := t.TempDir()
	run := func(string, ...string) ([]byte, error) { return nil, nil }
	if err := Apply(`C:\Tools\app.exe`, dir, run); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, filepath.Base(e.Name()))
		}
		t.Fatalf("leftover temp files: %v", names)
	}
}

func TestApplyPropagatesSchtasksError(t *testing.T) {
	dir := t.TempDir()
	run := func(string, ...string) ([]byte, error) {
		return UTF16LE("ERROR: Access is denied."), errors.New("exit status 1")
	}
	err := Apply(`C:\Tools\app.exe`, dir, run)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Access is denied") {
		t.Fatalf("error should include schtasks output: %v", err)
	}
}
