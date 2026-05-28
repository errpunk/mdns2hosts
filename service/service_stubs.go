//go:build !windows

package service

import "fmt"

func Install(names []string, interval, exePath string) error {
	return fmt.Errorf("service installation is only supported on Windows")
}

func Uninstall() error {
	return fmt.Errorf("service uninstallation is only supported on Windows")
}

func RunService() error {
	return fmt.Errorf("service mode is only supported on Windows")
}
