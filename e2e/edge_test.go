//go:build e2e

package e2e_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// TestDHCP_Malformed_Garbage verifies the server silently drops random and
// truncated garbage on :67 and remains responsive to a subsequent valid
// DISCOVER. Catches parser panics and unbounded allocations on bad input —
// the #1 way real DHCP servers crash in the wild.
func TestDHCP_Malformed_Garbage(t *testing.T) {
	requireClient(t)
	dst := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: 67}

	payloads := [][]byte{
		{},                                       // empty datagram
		{0x01},                                   // 1 byte
		bytes.Repeat([]byte{0xff}, 16),           // short trash
		bytes.Repeat([]byte{0x00}, 240),          // header-sized zeros, no magic cookie
		append(make([]byte, 236), 0xde, 0xad, 0xbe, 0xef), // wrong magic cookie
		// Valid header + magic cookie + option declaring 99 bytes with no payload.
		append(append(make([]byte, 236), 0x63, 0x82, 0x53, 0x63), 53, 99),
	}
	for i, p := range payloads {
		if _, err := clientConn.WriteTo(p, dst); err != nil {
			t.Fatalf("write garbage payload %d: %v", i, err)
		}
	}

	// Drain any spurious replies, then prove the server still serves real packets.
	buf := make([]byte, 1500)
	clientConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		if _, _, err := clientConn.ReadFrom(buf); err != nil {
			break
		}
	}

	mac := nextMAC()
	offer := sendRecv(t, makeDiscover(t, mac))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("server unresponsive after garbage: got %s", offer.MessageType())
	}
	t.Cleanup(func() {
		ack := sendRecv(t, makeRequestFromOffer(t, offer))
		if ack.MessageType() == dhcpv4.MessageTypeAck {
			sendOnly(t, makeRelease(t, mac, ack.YourIPAddr))
		}
	})
}

// TestDHCP_InitReboot_OwnIP verifies a REQUEST with option 50 and NO server
// identifier (RFC 2131 §4.3.2 INIT-REBOOT, e.g. after a client power cycle)
// is ACKed when the requested IP matches the client's existing lease.
func TestDHCP_InitReboot_OwnIP(t *testing.T) {
	requireClient(t)
	ack, mac := doFullHandshake(t)
	leasedIP := ack.YourIPAddr

	// INIT-REBOOT: requested IP, no server identifier, ciaddr unset.
	req, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(leasedIP)),
	)
	if err != nil {
		t.Fatalf("build init-reboot: %v", err)
	}
	reply := sendRecv(t, req)
	if reply.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("init-reboot for own IP: got %s, want ACK", reply.MessageType())
	}
	if !reply.YourIPAddr.Equal(leasedIP) {
		t.Errorf("init-reboot ACK %s != leased %s", reply.YourIPAddr, leasedIP)
	}
}

// TestDHCP_InitReboot_StolenIP verifies an INIT-REBOOT REQUEST (option 50, no
// server-id) for an IP currently held by a different client gets a NAK.
func TestDHCP_InitReboot_StolenIP(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	stolenIP := ack.YourIPAddr

	thief := nextMAC()
	req, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(thief),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(stolenIP)),
		// no server identifier — this is INIT-REBOOT, not SELECTING
	)
	if err != nil {
		t.Fatalf("build init-reboot steal: %v", err)
	}
	reply := sendRecv(t, req)
	if reply.MessageType() != dhcpv4.MessageTypeNak {
		t.Errorf("init-reboot for stolen IP: got %s, want NAK", reply.MessageType())
	}
}

// TestDHCP_Release_Unowned verifies a RELEASE from a client that does not hold
// the IP is silently ignored — the real lease holder must keep the IP.
func TestDHCP_Release_Unowned(t *testing.T) {
	requireClient(t)
	ack, ownerMAC := doFullHandshake(t)
	leasedIP := ack.YourIPAddr

	thief := nextMAC()
	sendOnly(t, makeRelease(t, thief, leasedIP))
	time.Sleep(100 * time.Millisecond)

	// Owner DISCOVERs again — server must still re-offer the same IP.
	offer := sendRecv(t, makeDiscover(t, ownerMAC))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("post spurious-release DISCOVER: got %s, want OFFER", offer.MessageType())
	}
	if !offer.YourIPAddr.Equal(leasedIP) {
		t.Errorf("spurious release leaked lease: re-offer %s, want unchanged %s", offer.YourIPAddr, leasedIP)
	}
}

// TestDHCP_NoPRL_StandardOptions verifies the server includes the standard
// option set (router, mask, DNS, lease time) even when the client omits
// option 55 (Parameter Request List). Some embedded clients send no PRL.
func TestDHCP_NoPRL_StandardOptions(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	disc, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(mac),
	)
	if err != nil {
		t.Fatalf("build bare discover: %v", err)
	}
	if len(disc.Options.Get(dhcpv4.OptionParameterRequestList)) != 0 {
		t.Fatal("test sanity: built DISCOVER unexpectedly carries a PRL")
	}

	offer := sendRecv(t, disc)
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("DISCOVER without PRL: got %s, want OFFER", offer.MessageType())
	}
	if len(offer.Router()) == 0 {
		t.Error("router (3) missing without PRL")
	}
	if offer.SubnetMask() == nil {
		t.Error("subnet mask (1) missing without PRL")
	}
	if len(offer.DNS()) == 0 {
		t.Error("DNS (6) missing without PRL")
	}
	if offer.IPAddressLeaseTime(0) == 0 {
		t.Error("lease time (51) missing without PRL")
	}

	t.Cleanup(func() {
		ack := sendRecv(t, makeRequestFromOffer(t, offer))
		if ack.MessageType() == dhcpv4.MessageTypeAck {
			sendOnly(t, makeRelease(t, mac, ack.YourIPAddr))
		}
	})
}
