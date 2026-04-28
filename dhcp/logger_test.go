package dhcp

import (
	"io"
	"log"
	"testing"
)

func TestEventLogger_DispatchesLevels(t *testing.T) {
	var got []LogEntry
	l := NewEventLogger(log.New(io.Discard, "", 0), func(e LogEntry) {
		got = append(got, e)
	})

	l.Info("server started")
	l.Packet("DISCOVER mac=aa:bb:cc")
	l.Warn("pool low")
	l.Error("bind failed")

	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(got))
	}
	cases := []struct{ level, msg string }{
		{"INFO", "server started"},
		{"PKT", "DISCOVER mac=aa:bb:cc"},
		{"WARN", "pool low"},
		{"ERROR", "bind failed"},
	}
	for i, c := range cases {
		if got[i].Level != c.level {
			t.Errorf("entry %d: level = %q, want %q", i, got[i].Level, c.level)
		}
		if got[i].Message != c.msg {
			t.Errorf("entry %d: message = %q, want %q", i, got[i].Message, c.msg)
		}
		if got[i].Time == "" {
			t.Errorf("entry %d: Time is empty", i)
		}
	}
}

func TestEventLogger_NilEmitDoesNotPanic(t *testing.T) {
	l := NewEventLogger(log.New(io.Discard, "", 0), nil)
	l.Info("ok")
	l.Packet("ok")
	l.Warn("ok")
	l.Error("ok")
}
