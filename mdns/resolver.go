package mdns

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const queryTimeout = 3 * time.Second

var mdnsV4 = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// Resolve queries an mDNS name and returns its IPv4 address via multicast DNS.
func Resolve(name string) (net.IP, error) {
	host := dns.Fqdn(name)

	// Try each IPv4 network interface
	ifaces, err := net.Interfaces()
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
	conn, err := net.ListenMulticastUDP("udp4", iface, mdnsV4)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	msg := new(dns.Msg)
	msg.Id = 0 // mDNS queries MUST have ID 0
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

		reply := new(dns.Msg)
		if err := reply.Unpack(buf[:n]); err != nil {
			continue
		}

		// mDNS responses should have QR bit set and be a response
		if !reply.Response {
			continue
		}

		for _, rr := range reply.Answer {
			if a, ok := rr.(*dns.A); ok {
				if strings.EqualFold(a.Hdr.Name, host) {
					return a.A, nil
				}
			}
		}
	}
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
