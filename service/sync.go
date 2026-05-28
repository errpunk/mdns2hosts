package service

import (
	"fmt"
	"net"
	"os"

	"github.com/liutao/mdns2hosts/hosts"
	"github.com/liutao/mdns2hosts/mdns"
)

type serviceLogger interface {
	Info(msg string)
	Warning(msg string)
	Error(msg string)
}

var (
	resolveAllNames = mdns.ResolveAll
	ensureHostsFile = hosts.EnsureBlock
	readHostsFile   = hosts.ReadHosts
	writeHostsFile  = hosts.WriteHosts
)

func syncConfiguredNames(names []string, logger serviceLogger) {
	results, errs := resolveAllNames(names)
	for _, err := range errs {
		logWarning(logger, err.Error())
	}

	if len(results) == 0 {
		return
	}

	if err := ensureHostsFile(); err != nil {
		logError(logger, fmt.Sprintf("ensure hosts file: %v", err))
		return
	}

	before, _, after, err := readHostsFile()
	if err != nil {
		logError(logger, fmt.Sprintf("read hosts: %v", err))
		return
	}

	if err := writeHostsFile(before, results, after); err != nil {
		logError(logger, fmt.Sprintf("write hosts: %v", err))
	}
}

func logInfo(logger serviceLogger, msg string) {
	if logger != nil {
		logger.Info(msg)
		return
	}
	fmt.Fprintln(os.Stderr, msg)
}

func logWarning(logger serviceLogger, msg string) {
	if logger != nil {
		logger.Warning(msg)
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
}

func logError(logger serviceLogger, msg string) {
	if logger != nil {
		logger.Error(msg)
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}

func cloneIPMap(in map[string]net.IP) map[string]net.IP {
	out := make(map[string]net.IP, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
