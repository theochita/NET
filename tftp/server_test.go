package tftp

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/theochita/NET/dhcp"
	pintftp "github.com/pin/tftp/v3"
)

type eventSink struct {
	mu     sync.Mutex
	events []Transfer
}

func (e *eventSink) add(t Transfer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, t)
}

func (e *eventSink) snapshot() []Transfer {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Transfer, len(e.events))
	copy(out, e.events)
	return out
}

func startServerForTest(t *testing.T, cfg TFTPConfig) (*Server, string, *eventSink) {
	t.Helper()
	configDir := t.TempDir()
	cfg.Interface = ""
	if cfg.Root == "" {
		cfg.Root = t.TempDir()
	}
	if !cfg.ReadEnabled && !cfg.WriteEnabled {
		cfg.ReadEnabled = true
		cfg.WriteEnabled = true
	}

	s, err := NewServer(configDir)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	s.SetLogger(dhcp.NewEventLogger(log.New(io.Discard, "", 0), nil))

	sink := &eventSink{}
	s.SetTransferEmitter(sink.add)

	if err := s.startWithListener("127.0.0.1:0"); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { s.Stop() })
	return s, s.conn.LocalAddr().String(), sink
}

func TestServer_ReadRoundtrip(t *testing.T) {
	root := t.TempDir()
	content := []byte("hello tftp world")
	if err := os.WriteFile(filepath.Join(root, "greeting.txt"), content, 0644); err != nil {
		t.Fatal(err)
	}
	_, addr, sink := startServerForTest(t, TFTPConfig{Root: root, ReadEnabled: true, WriteEnabled: true})

	c, err := pintftp.NewClient(addr)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := c.Receive("greeting.txt", "octet")
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if _, err := wt.WriteTo(&got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), content) {
		t.Errorf("got %q, want %q", got.Bytes(), content)
	}

	time.Sleep(50 * time.Millisecond)
	events := sink.snapshot()
	if len(events) == 0 {
		t.Fatal("no transfer events emitted")
	}
	last := events[len(events)-1]
	if last.Status != "ok" || last.Direction != "read" {
		t.Errorf("last event wrong: %+v", last)
	}
}

func TestServer_WriteRoundtrip(t *testing.T) {
	root := t.TempDir()
	_, addr, _ := startServerForTest(t, TFTPConfig{Root: root, ReadEnabled: true, WriteEnabled: true})

	c, err := pintftp.NewClient(addr)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := c.Send("uploaded.bin", "octet")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("upload payload")
	if _, err := rf.ReadFrom(bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	got, err := os.ReadFile(filepath.Join(root, "uploaded.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestServer_WriteDisabled(t *testing.T) {
	root := t.TempDir()
	_, addr, _ := startServerForTest(t, TFTPConfig{Root: root, ReadEnabled: true, WriteEnabled: false})

	c, err := pintftp.NewClient(addr)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := c.Send("nope.bin", "octet")
	if err != nil {
		return
	}
	_, err = rf.ReadFrom(bytes.NewReader([]byte("x")))
	if err == nil {
		t.Fatal("expected write to fail with writes disabled, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "disabled") &&
		!strings.Contains(strings.ToLower(err.Error()), "denied") {
		t.Logf("error was %v (acceptable — any non-nil error counts)", err)
	}
}

func TestServer_OverwriteRejected(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "protected.bin")
	if err := os.WriteFile(target, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	_, addr, _ := startServerForTest(t, TFTPConfig{Root: root, ReadEnabled: true, WriteEnabled: true})

	c, _ := pintftp.NewClient(addr)
	rf, err := c.Send("protected.bin", "octet")
	if err != nil {
		return
	}
	_, err = rf.ReadFrom(bytes.NewReader([]byte("new")))
	if err == nil {
		t.Fatal("expected overwrite rejection, got nil")
	}

	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("file was overwritten: %q", got)
	}
}

func TestServer_StartedAt(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	s, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.SaveConfig(TFTPConfig{
		Root:         root,
		ReadEnabled:  true,
		WriteEnabled: true,
	})
	if !s.StartedAt().IsZero() {
		t.Fatalf("StartedAt before Start() = %v, want zero", s.StartedAt())
	}
	before := time.Now()
	if err := s.startWithListener("127.0.0.1:0"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if s.StartedAt().IsZero() || s.StartedAt().Before(before) {
		t.Errorf("StartedAt after Start() = %v, want >= %v", s.StartedAt(), before)
	}
	s.Stop()
	if !s.StartedAt().IsZero() {
		t.Errorf("StartedAt after Stop() = %v, want zero", s.StartedAt())
	}
}
