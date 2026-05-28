package service

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func withRunnerDeps(t *testing.T) {
	t.Helper()
	origReadConfig := readServiceConfig
	origNewTicker := newServiceTicker
	t.Cleanup(func() {
		readServiceConfig = origReadConfig
		newServiceTicker = origNewTicker
	})
}

func TestRunSyncServiceStopsAfterInitialSync(t *testing.T) {
	withRunnerDeps(t)
	withSyncDeps(t)

	logger := &recordingLogger{}
	changes := make(chan serviceChange)
	statuses := make(chan serviceStatus, 4)
	ticks := make(chan time.Time)
	var syncs atomic.Int32

	readServiceConfig = func() ([]string, time.Duration, error) {
		return []string{"foo.local"}, time.Minute, nil
	}
	newServiceTicker = func(time.Duration) serviceTicker {
		return serviceTicker{C: ticks, Stop: func() {}}
	}
	resolveAllNames = func(names []string) (map[string]net.IP, []error) {
		syncs.Add(1)
		return nil, nil
	}

	done := make(chan struct {
		svcSpecificEC bool
		exitCode      uint32
	})
	go func() {
		svcSpecificEC, exitCode := runSyncService(changes, statuses, logger)
		done <- struct {
			svcSpecificEC bool
			exitCode      uint32
		}{svcSpecificEC: svcSpecificEC, exitCode: exitCode}
	}()

	requireServiceStatus(t, statuses, serviceStartPending)
	running := requireServiceStatus(t, statuses, serviceRunning)
	if running.Accepts != serviceStop|serviceShutdown {
		t.Fatalf("unexpected accepts: %v", running.Accepts)
	}
	waitForSyncs(t, &syncs, 1)

	changes <- serviceChange{Cmd: serviceStop}
	stopping := requireServiceStatus(t, statuses, serviceStopPending)
	if stopping.State != serviceStopPending {
		t.Fatalf("unexpected stop status: %+v", stopping)
	}

	result := <-done
	if result.svcSpecificEC || result.exitCode != 0 {
		t.Fatalf("unexpected exit: %+v", result)
	}
	if len(logger.info) != 2 {
		t.Fatalf("expected start and stop logs, got %v", logger.info)
	}
}

func TestRunSyncServiceConfigError(t *testing.T) {
	withRunnerDeps(t)

	logger := &recordingLogger{}
	statuses := make(chan serviceStatus, 1)
	readServiceConfig = func() ([]string, time.Duration, error) {
		return nil, 0, errors.New("config")
	}

	svcSpecificEC, exitCode := runSyncService(make(chan serviceChange), statuses, logger)

	if !svcSpecificEC || exitCode != 1 {
		t.Fatalf("unexpected exit: %v %d", svcSpecificEC, exitCode)
	}
	requireServiceStatus(t, statuses, serviceStartPending)
	if len(logger.errors) != 1 {
		t.Fatalf("expected config error log, got %v", logger.errors)
	}
}

func TestRunSyncServiceInterrogateAndTick(t *testing.T) {
	withRunnerDeps(t)
	withSyncDeps(t)

	changes := make(chan serviceChange)
	statuses := make(chan serviceStatus, 4)
	ticks := make(chan time.Time)
	var syncs atomic.Int32

	readServiceConfig = func() ([]string, time.Duration, error) {
		return []string{"foo.local"}, time.Minute, nil
	}
	newServiceTicker = func(time.Duration) serviceTicker {
		return serviceTicker{C: ticks, Stop: func() {}}
	}
	resolveAllNames = func([]string) (map[string]net.IP, []error) {
		syncs.Add(1)
		return nil, nil
	}

	done := make(chan struct{})
	go func() {
		runSyncService(changes, statuses, nil)
		close(done)
	}()

	requireServiceStatus(t, statuses, serviceStartPending)
	requireServiceStatus(t, statuses, serviceRunning)
	waitForSyncs(t, &syncs, 1)

	changes <- serviceChange{
		Cmd:           serviceInterrogate,
		CurrentStatus: serviceStatus{State: serviceRunning, Accepts: serviceStop},
	}
	status := requireServiceStatus(t, statuses, serviceRunning)
	if status.Accepts != serviceStop {
		t.Fatalf("unexpected interrogate status: %+v", status)
	}

	ticks <- time.Now()
	waitForSyncs(t, &syncs, 2)

	changes <- serviceChange{Cmd: serviceShutdown}
	requireServiceStatus(t, statuses, serviceStopPending)
	<-done
}

func requireServiceStatus(t *testing.T, statuses <-chan serviceStatus, state serviceState) serviceStatus {
	t.Helper()
	select {
	case status := <-statuses:
		if status.State != state {
			t.Fatalf("expected state %v, got %+v", state, status)
		}
		return status
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for state %v", state)
		return serviceStatus{}
	}
}

func waitForSyncs(t *testing.T, syncs *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		got := syncs.Load()
		if got >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d syncs, got %d", want, got)
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
