package dhcp

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// EncodeCustomOption converts a CustomOption to raw bytes based on its Type field.
func EncodeCustomOption(opt CustomOption) ([]byte, error) {
	switch opt.Type {
	case "ip":
		ip := net.ParseIP(opt.Value).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid IPv4 address: %q", opt.Value)
		}
		return []byte(ip), nil
	case "string":
		return []byte(opt.Value), nil
	case "uint8":
		var v uint8
		if _, err := fmt.Sscanf(opt.Value, "%d", &v); err != nil {
			return nil, fmt.Errorf("invalid uint8 %q: %w", opt.Value, err)
		}
		return []byte{v}, nil
	case "uint16":
		var v uint16
		if _, err := fmt.Sscanf(opt.Value, "%d", &v); err != nil {
			return nil, fmt.Errorf("invalid uint16 %q: %w", opt.Value, err)
		}
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, v)
		return b, nil
	case "uint32":
		var v uint32
		if _, err := fmt.Sscanf(opt.Value, "%d", &v); err != nil {
			return nil, fmt.Errorf("invalid uint32 %q: %w", opt.Value, err)
		}
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		return b, nil
	case "bool":
		lower := strings.ToLower(opt.Value)
		if lower == "true" || lower == "1" {
			return []byte{0x01}, nil
		}
		return []byte{0x00}, nil
	default:
		return nil, fmt.Errorf("unknown option type: %q", opt.Type)
	}
}

func uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// encodeDNSName encodes a single domain name as RFC 1035 DNS labels (no compression).
// "foo.bar.com" → \x03foo\x03bar\x03com\x00
func encodeDNSName(domain string) []byte {
	var buf []byte
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			continue
		}
		buf = append(buf, byte(len(label)))
		buf = append(buf, label...)
	}
	buf = append(buf, 0)
	return buf
}

// BuildOptions builds a list of dhcpv4.Modifier from a DHCPConfig.
// Standard options are skipped if a custom option uses the same code.
// serverIP is the bound interface's IPv4 address (used for siaddr and option 54).
func BuildOptions(cfg DHCPConfig, serverIP net.IP) ([]dhcpv4.Modifier, error) {
	// Index custom option codes so standard options don't duplicate them.
	custom := make(map[uint8]bool)
	for _, o := range cfg.Options {
		custom[o.Code] = true
	}

	var mods []dhcpv4.Modifier

	// Option 1 — Subnet Mask
	if cfg.Mask != "" && !custom[1] {
		if ip := net.ParseIP(cfg.Mask).To4(); ip != nil {
			mods = append(mods, dhcpv4.WithNetmask(net.IPMask(ip)))
		}
	}

	// Option 3 — Router
	if cfg.Router != "" && !custom[3] {
		if ip := net.ParseIP(cfg.Router).To4(); ip != nil {
			mods = append(mods, dhcpv4.WithRouter(ip))
		}
	}

	// Option 6 — DNS Servers
	if len(cfg.DNS) > 0 && !custom[6] {
		var b []byte
		for _, s := range cfg.DNS {
			ip := net.ParseIP(s).To4()
			if ip == nil {
				return nil, fmt.Errorf("invalid DNS address: %q", s)
			}
			b = append(b, ip...)
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(6), b)))
	}

	// Option 15 — Domain Name
	if cfg.DomainName != "" && !custom[15] {
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(15), []byte(cfg.DomainName))))
	}

	// Option 42 — NTP Servers
	if len(cfg.NTP) > 0 && !custom[42] {
		var b []byte
		for _, s := range cfg.NTP {
			ip := net.ParseIP(s).To4()
			if ip == nil {
				return nil, fmt.Errorf("invalid NTP address: %q", s)
			}
			b = append(b, ip...)
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(42), b)))
	}

	// Option 44 — WINS / NetBIOS Name Server
	if len(cfg.WINS) > 0 && !custom[44] {
		var b []byte
		for _, s := range cfg.WINS {
			ip := net.ParseIP(s).To4()
			if ip == nil {
				return nil, fmt.Errorf("invalid WINS address: %q", s)
			}
			b = append(b, ip...)
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(44), b)))
	}

	// Options 51 / 58 / 59 — Lease Time, T1 (renewal), T2 (rebind)
	if cfg.LeaseTime > 0 {
		lease := time.Duration(cfg.LeaseTime) * time.Second
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(lease)))
		// RFC 2131 §4.4.5 recommended defaults: T1 = 0.5 × lease, T2 = 0.875 × lease
		if !custom[58] {
			mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(
				dhcpv4.GenericOptionCode(58),
				uint32ToBytes(uint32(lease/2/time.Second)),
			)))
		}
		if !custom[59] {
			mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(
				dhcpv4.GenericOptionCode(59),
				uint32ToBytes(uint32(lease*7/8/time.Second)),
			)))
		}
	}

	// Option 54 — Server Identifier (always set from serverIP).
	// siaddr (boot-server field) is set only when a boot file is configured; for
	// DHCP-only use RFC 2131 says siaddr should be 0 (it means "next bootstrap server",
	// not the DHCP server itself — that role belongs to option 54).
	if serverIP != nil && serverIP.To4() != nil {
		if cfg.BootFile != "" {
			mods = append(mods, dhcpv4.WithServerIP(serverIP))
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptServerIdentifier(serverIP)))
	}

	// Option 67 — Boot File Name
	if cfg.BootFile != "" && !custom[67] {
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(67), []byte(cfg.BootFile))))
	}

	// Option 119 — Domain Search List (RFC 3397)
	if len(cfg.DomainSearch) > 0 && !custom[119] {
		var b []byte
		for _, domain := range cfg.DomainSearch {
			b = append(b, encodeDNSName(domain)...)
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(119), b)))
	}

	// Option 150 — TFTP Server Address (RFC 5859, Cisco)
	if cfg.TFTPServer != "" && !custom[150] {
		ip := net.ParseIP(cfg.TFTPServer).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid TFTPServer address: %q", cfg.TFTPServer)
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(150), []byte(ip))))
	}

	// Custom options (appended last; override standard if same code)
	for _, o := range cfg.Options {
		b, err := EncodeCustomOption(o)
		if err != nil {
			return nil, fmt.Errorf("custom option %d: %w", o.Code, err)
		}
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(o.Code), b)))
	}

	return mods, nil
}
