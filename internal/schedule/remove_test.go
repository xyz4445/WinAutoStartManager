package schedule

import (
	"errors"
	"strings"
	"testing"
)

const taskXML = `<?xml version="1.0" encoding="UTF-16"?>
<Task><Actions><Exec><Command>C:\Tools\autostart-manager.exe</Command></Exec></Actions></Task>`

func TestRemoveDeletesBothTasks(t *testing.T) {
	var deletes []string
	run := func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "/Query" {
			return []byte(taskXML), nil
		}
		if len(args) > 0 && args[0] == "/Delete" {
			deletes = append(deletes, strings.Join(args, " "))
			return nil, nil
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil
	}

	if err := Remove(run); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(deletes) != 2 {
		t.Fatalf("deletes = %v, want 2", deletes)
	}
	if !strings.Contains(deletes[0], AdminTaskName) || !strings.Contains(deletes[1], UserTaskName) {
		t.Fatalf("unexpected deletes: %v", deletes)
	}
}

func TestRemoveIgnoresMissingTasks(t *testing.T) {
	calls := 0
	run := func(name string, args ...string) ([]byte, error) {
		calls++
		if len(args) > 0 && args[0] == "/Query" {
			return nil, errors.New("not found")
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil
	}

	if err := Remove(run); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 queries only", calls)
	}
}

func TestRemoveReportsDeleteFailure(t *testing.T) {
	run := func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "/Query" {
			return []byte(taskXML), nil
		}
		return nil, errors.New("access denied")
	}
	if err := Remove(run); err == nil {
		t.Fatal("expected delete failure")
	}
}
