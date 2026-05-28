package hosts

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/txn2/txeh"
)

const (
	managedComment = "mdns2hosts"
	hostsFileName  = "hosts"
)

// HostsPath returns the full path to the Windows hosts file.
func HostsPath() string {
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	return filepath.Join(sysRoot, "System32", "drivers", "etc", hostsFileName)
}

// ReadHosts reads the system hosts file.
func ReadHosts() (before []string, managed map[string]net.IP, after []string, err error) {
	return ReadHostsFile(HostsPath())
}

// ReadHostsFile reads a hosts file and returns managed mdns2hosts entries.
// The before/after return values are retained for compatibility with callers
// that used the previous block-oriented API; txeh owns preservation now.
func ReadHostsFile(path string) (before []string, managed map[string]net.IP, after []string, err error) {
	h, err := loadHosts(path)
	if err != nil {
		return nil, nil, nil, err
	}

	managed = make(map[string]net.IP)
	for _, line := range h.GetHostFileLines() {
		if line.Comment != managedComment {
			continue
		}
		ip := net.ParseIP(line.Address)
		if ip == nil {
			continue
		}
		for _, host := range line.Hostnames {
			managed[host] = ip
		}
	}

	return nil, managed, nil, nil
}

// WriteHosts writes the system hosts file with managed mdns2hosts entries updated.
func WriteHosts(before []string, entries map[string]net.IP, after []string) error {
	return WriteHostsFile(HostsPath(), before, entries, after)
}

// RenderHosts renders the system hosts file with managed mdns2hosts entries updated.
func RenderHosts(before []string, entries map[string]net.IP, after []string) (string, error) {
	return RenderHostsFile(HostsPath(), before, entries, after)
}

// WriteHostsFile replaces all mdns2hosts-tagged entries in a hosts file.
// The before/after parameters are ignored; they are retained to keep the public
// package surface stable while txeh preserves unmanaged file content.
func WriteHostsFile(path string, before []string, entries map[string]net.IP, after []string) error {
	rendered, err := RenderHostsFile(path, before, entries, after)
	if err != nil {
		return err
	}

	return writeRendered(path, rendered)
}

// RenderHostsFile returns the hosts file content that WriteHostsFile would write.
// The before/after parameters are ignored; they are retained to keep the public
// package surface stable while txeh preserves unmanaged file content.
func RenderHostsFile(path string, before []string, entries map[string]net.IP, after []string) (string, error) {
	h, err := loadHosts(path)
	if err != nil {
		return "", err
	}

	h.RemoveByComment(managedComment)

	var hosts []string
	for host := range entries {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	for _, host := range hosts {
		ip := entries[host]
		if ip == nil {
			continue
		}
		h.AddHostWithComment(ip.String(), host, managedComment)
	}

	return toCRLF(h.RenderHostsFile()), nil
}

// EnsureBlock verifies that the hosts file can be loaded.
// mdns2hosts now uses comment-tagged entries instead of BEGIN/END block markers.
func EnsureBlock() error {
	return EnsureBlockFile(HostsPath())
}

// EnsureBlockFile verifies that the given hosts file can be loaded.
func EnsureBlockFile(path string) error {
	_, err := loadHosts(path)
	return err
}

// CleanBlock removes all mdns2hosts-managed entries.
func CleanBlock() error {
	return CleanBlockFile(HostsPath())
}

// CleanBlockFile removes all mdns2hosts-managed entries in the given file.
func CleanBlockFile(path string) error {
	return WriteHostsFile(path, nil, nil, nil)
}

func loadHosts(path string) (*txeh.Hosts, error) {
	h, err := txeh.NewHosts(&txeh.HostsConfig{
		ReadFilePath:    path,
		WriteFilePath:   path,
		MaxHostsPerLine: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("read hosts file %s: %w", path, err)
	}
	return h, nil
}

func writeRendered(path string, rendered string) error {
	data := []byte(rendered)
	tmpPath := path + ".mdns2hosts.tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp hosts: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot replace hosts file: %w", err)
	}

	return nil
}

func toCRLF(s string) string {
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.ReplaceAll(normalized, "\n", "\r\n")
}

func isManagedLine(line string) bool {
	return strings.TrimSpace(line) == "# "+managedComment ||
		strings.HasSuffix(strings.TrimSpace(line), " # "+managedComment)
}

func splitLines(data []byte) []string {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
	text := string(data)
	if strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text, "\n")
	}
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
