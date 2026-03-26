package utils

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

// GetSpawnbotHome returns the spawnbot home directory.
// Priority: $SPAWNBOT_HOME > ~/.spawnbot
func GetSpawnbotHome() string {
	if home := os.Getenv(config.EnvHome); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".spawnbot")
}

// GetDefaultConfigPath returns the default path to the spawnbot config file.
func GetDefaultConfigPath() string {
	if configPath := os.Getenv(config.EnvConfig); configPath != "" {
		return configPath
	}
	return filepath.Join(GetSpawnbotHome(), "config.json")
}

// FindSpawnbotBinary locates the spawnbot executable.
// Search order:
//  1. SPAWNBOT_BINARY environment variable (explicit override)
//  2. Same directory as the current executable
//  3. Falls back to "spawnbot" and relies on $PATH
func FindSpawnbotBinary() string {
	binaryName := "spawnbot"
	if runtime.GOOS == "windows" {
		binaryName = "spawnbot.exe"
	}

	if p := os.Getenv(config.EnvBinary); p != "" {
		if info, _ := os.Stat(p); info != nil && !info.IsDir() {
			return p
		}
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), binaryName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	return "spawnbot"
}

// GetLocalIP returns the local IP address of the machine.
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

// OpenBrowser automatically opens the given URL in the default browser.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
