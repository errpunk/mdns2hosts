// Package service provides Windows service integration for mdns2hosts.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const svcName = "mdns2hosts"
const svcDisplay = "mDNS to Hosts Sync Service"

// configPathForTest allows tests to override the config path.
var configPathForTest string

// ExePath returns the full path to the running executable.
func ExePath() (string, error) {
	return os.Executable()
}

// ConfigPath returns the path where service configuration is stored.
func ConfigPath() (string, error) {
	if configPathForTest != "" {
		return configPathForTest, nil
	}
	exe, err := ExePath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "mdns2hosts.conf"), nil
}

// SaveConfig writes the service configuration file.
func SaveConfig(names []string, interval string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data := fmt.Sprintf("NAMES=%s\nINTERVAL=%s\n", joinNames(names), interval)
	return os.WriteFile(path, []byte(data), 0644)
}

func joinNames(names []string) string {
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(n)
	}
	return b.String()
}

// ReadConfig reads the service configuration file.
func ReadConfig() (names []string, interval time.Duration, err error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot read config: %w", err)
	}

	var intervalStr string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "NAMES="); ok {
			names = strings.Split(after, ",")
		}
		if after, ok := strings.CutPrefix(line, "INTERVAL="); ok {
			intervalStr = after
		}
	}

	if len(names) == 0 {
		return nil, 0, fmt.Errorf("no names in config")
	}

	interval, err = time.ParseDuration(intervalStr)
	if err != nil {
		interval = 30 * time.Second
	}

	return names, interval, nil
}
