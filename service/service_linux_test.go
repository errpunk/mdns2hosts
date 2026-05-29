//go:build linux

package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	kservice "github.com/kardianos/service"
)

type fakeLinuxService struct {
	installed    bool
	stopped      bool
	uninstalled  bool
	ran          bool
	installErr   error
	stopErr      error
	uninstallErr error
	runErr       error
}

func (f *fakeLinuxService) Install() error {
	f.installed = true
	return f.installErr
}

func (f *fakeLinuxService) Stop() error {
	f.stopped = true
	return f.stopErr
}

func (f *fakeLinuxService) Uninstall() error {
	f.uninstalled = true
	return f.uninstallErr
}

func (f *fakeLinuxService) Run() error {
	f.ran = true
	return f.runErr
}

func withLinuxServiceDeps(t *testing.T) {
	t.Helper()
	origNewService := newLinuxService
	origConfigPath := configPathForTest
	origRead := readServiceConfig
	origTicker := newServiceTicker
	t.Cleanup(func() {
		newLinuxService = origNewService
		configPathForTest = origConfigPath
		readServiceConfig = origRead
		newServiceTicker = origTicker
	})
	configPathForTest = filepath.Join(t.TempDir(), "mdns2hosts.conf")
}

func TestInstallLinuxSuccess(t *testing.T) {
	withLinuxServiceDeps(t)
	fake := &fakeLinuxService{}
	var gotPath string
	newLinuxService = func(exePath string, program kservice.Interface) (linuxService, error) {
		gotPath = exePath
		if program == nil {
			t.Fatal("expected service program")
		}
		return fake, nil
	}

	if err := Install([]string{"foo.local"}, "10s", "/usr/local/bin/mdns2hosts"); err != nil {
		t.Fatal(err)
	}
	if !fake.installed {
		t.Fatal("service should be installed")
	}
	if gotPath != "/usr/local/bin/mdns2hosts" {
		t.Fatalf("unexpected executable path: %s", gotPath)
	}

	names, interval, err := ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if names[0] != "foo.local" || interval != 10*time.Second {
		t.Fatalf("config not saved: %v %v", names, interval)
	}
}

func TestInstallLinuxErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeLinuxService)
	}{
		{
			name: "save config",
			setup: func(*fakeLinuxService) {
				configPathForTest = t.TempDir()
			},
		},
		{
			name: "create service",
			setup: func(*fakeLinuxService) {
				newLinuxService = func(string, kservice.Interface) (linuxService, error) {
					return nil, errors.New("create")
				}
			},
		},
		{
			name: "install",
			setup: func(fake *fakeLinuxService) {
				fake.installErr = errors.New("install")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withLinuxServiceDeps(t)
			fake := &fakeLinuxService{}
			newLinuxService = func(string, kservice.Interface) (linuxService, error) {
				return fake, nil
			}
			tt.setup(fake)
			if err := Install([]string{"foo.local"}, "10s", "/bin/mdns2hosts"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUninstallLinux(t *testing.T) {
	tests := []struct {
		name    string
		service *fakeLinuxService
		newErr  error
		wantErr bool
	}{
		{name: "success", service: &fakeLinuxService{}},
		{name: "stop error ignored", service: &fakeLinuxService{stopErr: errors.New("stop")}},
		{name: "create error", newErr: errors.New("create"), wantErr: true},
		{name: "uninstall error", service: &fakeLinuxService{uninstallErr: errors.New("uninstall")}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withLinuxServiceDeps(t)
			newLinuxService = func(exePath string, program kservice.Interface) (linuxService, error) {
				if exePath != "" {
					t.Fatalf("unexpected executable path: %s", exePath)
				}
				if tt.newErr != nil {
					return nil, tt.newErr
				}
				return tt.service, nil
			}

			err := Uninstall()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
			if tt.service != nil && tt.newErr == nil && !tt.service.uninstalled {
				t.Fatal("service should be uninstalled")
			}
			if tt.service != nil && tt.newErr == nil && !tt.service.stopped {
				t.Fatal("service should be stopped before uninstall")
			}
		})
	}
}

func TestRunServiceLinux(t *testing.T) {
	withLinuxServiceDeps(t)
	fake := &fakeLinuxService{runErr: errors.New("run")}
	newLinuxService = func(exePath string, program kservice.Interface) (linuxService, error) {
		if exePath != "" {
			t.Fatalf("unexpected executable path: %s", exePath)
		}
		if program == nil {
			t.Fatal("expected service program")
		}
		return fake, nil
	}

	if err := RunService(); err == nil {
		t.Fatal("expected run error")
	}
	if !fake.ran {
		t.Fatal("service should run")
	}
}

func TestLinuxServiceConfig(t *testing.T) {
	cfg := linuxServiceConfig("/opt/mdns2hosts")
	if cfg.Name != svcName {
		t.Fatalf("unexpected name: %s", cfg.Name)
	}
	if cfg.Executable != "/opt/mdns2hosts" {
		t.Fatalf("unexpected executable: %s", cfg.Executable)
	}
	if len(cfg.Arguments) != 1 || cfg.Arguments[0] != "service-run" {
		t.Fatalf("unexpected arguments: %v", cfg.Arguments)
	}
}

func TestLinuxProgramStartStop(t *testing.T) {
	withLinuxServiceDeps(t)
	readServiceConfig = func() ([]string, time.Duration, error) {
		return []string{"foo.local"}, time.Hour, nil
	}
	tickerC := make(chan time.Time)
	newServiceTicker = func(time.Duration) serviceTicker {
		return serviceTicker{C: tickerC, Stop: func() {}}
	}

	program := &linuxProgram{}
	if err := program.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := program.Stop(nil); err != nil {
		t.Fatal(err)
	}
}
