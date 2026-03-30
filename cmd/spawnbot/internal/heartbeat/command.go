package heartbeat

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func NewHeartbeatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heartbeat",
		Short: "Manage heartbeat service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSetIntervalCommand(),
		newStatusCommand(),
	)

	return cmd
}

func newSetIntervalCommand() *cobra.Command {
	var minutes int

	cmd := &cobra.Command{
		Use:   "set-interval",
		Short: "Set heartbeat interval in minutes (minimum 5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if minutes < 5 {
				return fmt.Errorf("interval must be at least 5 minutes, got %d", minutes)
			}

			configPath := internal.GetConfigPath()
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}

			cfg.Heartbeat.Interval = minutes
			if err := config.SaveConfig(configPath, cfg); err != nil {
				return fmt.Errorf("error saving config: %w", err)
			}

			fmt.Printf("Heartbeat interval set to %d minutes.\n", minutes)
			fmt.Println("Restart the gateway for changes to take effect.")
			return nil
		},
	}

	cmd.Flags().IntVarP(&minutes, "minutes", "m", 30, "Interval in minutes (min 5)")
	cmd.MarkFlagRequired("minutes")

	return cmd
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current heartbeat configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := internal.LoadConfig()
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}

			info := map[string]any{
				"enabled":          cfg.Heartbeat.Enabled,
				"interval_minutes": cfg.Heartbeat.Interval,
			}

			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
