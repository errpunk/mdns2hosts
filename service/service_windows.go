//go:build windows

package service

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const svcName = "mdns2hosts"
const svcDisplay = "mDNS to Hosts Sync Service"

type serviceManager interface {
	Disconnect() error
	OpenService(name string) (managedService, error)
	CreateService(name, exepath string, c mgr.Config, args ...string) (managedService, error)
}

type managedService interface {
	Close() error
	Query() (svc.Status, error)
	Control(c svc.Cmd) (svc.Status, error)
	Delete() error
}

type realServiceManager struct {
	manager *mgr.Mgr
}

func (r realServiceManager) Disconnect() error {
	return r.manager.Disconnect()
}

func (r realServiceManager) OpenService(name string) (managedService, error) {
	return r.manager.OpenService(name)
}

func (r realServiceManager) CreateService(name, exepath string, c mgr.Config, args ...string) (managedService, error) {
	return r.manager.CreateService(name, exepath, c, args...)
}

var (
	connectServiceManager = func() (serviceManager, error) {
		m, err := mgr.Connect()
		if err != nil {
			return nil, err
		}
		return realServiceManager{manager: m}, nil
	}
	runWindowsService = svc.Run
	serviceStopSleep  = time.Sleep
)

// Install registers the Windows service.
func Install(names []string, interval, exePath string) error {
	m, err := connectServiceManager()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager (run as Admin?): %w", err)
	}
	defer m.Disconnect()

	// Check if already installed
	s, err := m.OpenService(svcName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s is already installed", svcName)
	}

	// Save config for the service to read at startup
	if err := SaveConfig(names, interval); err != nil {
		return fmt.Errorf("cannot save config: %w", err)
	}

	s, err = m.CreateService(svcName, exePath, mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: svcDisplay,
		Description: "Periodically syncs mDNS names to the hosts file",
	}, "service-run")
	if err != nil {
		return fmt.Errorf("cannot create service: %w", err)
	}
	s.Close()
	return nil
}

// Uninstall stops and removes the Windows service.
func Uninstall() error {
	m, err := connectServiceManager()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(svcName)
	if err != nil {
		return fmt.Errorf("service %s is not installed: %w", svcName, err)
	}
	defer s.Close()

	// Try to stop the service
	if status, err := s.Query(); err == nil && status.State != svc.Stopped {
		_, err = s.Control(svc.Stop)
		if err != nil {
			// Not fatal - proceed with deletion
		}
		// Wait a bit for stop
		serviceStopSleep(2 * time.Second)
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("cannot delete service: %w", err)
	}
	return nil
}

// RunService is the entry point when the binary is started by SCM.
func RunService() error {
	return runWindowsService(svcName, &handler{})
}
