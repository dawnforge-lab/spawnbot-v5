// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/agent"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/auth"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/cron"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/gateway"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/migrate"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/model"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/onboard"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/skills"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/status"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/picoclaw/internal/version"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func NewPicoclawCommand() *cobra.Command {
	short := fmt.Sprintf("%s picoclaw - Personal AI Assistant v%s\n\n", internal.Logo, config.GetVersion())

	cmd := &cobra.Command{
		Use:     "picoclaw",
		Short:   short,
		Example: "picoclaw version",
	}

	cmd.AddCommand(
		onboard.NewOnboardCommand(),
		agent.NewAgentCommand(),
		auth.NewAuthCommand(),
		gateway.NewGatewayCommand(),
		status.NewStatusCommand(),
		cron.NewCronCommand(),
		migrate.NewMigrateCommand(),
		skills.NewSkillsCommand(),
		model.NewModelCommand(),
		version.NewVersionCommand(),
	)

	return cmd
}

const (
	colorBlue = "\033[1;38;2;62;93;185m"
	colorRed  = "\033[1;38;2;213;70;70m"
	banner    = "\r\n" +
		colorBlue + "██████╗ ██╗ ██████╗ ██████╗ " + colorRed + " ██████╗██╗      █████╗ ██╗    ██╗\n" +
		colorBlue + "██╔══██╗██║██╔════╝██╔═══██╗" + colorRed + "██╔════╝██║     ██╔══██╗██║    ██║\n" +
		colorBlue + "██████╔╝██║██║     ██║   ██║" + colorRed + "██║     ██║     ███████║██║ █╗ ██║\n" +
		colorBlue + "██╔═══╝ ██║██║     ██║   ██║" + colorRed + "██║     ██║     ██╔══██║██║███╗██║\n" +
		colorBlue + "██║     ██║╚██████╗╚██████╔╝" + colorRed + "╚██████╗███████╗██║  ██║╚███╔███╔╝\n" +
		colorBlue + "╚═╝     ╚═╝ ╚═════╝ ╚═════╝ " + colorRed + " ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝\n " +
		"\033[0m\r\n"
)

func main() {
	fmt.Printf("%s", banner)
	cmd := NewPicoclawCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
