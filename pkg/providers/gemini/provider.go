// Package gemini implements a native Gemini provider using the official
// google.golang.org/genai SDK. This provides full access to Gemini features
// including safety settings, native tool calling, and streaming — features
// that are not available through the OpenAI-compatible shim.
package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers/protocoltypes"
)

type (
	ToolCall               = protocoltypes.ToolCall
	FunctionCall           = protocoltypes.FunctionCall
	LLMResponse            = protocoltypes.LLMResponse
	UsageInfo              = protocoltypes.UsageInfo
	Message                = protocoltypes.Message
	ToolDefinition         = protocoltypes.ToolDefinition
	ToolFunctionDefinition = protocoltypes.ToolFunctionDefinition
)

// Provider implements LLMProvider and StreamingProvider using the native
// Google GenAI SDK. Safety settings are set to BLOCK_NONE by default.
type Provider struct {
	client  *genai.Client
	timeout time.Duration
}

// Gemini thinking models (3.x) need more time than standard models.
const defaultTimeout = 300 * time.Second

// NewProvider creates a Gemini provider with the given API key.
func NewProvider(ctx context.Context, apiKey string) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating gemini client: %w", err)
	}
	return &Provider{client: client, timeout: defaultTimeout}, nil
}

// NewProviderWithTimeout creates a Gemini provider with custom timeout.
func NewProviderWithTimeout(ctx context.Context, apiKey string, timeoutSeconds int) (*Provider, error) {
	p, err := NewProvider(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds > 0 {
		p.timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return p, nil
}

func (p *Provider) GetDefaultModel() string { return "" }

// Chat sends a request to the Gemini API and returns the response.
func (p *Provider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	contents, systemInstruction := convertMessages(messages)
	config := buildConfig(tools, options, systemInstruction)

	resp, err := p.client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini API error: %w", err)
	}

	return convertResponse(resp), nil
}

// ChatStream implements streaming via the native Gemini streaming API.
func (p *Provider) ChatStream(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
	onChunk func(accumulated string),
) (*LLMResponse, error) {
	contents, systemInstruction := convertMessages(messages)
	config := buildConfig(tools, options, systemInstruction)

	var accumulated strings.Builder
	var lastResp *genai.GenerateContentResponse

	for resp, err := range p.client.Models.GenerateContentStream(ctx, model, contents, config) {
		if err != nil {
			return nil, fmt.Errorf("gemini stream error: %w", err)
		}
		lastResp = resp

		text := resp.Text()
		if text != "" {
			accumulated.WriteString(text)
			if onChunk != nil {
				onChunk(accumulated.String())
			}
		}
	}

	if lastResp == nil {
		return &LLMResponse{FinishReason: "stop"}, nil
	}

	// Build final response from accumulated text + any tool calls from last chunk.
	result := convertResponse(lastResp)
	if accumulated.Len() > 0 && result.Content == "" {
		result.Content = accumulated.String()
	}
	return result, nil
}

// convertMessages converts spawnbot Messages to Gemini Content format.
// System messages are extracted as a separate system instruction.
func convertMessages(messages []Message) ([]*genai.Content, *genai.Content) {
	var contents []*genai.Content
	var systemParts []*genai.Part

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			systemParts = append(systemParts, &genai.Part{Text: msg.Content})

		case "user":
			parts := []*genai.Part{{Text: msg.Content}}
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: parts,
			})

		case "assistant":
			parts := []*genai.Part{}
			if msg.Content != "" {
				parts = append(parts, &genai.Part{Text: msg.Content})
			}
			// Convert tool calls to Gemini function calls, preserving thought signatures
			for _, tc := range msg.ToolCalls {
				args := tc.Arguments
				if args == nil && tc.Function != nil {
					args = make(map[string]any)
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				part := &genai.Part{
					FunctionCall: &genai.FunctionCall{
						Name: toolCallName(tc),
						Args: args,
					},
				}
				// Restore thought signature from base64
				sig := tc.ThoughtSignature
				if sig == "" && tc.Function != nil {
					sig = tc.Function.ThoughtSignature
				}
				if sig != "" {
					if decoded, err := base64.StdEncoding.DecodeString(sig); err == nil {
						part.ThoughtSignature = decoded
					}
				}
				parts = append(parts, part)
			}
			if len(parts) > 0 {
				contents = append(contents, &genai.Content{
					Role:  "model",
					Parts: parts,
				})
			}

		case "tool":
			// Tool results go back as function responses
			var result map[string]any
			if err := json.Unmarshal([]byte(msg.Content), &result); err != nil {
				// If not valid JSON, wrap it
				result = map[string]any{"result": msg.Content}
			}
			// Find the function name from the tool_call_id by looking at
			// previous assistant messages. As a fallback, use the ID itself.
			fnName := msg.ToolCallID
			for i := len(contents) - 1; i >= 0; i-- {
				for _, part := range contents[i].Parts {
					if part.FunctionCall != nil {
						// Match by convention: tool_call_id often contains the function name
						fnName = part.FunctionCall.Name
						break
					}
				}
				if fnName != msg.ToolCallID {
					break
				}
			}
			contents = append(contents, &genai.Content{
				Role: "user",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						Name:     fnName,
						Response: result,
					},
				}},
			})
		}
	}

	var systemInstruction *genai.Content
	if len(systemParts) > 0 {
		systemInstruction = &genai.Content{Parts: systemParts}
	}

	return contents, systemInstruction
}

// buildConfig creates the GenerateContentConfig with safety settings and tools.
func buildConfig(tools []ToolDefinition, options map[string]any, systemInstruction *genai.Content) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{
		// Disable all safety filters for agent operation.
		SafetySettings: []*genai.SafetySetting{
			{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockThresholdBlockNone},
			{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockThresholdBlockNone},
			{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockThresholdBlockNone},
			{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockThresholdBlockNone},
			{Category: genai.HarmCategoryCivicIntegrity, Threshold: genai.HarmBlockThresholdBlockNone},
		},
		SystemInstruction: systemInstruction,
	}

	// Max tokens
	if maxTokens, ok := options["max_tokens"]; ok {
		switch v := maxTokens.(type) {
		case int:
			config.MaxOutputTokens = int32(v)
		case float64:
			config.MaxOutputTokens = int32(v)
		}
	}

	// Temperature
	if temp, ok := options["temperature"]; ok {
		switch v := temp.(type) {
		case float64:
			t := float32(v)
			config.Temperature = &t
		}
	}

	// Convert tools to Gemini format
	if len(tools) > 0 {
		var funcDecls []*genai.FunctionDeclaration
		for _, td := range tools {
			fd := &genai.FunctionDeclaration{
				Name:        td.Function.Name,
				Description: td.Function.Description,
			}
			if len(td.Function.Parameters) > 0 {
				fd.Parameters = convertSchema(td.Function.Parameters)
			}
			funcDecls = append(funcDecls, fd)
		}
		config.Tools = []*genai.Tool{{FunctionDeclarations: funcDecls}}
	}

	return config
}

// convertSchema converts an OpenAI-style JSON schema to Gemini's Schema type.
func convertSchema(params map[string]any) *genai.Schema {
	schema := &genai.Schema{}

	if t, ok := params["type"].(string); ok {
		switch t {
		case "object":
			schema.Type = genai.TypeObject
		case "array":
			schema.Type = genai.TypeArray
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		}
	}

	if desc, ok := params["description"].(string); ok {
		schema.Description = desc
	}

	if props, ok := params["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, propRaw := range props {
			if propMap, ok := propRaw.(map[string]any); ok {
				schema.Properties[name] = convertSchema(propMap)
			}
		}
	}

	if required, ok := params["required"].([]any); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	if items, ok := params["items"].(map[string]any); ok {
		schema.Items = convertSchema(items)
	}

	if enum, ok := params["enum"].([]any); ok {
		for _, e := range enum {
			if s, ok := e.(string); ok {
				schema.Enum = append(schema.Enum, s)
			}
		}
	}

	return schema
}

// convertResponse converts a Gemini response to spawnbot's LLMResponse.
func convertResponse(resp *genai.GenerateContentResponse) *LLMResponse {
	if resp == nil || len(resp.Candidates) == 0 {
		return &LLMResponse{FinishReason: "stop"}
	}

	candidate := resp.Candidates[0]
	result := &LLMResponse{
		FinishReason: convertFinishReason(candidate.FinishReason),
	}

	// Extract text and tool calls from parts
	var textParts []string
	var toolCalls []ToolCall

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" && !part.Thought {
				textParts = append(textParts, part.Text)
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				// Encode thought signature as base64 string for storage
				var sigStr string
				if len(part.ThoughtSignature) > 0 {
					sigStr = base64.StdEncoding.EncodeToString(part.ThoughtSignature)
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:               fmt.Sprintf("call_%s", part.FunctionCall.Name),
					Type:             "function",
					ThoughtSignature: sigStr,
					Function: &FunctionCall{
						Name:             part.FunctionCall.Name,
						Arguments:        string(argsJSON),
						ThoughtSignature: sigStr,
					},
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				})
			}
		}
	}

	result.Content = strings.Join(textParts, "")
	result.ToolCalls = toolCalls

	if len(toolCalls) > 0 && result.FinishReason == "stop" {
		result.FinishReason = "tool_calls"
	}

	// Usage
	if resp.UsageMetadata != nil {
		result.Usage = &UsageInfo{
			PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}

	return result
}

func convertFinishReason(fr genai.FinishReason) string {
	switch fr {
	case genai.FinishReasonStop:
		return "stop"
	case genai.FinishReasonMaxTokens:
		return "length"
	case genai.FinishReasonSafety:
		return "content_filter"
	default:
		return "stop"
	}
}

func toolCallName(tc ToolCall) string {
	if tc.Function != nil {
		return tc.Function.Name
	}
	return tc.Name
}
