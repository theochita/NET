//go:build e2e

package e2e_test

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

const serverIP = "172.99.0.1"

var (
	clientConn net.PacketConn
	macCounter uint32
)

// setupClientMode binds the UDP :68 socket used by all scenario tests and waits
// for the server to be ready. Called from TestMain when DHCP_E2E_CLIENT=1.
func setupClientMode() {
	conn, err := net.ListenPacket("udp4", "0.0.0.0:68")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: bind :68: %v\n", err)
		os.Exit(1)
	}
	clientConn = conn
	if err := waitForServer(10 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		os.Exit(1)
	}
}

// waitForServer probes the DHCP server until it responds or timeout elapses.
func waitForServer(timeout time.Duration) error {
	probe := func() bool {
		mac := net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0xfe}
		disc, err := dhcpv4.NewDiscovery(mac)
		if err != nil {
			return false
		}
		dst := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: 67}
		clientConn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
		if _, err := clientConn.WriteTo(disc.ToBytes(), dst); err != nil {
			return false
		}
		buf := make([]byte, 1500)
		clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := clientConn.ReadFrom(buf)
		return err == nil && n > 0
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if probe() {
			clientConn.SetDeadline(time.Time{}) // clear probe deadlines before tests run
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after %s", timeout)
}

// requireClient skips the test if not running inside the Docker client container.
func requireClient(t *testing.T) {
	t.Helper()
	if os.Getenv("DHCP_E2E_CLIENT") != "1" {
		t.Skip("scenario test — only runs inside Docker (DHCP_E2E_CLIENT=1)")
	}
	if clientConn == nil {
		t.Fatal("clientConn not initialized — TestMain did not call setupClientMode")
	}
}

// nextMAC returns a unique MAC address for each test to prevent lease conflicts.
func nextMAC() net.HardwareAddr {
	n := atomic.AddUint32(&macCounter, 1)
	return net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, byte(n >> 8), byte(n)}
}

// inPool returns true if ip is in the E2E server pool 172.99.0.100-111.
func inPool(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4[0] == 172 && ip4[1] == 99 && ip4[2] == 0 &&
		ip4[3] >= 100 && ip4[3] <= 111
}

// sendRecv sends pkt to the server and returns the first matching reply.
// Fatal after 3 seconds.
func sendRecv(t *testing.T, pkt *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	t.Helper()
	dst := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: 67}
	if _, err := clientConn.WriteTo(pkt.ToBytes(), dst); err != nil {
		t.Fatalf("sendRecv write: %v", err)
	}
	buf := make([]byte, 1500)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		clientConn.SetReadDeadline(deadline)
		n, _, err := clientConn.ReadFrom(buf)
		if err != nil {
			t.Fatalf("sendRecv recv: %v", err)
		}
		reply, err := dhcpv4.FromBytes(buf[:n])
		if err != nil {
			continue
		}
		if reply.TransactionID == pkt.TransactionID {
			return reply
		}
	}
	t.Fatalf("sendRecv: timeout waiting for reply to xid %x", pkt.TransactionID)
	return nil
}

// trySendRecv sends pkt and returns the first matching reply, or nil on timeout.
// Does not call t.Fatal — used when no reply is the expected outcome.
func trySendRecv(pkt *dhcpv4.DHCPv4, timeout time.Duration) *dhcpv4.DHCPv4 {
	dst := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: 67}
	clientConn.WriteTo(pkt.ToBytes(), dst)
	buf := make([]byte, 1500)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		clientConn.SetReadDeadline(deadline)
		n, _, err := clientConn.ReadFrom(buf)
		if err != nil {
			return nil
		}
		reply, err := dhcpv4.FromBytes(buf[:n])
		if err != nil {
			continue
		}
		if reply.TransactionID == pkt.TransactionID {
			return reply
		}
	}
	return nil
}

// sendOnly sends pkt without waiting for a reply.
func sendOnly(t *testing.T, pkt *dhcpv4.DHCPv4) {
	t.Helper()
	dst := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: 67}
	if _, err := clientConn.WriteTo(pkt.ToBytes(), dst); err != nil {
		t.Fatalf("sendOnly: %v", err)
	}
}

// doFullHandshake runs DISCOVER → OFFER → REQUEST → ACK, registers a RELEASE
// cleanup, and returns the ACK and the MAC used.
func doFullHandshake(t *testing.T) (*dhcpv4.DHCPv4, net.HardwareAddr) {
	t.Helper()
	mac := nextMAC()
	offer := sendRecv(t, makeDiscover(t, mac))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("doFullHandshake DISCOVER: got %s, want OFFER", offer.MessageType())
	}
	ack := sendRecv(t, makeRequestFromOffer(t, offer))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("doFullHandshake REQUEST: got %s, want ACK", ack.MessageType())
	}
	ip := ack.YourIPAddr
	t.Cleanup(func() { sendOnly(t, makeRelease(t, mac, ip)) })
	return ack, mac
}

// --- Packet builders ---

func makeDiscover(t *testing.T, mac net.HardwareAddr, opts ...dhcpv4.Modifier) *dhcpv4.DHCPv4 {
	t.Helper()
	pkt, err := dhcpv4.NewDiscovery(mac, opts...)
	if err != nil {
		t.Fatalf("makeDiscover: %v", err)
	}
	return pkt
}

func makeRequestFromOffer(t *testing.T, offer *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	t.Helper()
	req, err := dhcpv4.NewRequestFromOffer(offer)
	if err != nil {
		t.Fatalf("makeRequestFromOffer: %v", err)
	}
	return req
}

func makeRelease(t *testing.T, mac net.HardwareAddr, ip net.IP) *dhcpv4.DHCPv4 {
	t.Helper()
	pkt, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRelease),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithClientIP(ip),
	)
	if err != nil {
		t.Fatalf("makeRelease: %v", err)
	}
	return pkt
}

func makeInform(t *testing.T, mac net.HardwareAddr, clientIP net.IP) *dhcpv4.DHCPv4 {
	t.Helper()
	pkt, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeInform),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithClientIP(clientIP),
	)
	if err != nil {
		t.Fatalf("makeInform: %v", err)
	}
	return pkt
}

func makeDecline(t *testing.T, mac net.HardwareAddr, ip net.IP) *dhcpv4.DHCPv4 {
	t.Helper()
	pkt, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDecline),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(ip)),
	)
	if err != nil {
		t.Fatalf("makeDecline: %v", err)
	}
	return pkt
}

// uint32FromBytes decodes a 4-byte big-endian slice as uint32.
func uint32FromBytes(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}
