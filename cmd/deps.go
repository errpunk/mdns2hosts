package cmd

import (
	"net"

	"github.com/liutao/mdns2hosts/hosts"
	"github.com/liutao/mdns2hosts/service"
)

var (
	cleanHostsFile   = hosts.CleanBlock
	serviceExePath   = service.ExePath
	installService   = service.Install
	uninstallService = service.Uninstall
)

type hostsSnapshot struct {
	before  []string
	managed map[string]net.IP
	after   []string
}
