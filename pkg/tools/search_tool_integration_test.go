package tools

import (
	"context"
	"strings"
	"testing"
)

func TestDeferredToolLoading_FullFlow(t *testing.T) {
	// Setup: registry with core + hidden tools
	reg := NewToolRegistry()
	reg.Register(&mockSearchableTool{name: "read_file", desc: "Read file contents"})
	reg.Register(&mockSearchableTool{name: "exec", desc: "Execute shell commands"})
	reg.RegisterHidden(&mockSearchableTool{name: "web_search", desc: "Search the web using a search engine"})
	reg.RegisterHidden(&mockSearchableTool{name: "web_fetch", desc: "Fetch content from a URL"})
	reg.RegisterHidden(&mockSearchableTool{name: "spawn", desc: "Spawn an async subagent process"})
	reg.RegisterHiddenWithHint(&mockSearchableTool{name: "mcp_db_query", desc: "Query a database"}, "sql database postgres")

	searchTool := NewSearchTools(reg)
	reg.Register(searchTool) // Always core
	ctx := context.Background()

	// 1. Verify initial state: only core tools in provider defs
	defs := reg.ToProviderDefs()
	defNames := make(map[string]bool)
	for _, d := range defs {
		defNames[d.Function.Name] = true
	}
	if !defNames["read_file"] || !defNames["exec"] || !defNames["search_tools"] {
		t.Fatal("core tools should be in provider defs")
	}
	if defNames["web_search"] || defNames["spawn"] || defNames["mcp_db_query"] {
		t.Fatal("hidden tools should NOT be in provider defs before discovery")
	}

	// 2. Verify deferred names
	deferred := reg.GetDeferredNames()
	if len(deferred) != 4 {
		t.Fatalf("expected 4 deferred tools, got %d: %v", len(deferred), deferred)
	}

	// 3. Keyword search — find web tools
	res := searchTool.Execute(ctx, map[string]any{"query": "web"})
	if !strings.Contains(res.ForLLM, "web_search") || !strings.Contains(res.ForLLM, "web_fetch") {
		t.Errorf("keyword search should find web tools, got: %s", res.ForLLM)
	}
	// Keyword search should NOT activate
	if _, ok := reg.Get("web_search"); ok {
		t.Error("keyword search should not activate tools")
	}

	// 4. Select to activate
	res = searchTool.Execute(ctx, map[string]any{"query": "select:web_search,web_fetch"})
	if res.IsError {
		t.Fatalf("select should succeed: %s", res.ForLLM)
	}

	// 5. Verify activated tools are now in provider defs
	defs = reg.ToProviderDefs()
	defNames = make(map[string]bool)
	for _, d := range defs {
		defNames[d.Function.Name] = true
	}
	if !defNames["web_search"] || !defNames["web_fetch"] {
		t.Fatal("activated tools should be in provider defs")
	}
	if defNames["spawn"] || defNames["mcp_db_query"] {
		t.Fatal("non-activated hidden tools should still be excluded")
	}

	// 6. Deferred list should shrink
	deferred = reg.GetDeferredNames()
	if len(deferred) != 2 {
		t.Fatalf("expected 2 remaining deferred tools, got %d: %v", len(deferred), deferred)
	}

	// 7. Search by hint
	res = searchTool.Execute(ctx, map[string]any{"query": "postgres"})
	if !strings.Contains(res.ForLLM, "mcp_db_query") {
		t.Errorf("search by hint should find mcp_db_query, got: %s", res.ForLLM)
	}

	// 8. Tools stay discovered (session-persistent)
	for i := 0; i < 100; i++ {
		if _, ok := reg.Get("web_search"); !ok {
			t.Fatalf("discovered tool should stay available at iteration %d", i)
		}
	}

	// 9. Clone preserves discovery state
	clone := reg.Clone()
	if _, ok := clone.Get("web_search"); !ok {
		t.Fatal("cloned registry should preserve discovered tools")
	}
	if _, ok := clone.Get("spawn"); ok {
		t.Fatal("cloned registry should not have undiscovered tools gettable")
	}
}
