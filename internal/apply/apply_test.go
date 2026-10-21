package apply

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadResultSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	if err := WriteResult(path, nil); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}
	got, err := ReadResult(path)
	if err != nil {
		t.Fatalf("ReadResult: %v", err)
	}
	if !got.OK || got.Error != "" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestWriteReadResultFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	if err := WriteResult(path, errors.New("访问被拒绝")); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}
	got, err := ReadResult(path)
	if err != nil {
		t.Fatalf("ReadResult: %v", err)
	}
	if got.OK {
		t.Fatal("expected failure result")
	}
	if !strings.Contains(got.Error, "访问被拒绝") {
		t.Fatalf("error = %q", got.Error)
	}
}

func TestApplyAndReportSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	called := false
	code := ApplyAndReport(path, `C:\x.exe`, func() error {
		called = true
		return nil
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !called {
		t.Fatal("apply func not called")
	}
	res, err := ReadResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
}

func TestApplyAndReportFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	code := ApplyAndReport(path, `C:\x.exe`, func() error {
		return errors.New("schtasks failed")
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	res, err := ReadResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Error, "schtasks failed") {
		t.Fatalf("result = %+v", res)
	}
}

func TestReadResultMissing(t *testing.T) {
	if _, err := ReadResult(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing result")
	}
}

func TestWriteResultCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "result.json")
	if err := WriteResult(path, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
