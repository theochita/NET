// Package tftp implements a minimal TFTP server (RFC 1350 + RFC 2347/2348/2349
// options) built on github.com/pin/tftp/v3. It mirrors the shape of the dhcp
// package: a Server type manages lifecycle, config is persisted as JSON in
// configDir, and structured log events are emitted via a dhcp.EventLogger.
package tftp
