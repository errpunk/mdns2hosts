//go:build windows

package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type fakeManager struct {
	openService   managedService
	openErr       error
	createService managedService
	createErr     error
	disconnected  bool
	createName    string
	createPath    string
	createArgs    []string
}

func (f *fakeManager) Disconnect() error {
	f.disconnected = true
	return nil
}

func (f *fakeManager) OpenService(string) (managedService, error) {
	return f.openService, f.openErr
}

func (f *fakeManager) CreateService(name, exepath string, _ mgr.Config, args ...string) (managedService, error) {
	f.createName = name
	f.createPath = exepath
	f.createArgs = append([]string(nil), args...)
	return f.createService, f.createErr
}

type fakeService struct {
	status     svc.Status
	queryErr   error
	controlErr error
	deleteErr  error
	closed     bool
	deleted    bool
	controlled bool
}

func (f *fakeService) Close() error {
	f.closed = true
	return nil
}

func (f *fakeService) Query() (svc.Status, error) {
	return f.status, f.queryErr
}

func (f *fakeService) Control(svc.Cmd) (svc.Status, error) {
	f.controlled = true
	return svc.Status{State: svc.StopPending}, f.controlErr
}

func (f *fakeService) Delete() error {
	f.deleted = true
	return f.deleteErr
}

func withWindowsServiceDeps(t *testing.T) {
	t.Helper()
	origConnect := connectServiceManager
	origRun := runWindowsService
	origSleep := serviceStopSleep
	origConfigPath := configPathForTest
	t.Cleanup(func() {
		connectServiceManager = origConnect
		runWindowsService = origRun
		serviceStopSleep = origSleep
		configPathForTest = origConfigPath
	})
	serviceStopSleep = func(time.Duration) {}
	configPathForTest = filepath.Join(t.TempDir(), "mdns2hosts.conf")
}

func TestInstallWindowsSuccess(t *testing.T) {
	withWindowsServiceDeps(t)
	created := &fakeService{}
	manager := &fakeManager{
		openErr:       errors.New("not installed"),
		createService: created,
	}
	connectServiceManager = func() (serviceManager, error) {
		return manager, nil
	}

	if err := Install([]string{"foo.local"}, "10s", `C:\tools\mdns2hosts.exe`); err != nil {
		t.Fatal(err)
	}
	if !created.closed {
		t.Fatal("created service should be closed")
	}
	if !manager.disconnected {
		t.Fatal("manager should be disconnected")
	}
	if manager.createName != svcName {
		t.Fatalf("unexpected service name: %s", manager.createName)
	}
	if manager.createPath != `C:\tools\mdns2hosts.exe` {
		t.Fatalf("unexpected service path: %s", manager.createPath)
	}
	if len(manager.createArgs) != 1 || manager.createArgs[0] != "service-run" {
		t.Fatalf("unexpected service args: %v", manager.createArgs)
	}

	names, interval, err := ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if names[0] != "foo.local" || interval != 10*time.Second {
		t.Fatalf("config not saved: %v %v", names, interval)
	}
}

func TestInstallWindowsErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "connect",
			setup: func() {
				connectServiceManager = func() (serviceManager, error) {
					return nil, errors.New("connect")
				}
			},
		},
		{
			name: "already installed",
			setup: func() {
				connectServiceManager = func() (serviceManager, error) {
					return &fakeManager{openService: &fakeService{}}, nil
				}
			},
		},
		{
			name: "create",
			setup: func() {
				connectServiceManager = func() (serviceManager, error) {
					return &fakeManager{
						openErr:   errors.New("not installed"),
						createErr: errors.New("create"),
					}, nil
				}
			},
		},
		{
			name: "save config",
			setup: func() {
				configPathForTest = t.TempDir()
				connectServiceManager = func() (serviceManager, error) {
					return &fakeManager{openErr: errors.New("not installed")}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWindowsServiceDeps(t)
			tt.setup()
			if err := Install([]string{"foo.local"}, "10s", `C:\x.exe`); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUninstallWindowsSuccess(t *testing.T) {
	withWindowsServiceDeps(t)
	service := &fakeService{status: svc.Status{State: svc.Running}}
	connectServiceManager = func() (serviceManager, error) {
		return &fakeManager{openService: service}, nil
	}

	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	if !service.controlled || !service.deleted || !service.closed {
		t.Fatalf("unexpected service calls: %+v", service)
	}
}

func TestUninstallWindowsIgnoresStopErrors(t *testing.T) {
	tests := []struct {
		name    string
		service *fakeService
	}{
		{
			name:    "query",
			service: &fakeService{queryErr: errors.New("query")},
		},
		{
			name:    "control",
			service: &fakeService{status: svc.Status{State: svc.Running}, controlErr: errors.New("control")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWindowsServiceDeps(t)
			connectServiceManager = func() (serviceManager, error) {
				return &fakeManager{openService: tt.service}, nil
			}

			if err := Uninstall(); err != nil {
				t.Fatal(err)
			}
			if !tt.service.deleted {
				t.Fatal("service should still be deleted")
			}
		})
	}
}

func TestUninstallWindowsErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "connect",
			setup: func() {
				connectServiceManager = func() (serviceManager, error) {
					return nil, errors.New("connect")
				}
			},
		},
		{
			name: "open",
			setup: func() {
				connectServiceManager = func() (serviceManager, error) {
					return &fakeManager{openErr: errors.New("open")}, nil
				}
			},
		},
		{
			name: "delete",
			setup: func() {
				connectServiceManager = func() (serviceManager, error) {
					return &fakeManager{openService: &fakeService{status: svc.Status{State: svc.Stopped}, deleteErr: errors.New("delete")}}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWindowsServiceDeps(t)
			tt.setup()
			if err := Uninstall(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRunServiceWindows(t *testing.T) {
	withWindowsServiceDeps(t)
	runWindowsService = func(name string, h svc.Handler) error {
		if name != svcName || h == nil {
			t.Fatalf("unexpected run args: %s %v", name, h)
		}
		return errors.New("run")
	}
	if err := RunService(); err == nil {
		t.Fatal("expected run error")
	}
}
