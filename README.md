# NET

A desktop network engineering tool. Run a DHCP server, TFTP server, and Syslog receiver from a single GUI — aimed at network engineers provisioning switches, routers, and PXE clients on the bench.

Built with [Wails v2](https://wails.io) (Go backend + Vue 3 frontend).

## Features

### DHCP Server (RFC 2131)
- Configurable IP pool, lease time, subnet mask, router, DNS, NTP, WINS, domain search, boot file, TFTP server
- Custom DHCP options — codes 128–254, types: `ip`, `string`, `uint8`, `uint16`, `uint32`, `bool`
- Live lease table with expiry sweep
- Re-offers existing lease to returning MAC addresses
- DHCPINFORM support

### TFTP Server (RFC 1350 / 2347 / 2348)
- Read and write support (each independently toggleable)
- Block size negotiation (RFC 2348) — or pin a fixed block size
- Path jail — directory traversal rejected
- Transfer progress events in the UI, 50-entry history ring

### Syslog Receiver (RFC 3164)
- UDP listener (default port 514)
- 1000-entry message ring buffer, newest-first
- Filter by severity, source host, or keyword

## Requirements

| Dependency | Version |
|---|---|
| Go | 1.21+ |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v2 |
| Node | 18+ |
| **Linux:** webkit2gtk-4.1 | `libwebkit2gtk-4.1-dev` (Ubuntu/Debian) · `webkit2gtk-4.1` (Arch/Manjaro) |

## Build

```bash
# Install Wails CLI (once)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone and build
git clone https://github.com/theochita/NET
cd NET
wails build
```

The binary is written to `build/bin/NET` (Linux/macOS) or `build/bin/NET.exe` (Windows).

## Run

```bash
sudo ./build/bin/NET
```

> **Ports 67 (DHCP), 69 (TFTP), and 514 (Syslog) require elevated privileges.**
>
> Linux alternative to `sudo`:
> ```bash
> sudo setcap cap_net_bind_service=+ep ./build/bin/NET
> ./build/bin/NET
> ```

## Development

```bash
wails dev
```

Or run the Vite dev server separately for faster frontend iteration:

```bash
cd frontend && npm run dev   # dev server on :5173
```

## Testing

```bash
go test ./dhcp/... ./tftp/... ./syslog/... -v
go test ./dhcp/... ./tftp/... ./syslog/... -race   # data race check
```

Docker-based DHCP integration tests (requires Docker):

```bash
docker compose -f docker-compose.dhcp-test.yml up --build --abort-on-container-exit --exit-code-from client
docker compose -f docker-compose.dhcp-test.yml down
```

## License

[GNU General Public License v3.0](LICENSE)
