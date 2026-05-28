package mdns

import (
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type fakePacketConn struct {
	writeErr    error
	readPackets [][]byte
	readErr     error
	wrote       bool
	closed      bool
}

func (f *fakePacketConn) Close() error {
	f.closed = true
	return nil
}

func (f *fakePacketConn) WriteTo([]byte, net.Addr) (int, error) {
	f.wrote = true
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return 1, nil
}

func (f *fakePacketConn) SetReadDeadline(time.Time) error {
	return nil
}

func (f *fakePacketConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	if len(f.readPackets) > 0 {
		p := f.readPackets[0]
		f.readPackets = f.readPackets[1:]
		copy(buf, p)
		return len(p), mdnsV4, nil
	}
	if f.readErr != nil {
		return 0, nil, f.readErr
	}
	return 0, nil, io.EOF
}

func withResolverHooks(t *testing.T) {
	t.Helper()
	origLister := interfaceLister
	origListen := listenMulticastUDP
	origMsg := newDNSMessage
	t.Cleanup(func() {
		interfaceLister = origLister
		listenMulticastUDP = origListen
		newDNSMessage = origMsg
	})
}

func TestQueryTimeout(t *testing.T) {
	if queryTimeout != 3*time.Second {
		t.Errorf("expected 3s timeout, got %v", queryTimeout)
	}
}

func TestResolve_InterfaceListError(t *testing.T) {
	orig := interfaceLister
	interfaceLister = func() ([]net.Interface, error) {
		return nil, fmt.Errorf("simulated interface error")
	}
	defer func() { interfaceLister = orig }()

	_, err := Resolve("test.local")
	if err == nil {
		t.Error("expected error when interface listing fails")
	}
}

func TestResolve_NoMulticastInterfaces(t *testing.T) {
	orig := interfaceLister
	interfaceLister = func() ([]net.Interface, error) {
		return []net.Interface{
			{Name: "lo", Flags: net.FlagUp},
			{Name: "eth0", Flags: 0},
		}, nil
	}
	defer func() { interfaceLister = orig }()

	_, err := Resolve("test.local")
	if err == nil {
		t.Error("expected error when no multicast interfaces available")
	}
}

func TestResolve_NoSuchHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	_, err := Resolve("nonexistent-mdns-test-host.invalid")
	if err == nil {
		t.Error("expected error for nonexistent mDNS name")
	}
}

func TestResolve_MissingLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	_, err := Resolve("no-such-host-abc123.local")
	if err == nil {
		t.Error("expected timeout/error for unresolvable name")
	}
}

func TestResolveAll_EmptyList(t *testing.T) {
	results, errs := ResolveAll(nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestResolveAll_SingleInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	results, errs := ResolveAll([]string{"no-such-host-that-does-not-exist.local"})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestResolveAll_MultipleInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	names := []string{"a.invalid", "b.invalid", "c.invalid"}
	results, errs := ResolveAll(names)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errs))
	}
}

func TestResolveAll_WithInterfaceError(t *testing.T) {
	orig := interfaceLister
	interfaceLister = func() ([]net.Interface, error) {
		return nil, fmt.Errorf("simulated error")
	}
	defer func() { interfaceLister = orig }()

	results, errs := ResolveAll([]string{"test.local"})
	if len(results) != 0 {
		t.Errorf("expected 0 results with interface error, got %d", len(results))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestMdnsMulticastAddr(t *testing.T) {
	expected := net.IPv4(224, 0, 0, 251)
	if !mdnsV4.IP.Equal(expected) {
		t.Errorf("expected 224.0.0.251, got %s", mdnsV4.IP)
	}
	if mdnsV4.Port != 5353 {
		t.Errorf("expected port 5353, got %d", mdnsV4.Port)
	}
}

func TestResolve_WithNoInterfaces(t *testing.T) {
	orig := interfaceLister
	interfaceLister = func() ([]net.Interface, error) {
		return []net.Interface{}, nil
	}
	defer func() { interfaceLister = orig }()

	_, err := Resolve("test.local")
	if err == nil {
		t.Error("expected error for empty interface list")
	}
}

func TestResolveOnInterface_Success(t *testing.T) {
	withResolverHooks(t)

	resp := new(dns.Msg)
	resp.Response = true
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   "test.local.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    120,
		},
		A: net.ParseIP("10.0.0.99"),
	})
	data, err := resp.Pack()
	if err != nil {
		t.Fatal(err)
	}

	conn := &fakePacketConn{readPackets: [][]byte{data}}
	listenMulticastUDP = func(string, *net.Interface, *net.UDPAddr) (multicastPacketConn, error) {
		return conn, nil
	}

	ip, err := resolveOnInterface("test.local.", &net.Interface{Name: "eth0"})
	if err != nil {
		t.Fatal(err)
	}
	if !ip.Equal(net.ParseIP("10.0.0.99")) {
		t.Fatalf("unexpected IP: %v", ip)
	}
	if !conn.wrote || !conn.closed {
		t.Fatalf("connection not used/closed: wrote=%v closed=%v", conn.wrote, conn.closed)
	}
}

func TestResolveOnInterface_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "listen",
			setup: func() {
				listenMulticastUDP = func(string, *net.Interface, *net.UDPAddr) (multicastPacketConn, error) {
					return nil, errors.New("listen")
				}
			},
		},
		{
			name: "pack",
			setup: func() {
				listenMulticastUDP = func(string, *net.Interface, *net.UDPAddr) (multicastPacketConn, error) {
					return &fakePacketConn{}, nil
				}
				newDNSMessage = func() *dns.Msg {
					return &dns.Msg{Question: []dns.Question{{Name: "bad..name", Qtype: dns.TypeA, Qclass: dns.ClassINET}}}
				}
			},
		},
		{
			name: "write",
			setup: func() {
				listenMulticastUDP = func(string, *net.Interface, *net.UDPAddr) (multicastPacketConn, error) {
					return &fakePacketConn{writeErr: errors.New("write")}, nil
				}
			},
		},
		{
			name: "read",
			setup: func() {
				listenMulticastUDP = func(string, *net.Interface, *net.UDPAddr) (multicastPacketConn, error) {
					return &fakePacketConn{readErr: errors.New("read")}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withResolverHooks(t)
			tt.setup()
			if _, err := resolveOnInterface("test.local.", &net.Interface{Name: "eth0"}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseMDNSResponse_Garbage(t *testing.T) {
	ip, found, skip := parseMDNSResponse([]byte{0xff, 0xff, 0xff, 0xff}, "test.local.")
	if found {
		t.Error("should not find IP in garbage data")
	}
	if !skip {
		t.Error("should skip garbage data")
	}
	if ip != nil {
		t.Error("ip should be nil")
	}
}

func TestParseMDNSResponse_Empty(t *testing.T) {
	ip, found, skip := parseMDNSResponse([]byte{}, "test.local.")
	if found {
		t.Error("should not find IP in empty data")
	}
	if !skip {
		t.Error("should skip empty/invalid data")
	}
	if ip != nil {
		t.Error("ip should be nil")
	}
}

func TestParseMDNSResponse_QueryNotResponse(t *testing.T) {
	// Build a DNS query (not response) — QR bit not set
	msg := new(dns.Msg)
	msg.Id = 1
	msg.RecursionDesired = false
	msg.Response = false
	msg.Question = []dns.Question{
		{Name: "test.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
	}
	data, err := msg.Pack()
	if err != nil {
		t.Fatal(err)
	}

	ip, found, skip := parseMDNSResponse(data, "test.local.")
	if found {
		t.Error("should not find IP in query message")
	}
	if !skip {
		t.Error("should skip query (non-response) messages")
	}
	if ip != nil {
		t.Error("ip should be nil")
	}
}

func TestParseMDNSResponse_ValidResponse(t *testing.T) {
	resp := new(dns.Msg)
	resp.Response = true
	resp.Id = 1
	resp.RecursionDesired = false
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   "test.local.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    120,
		},
		A: net.ParseIP("10.0.0.99"),
	})
	data, err := resp.Pack()
	if err != nil {
		t.Fatal(err)
	}

	ip, found, skip := parseMDNSResponse(data, "test.local.")
	if !found {
		t.Error("should find IP in valid response")
	}
	if skip {
		t.Error("should not skip valid response")
	}
	if ip == nil || ip.String() != "10.0.0.99" {
		t.Errorf("expected 10.0.0.99, got %v", ip)
	}
}

func TestParseMDNSResponse_WrongName(t *testing.T) {
	resp := new(dns.Msg)
	resp.Response = true
	resp.Id = 1
	resp.RecursionDesired = false
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   "other.local.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    120,
		},
		A: net.ParseIP("10.0.0.99"),
	})
	data, err := resp.Pack()
	if err != nil {
		t.Fatal(err)
	}

	ip, found, skip := parseMDNSResponse(data, "test.local.")
	if found {
		t.Error("should not match wrong hostname")
	}
	if skip {
		t.Error("should not skip valid response (just no match)")
	}
	if ip != nil {
		t.Error("ip should be nil for wrong name")
	}
}

func TestParseMDNSResponse_NoAnswers(t *testing.T) {
	resp := new(dns.Msg)
	resp.Response = true
	resp.Id = 1
	resp.RecursionDesired = false
	// No answer records
	data, err := resp.Pack()
	if err != nil {
		t.Fatal(err)
	}

	ip, found, skip := parseMDNSResponse(data, "test.local.")
	if found {
		t.Error("should not find IP with no answers")
	}
	if skip {
		t.Error("should not skip response with empty answers")
	}
	if ip != nil {
		t.Error("ip should be nil")
	}
}

func TestParseMDNSResponse_AAAANotA(t *testing.T) {
	resp := new(dns.Msg)
	resp.Response = true
	resp.Id = 1
	resp.RecursionDesired = false
	resp.Answer = append(resp.Answer, &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   "test.local.",
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    120,
		},
		AAAA: net.ParseIP("::1"),
	})
	data, err := resp.Pack()
	if err != nil {
		t.Fatal(err)
	}

	ip, found, skip := parseMDNSResponse(data, "test.local.")
	if found {
		t.Error("should not match AAAA record for A query")
	}
	if skip {
		t.Error("should not skip AAAA response")
	}
	if ip != nil {
		t.Error("ip should be nil for AAAA-only response")
	}
}

func TestResolveAll_MixedValidInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}
	results, errs := ResolveAll([]string{"valid-name-that-does-not-exist.local", "also-invalid.local"})
	if len(results) != 0 {
		t.Logf("unexpected results: %v", results)
	}
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
}
