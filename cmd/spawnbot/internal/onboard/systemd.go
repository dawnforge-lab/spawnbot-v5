package onboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const gatewayServiceTemplate = `[Unit]
Description=Spawnbot Gateway (Telegram, channels, agent)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s gateway
Restart=on-failure
RestartSec=5
Environment=HOME=%s
%s

[Install]
WantedBy=default.target
`

const webServiceTemplate = `[Unit]
Description=Spawnbot Web UI
After=network-online.target spawnbot-gateway.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --no-browser --console
Restart=on-failure
RestartSec=5
Environment=HOME=%s

[Install]
WantedBy=default.target
`

// installSystemdServices creates and enables user-level systemd services
// for the gateway and web UI. Returns true if services were installed.
func installSystemdServices(spawnbotBin, webBin string, hasPassphrase bool) bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check systemd user session is available
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}

	serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		fmt.Printf("Warning: could not create systemd directory: %v\n", err)
		return false
	}

	home := os.Getenv("HOME")

	// Extra env for passphrase if encryption is enabled
	envLine := ""
	if hasPassphrase {
		envLine = "# Set passphrase before starting: systemctl --user set-environment SPAWNBOT_KEY_PASSPHRASE=<your-passphrase>\nEnvironment=SPAWNBOT_KEY_PASSPHRASE="
	}

	// Gateway service
	gatewayContent := fmt.Sprintf(gatewayServiceTemplate, spawnbotBin, home, envLine)
	gatewayPath := filepath.Join(serviceDir, "spawnbot-gateway.service")
	if err := os.WriteFile(gatewayPath, []byte(gatewayContent), 0o644); err != nil {
		fmt.Printf("Warning: could not write gateway service: %v\n", err)
		return false
	}

	// Web UI service (only if binary exists)
	if webBin != "" {
		webContent := fmt.Sprintf(webServiceTemplate, webBin, home)
		webPath := filepath.Join(serviceDir, "spawnbot-web.service")
		if err := os.WriteFile(webPath, []byte(webContent), 0o644); err != nil {
			fmt.Printf("Warning: could not write web service: %v\n", err)
		}
	}

	// Reload systemd and enable services
	run("systemctl", "--user", "daemon-reload")
	run("systemctl", "--user", "enable", "spawnbot-gateway.service")
	run("systemctl", "--user", "start", "spawnbot-gateway.service")

	if webBin != "" {
		run("systemctl", "--user", "enable", "spawnbot-web.service")
		run("systemctl", "--user", "start", "spawnbot-web.service")
	}

	// Enable lingering so services survive logout
	if user := os.Getenv("USER"); user != "" {
		run("loginctl", "enable-linger", user)
	}

	return true
}

// serviceStatus returns a one-line status for a systemd user service.
func serviceStatus(name string) string {
	out, err := exec.Command("systemctl", "--user", "is-active", name).Output()
	if err != nil {
		return "not running"
	}
	return strings.TrimSpace(string(out))
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
