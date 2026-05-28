package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExePath(t *testing.T) {
	path, err := ExePath()
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Error("ExePath returned empty string")
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "mdns2hosts.conf") {
		t.Errorf("expected config path to contain mdns2hosts.conf, got %s", path)
	}
}

func TestSaveAndReadConfig(t *testing.T) {
	// Override config path to use temp dir
	origExe, _ := os.Executable()
	origConfigPath := configPathForTest

	dir := t.TempDir()
	fakeExe := filepath.Join(dir, "mdns2hosts.exe")
	if err := os.WriteFile(fakeExe, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	configPathForTest = filepath.Join(dir, "mdns2hosts.conf")

	defer func() {
		configPathForTest = origConfigPath
	}()

	_ = origExe // suppress unused warning

	names := []string{"foo.local", "bar.local"}
	err := SaveConfig(names, "45s")
	if err != nil {
		t.Fatal(err)
	}

	readNames, interval, err := ReadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if len(readNames) != 2 {
		t.Errorf("expected 2 names, got %d", len(readNames))
	}
	if readNames[0] != "foo.local" {
		t.Errorf("expected foo.local, got %s", readNames[0])
	}
	if readNames[1] != "bar.local" {
		t.Errorf("expected bar.local, got %s", readNames[1])
	}
	if interval != 45*time.Second {
		t.Errorf("expected 45s interval, got %v", interval)
	}
}

func TestReadConfig_DefaultInterval(t *testing.T) {
	dir := t.TempDir()
	orig := configPathForTest
	configPathForTest = filepath.Join(dir, "mdns2hosts.conf")
	defer func() { configPathForTest = orig }()

	os.WriteFile(configPathForTest, []byte("NAMES=test.local\n"), 0644)

	names, interval, err := ReadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 1 || names[0] != "test.local" {
		t.Errorf("unexpected names: %v", names)
	}
	if interval != 30*time.Second {
		t.Errorf("expected default 30s interval, got %v", interval)
	}
}

func TestReadConfig_InvalidInterval(t *testing.T) {
	dir := t.TempDir()
	orig := configPathForTest
	configPathForTest = filepath.Join(dir, "mdns2hosts.conf")
	defer func() { configPathForTest = orig }()

	os.WriteFile(configPathForTest, []byte("NAMES=test.local\nINTERVAL=bad\n"), 0644)

	names, interval, err := ReadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if names[0] != "test.local" {
		t.Errorf("unexpected name: %s", names[0])
	}
	if interval != 30*time.Second {
		t.Errorf("expected fallback to 30s for invalid interval, got %v", interval)
	}
}

func TestReadConfig_NoNames(t *testing.T) {
	dir := t.TempDir()
	orig := configPathForTest
	configPathForTest = filepath.Join(dir, "mdns2hosts.conf")
	defer func() { configPathForTest = orig }()

	os.WriteFile(configPathForTest, []byte("INTERVAL=10s\n"), 0644)

	_, _, err := ReadConfig()
	if err == nil {
		t.Error("expected error for config with no names")
	}
}

func TestReadConfig_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	orig := configPathForTest
	configPathForTest = filepath.Join(dir, "nonexistent.conf")
	defer func() { configPathForTest = orig }()

	_, _, err := ReadConfig()
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		names    []string
		expected string
	}{
		{[]string{"a.local"}, "a.local"},
		{[]string{"a.local", "b.local"}, "a.local,b.local"},
		{[]string{}, ""},
		{[]string{"a.local", "b.local", "c.local"}, "a.local,b.local,c.local"},
	}

	for _, tt := range tests {
		result := joinNames(tt.names)
		if result != tt.expected {
			t.Errorf("joinNames(%v) = %q, want %q", tt.names, result, tt.expected)
		}
	}
}

func TestSaveConfig_Overwrite(t *testing.T) {
	dir := t.TempDir()
	orig := configPathForTest
	configPathForTest = filepath.Join(dir, "mdns2hosts.conf")
	defer func() { configPathForTest = orig }()

	err := SaveConfig([]string{"first.local"}, "10s")
	if err != nil {
		t.Fatal(err)
	}

	err = SaveConfig([]string{"second.local"}, "20s")
	if err != nil {
		t.Fatal(err)
	}

	names, interval, err := ReadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if names[0] != "second.local" {
		t.Errorf("expected second.local, got %s", names[0])
	}
	if interval != 20*time.Second {
		t.Errorf("expected 20s, got %v", interval)
	}
}
