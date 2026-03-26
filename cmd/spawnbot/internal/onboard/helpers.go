package onboard

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/credential"
)

// providerInfo holds the configuration details for a chosen LLM provider.
type providerInfo struct {
	modelName string
	model     string
	apiBase   string
}

// providerDefaults maps provider selection keys to their default model config.
var providerDefaults = map[string]providerInfo{
	"openrouter": {
		modelName: "anthropic/claude-sonnet-4",
		model:     "openrouter/anthropic/claude-sonnet-4",
		apiBase:   "https://openrouter.ai/api/v1",
	},
	"anthropic": {
		modelName: "claude-sonnet-4-20250514",
		model:     "anthropic/claude-sonnet-4-20250514",
		apiBase:   "https://api.anthropic.com/v1",
	},
	"openai": {
		modelName: "gpt-4o",
		model:     "openai/gpt-4o",
		apiBase:   "https://api.openai.com/v1",
	},
	"custom": {
		modelName: "custom-model",
		model:     "openai/custom-model",
		apiBase:   "http://localhost:8080/v1",
	},
}

// embeddingDefaults maps embedding choice keys to (provider, model, baseURL).
type embeddingInfo struct {
	provider string
	model    string
	baseURL  string
}

var embeddingDefaults = map[string]embeddingInfo{
	"gemini": {
		provider: "gemini",
		model:    "text-embedding-004",
		baseURL:  "https://generativelanguage.googleapis.com/v1beta",
	},
	"openai": {
		provider: "openai",
		model:    "text-embedding-3-small",
		baseURL:  "https://api.openai.com/v1",
	},
}

func onboard(encrypt bool) {
	configPath := internal.GetConfigPath()

	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
		if encrypt {
			sshKeyPath, _ := credential.DefaultSSHKeyPath()
			if _, err := os.Stat(sshKeyPath); err == nil {
				fmt.Printf("Config already exists at %s\n", configPath)
				fmt.Print("Overwrite config with defaults? (y/n): ")
				var response string
				fmt.Scanln(&response)
				if response != "y" {
					fmt.Println("Aborted.")
					return
				}
				configExists = false
			}
		}
	}

	var err error
	if encrypt {
		fmt.Println("\nSet up credential encryption")
		fmt.Println("-----------------------------")
		passphrase, pErr := promptPassphrase()
		if pErr != nil {
			fmt.Printf("Error: %v\n", pErr)
			os.Exit(1)
		}
		os.Setenv(credential.PassphraseEnvVar, passphrase)

		if err = setupSSHKey(); err != nil {
			fmt.Printf("Error generating SSH key: %v\n", err)
			os.Exit(1)
		}
	}

	// ── Interactive wizard ─────────────────────────────────────────────

	var (
		provider      string
		apiKey        string
		customBaseURL string
		userName      string
		approvalMode  string
		wantTelegram  bool
		telegramToken string
		embChoice     string
		embAPIKey     string
	)

	// Group 1: Provider selection
	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which LLM provider?").
				Options(
					huh.NewOption("OpenRouter (recommended -- access to 200+ models)", "openrouter"),
					huh.NewOption("Anthropic (Claude)", "anthropic"),
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("Custom OpenAI-compatible endpoint", "custom"),
				).
				Value(&provider),
		),
	)
	if err := providerForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Group 2: Custom base URL (only for custom provider)
	if provider == "custom" {
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API base URL").
					Placeholder("http://localhost:8080/v1").
					Value(&customBaseURL),
			),
		)
		if err := customForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if customBaseURL == "" {
			customBaseURL = "http://localhost:8080/v1"
		}
	}

	// Group 3: API key input (masked)
	apiKeyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("API Key").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	)
	if err := apiKeyForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Group 4: User name
	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What should I call you?").
				Placeholder("your name").
				Value(&userName),
		),
	)
	if err := nameForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if userName == "" {
		userName = "friend"
	}

	// Group 5: Approval mode
	approvalForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Tool approval mode").
				Options(
					huh.NewOption("YOLO -- auto-approve all tools (full autonomy)", "yolo"),
					huh.NewOption("Approval -- ask before dangerous actions", "approval"),
				).
				Value(&approvalMode),
		),
	)
	if err := approvalForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Group 6: Telegram (optional)
	telegramForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Connect Telegram?").
				Affirmative("Yes").
				Negative("No").
				Value(&wantTelegram),
		),
	)
	if err := telegramForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if wantTelegram {
		tgForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Telegram Bot Token").
					EchoMode(huh.EchoModePassword).
					Value(&telegramToken),
			),
		)
		if err := tgForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Group 7: Embedding provider
	embForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Embedding provider for memory").
				Options(
					huh.NewOption("Same as chat provider", "same"),
					huh.NewOption("Gemini (free tier)", "gemini"),
					huh.NewOption("OpenAI", "openai"),
				).
				Value(&embChoice),
		),
	)
	if err := embForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// If embedding provider is not "same", ask for a separate API key
	if embChoice != "same" {
		embKeyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Embedding API Key").
					EchoMode(huh.EchoModePassword).
					Value(&embAPIKey),
			),
		)
		if err := embKeyForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// ── Build configuration ────────────────────────────────────────────

	var cfg *config.Config
	if configExists {
		cfg, err = config.LoadConfig(configPath)
		if err != nil {
			fmt.Printf("Error loading existing config: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Set approval mode
	cfg.Agents.Defaults.ApprovalMode = approvalMode

	// Set up the selected provider model entry
	pi := providerDefaults[provider]
	if provider == "custom" && customBaseURL != "" {
		pi.apiBase = customBaseURL
	}

	newModel := &config.ModelConfig{
		ModelName: pi.modelName,
		Model:     pi.model,
		APIBase:   pi.apiBase,
	}
	newModel.SetAPIKey(apiKey)

	// Prepend the user's chosen model so it becomes the default (first entry)
	cfg.ModelList = append([]*config.ModelConfig{newModel}, cfg.ModelList...)

	// Set the default agent to use this model
	cfg.Agents.Defaults.Provider = pi.modelName

	// Configure Telegram if requested
	if wantTelegram && telegramToken != "" {
		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.SetToken(telegramToken)
	}

	// Configure embeddings
	configureEmbeddings(cfg, embChoice, embAPIKey, apiKey, provider)

	// Save config (this writes both config.json and .security.yml)
	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	// ── Workspace templates ────────────────────────────────────────────

	workspace := cfg.WorkspacePath()
	createWorkspaceTemplates(workspace, userName)

	// ── Success message ────────────────────────────────────────────────

	fmt.Printf("\n%s spawnbot is ready!\n", internal.Logo)
	fmt.Println()
	fmt.Printf("  Provider:  %s\n", provider)
	fmt.Printf("  Model:     %s\n", pi.modelName)
	fmt.Printf("  Mode:      %s\n", approvalMode)
	fmt.Printf("  User:      %s\n", userName)
	if wantTelegram {
		fmt.Println("  Telegram:  enabled")
	}
	fmt.Println()
	fmt.Println("Next steps:")
	if encrypt {
		fmt.Println("  1. Set your encryption passphrase before starting spawnbot:")
		fmt.Println("       export SPAWNBOT_KEY_PASSPHRASE=<your-passphrase>")
		fmt.Println()
		fmt.Println("  2. Start chatting:")
	} else {
		fmt.Println("  1. Start chatting:")
	}
	fmt.Println("       spawnbot agent -m \"Hello!\"")
}

// configureEmbeddings sets up the embeddings section of the config based on the
// user's choice during onboarding.
func configureEmbeddings(cfg *config.Config, embChoice, embAPIKey, chatAPIKey, chatProvider string) {
	if embChoice == "same" {
		// Derive embedding config from the chat provider
		switch chatProvider {
		case "openai":
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = "https://api.openai.com/v1"
		case "anthropic":
			// Anthropic doesn't have an embeddings endpoint; use Gemini as fallback
			cfg.Embeddings.Provider = "gemini"
			cfg.Embeddings.Model = "text-embedding-004"
			cfg.Embeddings.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
			// No API key set — user will need to add one manually
			fmt.Println("\nNote: Anthropic does not provide embeddings. Using Gemini defaults.")
			fmt.Println("      You will need to add a Gemini API key to config.json for memory features.")
			return
		case "openrouter":
			// OpenRouter doesn't provide embeddings either; default to Gemini
			cfg.Embeddings.Provider = "gemini"
			cfg.Embeddings.Model = "text-embedding-004"
			cfg.Embeddings.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
			fmt.Println("\nNote: OpenRouter does not provide embeddings. Using Gemini defaults.")
			fmt.Println("      You will need to add a Gemini API key to config.json for memory features.")
			return
		default:
			// Custom provider — try OpenAI-compatible embedding
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = providerDefaults[chatProvider].apiBase
		}
		cfg.Embeddings.APIKey = chatAPIKey
		return
	}

	ei, ok := embeddingDefaults[embChoice]
	if !ok {
		return
	}
	cfg.Embeddings.Provider = ei.provider
	cfg.Embeddings.Model = ei.model
	cfg.Embeddings.BaseURL = ei.baseURL
	if embAPIKey != "" {
		cfg.Embeddings.APIKey = embAPIKey
	}
}

// templateData is the data passed to workspace templates for rendering.
type templateData struct {
	UserName string
}

// promptPassphrase reads the encryption passphrase twice from the terminal
// (with echo disabled) and returns it. Returns an error if the passphrase is
// empty or if the two inputs do not match.
func promptPassphrase() (string, error) {
	fmt.Print("Enter passphrase for credential encryption: ")
	p1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	if len(p1) == 0 {
		return "", fmt.Errorf("passphrase must not be empty")
	}

	fmt.Print("Confirm passphrase: ")
	p2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading passphrase confirmation: %w", err)
	}

	if string(p1) != string(p2) {
		return "", fmt.Errorf("passphrases do not match")
	}
	return string(p1), nil
}

// setupSSHKey generates the spawnbot-specific SSH key at ~/.ssh/spawnbot_ed25519.key.
// If the key already exists the user is warned and asked to confirm overwrite.
// Answering anything other than "y" keeps the existing key (not an error).
func setupSSHKey() error {
	keyPath, err := credential.DefaultSSHKeyPath()
	if err != nil {
		return fmt.Errorf("cannot determine SSH key path: %w", err)
	}

	if _, err := os.Stat(keyPath); err == nil {
		fmt.Printf("\nWARNING: %s already exists.\n", keyPath)
		fmt.Println("    Overwriting will invalidate any credentials previously encrypted with this key.")
		fmt.Print("    Overwrite? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" {
			fmt.Println("Keeping existing SSH key.")
			return nil
		}
	}

	if err := credential.GenerateSSHKey(keyPath); err != nil {
		return err
	}
	fmt.Printf("SSH key generated: %s\n", keyPath)
	return nil
}

func createWorkspaceTemplates(workspace string, userName string) {
	err := copyEmbeddedToTarget(workspace, userName)
	if err != nil {
		fmt.Printf("Error copying workspace templates: %v\n", err)
	}
}

func copyEmbeddedToTarget(targetDir string, userName string) error {
	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("Failed to create target directory: %w", err)
	}

	td := templateData{UserName: userName}

	// Walk through all files in embed.FS
	err := fs.WalkDir(embeddedFiles, "workspace", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Read embedded file
		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read embedded file %s: %w", path, err)
		}

		new_path, err := filepath.Rel("workspace", path)
		if err != nil {
			return fmt.Errorf("Failed to get relative path for %s: %v\n", path, err)
		}

		// Build target file path
		targetPath := filepath.Join(targetDir, new_path)

		// Ensure target file's directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("Failed to create directory %s: %w", filepath.Dir(targetPath), err)
		}

		// Render templates for markdown files that contain template syntax
		if strings.HasSuffix(path, ".md") && bytes.Contains(data, []byte("{{")) {
			rendered, tErr := renderTemplate(string(data), td)
			if tErr != nil {
				return fmt.Errorf("Failed to render template %s: %w", path, tErr)
			}
			data = []byte(rendered)
		}

		// Write file
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("Failed to write file %s: %w", targetPath, err)
		}

		return nil
	})

	return err
}

// renderTemplate renders a text/template string with the given data.
func renderTemplate(text string, data templateData) (string, error) {
	tmpl, err := template.New("workspace").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
