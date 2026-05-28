package service

import (
	"fmt"
	"time"
)

type serviceCommand int

const (
	serviceInterrogate serviceCommand = 1 << iota
	serviceStop
	serviceShutdown
)

type serviceState int

const (
	serviceStartPending serviceState = iota + 1
	serviceRunning
	serviceStopPending
)

type serviceStatus struct {
	State   serviceState
	Accepts serviceCommand
}

type serviceChange struct {
	Cmd           serviceCommand
	CurrentStatus serviceStatus
}

type serviceTicker struct {
	C    <-chan time.Time
	Stop func()
}

var (
	readServiceConfig = ReadConfig
	newServiceTicker  = func(interval time.Duration) serviceTicker {
		ticker := time.NewTicker(interval)
		return serviceTicker{C: ticker.C, Stop: ticker.Stop}
	}
)

func runSyncService(changeReq <-chan serviceChange, statusCh chan serviceStatus, logger serviceLogger) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = serviceStop | serviceShutdown

	defer close(statusCh)
	statusCh <- serviceStatus{State: serviceStartPending}

	names, interval, err := readServiceConfig()
	if err != nil {
		logError(logger, fmt.Sprintf("failed to read config: %v", err))
		return true, 1
	}

	logInfo(logger, fmt.Sprintf("mdns2hosts service started, syncing %v every %s", names, interval))
	statusCh <- serviceStatus{State: serviceRunning, Accepts: cmdsAccepted}

	ticker := newServiceTicker(interval)
	defer ticker.Stop()

	syncConfiguredNames(names, logger)

	for {
		select {
		case c := <-changeReq:
			switch c.Cmd {
			case serviceInterrogate:
				statusCh <- c.CurrentStatus
			case serviceStop, serviceShutdown:
				logInfo(logger, "mdns2hosts service stopping")
				statusCh <- serviceStatus{State: serviceStopPending}
				return false, 0
			default:
			}
		case <-ticker.C:
			syncConfiguredNames(names, logger)
		}
	}
}
