//go:build windows

package service

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

type handler struct{}

type eventLogger struct {
	log *eventlog.Log
}

func (e eventLogger) Info(msg string) {
	_ = e.log.Info(1, msg)
}

func (e eventLogger) Warning(msg string) {
	_ = e.log.Warning(1, msg)
}

func (e eventLogger) Error(msg string) {
	_ = e.log.Error(1, msg)
}

func (h *handler) Execute(args []string, changeReq <-chan svc.ChangeRequest, statusCh chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	elog, err := eventlog.Open("mdns2hosts")
	if err != nil {
		elog = nil
	}
	if elog != nil {
		defer elog.Close()
	}

	var logger serviceLogger
	if elog != nil {
		logger = eventLogger{log: elog}
	}

	statusCh <- svc.Status{State: svc.StartPending}

	names, interval, err := ReadConfig()
	if err != nil {
		logError(logger, fmt.Sprintf("failed to read config: %v", err))
		return true, 1
	}

	logInfo(logger, fmt.Sprintf("mdns2hosts service started, syncing %v every %s", names, interval))

	statusCh <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do an initial sync
	syncConfiguredNames(names, logger)

	for {
		select {
		case c := <-changeReq:
			switch c.Cmd {
			case svc.Interrogate:
				statusCh <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				logInfo(logger, "mdns2hosts service stopping")
				statusCh <- svc.Status{State: svc.StopPending}
				return false, 0
			default:
			}
		case <-ticker.C:
			syncConfiguredNames(names, logger)
		}
	}
}
