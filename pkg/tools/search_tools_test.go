package tools

import (
	"context"
	"fmt"
	"testing"
)

// Dummy tool to fill the registry in our tests.
type mockSearchableTool struct {
	name string
	desc string
}

func (m *mockSearchableTool) Name() string        { return m.name }
func (m *mockSearchableTool) Description() string { return m.desc }
func (m *mockSearchableTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}

func (m *mockSearchableTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	return SilentResult("mock executed: " + m.name)
}

// Helper to initialize a populated ToolRegistry
func setupPopulatedRegistry() *ToolRegistry {
	reg := NewToolRegistry()

	// A core tool (NOT to be found by searches)
	reg.Register(&mockSearchableTool{
		name: "core_search",
		desc: "I am a visible core tool for searching files",
	})

	// Hidden tools (must be found by searches)
	reg.RegisterHidden(&mockSearchableTool{
		name: "mcp_read_file",
		desc: "Read the contents of a system file",
	})
	reg.RegisterHidden(&mockSearchableTool{
		name: "mcp_list_dir",
		desc: "List directories and files in the system",
	})
	reg.RegisterHidden(&mockSearchableTool{
		name: "mcp_fetch_net",
		desc: "Fetch data from a network database",
	})

	return reg
}


func TestSearchRegex_ZeroMaxResults(t *testing.T) {
	reg := setupPopulatedRegistry()

	res, err := reg.SearchRegex("mcp", 0)
	if err != nil {
		t.Fatalf("SearchRegex failed: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("Expected 0 results with maxSearchResults=0, got %d", len(res))
	}
}

func TestSearchBM25_ZeroMaxResults(t *testing.T) {
	reg := setupPopulatedRegistry()

	res := reg.SearchBM25("read file", 0)
	if len(res) != 0 {
		t.Errorf("Expected 0 results with maxSearchResults=0, got %d", len(res))
	}
}

func TestSearchRegex_DeterministicOrder(t *testing.T) {
	reg := NewToolRegistry()
	for i := 0; i < 20; i++ {
		reg.RegisterHidden(&mockSearchableTool{
			name: fmt.Sprintf("tool_%02d", i),
			desc: "searchable tool",
		})
	}

	// Run the same search multiple times and verify order is stable
	var firstRun []string
	for attempt := 0; attempt < 10; attempt++ {
		res, err := reg.SearchRegex("searchable", 20)
		if err != nil {
			t.Fatalf("SearchRegex failed: %v", err)
		}

		names := make([]string, len(res))
		for i, r := range res {
			names[i] = r.Name
		}

		if attempt == 0 {
			firstRun = names
		} else {
			for i, name := range names {
				if name != firstRun[i] {
					t.Fatalf("Non-deterministic order at attempt %d, index %d: got %q, want %q",
						attempt, i, name, firstRun[i])
				}
			}
		}
	}
}

func TestToolRegistry_SearchLimitsAndCoreFiltering(t *testing.T) {
	reg := NewToolRegistry()

	// Add 1 Core and 10 Hidden, all containing the word "match"
	reg.Register(&mockSearchableTool{"core_match", "I am core with match"})
	for i := 0; i < 10; i++ {
		reg.RegisterHidden(&mockSearchableTool{
			name: fmt.Sprintf("hidden_match_%d", i),
			desc: "this has a match",
		})
	}

	t.Run("Regex limits and core filtering", func(t *testing.T) {
		// Search with Regex and a limit of maxSearchResults = 4
		res, err := reg.SearchRegex("match", 4)
		if err != nil {
			t.Fatalf("SearchRegex failed: %v", err)
		}

		if len(res) != 4 {
			t.Errorf("Expected exactly 4 results due to limit, got %d", len(res))
		}

		for _, r := range res {
			if r.Name == "core_match" {
				t.Errorf("SearchRegex returned a Core tool, which should be excluded")
			}
		}
	})

	t.Run("BM25 limits and core filtering", func(t *testing.T) {
		// Search with BM25 and a limit of maxSearchResults = 3
		res := reg.SearchBM25("match", 3)

		if len(res) != 3 {
			t.Errorf("Expected exactly 3 results due to limit, got %d", len(res))
		}

		for _, r := range res {
			if r.Name == "core_match" {
				t.Errorf("SearchBM25 returned a Core tool, which should be excluded")
			}
		}
	})
}

func TestGet_HiddenToolDiscoveryLifecycle(t *testing.T) {
	reg := NewToolRegistry()
	reg.RegisterHidden(&mockSearchableTool{name: "hidden_tool", desc: "test"})

	// Not discovered → not gettable
	_, ok := reg.Get("hidden_tool")
	if ok {
		t.Error("Expected undiscovered hidden tool to NOT be gettable")
	}

	// Promote → gettable (session-persistent)
	reg.PromoteTools([]string{"hidden_tool"})
	_, ok = reg.Get("hidden_tool")
	if !ok {
		t.Error("Expected promoted hidden tool to be gettable")
	}

	// Stays gettable (no TTL decay)
	_, ok = reg.Get("hidden_tool")
	if !ok {
		t.Error("Expected discovered tool to stay gettable")
	}

	// Core tools remain always gettable
	reg.Register(&mockSearchableTool{name: "core_tool", desc: "core"})
	_, ok = reg.Get("core_tool")
	if !ok {
		t.Error("Expected core tool to always be gettable")
	}
}


func TestPromoteTools_ConcurrentAccess(t *testing.T) {
	reg := NewToolRegistry()
	for i := 0; i < 20; i++ {
		reg.RegisterHidden(&mockSearchableTool{
			name: fmt.Sprintf("concurrent_tool_%d", i),
			desc: "concurrent test tool",
		})
	}

	names := make([]string, 20)
	for i := 0; i < 20; i++ {
		names[i] = fmt.Sprintf("concurrent_tool_%d", i)
	}

	// Hammer PromoteTools and Get concurrently to detect races
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			reg.PromoteTools(names)
		}
		close(done)
	}()

	for i := 0; i < 1000; i++ {
		reg.Get(fmt.Sprintf("concurrent_tool_%d", i%20))
	}
	<-done
}
