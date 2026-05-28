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

func TestReadHostsFile_NoManagedEntries(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\r\n# some comment\r\n")

	before, managed, after, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if before != nil || after != nil {
		t.Fatalf("before/after are compatibility placeholders, got %v / %v", before, after)
	}
	if len(managed) != 0 {
		t.Fatalf("expected no managed entries, got %v", managed)
	}
}

func TestReadHostsFile_ManagedCommentEntries(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\r\n192.168.1.1 foo.local bar.local # mdns2hosts\r\n10.0.0.2 other.local # other-tool\r\n")

	_, managed, _, err := ReadHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(managed) != 2 {
		t.Fatalf("expected 2 managed entries, got %d: %v", len(managed), managed)
	}
	if !managed["foo.local"].Equal(net.ParseIP("192.168.1.1")) {
		t.Fatalf("foo.local = %v", managed["foo.local"])
	}
	if !managed["bar.local"].Equal(net.ParseIP("192.168.1.1")) {
		t.Fatalf("bar.local = %v", managed["bar.local"])
	}
	if managed["other.local"] != nil {
		t.Fatalf("other-tool entry should not be managed: %v", managed["other.local"])
	}
}

func TestWriteHostsFile_UsesManagedComment(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\r\n")
	entries := map[string]net.IP{
		"foo.local": net.ParseIP("192.168.1.1"),
	}

	if err := WriteHostsFile(path, nil, entries, nil); err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	if !strings.Contains(content, "192.168.1.1") || !strings.Contains(content, "foo.local") {
		t.Fatalf("managed entry missing:\n%s", content)
	}
	if !strings.Contains(content, "# mdns2hosts") {
		t.Fatalf("managed comment missing:\n%s", content)
	}
	if strings.Contains(content, "# BEGIN mdns2hosts") || strings.Contains(content, "# END mdns2hosts") {
		t.Fatalf("block markers should not be written:\n%s", content)
	}
}

func TestWriteHostsFile_ReplacesOnlyManagedEntries(t *testing.T) {
	content := strings.Join([]string{
		"127.0.0.1 localhost",
		"10.0.0.1 old.local # mdns2hosts",
		"10.0.0.2 keep.local # other-tool",
		"# tail comment",
		"",
	}, "\r\n")
	path := writeTemp(t, content)

	entries := map[string]net.IP{
		"new.local": net.ParseIP("10.0.0.9"),
	}
	if err := WriteHostsFile(path, nil, entries, nil); err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	if strings.Contains(output, "old.local") {
		t.Fatalf("old managed entry should be removed:\n%s", output)
	}
	if !strings.Contains(output, "10.0.0.9") || !strings.Contains(output, "new.local") {
		t.Fatalf("new managed entry missing:\n%s", output)
	}
	if !strings.Contains(output, "keep.local # other-tool") {
		t.Fatalf("unmanaged commented entry lost:\n%s", output)
	}
	if !strings.Contains(output, "# tail comment") {
		t.Fatalf("tail comment lost:\n%s", output)
	}
}

func TestWriteHostsFile_SortedOutput(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\r\n")
	entries := map[string]net.IP{
		"z.local": net.ParseIP("10.0.0.1"),
		"a.local": net.ParseIP("10.0.0.1"),
		"m.local": net.ParseIP("10.0.0.1"),
	}

	if err := WriteHostsFile(path, nil, entries, nil); err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	aPos := strings.Index(content, "a.local")
	mPos := strings.Index(content, "m.local")
	zPos := strings.Index(content, "z.local")
	if !(aPos < mPos && mPos < zPos) {
		t.Fatalf("entries not sorted alphabetically:\n%s", content)
	}
}

func TestWriteHostsFile_CRLF(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\n")
	entries := map[string]net.IP{
		"test.local": net.ParseIP("192.168.1.1"),
	}

	if err := WriteHostsFile(path, nil, entries, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\n") && !strings.Contains(string(data), "\r\n") {
		t.Fatalf("expected CRLF line endings, got %q", string(data))
	}
}

func TestWriteHostsFile_AtomicWrite(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\r\n")
	entries := map[string]net.IP{
		"test.local": net.ParseIP("10.0.0.1"),
	}

	if err := WriteHostsFile(path, nil, entries, nil); err != nil {
		t.Fatal(err)
	}

	tmpPath := path + ".mdns2hosts.tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Fatal("temp file should be cleaned up after write")
	}
}

func TestCleanBlockFile_RemovesManagedEntries(t *testing.T) {
	content := "127.0.0.1 localhost\r\n10.0.0.1 remove.local # mdns2hosts\r\n10.0.0.2 keep.local # other-tool\r\n"
	path := writeTemp(t, content)

	if err := CleanBlockFile(path); err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	if strings.Contains(output, "remove.local") {
		t.Fatalf("managed entry should be removed:\n%s", output)
	}
	if !strings.Contains(output, "keep.local # other-tool") {
		t.Fatalf("unmanaged entry should be preserved:\n%s", output)
	}
}

func TestEnsureBlockFile_LoadsExistingFile(t *testing.T) {
	path := writeTemp(t, "127.0.0.1 localhost\r\n")

	if err := EnsureBlockFile(path); err != nil {
		t.Fatal(err)
	}

	content := readTemp(t, path)
	if strings.Contains(content, "# BEGIN mdns2hosts") || strings.Contains(content, "# mdns2hosts") {
		t.Fatalf("ensure should not mutate the file:\n%s", content)
	}
}

func TestEnsureBlockFile_NonExistentFile(t *testing.T) {
	if err := EnsureBlockFile("/nonexistent/path/hosts"); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestWriteHostsFile_UnwritablePath(t *testing.T) {
	err := WriteHostsFile("/nonexistent/path/hosts", nil, map[string]net.IP{
		"test.local": net.ParseIP("1.1.1.1"),
	}, nil)
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestOldBlockContentIsNotSpecial(t *testing.T) {
	content := "127.0.0.1 localhost\r\n# BEGIN mdns2hosts\r\n10.0.0.1 old.local\r\n# END mdns2hosts\r\n"
	path := writeTemp(t, content)

	if err := WriteHostsFile(path, nil, map[string]net.IP{
		"new.local": net.ParseIP("10.0.0.2"),
	}, nil); err != nil {
		t.Fatal(err)
	}

	output := readTemp(t, path)
	if !strings.Contains(output, "# BEGIN mdns2hosts") || !strings.Contains(output, "old.local") {
		t.Fatalf("old block should be preserved as ordinary content:\n%s", output)
	}
	if !strings.Contains(output, "new.local") || !strings.Contains(output, "# mdns2hosts") {
		t.Fatalf("new managed comment entry missing:\n%s", output)
	}
}

func TestHostsPath_CustomSystemRoot(t *testing.T) {
	t.Setenv("SystemRoot", `D:\Win`)

	path := HostsPath()
	if !strings.Contains(path, `D:\Win`) {
		t.Fatalf("expected path under D:\\Win, got %s", path)
	}
}

func TestHostsPath_Default(t *testing.T) {
	t.Setenv("SystemRoot", "")

	path := HostsPath()
	if !strings.Contains(path, `C:\Windows`) {
		t.Fatalf("expected default C:\\Windows in path, got %s", path)
	}
	if !strings.Contains(path, "etc") || !strings.Contains(path, "hosts") {
		t.Fatalf("expected hosts path, got %s", path)
	}
}

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
	t.Setenv("SystemRoot", dir)
	return path
}

func TestReadHosts_Wrapper(t *testing.T) {
	setupFakeHosts(t, "127.0.0.1 localhost\r\n10.0.0.1 x.local # mdns2hosts\r\n")

	_, managed, _, err := ReadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if managed["x.local"] == nil || !managed["x.local"].Equal(net.ParseIP("10.0.0.1")) {
		t.Fatalf("x.local not found or wrong IP: %v", managed["x.local"])
	}
}

func TestWriteHosts_Wrapper(t *testing.T) {
	setupFakeHosts(t, "127.0.0.1 localhost\r\n")

	entries := map[string]net.IP{"w.local": net.ParseIP("192.168.1.1")}
	if err := WriteHosts(nil, entries, nil); err != nil {
		t.Fatal(err)
	}

	_, managed, _, err := ReadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if managed["w.local"] == nil {
		t.Fatal("entry not written via wrapper")
	}
}

func TestEnsureBlock_Wrapper(t *testing.T) {
	setupFakeHosts(t, "127.0.0.1 localhost\r\n")

	if err := EnsureBlock(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanBlock_Wrapper(t *testing.T) {
	setupFakeHosts(t, "127.0.0.1 localhost\r\n10.0.0.1 removed.local # mdns2hosts\r\n")

	if err := CleanBlock(); err != nil {
		t.Fatal(err)
	}

	_, managed, _, err := ReadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) != 0 {
		t.Fatalf("clean wrapper should remove all managed entries: %v", managed)
	}
}

func TestHelpers(t *testing.T) {
	if toCRLF("a\nb\r\nc") != "a\r\nb\r\nc" {
		t.Fatalf("unexpected CRLF normalization")
	}
	if !isManagedLine("1.1.1.1 a.local # mdns2hosts") {
		t.Fatalf("managed line not detected")
	}
	if got := splitLines([]byte("a\r\nb\n")); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected splitLines result: %v", got)
	}
}
