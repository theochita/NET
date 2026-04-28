package syslog

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/theochita/NET/dhcp"
)

const defaultPort = 514
const ringCap = 1000

// Server manages the syslog receiver lifecycle.
type Server struct {
	mu        sync.RWMutex
	config    SyslogConfig
	configDir string
	conn      *net.UDPConn
	logger    *dhcp.EventLogger
	// ring is a fixed-capacity circular buffer. head points to the slot where
	// the next message will be written. count tracks how many slots are filled
	// (capped at ringCap). Newest message is at ring[(head-1+ringCap)%ringCap].
	ring  [ringCap]SyslogMessage
	head  int   // next write slot
	count int   // number of valid entries (0..ringCap)
	total int64 // monotonic session counter
	emit  func(SyslogMessage)

	cancel    context.CancelFunc
	startedAt time.Time
}

// NewServer creates a Server backed by configDir. Config is loaded from disk
// if present; zero-value config is used otherwise.
func NewServer(configDir string) (*Server, error) {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}
	cfg, err := loadConfig(configDir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		config:    cfg,
		configDir: configDir,
	}
	s.logger = dhcp.NewEventLogger(log.Default(), nil)
	return s, nil
}

// SetLogger replaces the logger. Must be called before Start.
func (s *Server) SetLogger(l *dhcp.EventLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = l
}

// SetEmitter sets the function called for each received message (Wails event hook).
func (s *Server) SetEmitter(fn func(SyslogMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emit = fn
}

// GetConfig returns the current configuration.
func (s *Server) GetConfig() SyslogConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SaveConfig persists cfg to disk and updates the in-memory config.
func (s *Server) SaveConfig(cfg SyslogConfig) error {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
	return saveConfig(s.configDir, cfg)
}

// Start binds the UDP socket and begins receiving messages.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil // already running
	}

	port := int(s.config.Port)
	if port == 0 {
		port = defaultPort
	}

	var bindIP net.IP
	if s.config.Interface != "" {
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Name != s.config.Interface {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ip4 := ipnet.IP.To4(); ip4 != nil {
						bindIP = ip4
						break
					}
				}
			}
		}
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: port})
	if err != nil {
		return err
	}
	return s.listenLocked(conn)
}

// listen acquires mu, then calls listenLocked. Used by tests that pre-bind a conn.
func (s *Server) listen(conn *net.UDPConn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listenLocked(conn)
}

// listenLocked wires up a pre-bound conn. Must be called with s.mu held.
func (s *Server) listenLocked(conn *net.UDPConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.conn = conn
	s.cancel = cancel
	s.startedAt = time.Now()
	s.total = 0
	s.head = 0
	s.count = 0

	// notifyCh decouples the hot receive path from logging/emitting.
	// Buffered so a burst of messages doesn't block packet ingestion.
	notifyCh := make(chan SyslogMessage, 256)
	go s.notifyLoop(ctx, notifyCh)
	go s.receiveLoop(ctx, conn, notifyCh)
	return nil
}

// Stop shuts down the listener. Safe to call when already stopped.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
	_ = s.conn.Close()
	s.conn = nil
	s.startedAt = time.Time{}
}

// IsRunning reports whether the server is currently accepting messages.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancel != nil
}

// StartedAt returns the time Start was last called, or zero if not running.
func (s *Server) StartedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

// GetMessages returns a snapshot of the ring buffer, newest first.
func (s *Server) GetMessages() []SyslogMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.count == 0 {
		return nil
	}
	out := make([]SyslogMessage, s.count)
	for i := range s.count {
		// newest is at head-1, then head-2, etc., wrapping around
		idx := (s.head - 1 - i + ringCap) % ringCap
		out[i] = s.ring[idx]
	}
	return out
}

// MessageCount returns the session total number of received messages.
// Accurate even when the ring has wrapped past its 1000-entry cap.
func (s *Server) MessageCount() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.total
}

// ClearMessages empties the ring and resets the session counter.
func (s *Server) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.head = 0
	s.count = 0
	s.total = 0
}

// receiveLoop reads UDP datagrams, updates the ring buffer, and signals notifyLoop.
func (s *Server) receiveLoop(ctx context.Context, conn *net.UDPConn, notifyCh chan<- SyslogMessage) {
	buf := make([]byte, 65536)
	for {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		msg := parseMessage(buf[:n], addr.String(), time.Now())

		s.mu.Lock()
		s.ring[s.head] = msg
		s.head = (s.head + 1) % ringCap
		if s.count < ringCap {
			s.count++
		}
		s.total++
		s.mu.Unlock()

		// Non-blocking send: drop the notification if the channel is full rather
		// than stalling the receive loop.
		select {
		case notifyCh <- msg:
		default:
		}
	}
}

// notifyLoop drains notifyCh and invokes the logger and emitter.
// Running in its own goroutine keeps logging/emitting off the hot receive path.
func (s *Server) notifyLoop(ctx context.Context, notifyCh <-chan SyslogMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-notifyCh:
			s.mu.RLock()
			logger := s.logger
			emit := s.emit
			s.mu.RUnlock()

			logger.Info(fmt.Sprintf("syslog from %s [%s]: %s", msg.Hostname, msg.Tag, msg.Message))
			if emit != nil {
				emit(msg)
			}
		}
	}
}
