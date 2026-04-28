//go:build !windows

package dhcp

import "net"

// newServerConn returns nil on non-Windows — the library's conn_unix.go
// already sets SO_BROADCAST, SO_REUSEADDR, and SO_REUSEPORT correctly.
func newServerConn(_ *net.UDPAddr) (net.PacketConn, error) {
	return nil, nil
}
