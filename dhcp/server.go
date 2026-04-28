package dhcp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

const configFilename = "config.json"

// defaultLeaseTime is applied when DHCPConfig.LeaseTime is zero so that the
// option-51 value sent to clients matches the lease duration recorded in the
// store. RFC 2131 has no defined default; 24h is a common Tftpd64/dnsmasq value.
const defaultLeaseTime = 24 * time.Hour

// Server manages the DHCP server lifecycle and delegates to LeaseStore.
type Server struct {
	mu        sync.RWMutex
	store     *LeaseStore
	srv       *server4.Server
	config    DHCPConfig
	configDir string
	cancel    context.CancelFunc
	startedAt time.Time
	modifiers []dhcpv4.Modifier // pre-built per Start(); nil when stopped
	logger    *EventLogger
}

// NewServer creates a Server backed by configDir.
// Config is loaded from disk if present; defaults used otherwise.
func NewServer(configDir string) (*Server, error) {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}
	store, err := NewLeaseStore(configDir)
	if err != nil {
		return nil, err
	}
	s := &Server{store: store, configDir: configDir}
	s.logger = NewEventLogger(log.Default(), nil)
	if err := s.loadConfig(); err != nil && !os.IsNotExist(err) {
		s.logger.Warn(fmt.Sprintf("loading config: %v", err))
	}
	return s, nil
}

// SetLogger replaces the logger. Must be called before Start.
func (s *Server) SetLogger(l *EventLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = l
}

// GetConfig returns a copy of the current in-memory config (read-locked).
func (s *Server) GetConfig() DHCPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SaveConfig persists cfg to disk and updates in-memory config (write-locked).
func (s *Server) SaveConfig(cfg DHCPConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := atomicWriteFile(filepath.Join(s.configDir, configFilename), data); err != nil {
		return err
	}
	s.config = cfg
	return nil
}

// Start binds UDP :67 on the configured interface and begins serving.
// Returns an error if already running or if config is invalid.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return fmt.Errorf("server already running")
	}

	poolStart := net.ParseIP(s.config.PoolStart).To4()
	poolEnd := net.ParseIP(s.config.PoolEnd).To4()
	if poolStart == nil || poolEnd == nil {
		return fmt.Errorf("invalid pool range: %q – %q", s.config.PoolStart, s.config.PoolEnd)
	}

	if ipAfter(poolStart, poolEnd) {
		return fmt.Errorf("pool start must be before pool end")
	}

	if s.config.Interface == "" {
		return fmt.Errorf("no interface selected")
	}

	serverIP, ifaceMask, err := interfaceIPAndMask(s.config.Interface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", s.config.Interface, err)
	}

	// Apply lease-time default before BuildOptions so the wire-level option 51
	// matches what the lease store records. Handler also falls back to this.
	if s.config.LeaseTime == 0 {
		s.config.LeaseTime = uint32(defaultLeaseTime / time.Second)
	}

	mods, err := BuildOptions(s.config, serverIP)
	if err != nil {
		return fmt.Errorf("build options: %w", err)
	}
	s.modifiers = mods

	laddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 67}

	var serverOpts []server4.ServerOpt
	if pc, err := newServerConn(laddr); err != nil {
		s.modifiers = nil
		return fmt.Errorf("listen udp: %w", err)
	} else if pc != nil {
		serverOpts = append(serverOpts, server4.WithConn(pc))
	}

	srv, err := server4.NewServer(s.config.Interface, laddr, s.makeHandler(serverIP, poolStart, poolEnd, ifaceMask), serverOpts...)
	if err != nil {
		s.modifiers = nil
		return err
	}
	s.srv = srv

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Expiry sweeper
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.store.Sweep()
			}
		}
	}()

	// Packet server — capture logger while mu is still held (released above)
	srvLogger := s.logger
	go func() {
		if err := srv.Serve(); err != nil && ctx.Err() == nil {
			srvLogger.Error(fmt.Sprintf("server error: %v", err))
		}
	}()

	s.startedAt = time.Now()
	s.logger.Info(fmt.Sprintf("server started on %s :67 (offer dst=%s:68)", s.config.Interface, subnetBroadcastFromMask(serverIP, ifaceMask)))
	return nil
}

// Stop shuts down the server. Leases are kept on disk.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
	if s.srv != nil {
		s.srv.Close()
		s.srv = nil
	}
	s.modifiers = nil
	s.startedAt = time.Time{}
	s.logger.Info("server stopped")
}

// IsRunning reports whether the server is currently serving.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancel != nil
}

// StartedAt returns the time Start() succeeded, or the zero value if stopped.
func (s *Server) StartedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

// GetLeases returns a snapshot of all current leases.
func (s *Server) GetLeases() []Lease {
	return s.store.GetAll()
}

// ClearLeases removes all leases from memory and disk.
func (s *Server) ClearLeases() error {
	return s.store.Clear()
}

// replyDest returns the RFC 2131 §4.1 destination for a server-to-client reply:
//   - relay agent present  → giaddr:67
//   - client renewing      → ciaddr:68 (unicast)
//   - broadcast flag set   → 255.255.255.255:68 (required for unconfigured clients, e.g. Cisco IOS)
//   - fallback             → subnetBcast (rare: ciaddr=0 and broadcast flag unset)
func replyDest(req *dhcpv4.DHCPv4, subnetBcast *net.UDPAddr) *net.UDPAddr {
	if !req.GatewayIPAddr.IsUnspecified() {
		return &net.UDPAddr{IP: req.GatewayIPAddr, Port: 67}
	}
	if !req.ClientIPAddr.IsUnspecified() {
		return &net.UDPAddr{IP: req.ClientIPAddr, Port: 68}
	}
	if req.IsBroadcast() {
		return &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	}
	return subnetBcast
}

// leaseKey returns (clientKey, mac) for a DHCP request.
// If option 61 (Client Identifier) is present it becomes the lease-store key
// ("id:<hex>") so devices that send a stable client-id get consistent leases
// even when their MAC changes (e.g. VMs, iDRAC/iLO, managed switches).
// mac is always the chaddr hardware address, used for display and IP-conflict checks.
func leaseKey(req *dhcpv4.DHCPv4) (clientKey, mac string) {
	mac = req.ClientHWAddr.String()
	if cid := req.Options.Get(dhcpv4.GenericOptionCode(61)); len(cid) > 0 {
		return "id:" + hex.EncodeToString(cid), mac
	}
	return mac, mac
}

// parseRelayInfo extracts circuit-id and remote-id sub-options from RFC 3046
// Relay Agent Information option (option 82) data, returned as hex strings.
func parseRelayInfo(data []byte) (circuitID, remoteID string) {
	for i := 0; i+2 <= len(data); {
		t, l := data[i], int(data[i+1])
		i += 2
		if i+l > len(data) {
			break
		}
		val := data[i : i+l]
		switch t {
		case 1:
			circuitID = hex.EncodeToString(val)
		case 2:
			remoteID = hex.EncodeToString(val)
		}
		i += l
	}
	return
}

func (s *Server) makeHandler(serverIP, poolStart, poolEnd net.IP, ifaceMask net.IPMask) server4.Handler {
	store := s.store
	// Subnet-directed broadcast used as the fallback destination (ciaddr=0, broadcast flag unset).
	// On multi-NIC Windows hosts 255.255.255.255 can route via the wrong interface, so the
	// subnet broadcast provides a reliable fallback. Most clients set the broadcast flag on
	// initial acquire, so replyDest() will return 255.255.255.255 for them (RFC 2131 §4.1).
	// Use the INTERFACE's actual mask — not s.config.Mask (the DHCP scope mask) — so the
	// broadcast address matches the physical subnet the server is on.
	bcast := &net.UDPAddr{IP: subnetBroadcastFromMask(serverIP, ifaceMask), Port: 68}
	// NAKs must be broadcast to 255.255.255.255 (RFC 2131 §4.3.2) — client IP
	// state is undefined when we NAK, so subnet broadcast won't always reach it.
	nakAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	return func(conn net.PacketConn, peer net.Addr, req *dhcpv4.DHCPv4) {
		// take a local copy of modifiers under a read lock — handler runs in a separate goroutine
		s.mu.RLock()
		localMods := s.modifiers
		duration := time.Duration(s.config.LeaseTime) * time.Second
		logger := s.logger
		s.mu.RUnlock()

		if duration == 0 {
			duration = defaultLeaseTime
		}
		if localMods == nil {
			// Server was stopped between makeHandler registration and packet arrival
			return
		}

		clientKey, mac := leaseKey(req)
		logLine := fmt.Sprintf("type=%s peer=%s mac=%s", req.MessageType(), peer, req.ClientHWAddr)
		if clientKey != mac {
			logLine += fmt.Sprintf(" client-id=%s", clientKey[3:])
		}
		// RFC 3046: log relay agent info (option 82) when present
		if opt82 := req.Options.Get(dhcpv4.GenericOptionCode(82)); len(opt82) > 0 {
			circuit, remote := parseRelayInfo(opt82)
			logLine += fmt.Sprintf(" relay-circuit=%s relay-remote=%s", circuit, remote)
		}
		logger.Packet(logLine)

		switch req.MessageType() {

		case dhcpv4.MessageTypeDiscover:
			// Re-offer the existing lease for the same client if still active,
			// but never re-offer a DECLINED hold (client rejected that IP).
			var offerIP net.IP
			if existing, ok := store.GetByClient(clientKey); ok && existing.Hostname != "DECLINED" {
				offerIP = net.ParseIP(existing.IP).To4()
			}
			if offerIP == nil {
				// RFC 2131 §4.3.1: honor the client's preferred IP (option 50) if it
				// is within the pool and not already held by another client.
				if pref := req.RequestedIPAddress(); pref != nil {
					if p4 := pref.To4(); p4 != nil && !ipAfter(poolStart, p4) && !ipAfter(p4, poolEnd) {
						if held, inUse := store.IsAssigned(pref.String()); !inUse || held.MAC == mac {
							offerIP = p4
						}
					}
				}
			}
			if offerIP == nil {
				offerIP = store.NextFreeIP(poolStart, poolEnd)
			}
			if offerIP == nil {
				logger.Warn("pool exhausted")
				return
			}
			mods := append([]dhcpv4.Modifier{
				dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
				dhcpv4.WithYourIP(offerIP),
			}, localMods...)
			reply, err := dhcpv4.NewReplyFromRequest(req, mods...)
			if err != nil {
				logger.Error(fmt.Sprintf("create offer: %v", err))
				return
			}
			dst := replyDest(req, bcast)
			if _, err := conn.WriteTo(reply.ToBytes(), dst); err != nil {
				logger.Error(fmt.Sprintf("write offer: %v", err))
			} else {
				logger.Info(fmt.Sprintf("OFFER sent: ip=%s dst=%s mac=%s", offerIP, dst, mac))
			}

		case dhcpv4.MessageTypeRequest:
			// RFC 2131 §4.3.2: if option 54 (server identifier) is present and does
			// not match our IP, the client chose another server — silently discard.
			if sid := req.ServerIdentifier(); sid != nil && !sid.Equal(serverIP) {
				return
			}
			requested := req.RequestedIPAddress()
			if requested == nil || requested.IsUnspecified() {
				requested = req.ClientIPAddr
			}
			req4 := requested.To4()
			if req4 == nil || ipAfter(poolStart, req4) || ipAfter(req4, poolEnd) {
				reply, err := dhcpv4.NewReplyFromRequest(req, dhcpv4.WithMessageType(dhcpv4.MessageTypeNak))
				if err == nil {
					if _, err := conn.WriteTo(reply.ToBytes(), nakAddr); err != nil {
						logger.Error(fmt.Sprintf("write nak: %v", err))
					}
				}
				return
			}
			if existing, ok := store.IsAssigned(requested.String()); ok && existing.MAC != mac {
				// IP in use by another client — send NAK
				reply, err := dhcpv4.NewReplyFromRequest(req, dhcpv4.WithMessageType(dhcpv4.MessageTypeNak))
				if err == nil {
					if _, err := conn.WriteTo(reply.ToBytes(), nakAddr); err != nil {
						logger.Error(fmt.Sprintf("write nak: %v", err))
					}
				}
				return
			}
			hn := req.HostName()
			if hn == "" {
				if old, ok := store.GetByClient(clientKey); ok {
					hn = old.Hostname
				}
			}
			store.Assign(clientKey, mac, requested.String(), hn, duration)
			mods := append([]dhcpv4.Modifier{
				dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
				dhcpv4.WithYourIP(requested),
			}, localMods...)
			reply, err := dhcpv4.NewReplyFromRequest(req, mods...)
			if err != nil {
				logger.Error(fmt.Sprintf("create ack: %v", err))
				return
			}
			if _, err := conn.WriteTo(reply.ToBytes(), replyDest(req, bcast)); err != nil {
				logger.Error(fmt.Sprintf("write ack: %v", err))
			}

		case dhcpv4.MessageTypeRelease:
			store.Release(clientKey)

		case dhcpv4.MessageTypeDecline:
			// Hold the declined IP for the full lease duration to avoid re-offering it.
			// Ignore declines for IPs outside our pool — clients could decline anything.
			if ip := req.RequestedIPAddress(); ip != nil {
				if ip4 := ip.To4(); ip4 != nil && !ipAfter(poolStart, ip4) && !ipAfter(ip4, poolEnd) {
					store.Assign(clientKey, mac, ip.String(), "DECLINED", duration)
				}
			}

		case dhcpv4.MessageTypeInform:
			reply, err := dhcpv4.NewReplyFromRequest(req,
				dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			)
			if err != nil {
				return
			}
			for _, m := range localMods {
				m(reply)
			}
			// Do not assign an IP — client keeps its own address
			reply.YourIPAddr = net.IPv4zero
			if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
				logger.Error(fmt.Sprintf("INFORM reply error: %v", err))
			}
		}
	}
}

func (s *Server) loadConfig() error {
	data, err := os.ReadFile(filepath.Join(s.configDir, configFilename))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.config)
}

// interfaceIPAndMask returns the first IPv4 address and its network mask on the named interface.
func interfaceIPAndMask(name string) (net.IP, net.IPMask, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, err
	}
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			if ip4 := v.IP.To4(); ip4 != nil {
				mask4 := v.Mask
				if len(mask4) == 16 {
					mask4 = mask4[12:]
				}
				return ip4, mask4, nil
			}
		case *net.IPAddr:
			if ip4 := v.IP.To4(); ip4 != nil {
				return ip4, nil, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no IPv4 address found on interface %q", name)
}

// interfaceIP returns the first IPv4 address on the named interface.
func interfaceIP(name string) (net.IP, error) {
	ip, _, err := interfaceIPAndMask(name)
	return ip, err
}

// subnetBroadcastFromMask returns the directed broadcast for serverIP using the
// given net.IPMask (e.g. 192.168.122.255 for 192.168.122.1/255.255.255.0).
// Falls back to 255.255.255.255 when either input is nil/wrong length.
func subnetBroadcastFromMask(serverIP net.IP, mask net.IPMask) net.IP {
	ip4 := serverIP.To4()
	if ip4 == nil || len(mask) != 4 {
		return net.IPv4bcast
	}
	subnet := ip4.Mask(mask)
	if subnet == nil {
		return net.IPv4bcast
	}
	bcast := make(net.IP, 4)
	for i := range bcast {
		bcast[i] = subnet[i] | ^mask[i]
	}
	return bcast
}
