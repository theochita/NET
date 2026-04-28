//go:build windows

package dhcp

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

// newServerConn creates a UDP socket with SO_BROADCAST enabled.
// The Windows server4 stub (conn_windows.go) skips this, so OFFER/ACK packets
// sent to 255.255.255.255 are silently rejected by the OS.
func newServerConn(addr *net.UDPAddr) (net.PacketConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
			})
		},
	}
	return lc.ListenPacket(context.Background(), "udp4", addr.String())
}
