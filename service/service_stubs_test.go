//go:build !windows && !linux

package service

import (
	"testing"
)

func TestInstall_Stub(t *testing.T) {
	err := Install([]string{"test.local"}, "30s", "/fake/path")
	if err == nil {
		t.Error("expected error on non-Windows")
	}
}

func TestUninstall_Stub(t *testing.T) {
	err := Uninstall()
	if err == nil {
		t.Error("expected error on non-Windows")
	}
}

func TestRunService_Stub(t *testing.T) {
	err := RunService()
	if err == nil {
		t.Error("expected error on non-Windows")
	}
}
