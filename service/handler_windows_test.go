//go:build windows

package service

import (
	"net"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

func TestHandlerExecuteWindowsStop(t *testing.T) {
	withRunnerDeps(t)
	withSyncDeps(t)

	changes := make(chan svc.ChangeRequest)
	statuses := make(chan svc.Status, 4)
	ticks := make(chan time.Time)

	readServiceConfig = func() ([]string, time.Duration, error) {
		return []string{"foo.local"}, time.Minute, nil
	}
	newServiceTicker = func(time.Duration) serviceTicker {
		return serviceTicker{C: ticks, Stop: func() {}}
	}
	resolveAllNames = func([]string) (map[string]net.IP, []error) {
		return nil, nil
	}

	done := make(chan struct {
		svcSpecificEC bool
		exitCode      uint32
	})
	go func() {
		svcSpecificEC, exitCode := (&handler{}).Execute(nil, changes, statuses)
		done <- struct {
			svcSpecificEC bool
			exitCode      uint32
		}{svcSpecificEC: svcSpecificEC, exitCode: exitCode}
	}()

	requireWindowsStatus(t, statuses, svc.StartPending)
	running := requireWindowsStatus(t, statuses, svc.Running)
	if running.Accepts != svc.AcceptStop|svc.AcceptShutdown {
		t.Fatalf("unexpected accepts: %v", running.Accepts)
	}

	changes <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: svc.Status{State: svc.Running, Accepts: svc.AcceptStop}}
	interrogated := requireWindowsStatus(t, statuses, svc.Running)
	if interrogated.Accepts != svc.AcceptStop {
		t.Fatalf("unexpected interrogate status: %+v", interrogated)
	}

	changes <- svc.ChangeRequest{Cmd: svc.Stop}
	requireWindowsStatus(t, statuses, svc.StopPending)
	result := <-done
	if result.svcSpecificEC || result.exitCode != 0 {
		t.Fatalf("unexpected exit: %+v", result)
	}
	close(changes)
}

func TestWindowsStatusMappings(t *testing.T) {
	status := toServiceStatus(svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown,
	})
	if status.State != serviceRunning || status.Accepts != serviceStop|serviceShutdown {
		t.Fatalf("unexpected service status: %+v", status)
	}

	windowsStatus := toWindowsStatus(serviceStatus{
		State:   serviceStopPending,
		Accepts: serviceStop | serviceShutdown,
	})
	if windowsStatus.State != svc.StopPending || windowsStatus.Accepts != svc.AcceptStop|svc.AcceptShutdown {
		t.Fatalf("unexpected windows status: %+v", windowsStatus)
	}

	if toServiceCommand(svc.Pause) != 0 {
		t.Fatal("unknown Windows command should map to zero")
	}
	if toWindowsState(0) != svc.Stopped {
		t.Fatal("unknown service state should map to stopped")
	}
}

func requireWindowsStatus(t *testing.T, statuses <-chan svc.Status, state svc.State) svc.Status {
	t.Helper()
	select {
	case status := <-statuses:
		if status.State != state {
			t.Fatalf("expected state %v, got %+v", state, status)
		}
		return status
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for state %v", state)
		return svc.Status{}
	}
}
