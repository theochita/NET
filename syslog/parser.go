package syslog

import (
	"net"
	"strconv"
	"strings"
	"time"
)

// parseMessage parses a raw RFC 3164 syslog datagram leniently.
// It never returns an error — unstructured content is stored in Raw so no
// message is silently dropped.
func parseMessage(data []byte, peer string, receivedAt time.Time) SyslogMessage {
	raw := strings.TrimRight(string(data), "\x00\r\n ")
	msg := SyslogMessage{
		ReceivedAt: receivedAt,
		Peer:       peer,
		Facility:   1, // user (RFC 3164 default)
		Severity:   6, // Info (RFC 3164 default)
		Raw:        raw,
	}

	peerIP, _, err := net.SplitHostPort(peer)
	if err != nil {
		peerIP = peer
	}

	s := raw

	// 1. PRI: optional <digits> at start, value 0–191.
	if len(s) > 2 && s[0] == '<' {
		if end := strings.IndexByte(s, '>'); end > 0 && end < 5 {
			if pri, err := strconv.Atoi(s[1:end]); err == nil && pri >= 0 && pri <= 191 {
				msg.Facility = pri >> 3
				msg.Severity = pri & 7
			}
			s = s[end+1:]
		}
	}

	// 2. Timestamp: "Mmm _D HH:MM:SS" or "Mmm DD HH:MM:SS" (always 15 chars).
	// Go's _2 format matches both " 4" (space-padded) and "14" (two-digit) days.
	if len(s) >= 16 {
		ts := s[:15]
		year := receivedAt.Year()
		for _, layout := range []string{"Jan _2 15:04:05", "Jan 02 15:04:05"} {
			if t, err := time.Parse(layout, ts); err == nil {
				msg.ReceivedAt = time.Date(year, t.Month(), t.Day(),
					t.Hour(), t.Minute(), t.Second(), 0, receivedAt.Location())
				s = s[16:] // timestamp (15) + trailing space (1)
				break
			}
		}
	}
	// 3. Hostname: skip the RFC 3164 hostname token (so it doesn't bleed into tag parsing),
	// but always report the source IP — avoids misidentifying key=value fields from
	// non-RFC-3164 devices as the hostname.
	if idx := strings.IndexByte(s, ' '); idx > 0 {
		if !strings.ContainsAny(s[:idx], ":%[]") {
			s = s[idx+1:]
		}
	}
	msg.Hostname = peerIP

	// 4. Tag: up to first ':', '[', or space.
	if idx := strings.IndexAny(s, ":[ "); idx > 0 {
		msg.Tag = s[:idx]
		s = s[idx:]
		if len(s) > 0 && s[0] == '[' {
			if end := strings.IndexByte(s, ']'); end >= 0 {
				s = s[end+1:]
			}
		}
		if len(s) > 0 && s[0] == ':' {
			s = s[1:]
		}
		msg.Message = strings.TrimPrefix(s, " ")
	} else {
		msg.Message = s
	}

	return msg
}
