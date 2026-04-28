package syslog

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// newTestServer creates a Server bound to a random port on 127.0.0.1.
// The bound address is returned so tests can send datagrams to it.
func newTestServer(t *testing.T) (*Server, *net.UDPAddr) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	addr := conn.LocalAddr().(*net.UDPAddr)
	if err := s.listen(conn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	return s, addr
}

func sendUDP(t *testing.T, addr *net.UDPAddr, msg string) {
	t.Helper()
	c, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within 500ms")
}

func TestServer_ReceivesMessage(t *testing.T) {
	s, addr := newTestServer(t)
	sendUDP(t, addr, "<34>Aug  4 12:05:37 myhost myproc: hello test")

	waitFor(t, func() bool { return len(s.GetMessages()) >= 1 })

	msgs := s.GetMessages()
	if msgs[0].Hostname != "127.0.0.1" {
		t.Errorf("hostname = %q, want 127.0.0.1 (peer IP)", msgs[0].Hostname)
	}
	if msgs[0].Message != "hello test" {
		t.Errorf("message = %q, want %q", msgs[0].Message, "hello test")
	}
}

func TestServer_NewestFirst(t *testing.T) {
	s, addr := newTestServer(t)
	sendUDP(t, addr, "<6>Aug  4 12:00:00 h1 t: first")
	sendUDP(t, addr, "<6>Aug  4 12:00:01 h2 t: second")

	waitFor(t, func() bool { return len(s.GetMessages()) >= 2 })

	msgs := s.GetMessages()
	if msgs[0].Message != "second" {
		t.Errorf("expected newest first: got message %q, want second", msgs[0].Message)
	}
}

func TestServer_RingCapEnforced(t *testing.T) {
	s, addr := newTestServer(t)
	c, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for i := 0; i < 1001; i++ {
		c.Write([]byte(fmt.Sprintf("<6>Aug  4 12:00:00 host t: msg %d", i)))
	}

	waitFor(t, func() bool { return s.MessageCount() >= 1001 })

	if got := len(s.GetMessages()); got != 1000 {
		t.Errorf("ring cap: got %d messages, want 1000", got)
	}
}

func TestServer_MessageCount(t *testing.T) {
	s, addr := newTestServer(t)
	for i := 0; i < 5; i++ {
		sendUDP(t, addr, fmt.Sprintf("<6>Aug  4 12:00:00 h t: msg %d", i))
	}

	waitFor(t, func() bool { return s.MessageCount() >= 5 })

	if got := s.MessageCount(); got != 5 {
		t.Errorf("MessageCount = %d, want 5", got)
	}
}

func TestServer_ClearMessages(t *testing.T) {
	s, addr := newTestServer(t)
	sendUDP(t, addr, "<6>Aug  4 12:00:00 host t: msg")
	waitFor(t, func() bool { return len(s.GetMessages()) >= 1 })

	s.ClearMessages()

	if len(s.GetMessages()) != 0 {
		t.Error("expected empty ring after clear")
	}
	if s.MessageCount() != 0 {
		t.Error("expected zero count after clear")
	}
}

func TestServer_ConfigPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.SaveConfig(SyslogConfig{Port: 1514}); err != nil {
		t.Fatal(err)
	}

	s2, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s2.GetConfig().Port != 1514 {
		t.Errorf("config not persisted: port = %d, want 1514", s2.GetConfig().Port)
	}
}

func TestServer_IsRunning(t *testing.T) {
	s, _ := newTestServer(t)
	if !s.IsRunning() {
		t.Error("expected IsRunning = true after listen")
	}
	s.Stop()
	if s.IsRunning() {
		t.Error("expected IsRunning = false after Stop")
	}
}
