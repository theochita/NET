//go:build e2e

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("DHCP_E2E_CLIENT") == "1" {
		// Inside Docker client container: set up connection, run scenarios.
		setupClientMode()
		os.Exit(m.Run())
		return
	}
	// On host: m.Run() runs TestE2E (and skips all TestDHCP_* functions).
	os.Exit(m.Run())
}

// TestE2E is the host-side entry point. It orchestrates Docker Compose and
// streams the client's go test output to stdout.
func TestE2E(t *testing.T) {
	if os.Getenv("DHCP_E2E_CLIENT") == "1" {
		t.Skip("host-only orchestration test — skipped inside Docker")
		return
	}

	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..")
	composeFile := filepath.Join(root, "docker-compose.dhcp-test.yml")

	// Tear down any leftover containers from a previous run.
	exec.Command("docker", "compose", "-f", composeFile, "down", "--remove-orphans").Run()

	t.Cleanup(func() {
		exec.Command("docker", "compose", "-f", composeFile, "down", "--remove-orphans").Run()
	})

	cmd := exec.Command("docker", "compose",
		"-f", composeFile,
		"up", "--build",
		"--abort-on-container-exit",
		"--exit-code-from", "client",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("one or more E2E scenarios failed (see output above): %v", err)
	}
}
