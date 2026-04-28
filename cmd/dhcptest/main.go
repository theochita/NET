package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

const (
	serverPort = 67
	pktTimeout = 5 * time.Second
)

var (
	macA  = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}
	macA2 = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x02}
	macB  = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x03}
	macC  = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x04}
	macD  = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x05}
)

type runner struct {
	conn   net.PacketConn
	total  int
	passed int
	offer1 *dhcpv4.DHCPv4
	ack2   *dhcpv4.DHCPv4
}

func (r *runner) run(name string, fn func() (string, error)) {
	r.total++
	label := fmt.Sprintf("[%d/8]", r.total)
	detail, err := fn()
	if err != nil {
		fmt.Printf("%-6s %-25s FAIL  %s\n", label, name, err)
		return
	}
	r.passed++
	fmt.Printf("%-6s %-25s PASS  %s\n", label, name, detail)
}

func (r *runner) sendRecv(pkt *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: serverPort}
	if _, err := r.conn.WriteTo(pkt.ToBytes(), dst); err != nil {
		return nil, fmt.Errorf("send: %v", err)
	}
	deadline := time.Now().Add(pktTimeout)
	buf := make([]byte, 1500)
	for time.Now().Before(deadline) {
		r.conn.SetReadDeadline(deadline)
		n, _, err := r.conn.ReadFrom(buf)
		if err != nil {
			return nil, fmt.Errorf("recv timeout")
		}
		reply, err := dhcpv4.FromBytes(buf[:n])
		if err != nil {
			continue
		}
		if reply.TransactionID == pkt.TransactionID {
			return reply, nil
		}
	}
	return nil, fmt.Errorf("recv timeout")
}

func (r *runner) sendOnly(pkt *dhcpv4.DHCPv4) error {
	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: serverPort}
	_, err := r.conn.WriteTo(pkt.ToBytes(), dst)
	return err
}

func inPool(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4[0] == 172 && ip4[1] == 28 && ip4[2] == 0 &&
		ip4[3] >= 100 && ip4[3] <= 200
}

func main() {
	time.Sleep(1 * time.Second)

	conn, err := net.ListenPacket("udp4", "0.0.0.0:68")
	if err != nil {
		fmt.Fprintf(os.Stderr, "bind :68: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	r := &runner{conn: conn}

	// 1: DISCOVER → OFFER
	r.run("DISCOVER→OFFER", func() (string, error) {
		disc, err := dhcpv4.NewDiscovery(macA,
			dhcpv4.WithRequestedOptions(
				dhcpv4.OptionSubnetMask,
				dhcpv4.OptionRouter,
				dhcpv4.OptionDomainNameServer,
				dhcpv4.GenericOptionCode(42),
				dhcpv4.OptionIPAddressLeaseTime,
				dhcpv4.OptionServerIdentifier,
			),
		)
		if err != nil {
			return "", fmt.Errorf("build: %v", err)
		}
		reply, err := r.sendRecv(disc)
		if err != nil {
			return "", err
		}
		if reply.MessageType() != dhcpv4.MessageTypeOffer {
			return "", fmt.Errorf("got %s, want OFFER", reply.MessageType())
		}
		if !inPool(reply.YourIPAddr) {
			return "", fmt.Errorf("offered IP %s not in pool 172.28.0.100-200", reply.YourIPAddr)
		}
		r.offer1 = reply
		return fmt.Sprintf("offered=%s", reply.YourIPAddr), nil
	})

	// 2: REQUEST → ACK
	r.run("REQUEST→ACK", func() (string, error) {
		if r.offer1 == nil {
			return "", fmt.Errorf("skipped: scenario 1 failed")
		}
		req, err := dhcpv4.NewRequestFromOffer(r.offer1)
		if err != nil {
			return "", fmt.Errorf("build request: %v", err)
		}
		reply, err := r.sendRecv(req)
		if err != nil {
			return "", err
		}
		if reply.MessageType() != dhcpv4.MessageTypeAck {
			return "", fmt.Errorf("got %s, want ACK", reply.MessageType())
		}
		if !reply.YourIPAddr.Equal(r.offer1.YourIPAddr) {
			return "", fmt.Errorf("assigned %s, expected %s", reply.YourIPAddr, r.offer1.YourIPAddr)
		}
		r.ack2 = reply
		return fmt.Sprintf("assigned=%s", reply.YourIPAddr), nil
	})

	// 3: MAC re-offer
	r.run("MAC re-offer", func() (string, error) {
		if r.offer1 == nil {
			return "", fmt.Errorf("skipped: scenario 1 failed")
		}
		disc, err := dhcpv4.NewDiscovery(macA)
		if err != nil {
			return "", fmt.Errorf("build: %v", err)
		}
		reply, err := r.sendRecv(disc)
		if err != nil {
			return "", err
		}
		if reply.MessageType() != dhcpv4.MessageTypeOffer {
			return "", fmt.Errorf("got %s, want OFFER", reply.MessageType())
		}
		if !reply.YourIPAddr.Equal(r.offer1.YourIPAddr) {
			return "", fmt.Errorf("re-offered %s, expected same IP %s", reply.YourIPAddr, r.offer1.YourIPAddr)
		}
		return fmt.Sprintf("re-offered=%s", reply.YourIPAddr), nil
	})

	// 4: RELEASE + verify IP recycled
	r.run("RELEASE", func() (string, error) {
		if r.ack2 == nil {
			return "", fmt.Errorf("skipped: scenario 2 failed")
		}
		rel, err := dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeRelease),
			dhcpv4.WithClientIP(r.ack2.YourIPAddr),
		)
		if err != nil {
			return "", fmt.Errorf("build release: %v", err)
		}
		rel.ClientHWAddr = macA
		if err := r.sendOnly(rel); err != nil {
			return "", fmt.Errorf("send release: %v", err)
		}
		time.Sleep(150 * time.Millisecond)

		disc, err := dhcpv4.NewDiscovery(macC)
		if err != nil {
			return "", fmt.Errorf("build discover: %v", err)
		}
		reply, err := r.sendRecv(disc)
		if err != nil {
			return "", err
		}
		if reply.MessageType() != dhcpv4.MessageTypeOffer {
			return "", fmt.Errorf("got %s, want OFFER after release", reply.MessageType())
		}
		if !reply.YourIPAddr.Equal(r.ack2.YourIPAddr) {
			return "", fmt.Errorf("got %s, expected recycled IP %s", reply.YourIPAddr, r.ack2.YourIPAddr)
		}
		return fmt.Sprintf("ip recycled=%s", reply.YourIPAddr), nil
	})

	// 5: NAK on stolen IP
	r.run("NAK on stolen IP", func() (string, error) {
		disc, err := dhcpv4.NewDiscovery(macA2)
		if err != nil {
			return "", fmt.Errorf("build discover: %v", err)
		}
		offerA2, err := r.sendRecv(disc)
		if err != nil {
			return "", fmt.Errorf("discover for pre-assignment: %v", err)
		}
		if offerA2.MessageType() != dhcpv4.MessageTypeOffer {
			return "", fmt.Errorf("pre-assign: got %s, want OFFER", offerA2.MessageType())
		}
		reqA2, err := dhcpv4.NewRequestFromOffer(offerA2)
		if err != nil {
			return "", fmt.Errorf("build request: %v", err)
		}
		ackA2, err := r.sendRecv(reqA2)
		if err != nil {
			return "", fmt.Errorf("pre-assign request: %v", err)
		}
		if ackA2.MessageType() != dhcpv4.MessageTypeAck {
			return "", fmt.Errorf("pre-assign: got %s, want ACK", ackA2.MessageType())
		}
		stolenIP := ackA2.YourIPAddr

		reqB, err := dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
			dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(stolenIP)),
			dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP("172.28.0.1"))),
		)
		if err != nil {
			return "", fmt.Errorf("build steal request: %v", err)
		}
		reqB.ClientHWAddr = macB
		reply, err := r.sendRecv(reqB)
		if err != nil {
			return "", err
		}
		if reply.MessageType() != dhcpv4.MessageTypeNak {
			return "", fmt.Errorf("got %s, want NAK", reply.MessageType())
		}
		return fmt.Sprintf("got NAK for %s", stolenIP), nil
	})

	// 6: INFORM → ACK
	r.run("INFORM→ACK", func() (string, error) {
		clientIP := net.ParseIP("172.28.0.50").To4()
		inform, err := dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeInform),
			dhcpv4.WithClientIP(clientIP),
			dhcpv4.WithRequestedOptions(
				dhcpv4.OptionSubnetMask,
				dhcpv4.OptionRouter,
				dhcpv4.OptionDomainNameServer,
			),
		)
		if err != nil {
			return "", fmt.Errorf("build inform: %v", err)
		}
		inform.ClientHWAddr = macD
		reply, err := r.sendRecv(inform)
		if err != nil {
			return "", err
		}
		if reply.MessageType() != dhcpv4.MessageTypeAck {
			return "", fmt.Errorf("got %s, want ACK", reply.MessageType())
		}
		if !reply.YourIPAddr.Equal(net.IPv4zero) {
			return "", fmt.Errorf("YourIP=%s, want 0.0.0.0", reply.YourIPAddr)
		}
		router := reply.Router()
		if len(router) == 0 {
			return "", fmt.Errorf("router option missing in INFORM ACK")
		}
		return fmt.Sprintf("YourIP=0.0.0.0 router=%s", router[0]), nil
	})

	// 7: Options in ACK
	r.run("Options in ACK", func() (string, error) {
		if r.ack2 == nil {
			return "", fmt.Errorf("skipped: scenario 2 failed")
		}
		router := r.ack2.Router()
		if len(router) == 0 {
			return "", fmt.Errorf("no router option in ACK")
		}
		mask := r.ack2.SubnetMask()
		if mask == nil {
			return "", fmt.Errorf("no subnet mask option in ACK")
		}
		dns := r.ack2.DNS()
		if len(dns) == 0 {
			return "", fmt.Errorf("no DNS option in ACK")
		}
		return fmt.Sprintf("router=%s dns=%s", router[0], dns[0]), nil
	})

	// 8: Custom option 200 (IP) present in scenario 2's ACK
	r.run("Custom opt 200 IP", func() (string, error) {
		if r.ack2 == nil {
			return "", fmt.Errorf("skipped: scenario 2 failed")
		}
		raw := r.ack2.Options.Get(dhcpv4.GenericOptionCode(200))
		if len(raw) != 4 {
			return "", fmt.Errorf("option 200: got %d bytes, want 4", len(raw))
		}
		got := net.IP(raw).String()
		if got != "10.10.10.10" {
			return "", fmt.Errorf("option 200: got %s, want 10.10.10.10", got)
		}
		return fmt.Sprintf("opt200=%s", got), nil
	})

	fmt.Printf("\n%d/8 passed\n", r.passed)
	if r.passed < 8 {
		os.Exit(1)
	}
}
