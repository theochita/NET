package syslog

import (
	"testing"
	"time"
)

var refTime = time.Date(2024, 8, 4, 12, 0, 0, 0, time.UTC)

func TestParseMessage_WellFormed(t *testing.T) {
	raw := "<34>Aug  4 12:05:37 myhost myproc: connection accepted"
	msg := parseMessage([]byte(raw), "192.168.1.50:12345", refTime)
	// PRI 34 → facility=4 (auth), severity=2 (Critical)
	if msg.Facility != 4 {
		t.Errorf("facility = %d, want 4", msg.Facility)
	}
	if msg.Severity != 2 {
		t.Errorf("severity = %d, want 2", msg.Severity)
	}
	if msg.Hostname != "192.168.1.50" {
		t.Errorf("hostname = %q, want 192.168.1.50 (peer IP)", msg.Hostname)
	}
	if msg.Tag != "myproc" {
		t.Errorf("tag = %q, want myproc", msg.Tag)
	}
	if msg.Message != "connection accepted" {
		t.Errorf("message = %q, want %q", msg.Message, "connection accepted")
	}
}

func TestParseMessage_MissingPRI(t *testing.T) {
	raw := "Aug  4 12:05:37 myhost myproc: hello"
	msg := parseMessage([]byte(raw), "10.0.0.1:514", refTime)
	if msg.Severity != 6 {
		t.Errorf("severity = %d, want 6 (Info default)", msg.Severity)
	}
	if msg.Facility != 1 {
		t.Errorf("facility = %d, want 1 (user default)", msg.Facility)
	}
}

func TestParseMessage_MalformedTimestamp(t *testing.T) {
	raw := "<13>BADTIME myhost myproc: hello"
	msg := parseMessage([]byte(raw), "10.0.0.1:514", refTime)
	if !msg.ReceivedAt.Equal(refTime) {
		t.Errorf("ReceivedAt = %v, want %v (receive time fallback)", msg.ReceivedAt, refTime)
	}
	if msg.Raw == "" {
		t.Error("Raw must be preserved")
	}
}

func TestParseMessage_HostnameIsPeerIP(t *testing.T) {
	// Hostname field in message is ignored; peer IP is always used.
	raw := "<30>Aug  4 12:05:37 myhost myproc: hello"
	msg := parseMessage([]byte(raw), "10.0.0.2:514", refTime)
	if msg.Hostname != "10.0.0.2" {
		t.Errorf("hostname = %q, want peer IP 10.0.0.2", msg.Hostname)
	}
}

func TestParseMessage_CiscoTag(t *testing.T) {
	raw := "<190>Aug  4 12:05:37 sw-core-01 %SYS-5-CONFIG_I: Configured from console"
	msg := parseMessage([]byte(raw), "192.168.1.10:514", refTime)
	if msg.Hostname != "192.168.1.10" {
		t.Errorf("hostname = %q, want 192.168.1.10 (peer IP)", msg.Hostname)
	}
	if msg.Tag != "%SYS-5-CONFIG_I" {
		t.Errorf("tag = %q, want %%SYS-5-CONFIG_I", msg.Tag)
	}
	if msg.Message != "Configured from console" {
		t.Errorf("message = %q, want %q", msg.Message, "Configured from console")
	}
}

func TestParseMessage_PeerPortStripped(t *testing.T) {
	raw := "<13>Aug  4 12:05:37 %TAG: hi"
	msg := parseMessage([]byte(raw), "10.0.0.99:54321", refTime)
	if msg.Hostname != "10.0.0.99" {
		t.Errorf("hostname = %q, want 10.0.0.99 (no port)", msg.Hostname)
	}
}

func TestParseMessage_EmptyDatagram(t *testing.T) {
	// Must not panic.
	msg := parseMessage([]byte{}, "10.0.0.1:514", refTime)
	_ = msg
}
