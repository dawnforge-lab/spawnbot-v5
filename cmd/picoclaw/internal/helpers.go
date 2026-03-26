package internal

import (
	"os"
	"path/filepath"

	"github.com/dawnforge-lab/spawnbot-v5/pkg"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

const Logo = pkg.Logo

// GetSpawnbotHome returns the spawnbot home directory.
// Priority: $SPAWNBOT_HOME > ~/.spawnbot
func GetSpawnbotHome() string {
	if home := os.Getenv(config.EnvHome); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, pkg.DefaultSpawnbotHome)
}

func GetConfigPath() string {
	if configPath := os.Getenv(config.EnvConfig); configPath != "" {
		return configPath
	}
	return filepath.Join(GetSpawnbotHome(), "config.json")
}

func LoadConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig(GetConfigPath())
	if err != nil {
		return nil, err
	}
	logger.SetLevelFromString(cfg.Gateway.LogLevel)
	return cfg, nil
}

// FormatVersion returns the version string with optional git commit
// Deprecated: Use pkg/config.FormatVersion instead
func FormatVersion() string {
	return config.FormatVersion()
}

// FormatBuildInfo returns build time and go version info
// Deprecated: Use pkg/config.FormatBuildInfo instead
func FormatBuildInfo() (string, string) {
	return config.FormatBuildInfo()
}

// GetVersion returns the version string
// Deprecated: Use pkg/config.GetVersion instead
func GetVersion() string {
	return config.GetVersion()
}
