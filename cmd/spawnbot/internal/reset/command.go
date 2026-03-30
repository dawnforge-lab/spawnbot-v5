package reset

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal"
)

func NewResetCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Delete all configs, memories, and workspace — start fresh",
		Long: `Reset removes all spawnbot data so you can re-run 'spawnbot onboard'.

This deletes:
  - config.json and .security.yml
  - workspace/ (sessions, memory, skills, state, cron jobs)
  - logs/
  - systemd services (stopped and removed)

This keeps:
  - ~/.spawnbot/bin/ (the spawnbot binary)
  - ~/.spawnbot/go/ (the Go runtime)
  - Shell PATH entry`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				fmt.Println("This will DELETE all spawnbot data (configs, memories, sessions, skills, cron jobs).")
				fmt.Println("The binary and Go runtime will be preserved.")
				fmt.Print("\nType 'reset' to confirm: ")
				scanner := bufio.NewScanner(os.Stdin)
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "reset" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			home := internal.GetSpawnbotHome()
			return doReset(home)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func NewNukeCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "nuke",
		Short: "Completely uninstall spawnbot from the system",
		Long: `Nuke removes ALL traces of spawnbot from the system.

This deletes:
  - ~/.spawnbot/ (everything: binary, Go runtime, configs, workspace)
  - systemd services (stopped, disabled, removed)
  - SSH encryption key (~/.ssh/spawnbot_ed25519.key*)
  - PATH entry from shell rc file (.bashrc / .zshrc)

After nuke, spawnbot will no longer be available on the system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				fmt.Println("This will COMPLETELY REMOVE spawnbot from your system.")
				fmt.Println("Everything will be deleted: binary, configs, workspace, memories, Go runtime.")
				fmt.Print("\nType 'nuke' to confirm: ")
				scanner := bufio.NewScanner(os.Stdin)
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "nuke" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			home := internal.GetSpawnbotHome()
			return doNuke(home)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func doReset(home string) error {
	fmt.Println()

	// Stop and remove systemd services
	stopAndRemoveServices()

	// Remove config files
	for _, name := range []string{"config.json", ".security.yml"} {
		p := filepath.Join(home, name)
		if err := os.Remove(p); err == nil {
			fmt.Printf("  Removed %s\n", p)
		}
	}

	// Remove workspace
	ws := filepath.Join(home, "workspace")
	if err := os.RemoveAll(ws); err == nil {
		fmt.Printf("  Removed %s/\n", ws)
	}

	// Remove logs
	logs := filepath.Join(home, "logs")
	if err := os.RemoveAll(logs); err == nil {
		fmt.Printf("  Removed %s/\n", logs)
	}

	fmt.Println()
	fmt.Println("Reset complete. Run 'spawnbot onboard' to set up again.")
	return nil
}

func doNuke(home string) error {
	fmt.Println()

	// Stop and remove systemd services
	stopAndRemoveServices()

	// Remove SSH key
	userHome, _ := os.UserHomeDir()
	for _, name := range []string{"spawnbot_ed25519.key", "spawnbot_ed25519.key.pub"} {
		p := filepath.Join(userHome, ".ssh", name)
		if err := os.Remove(p); err == nil {
			fmt.Printf("  Removed %s\n", p)
		}
	}

	// Remove PATH from shell rc
	removePATHEntry(userHome)

	// Remove the entire spawnbot home directory
	if err := os.RemoveAll(home); err != nil {
		return fmt.Errorf("failed to remove %s: %w", home, err)
	}
	fmt.Printf("  Removed %s/\n", home)

	fmt.Println()
	fmt.Println("Nuke complete. Spawnbot has been fully removed from the system.")
	fmt.Println("Open a new terminal for PATH changes to take effect.")
	return nil
}

func stopAndRemoveServices() {
	if runtime.GOOS != "linux" {
		return
	}

	services := []string{"spawnbot-gateway", "spawnbot-web"}
	for _, svc := range services {
		unit := svc + ".service"

		// Stop
		if out, err := exec.Command("systemctl", "--user", "stop", unit).CombinedOutput(); err == nil {
			fmt.Printf("  Stopped %s\n", unit)
		} else {
			_ = out
		}

		// Disable
		if out, err := exec.Command("systemctl", "--user", "disable", unit).CombinedOutput(); err == nil {
			fmt.Printf("  Disabled %s\n", unit)
		} else {
			_ = out
		}

		// Remove service file
		userHome, _ := os.UserHomeDir()
		p := filepath.Join(userHome, ".config", "systemd", "user", unit)
		if err := os.Remove(p); err == nil {
			fmt.Printf("  Removed %s\n", p)
		}
	}

	// Reload systemd
	exec.Command("systemctl", "--user", "daemon-reload").Run()
}

func removePATHEntry(userHome string) {
	rcFiles := []string{
		filepath.Join(userHome, ".bashrc"),
		filepath.Join(userHome, ".zshrc"),
		filepath.Join(userHome, ".profile"),
		filepath.Join(userHome, ".bash_profile"),
	}

	pathLine := `export PATH="$HOME/.spawnbot/bin:$PATH"`

	for _, rc := range rcFiles {
		data, err := os.ReadFile(rc)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		var filtered []string
		removed := false
		for _, line := range lines {
			if strings.TrimSpace(line) == pathLine {
				removed = true
				continue
			}
			filtered = append(filtered, line)
		}

		if removed {
			if err := os.WriteFile(rc, []byte(strings.Join(filtered, "\n")), 0o644); err == nil {
				fmt.Printf("  Removed PATH entry from %s\n", rc)
			}
		}
	}
}
