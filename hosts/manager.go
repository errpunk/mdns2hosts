package hosts

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	blockBegin = "# BEGIN mdns2hosts"
	blockEnd   = "# END mdns2hosts"

	hostsFileName = "hosts"
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

// ReadHostsFile reads a hosts file and returns:
// - lines before the managed block (preserved as-is)
// - managed entries within the block
// - lines after the managed block (preserved as-is)
func ReadHostsFile(path string) (before []string, managed map[string]net.IP, after []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot open hosts file %s: %w", path, err)
	}
	defer f.Close()

	managed = make(map[string]net.IP)
	var inBlock bool
	var section int // 0=before, 1=block, 2=after

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == blockBegin {
			inBlock = true
			section = 1
			before = append(before, line)
			continue
		}
		if strings.TrimSpace(line) == blockEnd {
			inBlock = false
			section = 2
			after = append(after, line)
			continue
		}

		switch section {
		case 0:
			if !inBlock {
				before = append(before, line)
			}
		case 1:
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				ip := net.ParseIP(fields[0])
				if ip != nil {
					for _, h := range fields[1:] {
						managed[h] = ip
					}
				}
			}
		case 2:
			after = append(after, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("error reading hosts: %w", err)
	}

	return before, managed, after, nil
}

// WriteHosts writes the system hosts file with the managed block updated.
func WriteHosts(before []string, entries map[string]net.IP, after []string) error {
	return WriteHostsFile(HostsPath(), before, entries, after)
}

// WriteHostsFile writes a hosts file with the managed block updated.
// Lines before and after the block are preserved exactly.
func WriteHostsFile(path string, before []string, entries map[string]net.IP, after []string) error {
	var buf bytes.Buffer

	for _, line := range before {
		buf.WriteString(line)
		buf.WriteString("\r\n")
	}

	var hosts []string
	for h := range entries {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)

	for _, h := range hosts {
		ip := entries[h]
		fmt.Fprintf(&buf, "%s %s\r\n", ip.String(), h)
	}

	for _, line := range after {
		buf.WriteString(line)
		buf.WriteString("\r\n")
	}

	// Atomic write: write to temp file then rename
	tmpPath := path + ".mdns2hosts.tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("cannot write temp hosts: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot replace hosts file: %w", err)
	}

	return nil
}

// EnsureBlock ensures the managed block markers exist in the hosts file.
func EnsureBlock() error {
	return EnsureBlockFile(HostsPath())
}

// EnsureBlockFile ensures the managed block markers exist in the given file.
func EnsureBlockFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read hosts: %w", err)
	}

	if bytes.Contains(content, []byte(blockBegin)) && bytes.Contains(content, []byte(blockEnd)) {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open hosts for append: %w", err)
	}
	defer f.Close()

	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\r\n")) {
		f.Write([]byte("\r\n"))
	}

	fmt.Fprintf(f, "%s\r\n%s\r\n", blockBegin, blockEnd)
	return nil
}

// CleanBlock removes all content between the managed block markers.
func CleanBlock() error {
	return CleanBlockFile(HostsPath())
}

// CleanBlockFile removes all content between the managed block markers in the given file.
func CleanBlockFile(path string) error {
	before, _, after, err := ReadHostsFile(path)
	if err != nil {
		return err
	}
	return WriteHostsFile(path, before, nil, after)
}
