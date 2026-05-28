package hosts

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readTemp(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReadHostsFile_NoBlock(t *testing.T) {
	content := "127.0.0.1 localhost\r\n# some comment\r\n"
	path := writeTemp(t, content)

	before, managed, after, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(before) != 2 {
		t.Errorf("expected 2 before lines, got %d", len(before))
	}
	if before[0] != "127.0.0.1 localhost" {
		t.Errorf("expected before[0] = %q, got %q", "127.0.0.1 localhost", before[0])
	}
	if before[1] != "# some comment" {
		t.Errorf("expected before[1] = %q, got %q", "# some comment", before[1])
	}
	if len(managed) != 0 {
		t.Errorf("expected 0 managed entries, got %d", len(managed))
	}
	if len(after) != 0 {
		t.Errorf("expected 0 after lines, got %d", len(after))
	}
}

func TestReadHostsFile_WithBlock(t *testing.T) {
	content := "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\n192.168.1.1 foo.local\r\n# END mdns2hosts\r\ntail line\r\n"
	path := writeTemp(t, content)

	before, managed, after, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(before) != 2 {
		t.Errorf("expected 2 before lines, got %d: %v", len(before), before)
	}
	if before[0] != "127.0.0.1 localhost" {
		t.Errorf("unexpected before[0]: %q", before[0])
	}
	if !strings.Contains(before[1], blockBegin) {
		t.Errorf("expected block begin marker, got %q", before[1])
	}

	ip := managed["foo.local"]
	if ip == nil || ip.String() != "192.168.1.1" {
		t.Errorf("expected foo.local -> 192.168.1.1, got %v", ip)
	}

	if len(after) != 2 {
		t.Errorf("expected 2 after lines, got %d: %v", len(after), after)
	}
	if !strings.Contains(after[0], blockEnd) {
		t.Errorf("expected block end marker, got %q", after[0])
	}
	if after[1] != "tail line" {
		t.Errorf("expected tail line, got %q", after[1])
	}
}

func TestReadHostsFile_MultipleEntries(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n10.0.0.1 a.local b.local\r\n10.0.0.2 c.local\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(managed) != 3 {
		t.Fatalf("expected 3 managed entries, got %d", len(managed))
	}
	if managed["a.local"].String() != "10.0.0.1" {
		t.Errorf("a.local = %s", managed["a.local"])
	}
	if managed["b.local"].String() != "10.0.0.1" {
		t.Errorf("b.local = %s", managed["b.local"])
	}
	if managed["c.local"].String() != "10.0.0.2" {
		t.Errorf("c.local = %s", managed["c.local"])
	}
}

func TestReadHostsFile_EmptyBlock(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	before, managed, after, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(managed) != 0 {
		t.Errorf("expected 0 managed entries, got %d", len(managed))
	}
	if len(before) != 1 {
		t.Errorf("expected 1 before line, got %d", len(before))
	}
	if len(after) != 1 {
		t.Errorf("expected 1 after line, got %d", len(after))
	}
}

func TestReadHostsFile_InvalidIPIgnored(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\nnot-an-ip foo.local\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(managed) != 0 {
		t.Errorf("expected 0 managed entries (invalid IP), got %d", len(managed))
	}
}

func TestWriteHostsFile_Basic(t *testing.T) {
	path := writeTemp(t, "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n")

	entries := map[string]net.IP{
		"foo.local": net.ParseIP("192.168.1.1"),
	}
	err := WriteHostsFile(path, []string{"# BEGIN mdns2hosts"}, entries, []string{"# END mdns2hosts"})
	if err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	if !strings.Contains(content, "192.168.1.1 foo.local") {
		t.Errorf("expected entry in output, got:\n%s", content)
	}
	if !strings.Contains(content, "# BEGIN mdns2hosts") {
		t.Errorf("expected BEGIN marker, got:\n%s", content)
	}
	if !strings.Contains(content, "# END mdns2hosts") {
		t.Errorf("expected END marker, got:\n%s", content)
	}
}

func TestWriteHostsFile_PreservesBeforeAndAfter(t *testing.T) {
	content := "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\nold-entry.local\r\n# END mdns2hosts\r\n# tail comment\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	managed["new.local"] = net.ParseIP("10.0.0.1")

	before := []string{"127.0.0.1 localhost", "# BEGIN mdns2hosts"}
	after := []string{"# END mdns2hosts", "# tail comment"}
	err = WriteHostsFile(path, before, managed, after)
	if err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	if !strings.Contains(output, "127.0.0.1 localhost") {
		t.Error("before line lost")
	}
	if !strings.Contains(output, "# tail comment") {
		t.Error("after line lost")
	}
	if !strings.Contains(output, "10.0.0.1 new.local") {
		t.Error("new entry missing")
	}
	if strings.Contains(output, "old-entry.local") {
		t.Error("old entry should be removed")
	}
}

func TestWriteHostsFile_SortedOutput(t *testing.T) {
	path := writeTemp(t, "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n")

	entries := map[string]net.IP{
		"z.local": net.ParseIP("10.0.0.1"),
		"a.local": net.ParseIP("10.0.0.1"),
		"m.local": net.ParseIP("10.0.0.1"),
	}
	err := WriteHostsFile(path, []string{"# BEGIN mdns2hosts"}, entries, []string{"# END mdns2hosts"})
	if err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	aPos := strings.Index(content, "a.local")
	mPos := strings.Index(content, "m.local")
	zPos := strings.Index(content, "z.local")

	if !(aPos < mPos && mPos < zPos) {
		t.Errorf("entries not sorted alphabetically:\n%s", content)
	}
}

func TestWriteHostsFile_CRLF(t *testing.T) {
	path := writeTemp(t, "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n")

	entries := map[string]net.IP{
		"test.local": net.ParseIP("192.168.1.1"),
	}
	err := WriteHostsFile(path, []string{"# BEGIN mdns2hosts"}, entries, []string{"# END mdns2hosts"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, "\r\n") < 2 {
		t.Errorf("expected CRLF line endings, got:\n%q", content)
	}
}

func TestWriteHostsFile_AtomicWrite(t *testing.T) {
	path := writeTemp(t, "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n")

	entries := map[string]net.IP{
		"test.local": net.ParseIP("10.0.0.1"),
	}
	err := WriteHostsFile(path, []string{"# BEGIN mdns2hosts"}, entries, []string{"# END mdns2hosts"})
	if err != nil {
		t.Fatal(err)
	}

	// Temp file should not exist after rename
	tmpPath := path + ".mdns2hosts.tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temp file should be cleaned up after write")
	}

	content := readTemp(t, path)
	if !strings.Contains(content, "10.0.0.1 test.local") {
		t.Errorf("unexpected content:\n%s", content)
	}
}

func TestCleanBlockFile(t *testing.T) {
	content := "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\n192.168.1.1 foo.local\r\n# END mdns2hosts\r\ntail\r\n"
	path := writeTemp(t, content)

	err := CleanBlockFile(path)
	if err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	if strings.Contains(output, "foo.local") {
		t.Error("managed entries should be removed after clean")
	}
	if !strings.Contains(output, "127.0.0.1 localhost") {
		t.Error("before content should be preserved")
	}
	if !strings.Contains(output, "tail") {
		t.Error("after content should be preserved")
	}
	if !strings.Contains(output, blockBegin) {
		t.Error("block markers should remain")
	}
	if !strings.Contains(output, blockEnd) {
		t.Error("block markers should remain")
	}
}

func TestEnsureBlockFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new-hosts")
	os.WriteFile(path, []byte("127.0.0.1 localhost"), 0644)

	err := EnsureBlockFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	if !strings.Contains(content, blockBegin) {
		t.Error("block begin marker should be appended")
	}
	if !strings.Contains(content, blockEnd) {
		t.Error("block end marker should be appended")
	}
	if !strings.Contains(content, "127.0.0.1 localhost") {
		t.Error("original content should be preserved")
	}
}

func TestEnsureBlockFile_AlreadyExists(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	err := EnsureBlockFile(path)
	if err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	count := strings.Count(output, blockBegin)
	if count != 1 {
		t.Errorf("block markers should not be duplicated, found %d BEGIN markers", count)
	}
}

func TestEnsureBlockFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-hosts")
	os.WriteFile(path, []byte(""), 0644)

	err := EnsureBlockFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	if !strings.Contains(content, blockBegin) {
		t.Error("block markers should be created in empty file")
	}
}

func TestEnsureBlockFile_NonExistentFile(t *testing.T) {
	err := EnsureBlockFile("/nonexistent/path/hosts")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestHostsPath_CustomSystemRoot(t *testing.T) {
	os.Setenv("SystemRoot", `D:\Win`)
	defer os.Setenv("SystemRoot", "")

	path := HostsPath()
	if !strings.Contains(path, `D:\Win`) {
		t.Errorf("expected path under D:\\Win, got %s", path)
	}
}

func TestWriteHostsFile_EmptyEntries(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	err := WriteHostsFile(path, []string{"# BEGIN mdns2hosts"}, nil, []string{"# END mdns2hosts"})
	if err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	if !strings.Contains(output, blockBegin) {
		t.Error("begin marker should be present")
	}
	if !strings.Contains(output, blockEnd) {
		t.Error("end marker should be present")
	}
}

func TestWriteHostsFile_ReadOnlyDir(t *testing.T) {
	// Test that WriteHostsFile fails gracefully when cannot write
	path := "/nonexistent/path/hosts"
	err := WriteHostsFile(path, []string{"# BEGIN mdns2hosts"}, map[string]net.IP{"test.local": net.ParseIP("1.1.1.1")}, []string{"# END mdns2hosts"})
	if err == nil {
		t.Error("expected error for unwritable path")
	}
}

func TestReadHostsFile_ReadError(t *testing.T) {
	_, _, _, err := ReadHostsFile("/nonexistent/hosts")
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadHostsFile_BlockWithBlankLines(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n\r\n192.168.1.1 foo.local\r\n\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(managed) != 1 {
		t.Errorf("expected 1 entry (blank lines skipped), got %d", len(managed))
	}
}

func TestReadHostsFile_SingleLineNoCrlf(t *testing.T) {
	content := "# BEGIN mdns2hosts\n192.168.1.1 foo.local\n# END mdns2hosts\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if managed["foo.local"] == nil {
		t.Error("should parse entries even without CRLF")
	}
}

func TestHostsPath_Default(t *testing.T) {
	os.Unsetenv("SystemRoot")
	path := HostsPath()
	if !strings.Contains(path, `C:\Windows`) {
		t.Errorf("expected default C:\\Windows in path, got %s", path)
	}
}

// setupFakeHosts creates a fake Windows hosts directory structure and sets SystemRoot.
// Returns the hosts file path and a cleanup function.
func setupFakeHosts(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	etcDir := filepath.Join(dir, "System32", "drivers", "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(etcDir, "hosts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	os.Setenv("SystemRoot", dir)
	t.Cleanup(func() { os.Unsetenv("SystemRoot") })
	return path
}

func TestReadHosts_Wrapper(t *testing.T) {
	path := setupFakeHosts(t, "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\n10.0.0.1 x.local\r\n# END mdns2hosts\r\n")
	_ = path

	_, managed, _, err := ReadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if managed["x.local"] == nil || managed["x.local"].String() != "10.0.0.1" {
		t.Errorf("x.local not found or wrong IP: %v", managed["x.local"])
	}
}

func TestWriteHosts_Wrapper(t *testing.T) {
	setupFakeHosts(t, "before\r\n# BEGIN mdns2hosts\r\n# END mdns2hosts\r\nafter\r\n")

	entries := map[string]net.IP{"w.local": net.ParseIP("192.168.1.1")}
	err := WriteHosts([]string{"before", "# BEGIN mdns2hosts"}, entries, []string{"# END mdns2hosts", "after"})
	if err != nil {
		t.Fatal(err)
	}

	_, managed, _, _ := ReadHosts()
	if managed["w.local"] == nil {
		t.Error("entry not written via wrapper")
	}
}

func TestEnsureBlock_Wrapper(t *testing.T) {
	setupFakeHosts(t, "127.0.0.1 localhost\r\n")

	err := EnsureBlock()
	if err != nil {
		t.Fatal(err)
	}

	_, managed, _, err := ReadHosts()
	if err != nil {
		t.Fatal(err)
	}
	_ = managed // block exists, just verifying no error
}

func TestCleanBlock_Wrapper(t *testing.T) {
	setupFakeHosts(t, "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\n10.0.0.1 removed.local\r\n# END mdns2hosts\r\n")

	err := CleanBlock()
	if err != nil {
		t.Fatal(err)
	}

	_, managed, _, err := ReadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) != 0 {
		t.Error("clean wrapper should remove all managed entries")
	}
}

func TestWriteHostsFile_NoBeginMarker(t *testing.T) {
	// WriteHostsFile doesn't validate markers — it writes whatever is given.
	// This test verifies it works even when the "before" section lacks a BEGIN marker.
	path := writeTemp(t, "just content\r\n")

	entries := map[string]net.IP{"test.local": net.ParseIP("1.1.1.1")}
	err := WriteHostsFile(path, []string{"just content"}, entries, []string{"# END mdns2hosts"})
	if err != nil {
		t.Fatal(err)
	}
	content := readTemp(t, path)
	if !strings.Contains(content, "test.local") {
		t.Error("entry should be written")
	}
}

func TestReadHostsFile_BlockWithoutEnd(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n192.168.1.1 foo.local\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Everything after BEGIN is treated as block content
	if managed["foo.local"] == nil {
		t.Error("entry should be parsed even without END marker")
	}
}

func TestEnsureBlockFile_WithUnixLineEnding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	// File ends with LF only (no CRLF)
	os.WriteFile(path, []byte("127.0.0.1 localhost\n"), 0644)

	err := EnsureBlockFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	if !strings.Contains(content, blockBegin) {
		t.Error("block markers should be appended even with LF-only file")
	}
}

func TestHostsPath(t *testing.T) {
	path := HostsPath()
	if !strings.Contains(path, "hosts") {
		t.Errorf("expected path to contain 'hosts', got %s", path)
	}
	if !strings.Contains(path, "etc") {
		t.Errorf("expected path to contain 'etc', got %s", path)
	}
}

func TestReadHostsFile_CommentInBlock(t *testing.T) {
	content := "# BEGIN mdns2hosts\r\n# some comment\r\n192.168.1.1 foo.local\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if managed["foo.local"] == nil {
		t.Error("entry after comment should be parsed")
	}
	if len(managed) != 1 {
		t.Errorf("expected 1 entry, got %d", len(managed))
	}
}

func TestReadHostsFile_FileNotFound(t *testing.T) {
	_, _, _, err := ReadHostsFile("/nonexistent/path/hosts")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadHostsFile_BlockBeginOnly(t *testing.T) {
	// If only BEGIN marker with no END, everything after BEGIN is in the block
	content := "before\r\n# BEGIN mdns2hosts\r\n192.168.1.1 foo.local\r\nafter-without-end\r\n"
	path := writeTemp(t, content)

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if managed["foo.local"] == nil {
		t.Error("entry after BEGIN should be parsed even without END")
	}
}

func TestReadHostsFile_WriteRead_Roundtrip(t *testing.T) {
	content := "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\n10.0.0.1 a.local\r\n10.0.0.2 b.local\r\n# END mdns2hosts\r\n# tail\r\n"
	path := writeTemp(t, content)

	before, managed, after, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = WriteHostsFile(path, before, managed, after)
	if err != nil {
		t.Fatal(err)
	}

	// Read again and verify
	_, managed2, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(managed) != len(managed2) {
		t.Errorf("roundtrip lost entries: %d -> %d", len(managed), len(managed2))
	}
	for k, v := range managed {
		if !managed2[k].Equal(v) {
			t.Errorf("roundtrip mismatch for %s: %v vs %v", k, v, managed2[k])
		}
	}
}
