package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func setupDeferredRegistry() *ToolRegistry {
	reg := NewToolRegistry()
	reg.Register(&mockSearchableTool{name: "read_file", desc: "Read file contents"})
	reg.RegisterHidden(&mockSearchableTool{name: "web_search", desc: "Search the web using a search engine"})
	reg.RegisterHidden(&mockSearchableTool{name: "web_fetch", desc: "Fetch content from a URL"})
	reg.RegisterHidden(&mockSearchableTool{name: "spawn", desc: "Spawn an async subagent"})
	reg.RegisterHiddenWithHint(&mockSearchableTool{name: "mcp_srv_analyze", desc: "Analyze data patterns"}, "data analysis statistics")
	return reg
}

func TestSearchTools_SelectMode_ActivatesTool(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "select:web_search,web_fetch"})
	if res.IsError {
		t.Fatalf("Expected success, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "Activated 2 tools:") {
		t.Errorf("Expected 'Activated 2 tools:', got: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "web_search") {
		t.Errorf("Expected web_search in output, got: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "web_fetch") {
		t.Errorf("Expected web_fetch in output, got: %s", res.ForLLM)
	}

	// Verify actually promoted
	if _, ok := reg.Get("web_search"); !ok {
		t.Error("web_search should be gettable after promotion")
	}
	if _, ok := reg.Get("web_fetch"); !ok {
		t.Error("web_fetch should be gettable after promotion")
	}
}

func TestSearchTools_SelectMode_NotFound(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "select:nonexistent"})
	if !res.IsError {
		t.Fatalf("Expected error, got success: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "not found") {
		t.Errorf("Expected 'not found' in error, got: %s", res.ForLLM)
	}
	// Should list available deferred tools
	if !strings.Contains(res.ForLLM, "web_search") {
		t.Errorf("Expected available deferred tool names in error, got: %s", res.ForLLM)
	}
}

func TestSearchTools_SelectMode_PartialMatch(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "select:web_search,nonexistent"})
	if !res.IsError {
		t.Fatalf("Expected error for partial match, got success: %s", res.ForLLM)
	}
	// Should mention the activated tool
	if !strings.Contains(res.ForLLM, "web_search") {
		t.Errorf("Expected web_search mentioned, got: %s", res.ForLLM)
	}
	// Should mention the not-found tool
	if !strings.Contains(res.ForLLM, "nonexistent") {
		t.Errorf("Expected nonexistent mentioned, got: %s", res.ForLLM)
	}

	// web_search should still be promoted despite the error
	if _, ok := reg.Get("web_search"); !ok {
		t.Error("web_search should be gettable after partial promotion")
	}
}

func TestSearchTools_KeywordMode_FindsByName(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "web"})
	if res.IsError {
		t.Fatalf("Expected success, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "web_search") {
		t.Errorf("Expected web_search in results, got: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "web_fetch") {
		t.Errorf("Expected web_fetch in results, got: %s", res.ForLLM)
	}
	// Should NOT auto-activate
	if _, ok := reg.Get("web_search"); ok {
		t.Error("Keyword mode should NOT auto-activate tools")
	}
	// Should have hint to use select:
	if !strings.Contains(res.ForLLM, "select:") {
		t.Errorf("Expected select: hint in output, got: %s", res.ForLLM)
	}
}

func TestSearchTools_KeywordMode_FindsByDescription(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "subagent"})
	if res.IsError {
		t.Fatalf("Expected success, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "spawn") {
		t.Errorf("Expected spawn in results, got: %s", res.ForLLM)
	}
}

func TestSearchTools_KeywordMode_FindsBySearchHint(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "statistics"})
	if res.IsError {
		t.Fatalf("Expected success, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "mcp_srv_analyze") {
		t.Errorf("Expected mcp_srv_analyze in results, got: %s", res.ForLLM)
	}
}

func TestSearchTools_KeywordMode_NoResults(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "zzz_nonexistent"})
	if res.IsError {
		t.Fatalf("Expected silent result, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "No tools matched") {
		t.Errorf("Expected 'No tools matched', got: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "Available deferred tools:") {
		t.Errorf("Expected available deferred tools list, got: %s", res.ForLLM)
	}
}

func TestSearchTools_KeywordMode_MaxResults(t *testing.T) {
	reg := NewToolRegistry()
	for i := 0; i < 20; i++ {
		reg.RegisterHidden(&mockSearchableTool{
			name: fmt.Sprintf("tool_%02d", i),
			desc: "searchable tool for testing",
		})
	}
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "tool", "max_results": float64(3)})
	if res.IsError {
		t.Fatalf("Expected success, got error: %s", res.ForLLM)
	}
	// Count bullet lines
	bullets := 0
	for _, line := range strings.Split(res.ForLLM, "\n") {
		if strings.HasPrefix(line, "- ") {
			bullets++
		}
	}
	if bullets != 3 {
		t.Errorf("Expected 3 bullet results, got %d. Output:\n%s", bullets, res.ForLLM)
	}
}

func TestSearchTools_EmptyQuery(t *testing.T) {
	reg := setupDeferredRegistry()
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	for _, q := range []any{"", "   ", nil} {
		args := map[string]any{}
		if q != nil {
			args["query"] = q
		}
		res := tool.Execute(ctx, args)
		if !res.IsError {
			t.Errorf("Expected error for query=%v, got success: %s", q, res.ForLLM)
		}
	}
}

func TestSearchTools_AllDiscovered(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&mockSearchableTool{name: "read_file", desc: "Read file contents"})
	// No hidden tools at all
	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "anything"})
	if res.IsError {
		t.Fatalf("Expected silent result, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "No deferred tools available") {
		t.Errorf("Expected 'No deferred tools available', got: %s", res.ForLLM)
	}
}

func TestSearchTools_SelectMode_AlreadyDiscovered(t *testing.T) {
	reg := setupDeferredRegistry()
	// Pre-promote web_search
	reg.PromoteTools([]string{"web_search"})

	tool := &SearchTools{registry: reg}
	ctx := context.Background()

	res := tool.Execute(ctx, map[string]any{"query": "select:web_search"})
	if res.IsError {
		t.Fatalf("Expected success for already-discovered tool, got error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "Activated 1 tools:") {
		t.Errorf("Expected 'Activated 1 tools:', got: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "web_search") {
		t.Errorf("Expected web_search in output, got: %s", res.ForLLM)
	}
}
