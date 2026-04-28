//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// TestDHCP_Discover_Offer verifies DISCOVER → OFFER with an in-pool IP.
func TestDHCP_Discover_Offer(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	offer := sendRecv(t, makeDiscover(t, mac))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("got %s, want OFFER", offer.MessageType())
	}
	if !inPool(offer.YourIPAddr) {
		t.Errorf("offered IP %s not in pool 172.99.0.100-111", offer.YourIPAddr)
	}
}

// TestDHCP_Request_ACK verifies REQUEST after OFFER → ACK with the same IP.
func TestDHCP_Request_ACK(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	offer := sendRecv(t, makeDiscover(t, mac))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("DISCOVER: got %s, want OFFER", offer.MessageType())
	}
	ack := sendRecv(t, makeRequestFromOffer(t, offer))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("REQUEST: got %s, want ACK", ack.MessageType())
	}
	if !ack.YourIPAddr.Equal(offer.YourIPAddr) {
		t.Errorf("ACK IP %s != offered IP %s", ack.YourIPAddr, offer.YourIPAddr)
	}
	t.Cleanup(func() { sendOnly(t, makeRelease(t, mac, ack.YourIPAddr)) })
}

// TestDHCP_MAC_ReOffer verifies a second DISCOVER from the same MAC re-offers the same IP.
func TestDHCP_MAC_ReOffer(t *testing.T) {
	requireClient(t)
	mac := nextMAC()

	offer1 := sendRecv(t, makeDiscover(t, mac))
	ack := sendRecv(t, makeRequestFromOffer(t, offer1))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("initial ACK: got %s", ack.MessageType())
	}
	t.Cleanup(func() { sendOnly(t, makeRelease(t, mac, ack.YourIPAddr)) })

	offer2 := sendRecv(t, makeDiscover(t, mac))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("second DISCOVER: got %s, want OFFER", offer2.MessageType())
	}
	if !offer2.YourIPAddr.Equal(offer1.YourIPAddr) {
		t.Errorf("re-offered %s, want same IP %s", offer2.YourIPAddr, offer1.YourIPAddr)
	}
}

// TestDHCP_Release_Recycle verifies a released IP is offered to the next client.
func TestDHCP_Release_Recycle(t *testing.T) {
	requireClient(t)
	macA := nextMAC()
	macB := nextMAC()

	offer := sendRecv(t, makeDiscover(t, macA))
	ack := sendRecv(t, makeRequestFromOffer(t, offer))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("initial ACK: got %s", ack.MessageType())
	}
	assignedIP := ack.YourIPAddr

	sendOnly(t, makeRelease(t, macA, assignedIP))
	time.Sleep(100 * time.Millisecond)

	offer2 := sendRecv(t, makeDiscover(t, macB))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("post-release DISCOVER: got %s, want OFFER", offer2.MessageType())
	}
	if !offer2.YourIPAddr.Equal(assignedIP) {
		t.Errorf("got %s after release, want recycled %s", offer2.YourIPAddr, assignedIP)
	}
}

// TestDHCP_Inform_ACK verifies INFORM → ACK with YourIP=0.0.0.0 and options present.
func TestDHCP_Inform_ACK(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	clientIP := net.ParseIP("172.99.0.50").To4()

	ack := sendRecv(t, makeInform(t, mac, clientIP))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("INFORM: got %s, want ACK", ack.MessageType())
	}
	if !ack.YourIPAddr.Equal(net.IPv4zero) {
		t.Errorf("YourIP = %s, want 0.0.0.0", ack.YourIPAddr)
	}
	if len(ack.Router()) == 0 {
		t.Error("router option missing in INFORM ACK")
	}
}

// TestDHCP_ServerID_Mismatch verifies a REQUEST with a wrong server identifier
// is silently discarded (no reply within 1 second).
func TestDHCP_ServerID_Mismatch(t *testing.T) {
	requireClient(t)
	mac := nextMAC()

	offer := sendRecv(t, makeDiscover(t, mac))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("DISCOVER: got %s, want OFFER", offer.MessageType())
	}

	req, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(offer.YourIPAddr)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP("192.168.99.99").To4())),
	)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	reply := trySendRecv(req, 1*time.Second)
	if reply != nil {
		t.Errorf("got %s reply, want silence (server ID mismatch must be discarded)", reply.MessageType())
	}
}

// TestDHCP_PreferredIP_Honored verifies option 50 in DISCOVER for a free in-pool
// IP causes the server to offer that specific IP.
func TestDHCP_PreferredIP_Honored(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	preferred := net.ParseIP("172.99.0.105").To4()

	offer := sendRecv(t, makeDiscover(t, mac,
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(preferred)),
	))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("got %s, want OFFER", offer.MessageType())
	}
	if !offer.YourIPAddr.Equal(preferred) {
		t.Errorf("got %s, want preferred IP %s", offer.YourIPAddr, preferred)
	}
}

// TestDHCP_PreferredIP_Taken verifies option 50 for an already-leased IP causes
// the server to offer a different free IP instead.
func TestDHCP_PreferredIP_Taken(t *testing.T) {
	requireClient(t)
	macOwner := nextMAC()
	macWanter := nextMAC()

	offer := sendRecv(t, makeDiscover(t, macOwner))
	ack := sendRecv(t, makeRequestFromOffer(t, offer))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("owner ACK: got %s", ack.MessageType())
	}
	takenIP := ack.YourIPAddr
	t.Cleanup(func() { sendOnly(t, makeRelease(t, macOwner, takenIP)) })

	offer2 := sendRecv(t, makeDiscover(t, macWanter,
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(takenIP)),
	))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("wanter OFFER: got %s", offer2.MessageType())
	}
	if offer2.YourIPAddr.Equal(takenIP) {
		t.Errorf("server offered taken IP %s to a different client", takenIP)
	}
	if !inPool(offer2.YourIPAddr) {
		t.Errorf("fallback offer %s not in pool", offer2.YourIPAddr)
	}
}

// TestDHCP_Decline_Hold verifies a DECLINE causes the server to not re-offer
// the declined IP to the same client.
func TestDHCP_Decline_Hold(t *testing.T) {
	requireClient(t)
	mac := nextMAC()

	offer := sendRecv(t, makeDiscover(t, mac))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("DISCOVER: got %s, want OFFER", offer.MessageType())
	}
	declinedIP := offer.YourIPAddr

	sendOnly(t, makeDecline(t, mac, declinedIP))
	// DECLINED leases persist for the full lease time; release them in cleanup
	// so they don't exhaust the small pool for subsequent tests.
	t.Cleanup(func() { sendOnly(t, makeRelease(t, mac, declinedIP)) })
	time.Sleep(100 * time.Millisecond)

	offer2 := sendRecv(t, makeDiscover(t, mac))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("post-DECLINE DISCOVER: got %s, want OFFER", offer2.MessageType())
	}
	if offer2.YourIPAddr.Equal(declinedIP) {
		t.Errorf("server re-offered the declined IP %s to same client", declinedIP)
	}
}

// TestDHCP_Decline_OtherClient verifies a DECLINE also prevents the IP from
// being offered to other clients.
func TestDHCP_Decline_OtherClient(t *testing.T) {
	requireClient(t)
	macA := nextMAC()
	macB := nextMAC()

	offer := sendRecv(t, makeDiscover(t, macA))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("DISCOVER: got %s, want OFFER", offer.MessageType())
	}
	declinedIP := offer.YourIPAddr

	sendOnly(t, makeDecline(t, macA, declinedIP))
	t.Cleanup(func() { sendOnly(t, makeRelease(t, macA, declinedIP)) })
	time.Sleep(100 * time.Millisecond)

	offer2 := sendRecv(t, makeDiscover(t, macB))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("other client DISCOVER: got %s, want OFFER", offer2.MessageType())
	}
	if offer2.YourIPAddr.Equal(declinedIP) {
		t.Errorf("server offered declined IP %s to different client", declinedIP)
	}
}

// TestDHCP_Hostname_Preserved verifies that renewing without a hostname option
// still produces an ACK for the same IP (hostname is kept server-side).
func TestDHCP_Hostname_Preserved(t *testing.T) {
	requireClient(t)
	mac := nextMAC()

	offer := sendRecv(t, makeDiscover(t, mac))
	req, err := dhcpv4.NewRequestFromOffer(offer,
		dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(12), []byte("myhost"))),
	)
	if err != nil {
		t.Fatalf("build request with hostname: %v", err)
	}
	ack1 := sendRecv(t, req)
	if ack1.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("initial ACK: got %s", ack1.MessageType())
	}
	assignedIP := ack1.YourIPAddr
	t.Cleanup(func() { sendOnly(t, makeRelease(t, mac, assignedIP)) })

	// Renewal with no hostname option — server should still ACK the same IP.
	req2, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(assignedIP)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP("172.99.0.1").To4())),
	)
	if err != nil {
		t.Fatalf("build renewal: %v", err)
	}
	ack2 := sendRecv(t, req2)
	if ack2.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("renewal ACK: got %s", ack2.MessageType())
	}
	if !ack2.YourIPAddr.Equal(assignedIP) {
		t.Errorf("renewal ACK IP %s != original %s", ack2.YourIPAddr, assignedIP)
	}
}

// TestDHCP_ClientID_LeaseKey verifies option 61 (Client Identifier) is used as
// the lease key — a second DISCOVER with the same client-id but a different MAC
// re-offers the same IP.
func TestDHCP_ClientID_LeaseKey(t *testing.T) {
	requireClient(t)
	mac1 := nextMAC()
	mac2 := nextMAC()
	clientID := []byte{0xc1, 0x1e, 0x00, 0x01}
	opt61 := dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(61), clientID))

	offer := sendRecv(t, makeDiscover(t, mac1, opt61))
	req, err := dhcpv4.NewRequestFromOffer(offer, opt61)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	ack := sendRecv(t, req)
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("ACK: got %s", ack.MessageType())
	}
	assignedIP := ack.YourIPAddr
	t.Cleanup(func() {
		rel := makeRelease(t, mac1, assignedIP)
		rel.Options.Update(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(61), clientID))
		sendOnly(t, rel)
	})

	// Different MAC, same client-id → must re-offer the same IP.
	offer2 := sendRecv(t, makeDiscover(t, mac2, opt61))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("second DISCOVER: got %s, want OFFER", offer2.MessageType())
	}
	if !offer2.YourIPAddr.Equal(assignedIP) {
		t.Errorf("same client-id different MAC: got %s, want %s", offer2.YourIPAddr, assignedIP)
	}
}

// TestDHCP_PoolExhaustion fills all 12 pool IPs, verifies the 13th client gets
// no offer, then releases one and verifies the 13th client succeeds.
func TestDHCP_PoolExhaustion(t *testing.T) {
	requireClient(t)

	const poolSize = 12
	macs := make([]net.HardwareAddr, poolSize)
	ips := make([]net.IP, poolSize)

	for i := 0; i < poolSize; i++ {
		macs[i] = nextMAC()
		offer := sendRecv(t, makeDiscover(t, macs[i]))
		if offer.MessageType() != dhcpv4.MessageTypeOffer {
			t.Fatalf("client %d DISCOVER: got %s, want OFFER", i, offer.MessageType())
		}
		ack := sendRecv(t, makeRequestFromOffer(t, offer))
		if ack.MessageType() != dhcpv4.MessageTypeAck {
			t.Fatalf("client %d REQUEST: got %s, want ACK", i, ack.MessageType())
		}
		ips[i] = ack.YourIPAddr
	}

	t.Cleanup(func() {
		for i, mac := range macs {
			if ips[i] != nil {
				sendOnly(t, makeRelease(t, mac, ips[i]))
			}
		}
		time.Sleep(100 * time.Millisecond)
	})

	// 13th client — pool is full, expect no offer.
	mac13 := nextMAC()
	if reply := trySendRecv(makeDiscover(t, mac13), 1*time.Second); reply != nil {
		t.Errorf("pool exhausted but got %s for 13th client", reply.MessageType())
	}

	// Release one slot and verify 13th client now succeeds.
	sendOnly(t, makeRelease(t, macs[0], ips[0]))
	ips[0] = nil
	time.Sleep(100 * time.Millisecond)

	offer := sendRecv(t, makeDiscover(t, mac13))
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("after release: got %s, want OFFER", offer.MessageType())
	}
	if !inPool(offer.YourIPAddr) {
		t.Errorf("offer after release %s not in pool", offer.YourIPAddr)
	}
}

// TestDHCP_Option_Router verifies option 3 (Router) is 172.99.0.1.
func TestDHCP_Option_Router(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	routers := ack.Router()
	if len(routers) == 0 {
		t.Fatal("router option (3) missing")
	}
	if !routers[0].Equal(net.ParseIP("172.99.0.1").To4()) {
		t.Errorf("router = %s, want 172.99.0.1", routers[0])
	}
}

// TestDHCP_Option_SubnetMask verifies option 1 (Subnet Mask) is 255.255.255.0.
func TestDHCP_Option_SubnetMask(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	mask := ack.SubnetMask()
	if mask == nil {
		t.Fatal("subnet mask option (1) missing")
	}
	if mask.String() != "ffffff00" {
		t.Errorf("mask = %s, want ffffff00", mask.String())
	}
}

// TestDHCP_Option_DNS verifies option 6 contains 8.8.8.8 and 8.8.4.4 in order.
func TestDHCP_Option_DNS(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	dns := ack.DNS()
	if len(dns) != 2 {
		t.Fatalf("DNS option (6): got %d servers, want 2", len(dns))
	}
	if !dns[0].Equal(net.ParseIP("8.8.8.8").To4()) {
		t.Errorf("DNS[0] = %s, want 8.8.8.8", dns[0])
	}
	if !dns[1].Equal(net.ParseIP("8.8.4.4").To4()) {
		t.Errorf("DNS[1] = %s, want 8.8.4.4", dns[1])
	}
}

// TestDHCP_Option_LeaseTime verifies option 51 matches the configured 3600s.
func TestDHCP_Option_LeaseTime(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	lt := ack.IPAddressLeaseTime(0)
	if lt != 3600*time.Second {
		t.Errorf("lease time = %s, want 3600s", lt)
	}
}

// TestDHCP_Option_T1_T2 verifies renewal (58) and rebind (59) timers are
// T1=1800s (0.5×3600) and T2=3150s (0.875×3600).
func TestDHCP_Option_T1_T2(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)

	t1Raw := ack.Options.Get(dhcpv4.GenericOptionCode(58))
	if len(t1Raw) != 4 {
		t.Fatalf("T1 option (58): got %d bytes, want 4", len(t1Raw))
	}
	if t1 := time.Duration(binary.BigEndian.Uint32(t1Raw)) * time.Second; t1 != 1800*time.Second {
		t.Errorf("T1 = %s, want 1800s (0.5 × 3600s)", t1)
	}

	t2Raw := ack.Options.Get(dhcpv4.GenericOptionCode(59))
	if len(t2Raw) != 4 {
		t.Fatalf("T2 option (59): got %d bytes, want 4", len(t2Raw))
	}
	if t2 := time.Duration(binary.BigEndian.Uint32(t2Raw)) * time.Second; t2 != 3150*time.Second {
		t.Errorf("T2 = %s, want 3150s (0.875 × 3600s)", t2)
	}
}

// TestDHCP_Option_NTP verifies option 42 (NTP Servers) contains 172.99.0.1.
func TestDHCP_Option_NTP(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(42))
	if len(raw) != 4 {
		t.Fatalf("NTP option (42): got %d bytes, want 4", len(raw))
	}
	if !net.IP(raw).Equal(net.ParseIP("172.99.0.1").To4()) {
		t.Errorf("NTP = %s, want 172.99.0.1", net.IP(raw))
	}
}

// TestDHCP_Option_WINS verifies option 44 (WINS) contains 172.99.0.1.
func TestDHCP_Option_WINS(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(44))
	if len(raw) != 4 {
		t.Fatalf("WINS option (44): got %d bytes, want 4", len(raw))
	}
	if !net.IP(raw).Equal(net.ParseIP("172.99.0.1").To4()) {
		t.Errorf("WINS = %s, want 172.99.0.1", net.IP(raw))
	}
}

// TestDHCP_Option_DomainSearch verifies option 119 is RFC 1035 DNS-label encoded
// for ["test.local", "local"].
func TestDHCP_Option_DomainSearch(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(119))
	if len(raw) == 0 {
		t.Fatal("domain search option (119) missing")
	}
	// encodeDNSName("test.local") = \x04test\x05local\x00 (12 bytes)
	// encodeDNSName("local")      = \x05local\x00         (7 bytes)
	want := []byte{
		4, 't', 'e', 's', 't',
		5, 'l', 'o', 'c', 'a', 'l', 0,
		5, 'l', 'o', 'c', 'a', 'l', 0,
	}
	if !bytes.Equal(raw, want) {
		t.Errorf("domain search = %v, want %v", raw, want)
	}
}

// TestDHCP_Option_BootFile_TFTP verifies option 67 (Boot File) is "pxelinux.0"
// and option 150 (TFTP Server) is 172.99.0.1.
func TestDHCP_Option_BootFile_TFTP(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)

	bootFile := ack.Options.Get(dhcpv4.GenericOptionCode(67))
	if string(bootFile) != "pxelinux.0" {
		t.Errorf("boot file (67) = %q, want %q", string(bootFile), "pxelinux.0")
	}

	tftpRaw := ack.Options.Get(dhcpv4.GenericOptionCode(150))
	if len(tftpRaw) != 4 {
		t.Fatalf("TFTP server (150): got %d bytes, want 4", len(tftpRaw))
	}
	if !net.IP(tftpRaw).Equal(net.ParseIP("172.99.0.1").To4()) {
		t.Errorf("TFTP server = %s, want 172.99.0.1", net.IP(tftpRaw))
	}
}

// TestDHCP_CustomOpt_IP verifies code 200 type=ip encodes as 4 bytes (10.10.10.10).
func TestDHCP_CustomOpt_IP(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(200))
	if len(raw) != 4 {
		t.Fatalf("custom opt 200 (ip): got %d bytes, want 4", len(raw))
	}
	if !net.IP(raw).Equal(net.ParseIP("10.10.10.10").To4()) {
		t.Errorf("custom opt 200 = %s, want 10.10.10.10", net.IP(raw))
	}
}

// TestDHCP_CustomOpt_String verifies code 201 type=string encodes as raw bytes.
func TestDHCP_CustomOpt_String(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(201))
	if string(raw) != "hello" {
		t.Errorf("custom opt 201 (string) = %q, want %q", string(raw), "hello")
	}
}

// TestDHCP_CustomOpt_Uint8 verifies code 202 type=uint8 encodes as 1 byte (42 = 0x2a).
func TestDHCP_CustomOpt_Uint8(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(202))
	if len(raw) != 1 {
		t.Fatalf("custom opt 202 (uint8): got %d bytes, want 1", len(raw))
	}
	if raw[0] != 42 {
		t.Errorf("custom opt 202 = %d, want 42", raw[0])
	}
}

// TestDHCP_CustomOpt_Uint16 verifies code 203 type=uint16 encodes as 2 bytes big-endian (1234).
func TestDHCP_CustomOpt_Uint16(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(203))
	if len(raw) != 2 {
		t.Fatalf("custom opt 203 (uint16): got %d bytes, want 2", len(raw))
	}
	if got := binary.BigEndian.Uint16(raw); got != 1234 {
		t.Errorf("custom opt 203 = %d, want 1234", got)
	}
}

// TestDHCP_CustomOpt_Uint32 verifies code 204 type=uint32 encodes as 4 bytes big-endian (98765).
func TestDHCP_CustomOpt_Uint32(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(204))
	if len(raw) != 4 {
		t.Fatalf("custom opt 204 (uint32): got %d bytes, want 4", len(raw))
	}
	if got := binary.BigEndian.Uint32(raw); got != 98765 {
		t.Errorf("custom opt 204 = %d, want 98765", got)
	}
}

// TestDHCP_CustomOpt_Bool verifies code 205 type=bool encodes as 0x01.
func TestDHCP_CustomOpt_Bool(t *testing.T) {
	requireClient(t)
	ack, _ := doFullHandshake(t)
	raw := ack.Options.Get(dhcpv4.GenericOptionCode(205))
	if len(raw) != 1 {
		t.Fatalf("custom opt 205 (bool): got %d bytes, want 1", len(raw))
	}
	if raw[0] != 0x01 {
		t.Errorf("custom opt 205 = 0x%02x, want 0x01", raw[0])
	}
}

// TestDHCP_NAK_OutsidePool verifies a REQUEST for an out-of-pool IP receives a NAK.
func TestDHCP_NAK_OutsidePool(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	req, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(net.ParseIP("10.0.0.5").To4())),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP("172.99.0.1").To4())),
	)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	reply := sendRecv(t, req)
	if reply.MessageType() != dhcpv4.MessageTypeNak {
		t.Errorf("got %s, want NAK for out-of-pool IP", reply.MessageType())
	}
}

// TestDHCP_NAK_StolenIP verifies a REQUEST for another client's active lease receives a NAK.
func TestDHCP_NAK_StolenIP(t *testing.T) {
	requireClient(t)
	macOwner := nextMAC()
	macThief := nextMAC()

	offer := sendRecv(t, makeDiscover(t, macOwner))
	ack := sendRecv(t, makeRequestFromOffer(t, offer))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("owner ACK: got %s", ack.MessageType())
	}
	stolenIP := ack.YourIPAddr
	t.Cleanup(func() { sendOnly(t, makeRelease(t, macOwner, stolenIP)) })

	req, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(macThief),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(stolenIP)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP("172.99.0.1").To4())),
	)
	if err != nil {
		t.Fatalf("build steal request: %v", err)
	}
	reply := sendRecv(t, req)
	if reply.MessageType() != dhcpv4.MessageTypeNak {
		t.Errorf("got %s, want NAK for stolen IP", reply.MessageType())
	}
}

// TestDHCP_BroadcastFlag verifies a DISCOVER with the broadcast flag set receives
// an OFFER (proving the server sent to 255.255.255.255:68, which reaches all clients
// on the Docker bridge).
func TestDHCP_BroadcastFlag(t *testing.T) {
	requireClient(t)
	mac := nextMAC()
	disc, err := dhcpv4.NewDiscovery(mac)
	if err != nil {
		t.Fatalf("build discover: %v", err)
	}
	disc.SetBroadcast()

	offer := sendRecv(t, disc)
	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("got %s, want OFFER", offer.MessageType())
	}
	if !inPool(offer.YourIPAddr) {
		t.Errorf("offered IP %s not in pool", offer.YourIPAddr)
	}
}

// TestDHCP_Persistence_AcrossRestart verifies leases survive a server restart.
// Requires the Docker socket to be mounted at /var/run/docker.sock (see
// docker-compose.dhcp-test.yml). Self-skips when the socket is absent.
func TestDHCP_Persistence_AcrossRestart(t *testing.T) {
	requireClient(t)
	if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
		t.Skip("Docker socket not mounted — persistence test requires /var/run/docker.sock")
	}

	mac := nextMAC()
	offer := sendRecv(t, makeDiscover(t, mac))
	ack := sendRecv(t, makeRequestFromOffer(t, offer))
	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Fatalf("initial ACK: got %s", ack.MessageType())
	}
	assignedIP := ack.YourIPAddr

	// Restart the server container (fixed name set in docker-compose.dhcp-test.yml).
	out, err := exec.Command("docker", "restart", "dhcp-e2e-server").CombinedOutput()
	if err != nil {
		t.Fatalf("docker restart dhcp-e2e-server: %v\n%s", err, out)
	}

	// Wait for the server to come back up.
	if err := waitForServer(15 * time.Second); err != nil {
		t.Fatalf("server did not recover after restart: %v", err)
	}

	// Same MAC must be re-offered the same IP (lease persisted to disk).
	offer2 := sendRecv(t, makeDiscover(t, mac))
	if offer2.MessageType() != dhcpv4.MessageTypeOffer {
		t.Fatalf("post-restart DISCOVER: got %s, want OFFER", offer2.MessageType())
	}
	if !offer2.YourIPAddr.Equal(assignedIP) {
		t.Errorf("post-restart offer = %s, want %s (lease was not persisted)", offer2.YourIPAddr, assignedIP)
	}

	// Clean up by completing the handshake then releasing.
	ack2 := sendRecv(t, makeRequestFromOffer(t, offer2))
	if ack2.MessageType() == dhcpv4.MessageTypeAck {
		sendOnly(t, makeRelease(t, mac, ack2.YourIPAddr))
	}
}
