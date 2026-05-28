package cmd

import (
	"errors"
	"net"
	"testing"
	"time"
)

func withCmdDeps(t *testing.T) {
	t.Helper()

	origEnsure := ensureHostsFile
	origRead := readHostsFile
	origWrite := writeHostsFile
	origResolve := resolveAllNames
	origClean := cleanHostsFile
	origExe := serviceExePath
	origInstall := installService
	origUninstall := uninstallService
	origNames := svcNames
	origInterval := svcInterval

	t.Cleanup(func() {
		ensureHostsFile = origEnsure
		readHostsFile = origRead
		writeHostsFile = origWrite
		resolveAllNames = origResolve
		cleanHostsFile = origClean
		serviceExePath = origExe
		installService = origInstall
		uninstallService = origUninstall
		svcNames = origNames
		svcInterval = origInterval
	})
}

func TestRunSyncSuccess(t *testing.T) {
	withCmdDeps(t)
	var wrote map[string]net.IP

	ensureHostsFile = func() error { return nil }
	resolveAllNames = func(names []string) (map[string]net.IP, []error) {
		if len(names) != 1 || names[0] != "foo.local" {
			t.Fatalf("unexpected names: %v", names)
		}
		return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, nil
	}
	readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
		return []string{"before"}, nil, []string{"after"}, nil
	}
	writeHostsFile = func(before []string, entries map[string]net.IP, after []string) error {
		wrote = cloneIPMap(entries)
		return nil
	}

	if err := runSync(nil, []string{"foo.local"}); err != nil {
		t.Fatal(err)
	}
	if !wrote["foo.local"].Equal(net.ParseIP("10.0.0.1")) {
		t.Fatalf("unexpected written entries: %v", wrote)
	}
}

func TestRunSyncErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "ensure",
			setup: func() {
				ensureHostsFile = func() error { return errors.New("ensure") }
			},
		},
		{
			name: "no results",
			setup: func() {
				ensureHostsFile = func() error { return nil }
				resolveAllNames = func([]string) (map[string]net.IP, []error) {
					return nil, []error{errors.New("resolve")}
				}
			},
		},
		{
			name: "read",
			setup: func() {
				ensureHostsFile = func() error { return nil }
				resolveAllNames = func([]string) (map[string]net.IP, []error) {
					return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, nil
				}
				readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
					return nil, nil, nil, errors.New("read")
				}
			},
		},
		{
			name: "write",
			setup: func() {
				ensureHostsFile = func() error { return nil }
				resolveAllNames = func([]string) (map[string]net.IP, []error) {
					return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, nil
				}
				readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
					return nil, nil, nil, nil
				}
				writeHostsFile = func([]string, map[string]net.IP, []string) error {
					return errors.New("write")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withCmdDeps(t)
			tt.setup()
			if err := runSync(nil, []string{"foo.local"}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRunClean(t *testing.T) {
	withCmdDeps(t)
	called := false
	cleanHostsFile = func() error {
		called = true
		return nil
	}
	if err := runClean(nil, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("clean not called")
	}

	cleanHostsFile = func() error { return errors.New("clean") }
	if err := runClean(nil, nil); err == nil {
		t.Fatal("expected clean error")
	}
}

func TestRunInstall(t *testing.T) {
	withCmdDeps(t)
	svcNames = []string{"foo.local"}
	svcInterval = "10s"
	serviceExePath = func() (string, error) { return "mdns2hosts.exe", nil }
	installService = func(names []string, interval string, exePath string) error {
		if names[0] != "foo.local" || interval != "10s" || exePath != "mdns2hosts.exe" {
			t.Fatalf("unexpected install args: %v %s %s", names, interval, exePath)
		}
		return nil
	}
	if err := runInstall(nil, nil); err != nil {
		t.Fatal(err)
	}

	serviceExePath = func() (string, error) { return "", errors.New("exe") }
	if err := runInstall(nil, nil); err == nil {
		t.Fatal("expected exe path error")
	}

	serviceExePath = func() (string, error) { return "mdns2hosts.exe", nil }
	installService = func([]string, string, string) error { return errors.New("install") }
	if err := runInstall(nil, nil); err == nil {
		t.Fatal("expected install error")
	}
}

func TestRunUninstall(t *testing.T) {
	withCmdDeps(t)
	uninstallService = func() error { return nil }
	if err := runUninstall(nil, nil); err != nil {
		t.Fatal(err)
	}

	uninstallService = func() error { return errors.New("uninstall") }
	if err := runUninstall(nil, nil); err == nil {
		t.Fatal("expected uninstall error")
	}
}

func TestWatchHelpers(t *testing.T) {
	withCmdDeps(t)

	resolveAllNames = func(names []string) (map[string]net.IP, []error) {
		return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, []error{errors.New("warn")}
	}
	got := syncOnce([]string{"foo.local"})
	if !got["foo.local"].Equal(net.ParseIP("10.0.0.1")) {
		t.Fatalf("unexpected syncOnce result: %v", got)
	}

	a := map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}
	b := map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}
	c := map[string]net.IP{"foo.local": net.ParseIP("10.0.0.2")}
	if ipsChanged(a, b) {
		t.Fatal("same IPs should not be changed")
	}
	if !ipsChanged(a, c) || !ipsChanged(a, nil) {
		t.Fatal("different IP maps should be changed")
	}
}

func TestRunWatchWithStop(t *testing.T) {
	withCmdDeps(t)
	writes := 0
	resolveCalls := 0

	ensureHostsFile = func() error { return nil }
	resolveAllNames = func([]string) (map[string]net.IP, []error) {
		resolveCalls++
		if resolveCalls == 1 {
			return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, nil
		}
		return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.2")}, nil
	}
	readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
		return nil, nil, nil, nil
	}
	writeHostsFile = func([]string, map[string]net.IP, []string) error {
		writes++
		return nil
	}

	origInterval := watchInterval
	watchInterval = time.Millisecond
	t.Cleanup(func() { watchInterval = origInterval })

	stop := make(chan struct{})
	go func() {
		for writes == 0 {
			time.Sleep(time.Millisecond)
		}
		close(stop)
	}()

	if err := runWatchWithStop([]string{"foo.local"}, stop); err != nil {
		t.Fatal(err)
	}
	if writes == 0 {
		t.Fatal("watch should write after IP change")
	}
}

func TestRunWatchEnsureError(t *testing.T) {
	withCmdDeps(t)
	ensureHostsFile = func() error { return errors.New("ensure") }
	if err := runWatchWithStop([]string{"foo.local"}, make(chan struct{})); err == nil {
		t.Fatal("expected ensure error")
	}
}

func TestWriteHostsHelper(t *testing.T) {
	withCmdDeps(t)
	wrote := false
	readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
		return []string{"before"}, nil, []string{"after"}, nil
	}
	writeHostsFile = func(before []string, entries map[string]net.IP, after []string) error {
		wrote = true
		if before[0] != "before" || after[0] != "after" {
			t.Fatalf("unexpected preserved sections: %v %v", before, after)
		}
		return nil
	}
	writeHosts(map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")})
	if !wrote {
		t.Fatal("write not called")
	}

	readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
		return nil, nil, nil, errors.New("read")
	}
	writeHosts(map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")})

	readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
		return nil, nil, nil, nil
	}
	writeHostsFile = func([]string, map[string]net.IP, []string) error {
		return errors.New("write")
	}
	writeHosts(map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")})
}
