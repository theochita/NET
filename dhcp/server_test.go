package dhcp

import (
	"testing"
	"time"
)

func TestServer_StartedAt(t *testing.T) {
	dir := t.TempDir()
	s, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if !s.StartedAt().IsZero() {
		t.Fatalf("StartedAt before Start() = %v, want zero", s.StartedAt())
	}

	// Use an interface that always exists on Linux test hosts.
	s.SaveConfig(DHCPConfig{
		Interface: "lo",
		PoolStart: "127.0.0.1",
		PoolEnd:   "127.0.0.10",
		LeaseTime: 3600,
	})
	before := time.Now()
	if err := s.Start(); err != nil {
		t.Skipf("Start failed (expected in non-root CI): %v", err)
	}
	if s.StartedAt().IsZero() || s.StartedAt().Before(before) {
		t.Errorf("StartedAt after Start() = %v, want >= %v", s.StartedAt(), before)
	}
	s.Stop()
	if !s.StartedAt().IsZero() {
		t.Errorf("StartedAt after Stop() = %v, want zero", s.StartedAt())
	}
}
