package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// SearchTools is a unified tool that replaces separate BM25 and regex search tools.
// It supports two modes:
//   - select: (exact activation) — parse comma-separated tool names and promote them
//   - keyword (scored search) — informational search without auto-activation
type SearchTools struct {
	registry *ToolRegistry
}

func NewSearchTools(registry *ToolRegistry) *SearchTools {
	return &SearchTools{registry: registry}
}

func (s *SearchTools) Name() string {
	return "search_tools"
}

func (s *SearchTools) Description() string {
	return `Search for and activate deferred tools. Two modes:
- "select:<tool1>,<tool2>" — activate specific tools by exact name (comma-separated)
- "<keywords>" — search deferred tools by keyword (informational only, does not activate)
Use keyword mode to discover tool names, then select: mode to activate them.`
}

func (s *SearchTools) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": `Use "select:<name1>,<name2>" for exact activation, or keywords for scored search`,
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results for keyword search (default 5)",
			},
		},
		"required": []string{"query"},
	}
}

func (s *SearchTools) Execute(ctx context.Context, args map[string]any) *ToolResult {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return ErrorResult("Missing or empty 'query' argument.")
	}

	if strings.HasPrefix(query, "select:") {
		return s.executeSelect(query[len("select:"):])
	}
	return s.executeKeyword(query, args)
}

func (s *SearchTools) executeSelect(namesStr string) *ToolResult {
	rawNames := strings.Split(namesStr, ",")
	names := make([]string, 0, len(rawNames))
	for _, n := range rawNames {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ErrorResult("Missing or empty 'query' argument.")
	}

	s.registry.mu.RLock()

	var activated []string
	var notFound []string

	for _, name := range names {
		entry, exists := s.registry.tools[name]
		if !exists {
			notFound = append(notFound, name)
			continue
		}
		// Core tools and already-discovered tools count as activated (idempotent)
		_ = entry
		activated = append(activated, name)
	}

	s.registry.mu.RUnlock()

	// Promote non-core tools that were found
	if len(activated) > 0 {
		s.registry.PromoteTools(activated)
	}

	if len(notFound) == len(names) {
		// ALL not found
		deferred := s.registry.GetDeferredNames()
		return ErrorResult(fmt.Sprintf(
			"Tools not found: %s. Available deferred tools: %s",
			strings.Join(notFound, ", "),
			strings.Join(deferred, ", "),
		))
	}

	if len(notFound) > 0 {
		// SOME not found
		return ErrorResult(fmt.Sprintf(
			"Activated %d tools: %s. Not found: %s",
			len(activated),
			formatActivatedList(s.registry, activated),
			strings.Join(notFound, ", "),
		))
	}

	// All found
	return SilentResult(fmt.Sprintf(
		"Activated %d tools:\n%s",
		len(activated),
		formatActivatedList(s.registry, activated),
	))
}

func formatActivatedList(reg *ToolRegistry, names []string) string {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	parts := make([]string, 0, len(names))
	for _, name := range names {
		desc := ""
		if entry, ok := reg.tools[name]; ok {
			desc = entry.Tool.Description()
		}
		parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
	}
	return strings.Join(parts, "\n")
}

func (s *SearchTools) executeKeyword(query string, args map[string]any) *ToolResult {
	deferred := s.registry.GetDeferredTools()
	if len(deferred) == 0 {
		return SilentResult("No deferred tools available. All tools are already active.")
	}

	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok && int(mr) > 0 {
		maxResults = int(mr)
	}

	queryTokens := tokenize(query)

	type scored struct {
		name  string
		desc  string
		score int
	}

	var results []scored
	for _, entry := range deferred {
		name := entry.Tool.Name()
		desc := entry.Tool.Description()
		nameTokens := tokenize(name)
		descTokens := tokenize(desc)
		hintTokens := tokenize(entry.SearchHint)

		score := 0
		for _, qt := range queryTokens {
			for _, nt := range nameTokens {
				if qt == nt {
					score += 10
				} else if strings.Contains(nt, qt) || strings.Contains(qt, nt) {
					score += 5
				}
			}
			for _, ht := range hintTokens {
				if qt == ht {
					score += 4
				}
			}
			for _, dt := range descTokens {
				if qt == dt {
					score += 2
				}
			}
		}

		if score > 0 {
			results = append(results, scored{name: name, desc: desc, score: score})
		}
	}

	// Sort by score desc, then name asc for determinism
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].name < results[j].name
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	if len(results) == 0 {
		deferredNames := s.registry.GetDeferredNames()
		return SilentResult(fmt.Sprintf(
			"No tools matched query '%s'. Available deferred tools: %s",
			query,
			strings.Join(deferredNames, ", "),
		))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matching tools:\n", len(results)))
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", r.name, r.desc))
	}
	sb.WriteString("\nTo activate, use select: mode, e.g. select:")
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.name
	}
	sb.WriteString(strings.Join(names, ","))

	return SilentResult(sb.String())
}

// tokenize splits a string into lowercase tokens on underscores, spaces, and hyphens,
// filtering out "mcp" tokens (noise from MCP tool naming convention).
func tokenize(s string) []string {
	s = strings.ToLower(s)
	// Replace separators with spaces
	s = strings.NewReplacer("_", " ", "-", " ").Replace(s)
	parts := strings.Fields(s)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "mcp" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}
