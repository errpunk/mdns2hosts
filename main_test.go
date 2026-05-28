package main

import (
	"errors"
	"testing"
)

func TestRunVersion(t *testing.T) {
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if code := run([]string{"-v"}); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunExecute(t *testing.T) {
	orig := executeCommand
	t.Cleanup(func() { executeCommand = orig })

	executeCommand = func() error { return nil }
	if code := run(nil); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	executeCommand = func() error { return errors.New("execute") }
	if code := run(nil); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}
