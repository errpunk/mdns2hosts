//go:build linux

package service

import (
	"fmt"

	kservice "github.com/kardianos/service"
)

type linuxService interface {
	Install() error
	Stop() error
	Uninstall() error
	Run() error
}

type linuxProgram struct {
	done      chan struct{}
	changeReq chan serviceChange
}

type kardianosService struct {
	service kservice.Service
}

func (k kardianosService) Install() error {
	return k.service.Install()
}

func (k kardianosService) Stop() error {
	return k.service.Stop()
}

func (k kardianosService) Uninstall() error {
	return k.service.Uninstall()
}

func (k kardianosService) Run() error {
	return k.service.Run()
}

var newLinuxService = func(exePath string, program kservice.Interface) (linuxService, error) {
	s, err := kservice.New(program, linuxServiceConfig(exePath))
	if err != nil {
		return nil, err
	}
	return kardianosService{service: s}, nil
}

// Install registers the Linux service.
func Install(names []string, interval, exePath string) error {
	if err := SaveConfig(names, interval); err != nil {
		return fmt.Errorf("cannot save config: %w", err)
	}

	s, err := newLinuxService(exePath, &linuxProgram{})
	if err != nil {
		return fmt.Errorf("cannot create service: %w", err)
	}
	if err := s.Install(); err != nil {
		return fmt.Errorf("cannot install service: %w", err)
	}
	return nil
}

// Uninstall removes the Linux service.
func Uninstall() error {
	s, err := newLinuxService("", &linuxProgram{})
	if err != nil {
		return fmt.Errorf("cannot create service: %w", err)
	}
	_ = s.Stop()
	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("cannot uninstall service: %w", err)
	}
	return nil
}

// RunService is the entry point when the binary is started by the service manager.
func RunService() error {
	program := &linuxProgram{}
	s, err := newLinuxService("", program)
	if err != nil {
		return fmt.Errorf("cannot create service: %w", err)
	}
	return s.Run()
}

func linuxServiceConfig(exePath string) *kservice.Config {
	return &kservice.Config{
		Name:        svcName,
		DisplayName: svcDisplay,
		Description: "Periodically syncs mDNS names to the hosts file",
		Executable:  exePath,
		Arguments:   []string{"service-run"},
	}
}

func (p *linuxProgram) Start(kservice.Service) error {
	p.done = make(chan struct{})
	p.changeReq = make(chan serviceChange)
	statusCh := make(chan serviceStatus)

	go func() {
		for range statusCh {
		}
	}()

	go func() {
		defer close(p.done)
		runSyncService(p.changeReq, statusCh, nil)
	}()
	return nil
}

func (p *linuxProgram) Stop(kservice.Service) error {
	if p.done == nil || p.changeReq == nil {
		return nil
	}

	select {
	case <-p.done:
		return nil
	case p.changeReq <- serviceChange{Cmd: serviceStop}:
	default:
		p.changeReq <- serviceChange{Cmd: serviceStop}
	}

	<-p.done
	return nil
}
