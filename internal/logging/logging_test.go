package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 10, 21, 8, 31, 2, 0, time.Local)

func TestLogLineFormat(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir, "admin", fixedNow)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Printf("START ok pid=%d name=%s", 1234, "AutoHotkey")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(PathFor(dir, "admin", fixedNow))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	want := "2026-10-21 08:31:02 [admin] START ok pid=1234 name=AutoHotkey\n"
	if string(raw) != want {
		t.Fatalf("log = %q, want %q", raw, want)
	}
}

func TestRotatesAtTwoMegabytes(t *testing.T) {
	dir := t.TempDir()
	path := PathFor(dir, "user", fixedNow)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte("x"), 2<<20)
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := Open(dir, "user", fixedNow)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Printf("hello")
	l.Close()

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated file missing: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "hello") {
		t.Fatalf("new log does not contain new line: %q", raw)
	}
	if strings.Contains(string(raw), "xxxx") {
		t.Fatal("new log still contains old content")
	}
}

func TestPruneKeepsRecentFiles(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(logs, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		ts := fixedNow.Add(-age)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldFile := mk("launcher-20261006-admin.log", 15*24*time.Hour)
	midFile := mk("launcher-20261008-admin.log", 13*24*time.Hour)
	newFile := mk("launcher-20261021-user.log", 0)

	if err := Prune(dir, fixedNow, 14); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatal("15-day-old file should be deleted")
	}
	if _, err := os.Stat(midFile); err != nil {
		t.Fatal("13-day-old file should be kept")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatal("today's file should be kept")
	}
}

func TestConcurrentWritesDontInterleave(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir, "admin", fixedNow)
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 20
	const lines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < lines; j++ {
				l.Printf("line-%02d-%02d", n, j)
			}
		}(i)
	}
	wg.Wait()
	l.Close()

	raw, err := os.ReadFile(PathFor(dir, "admin", fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Count(string(raw), "\n")
	if got != goroutines*lines {
		t.Fatalf("line count = %d, want %d", got, goroutines*lines)
	}
}

func TestOpenCreatesLogsDirectory(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir, "admin", fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	if _, err := os.Stat(filepath.Join(dir, "logs")); err != nil {
		t.Fatalf("logs dir missing: %v", err)
	}
}
