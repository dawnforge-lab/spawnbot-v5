// Spawnbot - Personal AI assistant
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/agent"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/auth"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/cron"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/gateway"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/migrate"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/model"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/onboard"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/skills"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/status"
	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal/version"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func NewSpawnbotCommand() *cobra.Command {
	short := fmt.Sprintf("%s spawnbot - Personal AI Assistant v%s\n\n", internal.Logo, config.GetVersion())

	cmd := &cobra.Command{
		Use:     "spawnbot",
		Short:   short,
		Example: "spawnbot version",
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
	cmd := NewSpawnbotCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
