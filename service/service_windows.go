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

// Install registers the Windows service.
func Install(names []string, interval, exePath string) error {
	m, err := mgr.Connect()
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

	// Build the binary path with arguments to run as a service
	binPath := fmt.Sprintf(`"%s" service-run`, exePath)

	s, err = m.CreateService(svcName, binPath, mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: svcDisplay,
		Description: "Periodically syncs mDNS names to the hosts file",
	})
	if err != nil {
		return fmt.Errorf("cannot create service: %w", err)
	}
	s.Close()
	return nil
}

// Uninstall stops and removes the Windows service.
func Uninstall() error {
	m, err := mgr.Connect()
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
		time.Sleep(2 * time.Second)
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("cannot delete service: %w", err)
	}
	return nil
}

// RunService is the entry point when the binary is started by SCM.
func RunService() error {
	return svc.Run(svcName, &handler{})
}
