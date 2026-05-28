//go:build windows

package service

import (
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

	return runSyncService(
		mapServiceChanges(changeReq),
		mapServiceStatuses(statusCh),
		logger,
	)
}

func mapServiceChanges(in <-chan svc.ChangeRequest) <-chan serviceChange {
	out := make(chan serviceChange)
	go func() {
		for c := range in {
			out <- serviceChange{
				Cmd:           toServiceCommand(c.Cmd),
				CurrentStatus: toServiceStatus(c.CurrentStatus),
			}
		}
	}()
	return out
}

func mapServiceStatuses(out chan<- svc.Status) chan serviceStatus {
	in := make(chan serviceStatus)
	go func() {
		for status := range in {
			out <- toWindowsStatus(status)
		}
	}()
	return in
}

func toServiceCommand(cmd svc.Cmd) serviceCommand {
	switch cmd {
	case svc.Interrogate:
		return serviceInterrogate
	case svc.Stop:
		return serviceStop
	case svc.Shutdown:
		return serviceShutdown
	default:
		return 0
	}
}

func toServiceStatus(status svc.Status) serviceStatus {
	return serviceStatus{
		State:   toServiceState(status.State),
		Accepts: toServiceAccepts(status.Accepts),
	}
}

func toServiceAccepts(accepts svc.Accepted) serviceCommand {
	var out serviceCommand
	if accepts&svc.AcceptStop != 0 {
		out |= serviceStop
	}
	if accepts&svc.AcceptShutdown != 0 {
		out |= serviceShutdown
	}
	return out
}

func toServiceState(state svc.State) serviceState {
	switch state {
	case svc.StartPending:
		return serviceStartPending
	case svc.Running:
		return serviceRunning
	case svc.StopPending:
		return serviceStopPending
	default:
		return 0
	}
}

func toWindowsStatus(status serviceStatus) svc.Status {
	return svc.Status{
		State:   toWindowsState(status.State),
		Accepts: toWindowsAccepts(status.Accepts),
	}
}

func toWindowsState(state serviceState) svc.State {
	switch state {
	case serviceStartPending:
		return svc.StartPending
	case serviceRunning:
		return svc.Running
	case serviceStopPending:
		return svc.StopPending
	default:
		return svc.Stopped
	}
}

func toWindowsAccepts(accepts serviceCommand) svc.Accepted {
	var out svc.Accepted
	if accepts&serviceStop != 0 {
		out |= svc.AcceptStop
	}
	if accepts&serviceShutdown != 0 {
		out |= svc.AcceptShutdown
	}
	return out
}
