package tftp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/theochita/NET/dhcp"
	"github.com/google/uuid"
	pintftp "github.com/pin/tftp/v3"
)

// Server manages the TFTP server lifecycle. It wraps pin/tftp/v3.
type Server struct {
	mu        sync.RWMutex
	config    TFTPConfig
	configDir string
	srv       *pintftp.Server
	conn      *net.UDPConn
	logger    *dhcp.EventLogger
	history   *History
	active    map[string]*Transfer
	emitXfer  func(Transfer)
	cancel    context.CancelFunc
	startedAt time.Time
}

// NewServer creates a Server backed by configDir. Config is loaded from disk
// if present; the zero-value config is used otherwise.
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
		history:   NewHistory(50),
		active:    make(map[string]*Transfer),
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

// SetTransferEmitter sets the function used to push Transfer updates to the
// frontend. Must be called before Start.
func (s *Server) SetTransferEmitter(fn func(Transfer)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitXfer = fn
}

// GetConfig returns a copy of the current in-memory config.
func (s *Server) GetConfig() TFTPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SaveConfig persists cfg to disk and updates the in-memory config.
func (s *Server) SaveConfig(cfg TFTPConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := saveConfig(s.configDir, cfg); err != nil {
		return err
	}
	s.config = cfg
	return nil
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

// GetActiveTransfers returns a snapshot of in-flight transfers.
func (s *Server) GetActiveTransfers() []Transfer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Transfer, 0, len(s.active))
	for _, t := range s.active {
		out = append(out, *t)
	}
	return out
}

// GetTransferHistory returns a snapshot of completed transfers, newest-first.
func (s *Server) GetTransferHistory() []Transfer {
	return s.history.Snapshot()
}

// ClearTransferHistory empties the history buffer.
func (s *Server) ClearTransferHistory() error {
	s.history.Clear()
	return nil
}

// Start binds UDP :69 on the configured interface and begins serving.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return fmt.Errorf("server already running")
	}
	if s.config.Root == "" {
		return fmt.Errorf("root directory not configured")
	}
	if err := os.MkdirAll(s.config.Root, 0755); err != nil {
		return fmt.Errorf("create root: %w", err)
	}

	bindIP := net.IPv4zero
	if s.config.Interface != "" {
		ip, err := interfaceIP(s.config.Interface)
		if err != nil {
			return fmt.Errorf("interface %q: %w", s.config.Interface, err)
		}
		bindIP = ip
	}
	laddr := &net.UDPAddr{IP: bindIP, Port: 69}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return fmt.Errorf("bind %s:69: %w", bindIP, err)
	}
	s.conn = conn

	srv := pintftp.NewServer(s.readHandler, s.writeHandler)
	srv.SetTimeout(5 * time.Second)
	if s.config.BlockSize > 0 {
		srv.SetBlockSize(int(s.config.BlockSize))
		// Disable MTU-based clamping so the configured cap is the effective cap.
		// With smartBlock on (pin/tftp default), every negotiated blksize is
		// clamped to intf.MTU - 48 (~1452 on 1500 MTU), ignoring SetBlockSize.
		srv.SetBlockSizeNegotiation(false)
	}
	s.srv = srv

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	srvLogger := s.logger
	go func() {
		if err := srv.Serve(conn); err != nil && ctx.Err() == nil {
			srvLogger.Error(fmt.Sprintf("tftp serve error: %v", err))
		}
	}()

	s.startedAt = time.Now()
	s.logger.Info(fmt.Sprintf("tftp server started on %s :69 (root=%s)", bindIP, s.config.Root))
	return nil
}

// startWithListener binds to the given address (e.g. "127.0.0.1:0") instead
// of the configured interface + :69. Used by tests to avoid privilege.
func (s *Server) startWithListener(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return fmt.Errorf("server already running")
	}
	if s.config.Root == "" {
		return fmt.Errorf("root not configured")
	}
	if err := os.MkdirAll(s.config.Root, 0755); err != nil {
		return err
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return err
	}
	s.conn = conn
	srv := pintftp.NewServer(s.readHandler, s.writeHandler)
	srv.SetTimeout(2 * time.Second)
	if s.config.BlockSize > 0 {
		srv.SetBlockSize(int(s.config.BlockSize))
		srv.SetBlockSizeNegotiation(false)
	}
	s.srv = srv

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	srvLogger := s.logger
	go func() {
		if err := srv.Serve(conn); err != nil && ctx.Err() == nil {
			srvLogger.Error(fmt.Sprintf("tftp serve error: %v", err))
		}
	}()
	s.startedAt = time.Now()
	return nil
}

// Stop shuts down the server. In-flight transfers are aborted.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
	s.startedAt = time.Time{}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	if s.srv != nil {
		s.srv.Shutdown()
		s.srv = nil
	}
	for id, t := range s.active {
		t.Status = "error"
		t.Error = "server stopped"
		t.EndedAt = time.Now()
		s.history.Add(*t)
		if s.emitXfer != nil {
			s.emitXfer(*t)
		}
		delete(s.active, id)
	}
	s.logger.Info("tftp server stopped")
}

func (s *Server) readHandler(filename string, rf io.ReaderFrom) error {
	s.mu.RLock()
	cfg := s.config
	logger := s.logger
	emit := s.emitXfer
	s.mu.RUnlock()

	if !cfg.ReadEnabled {
		logger.Warn(fmt.Sprintf("read denied (RRQ %s): reads disabled", filename))
		return fmt.Errorf("reads disabled")
	}

	peer := remoteAddr(rf)

	full, err := resolvePath(cfg.Root, filename)
	if err != nil {
		logger.Warn(fmt.Sprintf("read denied (RRQ %s): %v", filename, err))
		return err
	}
	f, err := os.Open(full)
	if err != nil {
		logger.Warn(fmt.Sprintf("RRQ %s: %v", filename, err))
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()
	if ot, ok := rf.(interface{ SetSize(int64) }); ok {
		ot.SetSize(size)
	}

	xfer := &Transfer{
		ID:        uuid.NewString(),
		Peer:      peer,
		Filename:  filename,
		Direction: "read",
		Size:      size,
		StartedAt: time.Now(),
		Status:    "active",
	}
	s.trackStart(xfer, emit)
	logger.Info(fmt.Sprintf("RRQ %s -> %s (%d bytes)", peer, filename, size))

	throttled := progressThrottle(size, func(b int64) {
		s.trackProgress(xfer.ID, b, emit)
	})
	cr := newCountingReader(f, throttled)
	written, err := rf.ReadFrom(cr)
	s.trackEnd(xfer.ID, written, err, emit)
	if err != nil {
		logger.Error(fmt.Sprintf("RRQ %s -> %s failed after %d bytes: %v", peer, filename, written, err))
	} else {
		logger.Info(fmt.Sprintf("RRQ %s -> %s ok (%d bytes)", peer, filename, written))
	}
	return err
}

func (s *Server) writeHandler(filename string, wt io.WriterTo) error {
	s.mu.RLock()
	cfg := s.config
	logger := s.logger
	emit := s.emitXfer
	s.mu.RUnlock()

	peer := remoteAddr(wt)

	if !cfg.WriteEnabled {
		logger.Warn(fmt.Sprintf("write denied (WRQ %s from %s): writes disabled", filename, peer))
		return fmt.Errorf("writes disabled")
	}

	full, err := resolvePath(cfg.Root, filename)
	if err != nil {
		logger.Warn(fmt.Sprintf("write denied (WRQ %s): %v", filename, err))
		return err
	}

	if st, err := os.Stat(full); err == nil && !st.IsDir() && st.Size() > 0 {
		logger.Warn(fmt.Sprintf("WRQ %s: refusing overwrite of existing file", filename))
		return fmt.Errorf("file already exists")
	}

	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		logger.Error(fmt.Sprintf("WRQ %s: create: %v", filename, err))
		return err
	}
	defer f.Close()

	var size int64
	if sizer, ok := wt.(interface{ Size() (int64, bool) }); ok {
		if v, known := sizer.Size(); known {
			size = v
		}
	}

	xfer := &Transfer{
		ID:        uuid.NewString(),
		Peer:      peer,
		Filename:  filename,
		Direction: "write",
		Size:      size,
		StartedAt: time.Now(),
		Status:    "active",
	}
	s.trackStart(xfer, emit)
	logger.Info(fmt.Sprintf("WRQ %s -> %s (%d bytes)", peer, filename, size))

	throttled := progressThrottle(size, func(b int64) {
		s.trackProgress(xfer.ID, b, emit)
	})
	cw := newCountingWriter(f, throttled)
	n, err := wt.WriteTo(cw)
	s.trackEnd(xfer.ID, n, err, emit)
	if err != nil {
		logger.Error(fmt.Sprintf("WRQ %s from %s failed after %d bytes: %v", filename, peer, n, err))
	} else {
		logger.Info(fmt.Sprintf("WRQ %s from %s ok (%d bytes)", filename, peer, n))
	}
	return err
}

// remoteAddr extracts the peer address from a pin/tftp transfer handle.
func remoteAddr(v any) string {
	if ra, ok := v.(interface{ RemoteAddr() net.UDPAddr }); ok {
		a := ra.RemoteAddr()
		return a.String()
	}
	if ra, ok := v.(interface{ RemoteAddr() *net.UDPAddr }); ok {
		a := ra.RemoteAddr()
		if a != nil {
			return a.String()
		}
	}
	return "unknown"
}

func (s *Server) trackStart(t *Transfer, emit func(Transfer)) {
	s.mu.Lock()
	s.active[t.ID] = t
	snap := *t
	s.mu.Unlock()
	if emit != nil {
		emit(snap)
	}
}

func (s *Server) trackProgress(id string, bytes int64, emit func(Transfer)) {
	s.mu.Lock()
	t, ok := s.active[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	t.Bytes = bytes
	snap := *t
	s.mu.Unlock()
	if emit != nil {
		emit(snap)
	}
}

func (s *Server) trackEnd(id string, bytes int64, err error, emit func(Transfer)) {
	s.mu.Lock()
	t, ok := s.active[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	t.Bytes = bytes
	t.EndedAt = time.Now()
	if err != nil {
		t.Status = "error"
		t.Error = err.Error()
	} else {
		t.Status = "ok"
	}
	snap := *t
	delete(s.active, id)
	s.history.Add(snap)
	s.mu.Unlock()
	if emit != nil {
		emit(snap)
	}
}

// interfaceIP returns the first IPv4 address on the named interface.
func interfaceIP(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip4 := ip.To4(); ip4 != nil {
			return ip4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address on interface %q", name)
}
