package service

import (
	"errors"
	"net"
	"testing"
)

type recordingLogger struct {
	info     []string
	warnings []string
	errors   []string
}

func (r *recordingLogger) Info(msg string) {
	r.info = append(r.info, msg)
}

func (r *recordingLogger) Warning(msg string) {
	r.warnings = append(r.warnings, msg)
}

func (r *recordingLogger) Error(msg string) {
	r.errors = append(r.errors, msg)
}

func withSyncDeps(t *testing.T) {
	t.Helper()
	origResolve := resolveAllNames
	origEnsure := ensureHostsFile
	origRead := readHostsFile
	origWrite := writeHostsFile
	t.Cleanup(func() {
		resolveAllNames = origResolve
		ensureHostsFile = origEnsure
		readHostsFile = origRead
		writeHostsFile = origWrite
	})
}

func TestSyncConfiguredNamesSuccess(t *testing.T) {
	withSyncDeps(t)
	logger := &recordingLogger{}
	var wrote map[string]net.IP

	resolveAllNames = func(names []string) (map[string]net.IP, []error) {
		if len(names) != 1 || names[0] != "foo.local" {
			t.Fatalf("unexpected names: %v", names)
		}
		return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, nil
	}
	ensureHostsFile = func() error { return nil }
	readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
		return []string{"before"}, nil, []string{"after"}, nil
	}
	writeHostsFile = func(before []string, entries map[string]net.IP, after []string) error {
		if before[0] != "before" || after[0] != "after" {
			t.Fatalf("unexpected preserved sections: %v %v", before, after)
		}
		wrote = cloneIPMap(entries)
		return nil
	}

	syncConfiguredNames([]string{"foo.local"}, logger)

	if !wrote["foo.local"].Equal(net.ParseIP("10.0.0.1")) {
		t.Fatalf("unexpected written entries: %v", wrote)
	}
	if len(logger.errors) != 0 {
		t.Fatalf("unexpected errors: %v", logger.errors)
	}
}

func TestSyncConfiguredNamesWarningsAndNoResults(t *testing.T) {
	withSyncDeps(t)
	logger := &recordingLogger{}
	ensureCalled := false

	resolveAllNames = func([]string) (map[string]net.IP, []error) {
		return nil, []error{errors.New("resolve failed")}
	}
	ensureHostsFile = func() error {
		ensureCalled = true
		return nil
	}

	syncConfiguredNames([]string{"foo.local"}, logger)

	if len(logger.warnings) != 1 {
		t.Fatalf("expected warning, got %v", logger.warnings)
	}
	if ensureCalled {
		t.Fatal("ensure should not be called without resolved results")
	}
}

func TestSyncConfiguredNamesErrorBranches(t *testing.T) {
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
			name: "read",
			setup: func() {
				ensureHostsFile = func() error { return nil }
				readHostsFile = func() ([]string, map[string]net.IP, []string, error) {
					return nil, nil, nil, errors.New("read")
				}
			},
		},
		{
			name: "write",
			setup: func() {
				ensureHostsFile = func() error { return nil }
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
			withSyncDeps(t)
			logger := &recordingLogger{}
			resolveAllNames = func([]string) (map[string]net.IP, []error) {
				return map[string]net.IP{"foo.local": net.ParseIP("10.0.0.1")}, nil
			}
			tt.setup()

			syncConfiguredNames([]string{"foo.local"}, logger)

			if len(logger.errors) != 1 {
				t.Fatalf("expected one error, got %v", logger.errors)
			}
		})
	}
}

func TestServiceLogFallbacks(t *testing.T) {
	logInfo(nil, "info")
	logWarning(nil, "warning")
	logError(nil, "error")

	logger := &recordingLogger{}
	logInfo(logger, "info")
	logWarning(logger, "warning")
	logError(logger, "error")

	if len(logger.info) != 1 || len(logger.warnings) != 1 || len(logger.errors) != 1 {
		t.Fatalf("unexpected logger records: %+v", logger)
	}
}
