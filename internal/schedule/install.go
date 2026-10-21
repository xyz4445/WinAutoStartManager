package schedule

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// Runner executes an external command and returns its standard output.
type Runner func(name string, args ...string) ([]byte, error)

// ExecRunner runs the command through the OS. Standard output and standard
// error are captured separately so that a diagnostic line on stderr cannot
// corrupt the XML written to stdout.
var ExecRunner Runner = func(name string, args ...string) ([]byte, error) {
	cmd := newCommand(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		combined := append(append([]byte{}, stdout.Bytes()...), stderr.Bytes()...)
		if msg := strings.TrimSpace(decodeOutput(combined)); msg != "" {
			return combined, fmt.Errorf("%s 执行失败: %w: %s", name, err, msg)
		}
		return combined, fmt.Errorf("%s 执行失败: %w", name, err)
	}
	return stdout.Bytes(), nil
}

// Status describes the installed state of a scheduled task.
type Status string

const (
	StatusMissing Status = "missing"
	StatusStale   Status = "stale"
	StatusOK      Status = "ok"
	// StatusUnknown means the state could not be determined. Callers must not
	// treat it as "not installed".
	StatusUnknown Status = "unknown"
)

// decodeOutput converts schtasks output to a string. schtasks writes UTF-16LE
// on some systems, UTF-8 on others, and the system ANSI code page (for
// example GBK on Chinese Windows) when its output is redirected to a pipe.
func decodeOutput(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		return decodeUTF16LE(b[2:])
	}
	if looksUTF16LE(b) {
		return decodeUTF16LE(b)
	}
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(b) {
		return string(b)
	}
	if s, ok := decodeANSI(b); ok {
		return s
	}
	return string(b)
}

func decodeUTF16LE(b []byte) string {
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(units))
}

// looksUTF16LE recognizes UTF-16LE text that lacks a byte order mark by the
// zero high bytes of its mostly-ASCII XML markup.
func looksUTF16LE(b []byte) bool {
	if len(b) < 4 || len(b)%2 != 0 {
		return false
	}
	zeros := 0
	for i := 1; i < len(b); i += 2 {
		if b[i] == 0 {
			zeros++
		}
	}
	return zeros*4 >= len(b)
}

// parseQueryXML decodes an already-decoded XML string whose declaration may
// still say UTF-16.
func parseQueryXML(s string, v any) error {
	dec := xml.NewDecoder(strings.NewReader(s))
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	return dec.Decode(v)
}

// checkRetryDelay is the pause before a failed status query is retried. Tests
// set it to zero.
var checkRetryDelay = 150 * time.Millisecond

// Check reports whether the task exists and points at exePath. A query or
// parse failure yields StatusUnknown, never StatusMissing.
func Check(exePath string, t Tier, run Runner) (Status, error) {
	st, err := checkOnce(exePath, t, run)
	if st != StatusUnknown {
		return st, err
	}
	if checkRetryDelay > 0 {
		time.Sleep(checkRetryDelay)
	}
	return checkOnce(exePath, t, run)
}

func checkOnce(exePath string, t Tier, run Runner) (Status, error) {
	name := TaskName(t)
	out, err := run("schtasks", "/Query", "/TN", name, "/XML")
	if err != nil {
		exists, listErr := taskListed(t, run)
		switch {
		case listErr != nil:
			return StatusUnknown, fmt.Errorf("查询任务 %s 失败: %w；核对任务列表也失败: %v", name, err, listErr)
		case !exists:
			return StatusMissing, nil
		default:
			return StatusUnknown, fmt.Errorf("任务 %s 存在但无法读取: %w", name, err)
		}
	}

	var task struct {
		Actions struct {
			Exec struct {
				Command string `xml:"Command"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	if err := parseQueryXML(decodeOutput(out), &task); err != nil {
		return StatusUnknown, fmt.Errorf("解析任务 %s 的定义失败: %w", name, err)
	}
	cmd := strings.TrimSpace(task.Actions.Exec.Command)
	if cmd == "" {
		return StatusUnknown, fmt.Errorf("任务 %s 的定义缺少启动命令", name)
	}
	if strings.EqualFold(filepath.Clean(cmd), filepath.Clean(exePath)) {
		return StatusOK, nil
	}
	return StatusStale, nil
}

// taskListed reports whether the task appears in schtasks' task listing. It is
// used only when reading the task XML fails, to tell "missing" apart from
// "exists but unreadable". Task names are ASCII, so this check survives the
// localized output encoding.
func taskListed(t Tier, run Runner) (bool, error) {
	out, err := run("schtasks", "/Query", "/FO", "CSV", "/NH")
	if err != nil {
		return false, err
	}
	return strings.Contains(decodeOutput(out), `"`+TaskName(t)+`"`), nil
}

// Remove deletes both scheduled tasks. Missing tasks are ignored, so turning
// auto-start off is idempotent.
func Remove(run Runner) error {
	var errs []string
	for _, t := range []Tier{TierAdmin, TierUser} {
		if _, err := run("schtasks", "/Query", "/TN", TaskName(t), "/XML"); err != nil {
			continue
		}
		out, err := run("schtasks", "/Delete", "/TN", TaskName(t), "/F")
		if err == nil {
			continue
		}
		// The task may have disappeared between query and delete.
		if _, qerr := run("schtasks", "/Query", "/TN", TaskName(t), "/XML"); qerr != nil {
			continue
		}
		msg := strings.TrimSpace(decodeOutput(out))
		if msg == "" {
			errs = append(errs, fmt.Sprintf("删除任务 %s 失败: %v", TaskName(t), err))
		} else {
			errs = append(errs, fmt.Sprintf("删除任务 %s 失败: %v: %s", TaskName(t), err, msg))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "；"))
	}
	return nil
}

// Apply writes both scheduled tasks. It requires an elevated process.
func Apply(exePath, tmpDir string, run Runner) error {
	for _, t := range []Tier{TierAdmin, TierUser} {
		body, err := XML(exePath, t)
		if err != nil {
			return err
		}
		f, err := os.CreateTemp(tmpDir, "autostart-task-*.xml")
		if err != nil {
			return fmt.Errorf("创建任务临时文件失败: %w", err)
		}
		path := f.Name()
		if _, err := f.Write(UTF16LE(body)); err != nil {
			f.Close()
			os.Remove(path)
			return fmt.Errorf("写入任务临时文件失败: %w", err)
		}
		if err := f.Close(); err != nil {
			os.Remove(path)
			return fmt.Errorf("写入任务临时文件失败: %w", err)
		}

		out, err := run("schtasks", "/Create", "/TN", TaskName(t), "/XML", path, "/F")
		os.Remove(path)
		if err != nil {
			if msg := strings.TrimSpace(decodeOutput(out)); msg != "" {
				return fmt.Errorf("注册任务 %s 失败: %w: %s", TaskName(t), err, msg)
			}
			return fmt.Errorf("注册任务 %s 失败: %w", TaskName(t), err)
		}
	}
	return nil
}
