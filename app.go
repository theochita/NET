package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/theochita/NET/dhcp"
	syslogsrv "github.com/theochita/NET/syslog"
	"github.com/theochita/NET/tftp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. All DHCP logic lives in dhcp.Server.
type App struct {
	ctx    context.Context
	server *dhcp.Server
	logger *dhcp.EventLogger
	tftp   *tftp.Server
	syslog *syslogsrv.Server
}

// NewApp creates the App and initialises the DHCP server with the user config dir.
func NewApp() *App {
	dir := configDir()
	server, err := dhcp.NewServer(dir)
	if err != nil {
		// Fallback: use temp dir so the app still starts.
		server, _ = dhcp.NewServer(os.TempDir())
	}
	tsrv, err := tftp.NewServer(dir)
	if err != nil {
		tsrv, _ = tftp.NewServer(os.TempDir())
	}
	if cfg := tsrv.GetConfig(); cfg.Root == "" {
		cfg.Root = defaultTFTPRoot(dir)
		cfg.ReadEnabled = true
		cfg.WriteEnabled = true
		_ = tsrv.SaveConfig(cfg)
	}
	ssrv, err := syslogsrv.NewServer(dir)
	if err != nil {
		ssrv, _ = syslogsrv.NewServer(os.TempDir())
	}
	return &App{server: server, tftp: tsrv, syslog: ssrv}
}

// defaultTFTPRoot returns <cwd>/tftp, falling back to <configDir>/tftp if
// cwd is not writable.
func defaultTFTPRoot(configDir string) string {
	cwd, err := os.Getwd()
	if err == nil {
		cand := filepath.Join(cwd, "tftp")
		if err := os.MkdirAll(cand, 0755); err == nil {
			return cand
		}
	}
	return filepath.Join(configDir, "tftp")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.logger = dhcp.NewEventLogger(log.Default(), func(e dhcp.LogEntry) {
		runtime.EventsEmit(ctx, "dhcp:log", e)
	})
	a.server.SetLogger(a.logger)

	tftpLogger := dhcp.NewEventLogger(log.Default(), func(e dhcp.LogEntry) {
		runtime.EventsEmit(ctx, "tftp:log", e)
	})
	a.tftp.SetLogger(tftpLogger)
	a.tftp.SetTransferEmitter(func(tr tftp.Transfer) {
		runtime.EventsEmit(ctx, "tftp:transfer", tr)
	})

	syslogLogger := dhcp.NewEventLogger(log.Default(), func(e dhcp.LogEntry) {
		runtime.EventsEmit(ctx, "syslog:log", e)
	})
	a.syslog.SetLogger(syslogLogger)
	a.syslog.SetEmitter(func(msg syslogsrv.SyslogMessage) {
		runtime.EventsEmit(ctx, "syslog:message", msg)
	})
}

// GetInterfaces returns all network interfaces that have at least one IPv4 address.
func (a *App) GetInterfaces() []dhcp.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var result []dhcp.Interface
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		var ips, masks []string
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if ip4 := v.IP.To4(); ip4 != nil {
					ips = append(ips, ip4.String())
					masks = append(masks, net.IP(v.Mask).String())
				}
			case *net.IPAddr:
				if ip4 := v.IP.To4(); ip4 != nil {
					ips = append(ips, ip4.String())
					masks = append(masks, "")
				}
			}
		}
		if len(ips) > 0 {
			result = append(result, dhcp.Interface{Name: iface.Name, IPs: ips, Masks: masks})
		}
	}
	return result
}

// GetConfig returns the current DHCP configuration.
func (a *App) GetConfig() dhcp.DHCPConfig {
	return a.server.GetConfig()
}

// SaveConfig persists the DHCP configuration to disk.
func (a *App) SaveConfig(cfg dhcp.DHCPConfig) error {
	return a.server.SaveConfig(cfg)
}

// StartDHCP starts the DHCP server using the saved configuration.
func (a *App) StartDHCP() error {
	err := a.server.Start()
	if err != nil {
		a.logger.Error(fmt.Sprintf("start failed: %v", err))
	}
	return err
}

// StopDHCP stops the DHCP server. Leases are preserved.
func (a *App) StopDHCP() {
	a.server.Stop()
}

// IsDHCPRunning reports whether the DHCP server is currently active.
func (a *App) IsDHCPRunning() bool {
	return a.server.IsRunning()
}

// DHCPStartedAt returns the time the DHCP server last started, or the zero
// value if it is not running.
func (a *App) DHCPStartedAt() time.Time {
	return a.server.StartedAt()
}

// TFTPStartedAt returns the time the TFTP server last started, or the zero
// value if it is not running.
func (a *App) TFTPStartedAt() time.Time {
	return a.tftp.StartedAt()
}

// GetLeases returns the current lease table.
func (a *App) GetLeases() []dhcp.Lease {
	return a.server.GetLeases()
}

// ClearLeases wipes all leases from memory and disk.
func (a *App) ClearLeases() error {
	return a.server.ClearLeases()
}

// ---- TFTP ----

func (a *App) GetTFTPConfig() tftp.TFTPConfig { return a.tftp.GetConfig() }

func (a *App) SaveTFTPConfig(cfg tftp.TFTPConfig) error { return a.tftp.SaveConfig(cfg) }

func (a *App) StartTFTP() error { return a.tftp.Start() }

func (a *App) StopTFTP() { a.tftp.Stop() }

func (a *App) IsTFTPRunning() bool { return a.tftp.IsRunning() }

func (a *App) GetActiveTransfers() []tftp.Transfer { return a.tftp.GetActiveTransfers() }

func (a *App) GetTransferHistory() []tftp.Transfer { return a.tftp.GetTransferHistory() }

func (a *App) ClearTransferHistory() error { return a.tftp.ClearTransferHistory() }

// OpenTFTPFolder opens the configured root directory in the native file manager.
func (a *App) OpenTFTPFolder() error {
	cfg := a.tftp.GetConfig()
	if cfg.Root == "" {
		return fmt.Errorf("no root directory configured")
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", cfg.Root)
	case "windows":
		cmd = exec.Command("explorer", cfg.Root)
	default:
		// xdg-open refuses to run as root (Nautilus, Thunar, etc. all bail).
		// When sudo'd, delegate to the original user and pass display env vars.
		if os.Getuid() == 0 {
			if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
				args := []string{"-u", sudoUser, "env"}
				for _, k := range []string{"DISPLAY", "WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR"} {
					if v := os.Getenv(k); v != "" {
						args = append(args, k+"="+v)
					}
				}
				args = append(args, "xdg-open", cfg.Root)
				cmd = exec.Command("sudo", args...)
			} else {
				cmd = exec.Command("xdg-open", cfg.Root)
			}
		} else {
			cmd = exec.Command("xdg-open", cfg.Root)
		}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open file manager: %w", err)
	}
	return nil
}

// PickTFTPFolder shows a native directory picker and returns the chosen path.
func (a *App) PickTFTPFolder() (string, error) {
	cfg := a.tftp.GetConfig()
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Select TFTP Root",
		DefaultDirectory: cfg.Root,
	})
}

// ---- Syslog ----

func (a *App) GetSyslogConfig() syslogsrv.SyslogConfig { return a.syslog.GetConfig() }

func (a *App) SaveSyslogConfig(cfg syslogsrv.SyslogConfig) error { return a.syslog.SaveConfig(cfg) }

func (a *App) StartSyslog() error { return a.syslog.Start() }

func (a *App) StopSyslog() { a.syslog.Stop() }

func (a *App) IsSyslogRunning() bool { return a.syslog.IsRunning() }

func (a *App) SyslogStartedAt() time.Time { return a.syslog.StartedAt() }

func (a *App) GetSyslogMessages() []syslogsrv.SyslogMessage { return a.syslog.GetMessages() }

func (a *App) SyslogMessageCount() int64 { return a.syslog.MessageCount() }

func (a *App) ClearSyslogMessages() error { a.syslog.ClearMessages(); return nil }

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "net")
}
