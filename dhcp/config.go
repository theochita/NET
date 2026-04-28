package dhcp

import "time"

// No JSON tags: Wails uses encoding/json which defaults to the exact field name
// (PascalCase). Vue components access these as cfg.Interface, cfg.PoolStart, etc.

type DHCPConfig struct {
	Interface    string
	PoolStart    string
	PoolEnd      string
	LeaseTime    uint32         // seconds
	Router       string
	Mask         string
	DNS          []string
	NTP          []string
	DomainName   string
	DomainSearch []string // RFC 3397 option 119
	WINS         []string
	BootFile     string
	TFTPServer   string   // RFC 5859 option 150 (Cisco TFTP server address)
	Options      []CustomOption
}

type CustomOption struct {
	Code  uint8
	Type  string // "ip", "string", "uint8", "uint16", "uint32", "bool"
	Value string // human-readable; encoded at server start
}

type Lease struct {
	MAC       string
	ClientID  string // RFC 2132 option 61 value as hex; empty when lease is keyed by MAC
	IP        string
	Hostname  string
	ExpiresAt time.Time
}

type Interface struct {
	Name  string
	IPs   []string
	Masks []string // parallel to IPs; e.g. "255.255.255.0"
}
