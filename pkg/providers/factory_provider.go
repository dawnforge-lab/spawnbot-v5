// Spawnbot - Personal AI assistant
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/discovery"
	anthropicmessages "github.com/dawnforge-lab/spawnbot-v5/pkg/providers/anthropic_messages"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers/azure"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers/bedrock"
	geminiProvider "github.com/dawnforge-lab/spawnbot-v5/pkg/providers/gemini"
	openai_compat "github.com/dawnforge-lab/spawnbot-v5/pkg/providers/openai_compat"
)

// createClaudeAuthProvider creates a Claude provider using OAuth credentials from auth store.
func createClaudeAuthProvider() (LLMProvider, error) {
	cred, err := getCredential("anthropic")
	if err != nil {
		return nil, fmt.Errorf("loading auth credentials: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("no credentials for anthropic. Run: spawnbot auth login --provider anthropic")
	}
	return NewClaudeProviderWithTokenSource(cred.AccessToken, createClaudeTokenSource()), nil
}

// createCodexAuthProvider creates a Codex provider using OAuth credentials from auth store.
func createCodexAuthProvider() (LLMProvider, error) {
	cred, err := getCredential("openai")
	if err != nil {
		return nil, fmt.Errorf("loading auth credentials: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("no credentials for openai. Run: spawnbot auth login --provider openai")
	}
	return NewCodexProviderWithTokenSource(cred.AccessToken, cred.AccountID, createCodexTokenSource()), nil
}

// ExtractProtocol extracts the protocol prefix and model identifier from a model string.
// If no prefix is specified, it defaults to "openai".
// Examples:
//   - "openai/gpt-4o" -> ("openai", "gpt-4o")
//   - "anthropic/claude-sonnet-4.6" -> ("anthropic", "claude-sonnet-4.6")
//   - "gpt-4o" -> ("openai", "gpt-4o")  // default protocol
func ExtractProtocol(model string) (protocol, modelID string) {
	model = strings.TrimSpace(model)
	protocol, modelID, found := strings.Cut(model, "/")
	if !found {
		return "openai", model
	}
	return protocol, modelID
}

// CreateProviderFromConfig creates a provider based on the ModelConfig.
// It uses the protocol prefix in the Model field to determine which provider to create.
// Supported protocol families include OpenAI-compatible prefixes (e.g., openai, openrouter, groq, gemini),
// Azure OpenAI, Amazon Bedrock, Anthropic (including messages), and various CLI/compatibility shims.
// See the switch on protocol in this function for the authoritative list.
// Returns the provider, the model ID (without protocol prefix), and any error.
func CreateProviderFromConfig(cfg *config.ModelConfig) (LLMProvider, string, error) {
	if cfg == nil {
		return nil, "", fmt.Errorf("config is nil")
	}

	if cfg.Model == "" {
		return nil, "", fmt.Errorf("model is required")
	}

	protocol, modelID := ExtractProtocol(cfg.Model)

	switch protocol {
	case "openai":
		// OpenAI with OAuth/token auth (Codex-style)
		if cfg.AuthMethod == "oauth" || cfg.AuthMethod == "token" {
			provider, err := createCodexAuthProvider()
			if err != nil {
				return nil, "", err
			}
			return provider, modelID, nil
		}
		// OpenAI with API key
		if cfg.APIKey() == "" && cfg.APIBase == "" {
			return nil, "", fmt.Errorf("api_key or api_base is required for HTTP-based protocol %q", protocol)
		}
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = getDefaultAPIBase(protocol)
		}
		return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.Proxy,
			cfg.MaxTokensField,
			cfg.RequestTimeout,
			cfg.ExtraBody,
		), modelID, nil

	case "azure", "azure-openai":
		// Azure OpenAI uses deployment-based URLs, api-key header auth,
		// and always sends max_completion_tokens.
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for azure protocol")
		}
		if cfg.APIBase == "" {
			return nil, "", fmt.Errorf(
				"api_base is required for azure protocol (e.g., https://your-resource.openai.azure.com)",
			)
		}
		return azure.NewProviderWithTimeout(
			cfg.APIKey(),
			cfg.APIBase,
			cfg.Proxy,
			cfg.RequestTimeout,
		), modelID, nil

	case "bedrock":
		// AWS Bedrock uses AWS SDK credentials (env vars, profiles, IAM roles, etc.)
		// api_base can be:
		//   - A full endpoint URL: https://bedrock-runtime.us-east-1.amazonaws.com
		//   - A region name: us-east-1 (AWS SDK resolves endpoint automatically)
		var opts []bedrock.Option
		if cfg.APIBase != "" {
			if !strings.Contains(cfg.APIBase, "://") {
				// Treat as region: let AWS SDK resolve the correct endpoint
				// (supports all AWS partitions: aws, aws-cn, aws-us-gov, etc.)
				opts = append(opts, bedrock.WithRegion(cfg.APIBase))
			} else {
				// Full endpoint URL provided (for custom endpoints or testing)
				opts = append(opts, bedrock.WithBaseEndpoint(cfg.APIBase))
			}
		}
		// Use a separate timeout for AWS config loading (credential resolution can block)
		initTimeout := 30 * time.Second
		if cfg.RequestTimeout > 0 {
			reqTimeout := time.Duration(cfg.RequestTimeout) * time.Second
			// Set request timeout for API calls
			opts = append(opts, bedrock.WithRequestTimeout(reqTimeout))
			// Ensure init timeout is at least as large as request timeout
			if reqTimeout > initTimeout {
				initTimeout = reqTimeout
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
		defer cancel()
		// Note: AWS_PROFILE env var is automatically used by AWS SDK
		provider, err := bedrock.NewProvider(ctx, opts...)
		if err != nil {
			return nil, "", fmt.Errorf("creating bedrock provider: %w", err)
		}
		return provider, modelID, nil

	case "gemini":
		// Native Gemini provider via official google.golang.org/genai SDK.
		// Provides full access to safety settings (BLOCK_NONE by default),
		// native tool calling, and streaming.
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for gemini protocol")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		provider, err := geminiProvider.NewProviderWithTimeout(ctx, cfg.APIKey(), cfg.RequestTimeout)
		if err != nil {
			return nil, "", fmt.Errorf("creating gemini provider: %w", err)
		}
		return provider, modelID, nil

	case "qwen", "qwen-intl", "qwen-international", "dashscope-intl",
		"qwen-us", "dashscope-us", "coding-plan", "alibaba-coding", "qwen-coding":
		// Alibaba/DashScope: disable data inspection so requests are not
		// logged or filtered by the platform.
		if cfg.APIKey() == "" && cfg.APIBase == "" {
			return nil, "", fmt.Errorf("api_key or api_base is required for HTTP-based protocol %q", protocol)
		}
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = getDefaultAPIBase(protocol)
		}
		return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.Proxy,
			cfg.MaxTokensField,
			cfg.RequestTimeout,
			cfg.ExtraBody,
			openai_compat.WithExtraHeaders(map[string]string{
				"X-DashScope-DataInspection": `{"input":"disable","output":"disable"}`,
			}),
		), modelID, nil

	case "kimi-coding":
		// Kimi Coding uses a dedicated coding endpoint with OpenAI-compatible API.
		// Requires User-Agent identifying a coding agent, separate subscription key.
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for kimi-coding protocol (get one at https://www.kimi.com/code/en)")
		}
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = getDefaultAPIBase(protocol)
		}
		return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.Proxy,
			cfg.MaxTokensField,
			cfg.RequestTimeout,
			cfg.ExtraBody,
			openai_compat.WithExtraHeaders(map[string]string{
				"User-Agent": "claude-code/0.1.0",
			}),
		), modelID, nil

	case "zhipu-coding":
		// Zhipu AI Coding plan uses a different API path from standard Zhipu.
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for zhipu-coding protocol")
		}
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = getDefaultAPIBase(protocol)
		}
		return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.Proxy,
			cfg.MaxTokensField,
			cfg.RequestTimeout,
			cfg.ExtraBody,
		), modelID, nil

	case "minimax", "minimax-coding":
		// Minimax requires reasoning_split: true in the request body.
		// minimax-coding uses the same API with a separate coding plan key.
		if cfg.APIKey() == "" && cfg.APIBase == "" {
			return nil, "", fmt.Errorf("api_key or api_base is required for HTTP-based protocol %q", protocol)
		}
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = getDefaultAPIBase(protocol)
		}
		extraBody := cfg.ExtraBody
		if extraBody == nil {
			extraBody = make(map[string]any)
		}
		if _, ok := extraBody["reasoning_split"]; !ok {
			extraBody["reasoning_split"] = true
		}
		return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.Proxy,
			cfg.MaxTokensField,
			cfg.RequestTimeout,
			extraBody,
		), modelID, nil

	case "anthropic":
		if cfg.AuthMethod == "oauth" || cfg.AuthMethod == "token" {
			// Use OAuth credentials from auth store
			provider, err := createClaudeAuthProvider()
			if err != nil {
				return nil, "", err
			}
			return provider, modelID, nil
		}
		// Use API key with HTTP API
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = "https://api.anthropic.com/v1"
		}
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for anthropic protocol (model: %s)", cfg.Model)
		}
		return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.Proxy,
			cfg.MaxTokensField,
			cfg.RequestTimeout,
			cfg.ExtraBody,
		), modelID, nil

	case "anthropic-messages":
		// Anthropic Messages API with native format (HTTP-based, no SDK)
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = "https://api.anthropic.com/v1"
		}
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for anthropic-messages protocol (model: %s)", cfg.Model)
		}
		return anthropicmessages.NewProviderWithTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.RequestTimeout,
		), modelID, nil

	case "coding-plan-anthropic", "alibaba-coding-anthropic":
		// Alibaba Coding Plan with Anthropic-compatible API
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = getDefaultAPIBase(protocol)
		}
		if cfg.APIKey() == "" {
			return nil, "", fmt.Errorf("api_key is required for %q protocol (model: %s)", protocol, cfg.Model)
		}
		return anthropicmessages.NewProviderWithTimeout(
			cfg.APIKey(),
			apiBase,
			cfg.RequestTimeout,
		), modelID, nil

	case "antigravity":
		return NewAntigravityProvider(), modelID, nil

	case "claude-cli", "claudecli":
		workspace := cfg.Workspace
		if workspace == "" {
			workspace = "."
		}
		return NewClaudeCliProvider(workspace), modelID, nil

	case "codex-cli", "codexcli":
		workspace := cfg.Workspace
		if workspace == "" {
			workspace = "."
		}
		return NewCodexCliProvider(workspace), modelID, nil

	case "github-copilot", "copilot":
		apiBase := cfg.APIBase
		if apiBase == "" {
			apiBase = "localhost:4321"
		}
		connectMode := cfg.ConnectMode
		if connectMode == "" {
			connectMode = "grpc"
		}
		provider, err := NewGitHubCopilotProvider(apiBase, connectMode, modelID)
		if err != nil {
			return nil, "", err
		}
		return provider, modelID, nil

	default:
		// Check the provider registry — any OpenAI-compatible provider
		// registered in discovery.Providers works automatically.
		if discovery.IsOpenAICompat(protocol) {
			if cfg.APIKey() == "" && cfg.APIBase == "" {
				return nil, "", fmt.Errorf("api_key or api_base is required for HTTP-based protocol %q", protocol)
			}
			apiBase := cfg.APIBase
			if apiBase == "" {
				apiBase = discovery.DefaultAPIBase(protocol)
			}
			return NewHTTPProviderWithMaxTokensFieldAndRequestTimeout(
				cfg.APIKey(),
				apiBase,
				cfg.Proxy,
				cfg.MaxTokensField,
				cfg.RequestTimeout,
				cfg.ExtraBody,
			), modelID, nil
		}
		return nil, "", fmt.Errorf("unknown protocol %q in model %q", protocol, cfg.Model)
	}
}

// getDefaultAPIBase returns the default API base URL for a given protocol.
// Most providers are looked up from the discovery registry. Only aliases
// not in the registry need explicit entries here.
func getDefaultAPIBase(protocol string) string {
	// Check the registry first (single source of truth)
	if base := discovery.DefaultAPIBase(protocol); base != "" {
		return base
	}
	// Aliases and internal protocols not in the public registry
	switch protocol {
	case "qwen-international", "dashscope-intl":
		return "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	case "qwen-us", "dashscope-us":
		return "https://dashscope-us.aliyuncs.com/compatible-mode/v1"
	case "coding-plan", "alibaba-coding", "qwen-coding":
		return "https://coding-intl.dashscope.aliyuncs.com/v1"
	case "coding-plan-anthropic", "alibaba-coding-anthropic":
		return "https://coding-intl.dashscope.aliyuncs.com/apps/anthropic"
	default:
		return ""
	}
}
