package mdns

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const queryTimeout = 3 * time.Second

var mdnsV4 = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// Test hooks
var (
	interfaceLister    = net.Interfaces
	listenMulticastUDP = defaultListenMulticastUDP
	newDNSMessage      = func() *dns.Msg { return new(dns.Msg) }
)

type multicastPacketConn interface {
	io.Closer
	WriteTo([]byte, net.Addr) (int, error)
	SetReadDeadline(time.Time) error
	ReadFrom([]byte) (int, net.Addr, error)
}

func defaultListenMulticastUDP(network string, iface *net.Interface, group *net.UDPAddr) (multicastPacketConn, error) {
	return net.ListenMulticastUDP(network, iface, group)
}

// Resolve queries an mDNS name and returns its IPv4 address via multicast DNS.
func Resolve(name string) (net.IP, error) {
	host := dns.Fqdn(name)

	ifaces, err := interfaceLister()
	if err != nil {
		return nil, fmt.Errorf("cannot list interfaces: %w", err)
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		ip, err := resolveOnInterface(host, &iface)
		if err == nil {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no mDNS response for %s on any interface", name)
}

func resolveOnInterface(host string, iface *net.Interface) (net.IP, error) {
	conn, err := listenMulticastUDP("udp4", iface, mdnsV4)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	msg := newDNSMessage()
	msg.Id = 0
	msg.RecursionDesired = false
	msg.SetQuestion(host, dns.TypeA)

	data, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	if _, err := conn.WriteTo(data, mdnsV4); err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(queryTimeout))

	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return nil, err
		}

		ip, found, skip := parseMDNSResponse(buf[:n], host)
		if found {
			return ip, nil
		}
		if skip {
			continue
		}
	}
}

// parseMDNSResponse attempts to extract an A record from a raw DNS response.
// Returns: (ip, found, skip) — skip=true means the caller should continue reading.
func parseMDNSResponse(data []byte, host string) (net.IP, bool, bool) {
	reply := new(dns.Msg)
	if err := reply.Unpack(data); err != nil {
		return nil, false, true // skip non-DNS data
	}

	if !reply.Response {
		return nil, false, true // skip queries
	}

	for _, rr := range reply.Answer {
		if a, ok := rr.(*dns.A); ok {
			if strings.EqualFold(a.Hdr.Name, host) {
				return a.A, true, false
			}
		}
	}

	return nil, false, false // valid response but no matching A record
}

// ResolveAll resolves multiple mDNS names concurrently.
func ResolveAll(names []string) (map[string]net.IP, []error) {
	type result struct {
		name string
		ip   net.IP
		err  error
	}

	ch := make(chan result, len(names))
	for _, name := range names {
		go func(n string) {
			ip, err := Resolve(n)
			ch <- result{name: n, ip: ip, err: err}
		}(name)
	}

	results := make(map[string]net.IP, len(names))
	var errs []error
	for range len(names) {
		r := <-ch
		if r.err != nil {
			errs = append(errs, r.err)
		} else {
			results[r.name] = r.ip
		}
	}
	return results, errs
}
