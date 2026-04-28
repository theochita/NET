package dhcp

import (
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// fakeConn captures WriteTo calls for inspection in tests.
type fakeConn struct {
	mu      sync.Mutex
	packets [][]byte
	addrs   []net.Addr
}

func (f *fakeConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	f.packets = append(f.packets, cp)
	f.addrs = append(f.addrs, addr)
	return len(b), nil
}
func (f *fakeConn) ReadFrom(b []byte) (int, net.Addr, error) { return 0, nil, nil }
func (f *fakeConn) Close() error                             { return nil }
func (f *fakeConn) LocalAddr() net.Addr                      { return &net.UDPAddr{} }
func (f *fakeConn) SetDeadline(t time.Time) error            { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error        { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error       { return nil }

func (f *fakeConn) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.packets)
}

func (f *fakeConn) lastReply(t *testing.T) *dhcpv4.DHCPv4 {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.packets) == 0 {
		t.Fatal("no reply written")
	}
	pkt, err := dhcpv4.FromBytes(f.packets[len(f.packets)-1])
	if err != nil {
		t.Fatalf("parse reply: %v", err)
	}
	return pkt
}

func (f *fakeConn) lastAddr(t *testing.T) *net.UDPAddr {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.addrs) == 0 {
		t.Fatal("no reply written")
	}
	return f.addrs[len(f.addrs)-1].(*net.UDPAddr)
}

// newTestHandler creates a handler wired to a fresh LeaseStore and fakeConn.
// Pool: 192.168.1.100–200, server IP: 192.168.1.1.
func newTestHandler(t *testing.T) (func(net.PacketConn, net.Addr, *dhcpv4.DHCPv4), *LeaseStore, *fakeConn) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewLeaseStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	serverIP := net.ParseIP("192.168.1.1").To4()
	poolStart := net.ParseIP("192.168.1.100").To4()
	poolEnd := net.ParseIP("192.168.1.200").To4()

	s := &Server{
		store:  store,
		config: DHCPConfig{LeaseTime: 3600, Mask: "255.255.255.0"},
		logger: NewEventLogger(log.Default(), nil),
	}
	mods, err := BuildOptions(s.config, serverIP)
	if err != nil {
		t.Fatal(err)
	}
	s.modifiers = mods

	fc := &fakeConn{}
	return s.makeHandler(serverIP, poolStart, poolEnd, net.IPMask{255, 255, 255, 0}), store, fc
}

func makeReq(t *testing.T, mac net.HardwareAddr, msgType dhcpv4.MessageType, opts ...dhcpv4.Modifier) *dhcpv4.DHCPv4 {
	t.Helper()
	base := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(msgType),
		dhcpv4.WithHwAddr(mac),
	}
	req, err := dhcpv4.New(append(base, opts...)...)
	if err != nil {
		t.Fatalf("dhcpv4.New: %v", err)
	}
	return req
}

func withReqIP(ip string) dhcpv4.Modifier {
	return dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(net.ParseIP(ip).To4()))
}

var testMAC = net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
var otherMAC = net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

func TestHandler_NAK_OutsidePool(t *testing.T) {
	handler, _, fc := newTestHandler(t)
	req := makeReq(t, testMAC, dhcpv4.MessageTypeRequest, withReqIP("10.0.0.5"))
	handler(fc, &net.UDPAddr{}, req)

	reply := fc.lastReply(t)
	if reply.MessageType() != dhcpv4.MessageTypeNak {
		t.Errorf("expected NAK, got %s", reply.MessageType())
	}
	if !fc.lastAddr(t).IP.Equal(net.IPv4bcast) {
		t.Errorf("NAK must go to 255.255.255.255, got %s", fc.lastAddr(t).IP)
	}
}

func TestHandler_NAK_IPTakenByOtherMAC(t *testing.T) {
	handler, store, fc := newTestHandler(t)
	store.Assign(otherMAC.String(), otherMAC.String(), "192.168.1.150", "other", time.Hour)

	req := makeReq(t, testMAC, dhcpv4.MessageTypeRequest, withReqIP("192.168.1.150"))
	handler(fc, &net.UDPAddr{}, req)

	reply := fc.lastReply(t)
	if reply.MessageType() != dhcpv4.MessageTypeNak {
		t.Errorf("expected NAK, got %s", reply.MessageType())
	}
	if !fc.lastAddr(t).IP.Equal(net.IPv4bcast) {
		t.Errorf("NAK must go to 255.255.255.255, got %s", fc.lastAddr(t).IP)
	}
}

func TestHandler_ACK_InPool(t *testing.T) {
	handler, _, fc := newTestHandler(t)
	req := makeReq(t, testMAC, dhcpv4.MessageTypeRequest, withReqIP("192.168.1.100"))
	handler(fc, &net.UDPAddr{}, req)

	reply := fc.lastReply(t)
	if reply.MessageType() != dhcpv4.MessageTypeAck {
		t.Errorf("expected ACK, got %s", reply.MessageType())
	}
}

func TestHandler_Decline_NotReoffered(t *testing.T) {
	handler, _, fc := newTestHandler(t)

	// Client declines 192.168.1.100 (in-pool).
	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeDecline, withReqIP("192.168.1.100")))

	// Same MAC sends DISCOVER — must NOT be re-offered the declined IP.
	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeDiscover))

	reply := fc.lastReply(t)
	if reply.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("expected OFFER, got %s", reply.MessageType())
	}
	if reply.YourIPAddr.Equal(net.ParseIP("192.168.1.100").To4()) {
		t.Error("server re-offered the declined IP 192.168.1.100")
	}
}

func TestHandler_Decline_OutsidePool_Ignored(t *testing.T) {
	handler, store, fc := newTestHandler(t)

	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeDecline, withReqIP("10.0.0.5")))

	if fc.count() != 0 {
		t.Error("out-of-pool DECLINE should produce no reply")
	}
	if _, ok := store.GetByClient(testMAC.String()); ok {
		t.Error("out-of-pool DECLINE must not create a lease")
	}
}

func TestHandler_LeaseTimeDefaultMatchesStore(t *testing.T) {
	// LeaseTime=0 must yield matching wire option 51 and stored ExpiresAt
	// (both fall back to defaultLeaseTime).
	dir := t.TempDir()
	store, err := NewLeaseStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	serverIP := net.ParseIP("192.168.1.1").To4()
	poolStart := net.ParseIP("192.168.1.100").To4()
	poolEnd := net.ParseIP("192.168.1.200").To4()

	cfg := DHCPConfig{LeaseTime: uint32(defaultLeaseTime / time.Second), Mask: "255.255.255.0"}
	mods, err := BuildOptions(cfg, serverIP)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		store:     store,
		config:    cfg,
		modifiers: mods,
		logger:    NewEventLogger(log.New(io.Discard, "", 0), nil),
	}
	handler := s.makeHandler(serverIP, poolStart, poolEnd, net.IPMask{255, 255, 255, 0})
	fc := &fakeConn{}

	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeRequest, withReqIP("192.168.1.100")))

	reply := fc.lastReply(t)
	wireLease := reply.IPAddressLeaseTime(0)
	if wireLease != defaultLeaseTime {
		t.Errorf("wire lease time = %s, want %s", wireLease, defaultLeaseTime)
	}
	lease, ok := store.GetByClient(testMAC.String())
	if !ok {
		t.Fatal("no lease recorded")
	}
	delta := time.Until(lease.ExpiresAt) - defaultLeaseTime
	if delta < -2*time.Second || delta > 2*time.Second {
		t.Errorf("stored lease expiry off by %s from default %s", delta, defaultLeaseTime)
	}
}

func TestHandler_Hostname_Preserved_OnRenew(t *testing.T) {
	handler, store, fc := newTestHandler(t)

	// Initial REQUEST with hostname.
	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeRequest,
		withReqIP("192.168.1.100"),
		dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(12), []byte("myhost"))),
	))

	// Renewal without hostname option.
	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeRequest, withReqIP("192.168.1.100")))

	lease, ok := store.GetByClient(testMAC.String())
	if !ok {
		t.Fatal("no lease after renewal")
	}
	if lease.Hostname != "myhost" {
		t.Errorf("hostname not preserved on renew: got %q, want %q", lease.Hostname, "myhost")
	}
}

// withClientID adds RFC 2132 option 61 (Client Identifier) to a request.
func withClientID(id []byte) dhcpv4.Modifier {
	return dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(61), id))
}

func TestHandler_ClientID_UsedAsLeaseKey(t *testing.T) {
	handler, store, fc := newTestHandler(t)
	clientID := []byte{0xde, 0xad, 0xbe, 0xef}

	// REQUEST with option 61 — lease must be keyed by client-id, not MAC.
	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeRequest,
		withReqIP("192.168.1.100"),
		withClientID(clientID),
	))

	reply := fc.lastReply(t)
	if reply.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("expected ACK, got %s", reply.MessageType())
	}

	// Lease must be retrievable by the client-id key, not by MAC.
	expectedKey := "id:deadbeef"
	l, ok := store.GetByClient(expectedKey)
	if !ok {
		t.Fatal("no lease found under client-id key")
	}
	if l.ClientID != "deadbeef" {
		t.Errorf("ClientID = %q, want %q", l.ClientID, "deadbeef")
	}
	if l.MAC != testMAC.String() {
		t.Errorf("MAC = %q, want %q", l.MAC, testMAC.String())
	}

	// Same client-id on a second REQUEST must renew the same lease.
	handler(fc, &net.UDPAddr{}, makeReq(t, testMAC, dhcpv4.MessageTypeRequest,
		withReqIP("192.168.1.100"),
		withClientID(clientID),
	))
	if _, ok := store.GetByClient(expectedKey); !ok {
		t.Fatal("lease disappeared after renewal")
	}
}

func TestHandler_RelayInfo_LoggedAndACKed(t *testing.T) {
	// Option 82 must not break normal ACK flow — the server reads it for logging only.
	handler, _, fc := newTestHandler(t)

	opt82 := []byte{
		1, 4, 0x0a, 0x01, 0x01, 0x01, // sub-option 1 (circuit-id): 4 bytes
		2, 3, 0xaa, 0xbb, 0xcc,        // sub-option 2 (remote-id):  3 bytes
	}
	req := makeReq(t, testMAC, dhcpv4.MessageTypeRequest,
		withReqIP("192.168.1.100"),
		dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(82), opt82)),
	)
	handler(fc, &net.UDPAddr{}, req)

	reply := fc.lastReply(t)
	if reply.MessageType() != dhcpv4.MessageTypeAck {
		t.Errorf("expected ACK with option 82 present, got %s", reply.MessageType())
	}
}

func TestParseRelayInfo(t *testing.T) {
	data := []byte{
		1, 4, 0x0a, 0x01, 0x01, 0x01, // circuit-id: 0a010101
		2, 3, 0xaa, 0xbb, 0xcc,        // remote-id:  aabbcc
	}
	circuit, remote := parseRelayInfo(data)
	if circuit != "0a010101" {
		t.Errorf("circuit-id = %q, want %q", circuit, "0a010101")
	}
	if remote != "aabbcc" {
		t.Errorf("remote-id = %q, want %q", remote, "aabbcc")
	}
}
