package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers"
)

const (
	// PeriodicFlushInterval is the number of messages between periodic memory flushes.
	PeriodicFlushInterval = 15

	// memoryFlushTimeout is the maximum time allowed for a memory flush LLM call.
	memoryFlushTimeout = 30 * time.Second

	// memoryFlushMaxRetries is the number of LLM retry attempts for extraction.
	memoryFlushMaxRetries = 2
)

// memoryFlushTracker tracks per-session message counts for periodic flushing.
// Key: sessionKey, Value: message count since last flush.
var memoryFlushTracker sync.Map

// memoryFlushExtractPrompt asks the LLM to extract key facts from messages.
// It includes existing daily notes for deduplication.
const memoryFlushExtractPrompt = `Extract key facts from this conversation segment that should be preserved in long-term memory.

RULES:
- Only extract decisions, preferences, important facts, task outcomes, and actionable conclusions
- Skip routine chatter, greetings, and information that is only relevant in the moment
- Each fact should be a single concise bullet point
- Do NOT repeat facts already present in "EXISTING NOTES" below
- If all facts are already covered by existing notes, respond with exactly: NOTHING_NEW
- Output ONLY the bullet points (one per line, starting with "- "), or NOTHING_NEW

EXISTING NOTES:
%s

CONVERSATION:
%s`

// flushMemoryPreCompaction extracts key facts from messages about to be dropped
// during summarization and writes them to daily notes.
func (al *AgentLoop) flushMemoryPreCompaction(
	agent *AgentInstance,
	messages []providers.Message,
) {
	if agent.MemoryStore == nil || len(messages) == 0 {
		return
	}

	existing := agent.MemoryStore.ReadToday()
	facts := al.extractKeyFacts(agent, messages, existing)
	if facts == "" {
		return
	}

	if err := agent.MemoryStore.AppendToday(facts); err != nil {
		logger.WarnCF("agent", "Failed to flush memory pre-compaction",
			map[string]any{"error": err.Error(), "agent_id": agent.ID})
		return
	}

	logger.InfoCF("agent", "Memory flushed pre-compaction",
		map[string]any{
			"agent_id": agent.ID,
			"facts":    strings.Count(facts, "\n") + 1,
		})
}

// maybePeriodicFlush checks if enough messages have accumulated since the last
// flush and writes key facts to daily notes. Called at the end of each turn.
func (al *AgentLoop) maybePeriodicFlush(agent *AgentInstance, sessionKey string) {
	if agent.MemoryStore == nil {
		return
	}

	// Increment message counter for this session.
	trackerKey := agent.ID + ":" + sessionKey
	countVal, _ := memoryFlushTracker.LoadOrStore(trackerKey, 0)
	count := countVal.(int)

	history := agent.Sessions.GetHistory(sessionKey)
	count = len(history)
	memoryFlushTracker.Store(trackerKey, count)

	if count < PeriodicFlushInterval {
		return
	}

	// Check if count crossed a flush boundary.
	// We flush at 15, 30, 45, ... messages.
	prevCheckpoint := (count - 1) / PeriodicFlushInterval * PeriodicFlushInterval
	currCheckpoint := count / PeriodicFlushInterval * PeriodicFlushInterval
	if currCheckpoint <= prevCheckpoint || currCheckpoint == 0 {
		return
	}

	// Take the last PeriodicFlushInterval messages for extraction.
	start := len(history) - PeriodicFlushInterval
	if start < 0 {
		start = 0
	}
	recentMessages := history[start:]

	go func() {
		existing := agent.MemoryStore.ReadToday()
		facts := al.extractKeyFacts(agent, recentMessages, existing)
		if facts == "" {
			return
		}

		if err := agent.MemoryStore.AppendToday(facts); err != nil {
			logger.WarnCF("agent", "Failed periodic memory flush",
				map[string]any{"error": err.Error(), "agent_id": agent.ID})
			return
		}

		logger.InfoCF("agent", "Periodic memory flush completed",
			map[string]any{
				"agent_id":      agent.ID,
				"session_key":   sessionKey,
				"message_count": count,
				"facts":         strings.Count(facts, "\n") + 1,
			})
	}()
}

// extractKeyFacts uses the LLM to extract key facts from messages,
// deduplicating against existing daily notes.
func (al *AgentLoop) extractKeyFacts(
	agent *AgentInstance,
	messages []providers.Message,
	existingNotes string,
) string {
	ctx, cancel := context.WithTimeout(context.Background(), memoryFlushTimeout)
	defer cancel()

	// Build conversation text from messages.
	var convBuf strings.Builder
	for _, m := range messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		// Truncate very long messages to keep the prompt reasonable.
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Fprintf(&convBuf, "%s: %s\n", m.Role, content)
	}

	if convBuf.Len() == 0 {
		return ""
	}

	if existingNotes == "" {
		existingNotes = "(none)"
	}

	prompt := fmt.Sprintf(memoryFlushExtractPrompt, existingNotes, convBuf.String())

	resp, err := al.retryLLMCall(ctx, agent, prompt, memoryFlushMaxRetries)
	if err != nil || resp == nil || resp.Content == "" {
		logger.Debug("Memory flush: LLM extraction failed or returned empty")
		return ""
	}

	result := strings.TrimSpace(resp.Content)

	// LLM indicated no new facts to store.
	if result == "NOTHING_NEW" {
		return ""
	}

	// Validate output: should contain bullet points.
	if !strings.Contains(result, "- ") {
		return ""
	}

	return "\n## Memory Flush\n\n" + result
}
