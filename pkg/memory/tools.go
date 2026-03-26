package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/tools"
)

// MemoryStoreTool stores a new memory chunk into the memory system.
type MemoryStoreTool struct {
	store    *SQLiteStore
	embedder EmbeddingProvider // may be nil
}

// NewMemoryStoreTool creates a new MemoryStoreTool.
// embedder may be nil; in that case chunks are stored without vector embeddings.
func NewMemoryStoreTool(store *SQLiteStore, embedder EmbeddingProvider) *MemoryStoreTool {
	return &MemoryStoreTool{store: store, embedder: embedder}
}

func (t *MemoryStoreTool) Name() string { return "memory_store" }

func (t *MemoryStoreTool) Description() string {
	return "Store a new memory. Use this when you learn something worth remembering about the user, project, or conversation."
}

func (t *MemoryStoreTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "The content to remember.",
			},
			"heading": map[string]any{
				"type":        "string",
				"description": "Optional heading or category for this memory.",
			},
		},
		"required": []string{"content"},
	}
}

func (t *MemoryStoreTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	content, ok := args["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return tools.ErrorResult("content is required and must be a non-empty string")
	}

	heading, _ := args["heading"].(string)

	chunk := Chunk{
		Content:    content,
		SourceFile: "agent",
		Heading:    heading,
	}

	var err error
	if t.embedder != nil {
		var embedding []float32
		embedding, err = t.embedder.Embed(ctx, content)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("failed to embed content: %v", err)).WithError(err)
		}
		err = t.store.StoreWithEmbedding(chunk, embedding)
	} else {
		err = t.store.Store(chunk)
	}

	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to store memory: %v", err)).WithError(err)
	}

	// Truncate long content for the confirmation message.
	preview := content
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	return tools.SilentResult(fmt.Sprintf("Memory stored: %s", preview))
}

// MemorySearchTool searches stored memories using hybrid FTS + vector search.
type MemorySearchTool struct {
	store    *SQLiteStore
	embedder EmbeddingProvider // may be nil; nil means FTS-only
}

// NewMemorySearchTool creates a new MemorySearchTool.
// embedder may be nil; in that case only FTS keyword search is used.
func NewMemorySearchTool(store *SQLiteStore, embedder EmbeddingProvider) *MemorySearchTool {
	return &MemorySearchTool{store: store, embedder: embedder}
}

func (t *MemorySearchTool) Name() string { return "memory_search" }

func (t *MemorySearchTool) Description() string {
	return "Search memories using keywords and semantic similarity. Returns the most relevant stored memories."
}

func (t *MemorySearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default 5).",
				"default":     5,
			},
		},
		"required": []string{"query"},
	}
}

func (t *MemorySearchTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return tools.ErrorResult("query is required and must be a non-empty string")
	}

	limit := 5
	if raw, exists := args["limit"]; exists {
		switch v := raw.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		case int64:
			limit = int(v)
		}
	}
	if limit <= 0 {
		limit = 5
	}

	var chunks []Chunk

	if t.embedder != nil {
		searcher := NewHybridSearcher(t.store, t.embedder)
		scored, err := searcher.Search(ctx, query, limit)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("search failed: %v", err)).WithError(err)
		}
		for _, sc := range scored {
			chunks = append(chunks, sc.Chunk)
		}
	} else {
		// FTS-only fallback when no embedder is available.
		results, err := t.store.SearchFTS(query, limit)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("search failed: %v", err)).WithError(err)
		}
		chunks = results
	}

	if len(chunks) == 0 {
		return tools.NewToolResult("No memories found.")
	}

	return tools.NewToolResult(formatChunks(chunks))
}

// MemoryRecallTool retrieves memories by source file or heading.
type MemoryRecallTool struct {
	store *SQLiteStore
}

// NewMemoryRecallTool creates a new MemoryRecallTool.
func NewMemoryRecallTool(store *SQLiteStore) *MemoryRecallTool {
	return &MemoryRecallTool{store: store}
}

func (t *MemoryRecallTool) Name() string { return "memory_recall" }

func (t *MemoryRecallTool) Description() string {
	return "Retrieve memories by source file or heading. Use when you know which file or topic to look up."
}

func (t *MemoryRecallTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_file": map[string]any{
				"type":        "string",
				"description": "Filter memories by source file name.",
			},
			"heading": map[string]any{
				"type":        "string",
				"description": "Filter memories by heading or category.",
			},
		},
	}
}

func (t *MemoryRecallTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	sourceFile, _ := args["source_file"].(string)
	heading, _ := args["heading"].(string)

	if strings.TrimSpace(sourceFile) == "" && strings.TrimSpace(heading) == "" {
		return tools.ErrorResult("at least one of source_file or heading is required")
	}

	chunks, err := t.store.RecallBySource(sourceFile, heading, 50)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("recall failed: %v", err)).WithError(err)
	}

	if len(chunks) == 0 {
		return tools.NewToolResult("No memories found.")
	}

	return tools.NewToolResult(formatChunks(chunks))
}

// formatChunks renders a slice of chunks as human-readable text for the LLM.
func formatChunks(chunks []Chunk) string {
	var sb strings.Builder
	for i, ch := range chunks {
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		if ch.Heading != "" {
			sb.WriteString(fmt.Sprintf("[%s] ", ch.Heading))
		}
		if ch.SourceFile != "" && ch.SourceFile != "agent" {
			sb.WriteString(fmt.Sprintf("(%s) ", ch.SourceFile))
		}
		sb.WriteString(ch.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
