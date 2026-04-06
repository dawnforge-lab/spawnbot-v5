package providers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// extractToolCallsFromText parses tool call JSON from response text.
// Both ClaudeCliProvider and CodexCliProvider use this to extract
// tool calls that the model outputs in its response text.
func extractToolCallsFromText(text string) []ToolCall {
	start := strings.Index(text, `{"tool_calls"`)
	if start == -1 {
		return nil
	}

	end := findMatchingBrace(text, start)
	if end == start {
		return nil
	}

	jsonStr := text[start:end]

	var wrapper struct {
		ToolCalls []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &wrapper); err != nil {
		return nil
	}

	var result []ToolCall
	for _, tc := range wrapper.ToolCalls {
		var args map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &args)

		result = append(result, ToolCall{
			ID:        tc.ID,
			Type:      tc.Type,
			Name:      tc.Function.Name,
			Arguments: args,
			Function: &FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return result
}

// stripToolCallsFromText removes tool call JSON from response text.
func stripToolCallsFromText(text string) string {
	start := strings.Index(text, `{"tool_calls"`)
	if start == -1 {
		return text
	}

	end := findMatchingBrace(text, start)
	if end == start {
		return text
	}

	return strings.TrimSpace(text[:start] + text[end:])
}

// xmlInvokePattern matches <invoke name="..."> or <invoke name="..." ...> tags.
var xmlInvokePattern = regexp.MustCompile(`<invoke\s+name="([^"]+)"`)

// xmlParamPattern matches <parameter name="..." ...>value</parameter> tags.
var xmlParamPattern = regexp.MustCompile(`<parameter\s+name="([^"]+)"[^>]*>([\s\S]*?)</parameter>`)

// extractToolCallsFromXML parses XML-style tool calls from response text.
// Some models (e.g. DeepSeek via Ollama) emit tool calls as XML instead of
// using the structured tool_calls response field:
//
//	<functioncalls>
//	<invoke name="read_file">
//	<parameter name="path">SOUL.md</parameter>
//	</invoke>
//	</functioncalls>
func extractToolCallsFromXML(text string) []ToolCall {
	if !strings.Contains(text, "<invoke") {
		return nil
	}

	// Split on <invoke to handle multiple tool calls
	parts := strings.Split(text, "<invoke")
	var result []ToolCall
	for i, part := range parts {
		if i == 0 {
			continue // skip text before first <invoke
		}

		// Re-add the <invoke prefix for regex matching
		part = "<invoke" + part

		nameMatch := xmlInvokePattern.FindStringSubmatch(part)
		if nameMatch == nil {
			continue
		}
		toolName := nameMatch[1]

		// Extract parameters
		args := make(map[string]any)
		paramMatches := xmlParamPattern.FindAllStringSubmatch(part, -1)
		for _, pm := range paramMatches {
			paramName := pm[1]
			paramValue := strings.TrimSpace(pm[2])

			// Try to parse as JSON for structured values (booleans, numbers, objects)
			var jsonVal any
			if err := json.Unmarshal([]byte(paramValue), &jsonVal); err == nil {
				args[paramName] = jsonVal
			} else {
				args[paramName] = paramValue
			}
		}

		argsJSON, _ := json.Marshal(args)
		result = append(result, ToolCall{
			ID:        fmt.Sprintf("xml_call_%d", i),
			Name:      toolName,
			Arguments: args,
			Function: &FunctionCall{
				Name:      toolName,
				Arguments: string(argsJSON),
			},
		})
	}

	return result
}

// stripXMLToolCallsFromText removes XML tool call blocks from response text.
func stripXMLToolCallsFromText(text string) string {
	// Remove <functioncalls>...</functioncalls> blocks
	re := regexp.MustCompile(`(?s)<functioncalls>.*?</functioncalls>`)
	text = re.ReplaceAllString(text, "")
	// Remove standalone <invoke>...</invoke> blocks not wrapped in <functioncalls>
	re2 := regexp.MustCompile(`(?s)<invoke\s[^>]*>.*?</invoke>`)
	text = re2.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

// ExtractToolCallsFromContent attempts to extract tool calls from response
// content text. Used as a fallback when the structured tool_calls field is
// empty but the model embedded tool calls in its text output.
// Returns extracted tool calls and the remaining content with tool call markup removed.
func ExtractToolCallsFromContent(content string) ([]ToolCall, string) {
	// Try JSON format first ({"tool_calls": [...]})
	if calls := extractToolCallsFromText(content); len(calls) > 0 {
		return calls, stripToolCallsFromText(content)
	}
	// Try XML format (<functioncalls><invoke ...>)
	if calls := extractToolCallsFromXML(content); len(calls) > 0 {
		return calls, stripXMLToolCallsFromText(content)
	}
	return nil, content
}
