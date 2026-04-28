package syslog

import "time"

// No JSON tags: Wails serialises field names as-is (PascalCase).

// SyslogConfig is the persisted server configuration.
type SyslogConfig struct {
	Interface string // "" = all interfaces
	Port      uint16 // 0 defaults to 514
}

// SyslogMessage is a single parsed syslog datagram.
type SyslogMessage struct {
	ReceivedAt time.Time
	Peer       string // source "ip:port"
	Hostname   string // from message body; falls back to peer IP
	Facility   int    // 0–23
	Severity   int    // 0=Emergency … 7=Debug
	Tag        string // process name or Cisco facility tag
	Message    string // message body
	Raw        string // original datagram (preserved for unparseable messages)
}
