//go:build windows

package service

import (
	"fmt"
	"os"
	"time"

	"github.com/liutao/mdns2hosts/hosts"
	"github.com/liutao/mdns2hosts/mdns"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

type handler struct{}

func (h *handler) Execute(args []string, changeReq <-chan svc.ChangeRequest, statusCh chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	elog, err := eventlog.Open("mdns2hosts")
	if err != nil {
		elog = nil
	}

	statusCh <- svc.Status{State: svc.StartPending}

	names, interval, err := ReadConfig()
	if err != nil {
		if elog != nil {
			elog.Error(1, fmt.Sprintf("failed to read config: %v", err))
		}
		return true, 1
	}

	if elog != nil {
		elog.Info(1, fmt.Sprintf("mdns2hosts service started, syncing %v every %s", names, interval))
	}

	statusCh <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do an initial sync
	doServiceSync(names, elog)

	for {
		select {
		case c := <-changeReq:
			switch c.Cmd {
			case svc.Interrogate:
				statusCh <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				if elog != nil {
					elog.Info(1, "mdns2hosts service stopping")
				}
				statusCh <- svc.Status{State: svc.StopPending}
				return false, 0
			default:
			}
		case <-ticker.C:
			doServiceSync(names, elog)
		}
	}
}

func doServiceSync(names []string, elog *eventlog.Log) {
	results, errs := mdns.ResolveAll(names)
	for _, e := range errs {
		if elog != nil {
			elog.Warning(1, e.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
		}
	}

	if len(results) == 0 {
		return
	}

	if err := hosts.EnsureBlock(); err != nil {
		if elog != nil {
			elog.Error(2, fmt.Sprintf("ensure block: %v", err))
		}
		return
	}

	before, _, after, err := hosts.ReadHosts()
	if err != nil {
		if elog != nil {
			elog.Error(3, fmt.Sprintf("read hosts: %v", err))
		}
		return
	}

	if err := hosts.WriteHosts(before, results, after); err != nil {
		if elog != nil {
			elog.Error(4, fmt.Sprintf("write hosts: %v", err))
		}
	}
}
