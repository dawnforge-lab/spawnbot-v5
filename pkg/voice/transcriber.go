package voice

import (
	"context"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers"
)

type Transcriber interface {
	Name() string
	Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error)
}

type TranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

func supportsAudioTranscription(model string) bool {
	protocol, _ := providers.ExtractProtocol(model)

	switch protocol {
	case "openai", "azure", "azure-openai",
		"litellm", "openrouter", "groq", "zhipu", "gemini", "nvidia",
		"ollama", "moonshot", "shengsuanyun", "deepseek", "cerebras",
		"vivgrid", "volcengine", "vllm", "qwen", "qwen-intl", "qwen-international", "dashscope-intl",
		"qwen-us", "dashscope-us", "mistral", "avian", "minimax", "longcat", "modelscope", "novita",
		"coding-plan", "alibaba-coding", "qwen-coding":
		// These protocols all go through the OpenAI-compatible or Azure provider path in
		// providers.CreateProviderFromConfig, so they are the only ones that can supply
		// the audio media payload shape expected by NewAudioModelTranscriber.

		// Technical debt: this allows any model string under these protocols, but not
		// every model actually supports the audio transcription endpoint. For example,
		// under "openai" only whisper-1 / gpt-4o-audio-preview support it, while
		// chat-only models (gpt-4o, o1, etc.) will fail at the provider call site.
		// Restriction by modelID is deferred because it requires maintaining a
		// per-protocol allowlist that would need to be updated whenever providers
		// add or rename audio-capable models.
		return true
	default:
		return false
	}
}

// DetectTranscriber inspects cfg and returns the appropriate Transcriber, or
// nil if no supported transcription provider is configured.
func DetectTranscriber(cfg *config.Config) Transcriber {
	if modelName := strings.TrimSpace(cfg.Voice.ModelName); modelName != "" {
		modelCfg, err := cfg.GetModelConfig(modelName)
		if err != nil {
			return nil
		}
		if supportsAudioTranscription(modelCfg.Model) {
			return NewAudioModelTranscriber(modelCfg)
		}
	}

	// ElevenLabs voice config (supports Scribe STT).
	if key := strings.TrimSpace(cfg.Voice.ElevenLabsAPIKey); key != "" {
		return NewElevenLabsTranscriber(key)
	}
	// Fall back to any model-list entry that uses the groq/ protocol.
	for _, mc := range cfg.ModelList {
		if strings.HasPrefix(mc.Model, "groq/") && mc.APIKey() != "" {
			return NewGroqTranscriber(mc.APIKey())
		}
	}
	return nil
}
