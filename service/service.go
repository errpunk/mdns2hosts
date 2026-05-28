// Package service provides Windows service integration for mdns2hosts.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExePath returns the full path to the running executable.
func ExePath() (string, error) {
	return os.Executable()
}

// ConfigPath returns the path where service configuration is stored.
func ConfigPath() (string, error) {
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
