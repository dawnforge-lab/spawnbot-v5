// Spawnbot - Personal AI assistant
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tuicfg "github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw-launcher-tui/config"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw-launcher-tui/ui"
)

func main() {
	configPath := tuicfg.DefaultConfigPath()
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		cmd := exec.Command("spawnbot", "onboard")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	cfg, err := tuicfg.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "spawnbot-launcher-tui: %v\n", err)
		os.Exit(1)
	}

	app := ui.New(cfg, configPath)
	// Bind model selection hook to sync to main config
	app.OnModelSelected = func(scheme tuicfg.Scheme, user tuicfg.User, modelID string) {
		_ = tuicfg.SyncSelectedModelToMainConfig(scheme, user, modelID)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "spawnbot-launcher-tui: %v\n", err)
		os.Exit(1)
	}
}
