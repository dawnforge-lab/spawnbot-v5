package onboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/dawnforge-lab/spawnbot-v5/cmd/spawnbot/internal"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/credential"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/discovery"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/workspace"
)

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
		providerKey   string
		apiKey        string
		customBaseURL string
		selectedModel string
		userName      string
		approvalMode  string
		wantTelegram  bool
		telegramToken string
		telegramUsers string
		embChoice     string
		embAPIKey     string
	)

	// Group 1: Provider selection — built from discovery catalog
	providerOpts := make([]huh.Option[string], 0, len(discovery.Providers)+1)
	for _, p := range discovery.Providers {
		providerOpts = append(providerOpts, huh.NewOption(p.Name, p.Key))
	}
	providerOpts = append(providerOpts, huh.NewOption("Custom OpenAI-compatible endpoint", "custom"))

	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which LLM provider?").
				Options(providerOpts...).
				Value(&providerKey).
				Height(15),
		),
	)
	if err := providerForm.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Resolve provider info
	prov := discovery.FindProvider(providerKey)
	apiBase := ""
	if prov != nil {
		apiBase = prov.APIBase
	}

	// Group 2: Custom base URL (only for custom or providers without default base)
	if providerKey == "custom" || apiBase == "" {
		placeholder := "http://localhost:8080/v1"
		if providerKey == "azure" {
			placeholder = "https://your-resource.openai.azure.com"
		}
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API base URL").
					Placeholder(placeholder).
					Value(&customBaseURL),
			),
		)
		if err := customForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if customBaseURL != "" {
			apiBase = customBaseURL
		} else if providerKey == "custom" {
			apiBase = "http://localhost:8080/v1"
		}
	}

	// Group 3: API key input (skip for local providers)
	isLocal := prov != nil && prov.Local
	if !isLocal {
		keyHint := ""
		if prov != nil && prov.KeyHint != "" {
			keyHint = fmt.Sprintf("API Key (%s)", prov.KeyHint)
		} else {
			keyHint = "API Key"
		}

		apiKeyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(keyHint).
					EchoMode(huh.EchoModePassword).
					Value(&apiKey),
			),
		)
		if err := apiKeyForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Group 4: Model selection — discover from provider API
	fmt.Println("\nDiscovering available models...")
	models, discErr := discovery.DiscoverModels(providerKey, apiBase, apiKey)
	if discErr != nil {
		fmt.Printf("Warning: could not discover models: %v\n", discErr)
	}

	if len(models) > 0 {
		modelOpts := make([]huh.Option[string], 0, len(models))
		for _, m := range models {
			label := m.ID
			if m.OwnedBy != "" {
				label = fmt.Sprintf("%s (%s)", m.ID, m.OwnedBy)
			}
			modelOpts = append(modelOpts, huh.NewOption(label, m.ID))
		}

		modelForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Select model (%d available)", len(models))).
					Options(modelOpts...).
					Value(&selectedModel).
					Height(15),
			),
		)
		if err := modelForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// No discovery — ask the user to type a model name
		modelInputForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Model name (e.g., gpt-4o, claude-sonnet-4, llama-3-70b)").
					Value(&selectedModel),
			),
		)
		if err := modelInputForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if selectedModel == "" {
			selectedModel = "gpt-4o"
		}
	}

	// Build the model string with protocol prefix
	protocol := providerKey
	if providerKey == "custom" {
		protocol = "openai"
	}
	fullModel := protocol + "/" + selectedModel

	// Group 5: User name
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

	// Group 6: Approval mode
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

	// Group 7: Telegram (optional)
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
				huh.NewInput().
					Title("Allowed Telegram user IDs (comma-separated, or * for all)").
					Description("Send /start to @userinfobot to find your ID").
					Placeholder("123456789").
					Value(&telegramUsers),
			),
		)
		if err := tgForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Group 8: Embedding provider
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

	// Anthropic and OpenRouter don't provide embeddings.
	if embChoice == "same" && (providerKey == "anthropic" || providerKey == "openrouter") {
		fmt.Printf("\nNote: %s does not provide embeddings. Switching to Gemini (free tier).\n", providerKey)
		embChoice = "gemini"
	}

	if embChoice != "same" {
		embKeyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Embedding API Key (Gemini: https://aistudio.google.com/apikey)").
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

	cfg.Agents.Defaults.ApprovalMode = approvalMode

	newModel := &config.ModelConfig{
		ModelName: selectedModel,
		Model:     fullModel,
		APIBase:   apiBase,
	}
	newModel.SetAPIKey(apiKey)

	cfg.ModelList = append([]*config.ModelConfig{newModel}, cfg.ModelList...)
	cfg.Agents.Defaults.Provider = selectedModel
	cfg.Agents.Defaults.ModelName = selectedModel

	if wantTelegram && telegramToken != "" {
		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.SetToken(telegramToken)
		if telegramUsers != "" {
			var users []string
			for _, u := range strings.Split(telegramUsers, ",") {
				u = strings.TrimSpace(u)
				if u != "" {
					users = append(users, u)
				}
			}
			if len(users) > 0 {
				cfg.Channels.Telegram.AllowFrom = config.FlexibleStringSlice(users)
			}
		}
	}

	configureEmbeddings(cfg, embChoice, embAPIKey, apiKey, providerKey, apiBase)

	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	// ── Workspace templates ────────────────────────────────────────────

	ws := cfg.WorkspacePath()
	createWorkspaceTemplates(ws, userName)

	// ── Success message ────────────────────────────────────────────────

	fmt.Printf("\n%s spawnbot is ready!\n", internal.Logo)
	fmt.Println()
	fmt.Printf("  Provider:  %s\n", providerKey)
	fmt.Printf("  Model:     %s\n", selectedModel)
	fmt.Printf("  Mode:      %s\n", approvalMode)
	fmt.Printf("  User:      %s\n", userName)
	if wantTelegram {
		fmt.Println("  Telegram:  enabled")
		if telegramUsers != "" && telegramUsers != "*" {
			fmt.Printf("  Allowed:   %s\n", telegramUsers)
		}
	}
	fmt.Println()
	if encrypt {
		fmt.Println("Set your encryption passphrase before starting:")
		fmt.Println("  export SPAWNBOT_KEY_PASSPHRASE=<your-passphrase>")
		fmt.Println()
	}
	fmt.Println("Start chatting:")
	fmt.Println()
	fmt.Println("  CLI (interactive):")
	fmt.Println("    spawnbot agent")
	fmt.Println()
	fmt.Println("  Web UI:")
	fmt.Println("    spawnbot-web")
	fmt.Println("    http://localhost:18800")
	if wantTelegram {
		fmt.Println()
		fmt.Println("  Telegram + all channels:")
		fmt.Println("    spawnbot gateway")
	}
}

// configureEmbeddings sets up the embeddings section of the config based on the
// user's choice during onboarding.
func configureEmbeddings(cfg *config.Config, embChoice, embAPIKey, chatAPIKey, chatProvider, chatAPIBase string) {
	if embChoice == "same" {
		switch chatProvider {
		case "openai":
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = "https://api.openai.com/v1"
			cfg.Embeddings.APIKey = chatAPIKey
		case "anthropic", "openrouter":
			cfg.Embeddings.Provider = "gemini"
			cfg.Embeddings.Model = "text-embedding-004"
			cfg.Embeddings.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
			if embAPIKey != "" {
				cfg.Embeddings.APIKey = embAPIKey
			}
		default:
			cfg.Embeddings.Provider = "openai"
			cfg.Embeddings.Model = "text-embedding-3-small"
			cfg.Embeddings.BaseURL = chatAPIBase
			cfg.Embeddings.APIKey = chatAPIKey
		}
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

func createWorkspaceTemplates(ws string, userName string) {
	if err := workspace.Deploy(ws, workspace.TemplateData{UserName: userName}); err != nil {
		fmt.Printf("Error copying workspace templates: %v\n", err)
	}
}
