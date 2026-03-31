package struggles

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

const (
	repeatedToolThreshold = 3
	maxContextLen         = 200
)

var correctionPatterns = regexp.MustCompile(
	`(?i)^(no[,.\s]|wrong|not that|I said|try again|instead[,.\s]|that's not|you should have|I meant|I asked for)`,
)

// Collector detects struggle signals during conversations and appends them to a JSONL log.
// It implements agent.EventObserver — mount it on the HookManager to receive events.
type Collector struct {
	logPath string

	mu             sync.Mutex
	turnToolCounts map[string]int
	turnSession    string
}

// NewCollector creates a new struggle signal collector.
func NewCollector(logPath string) *Collector {
	return &Collector{
		logPath:        logPath,
		turnToolCounts: make(map[string]int),
	}
}

// HandleTurnStart processes a turn start event.
// Resets per-turn tool counters and checks for user correction patterns.
func (c *Collector) HandleTurnStart(userMessage, session string) {
	c.mu.Lock()
	c.turnToolCounts = make(map[string]int)
	c.turnSession = session
	c.mu.Unlock()

	if userMessage != "" && correctionPatterns.MatchString(strings.TrimSpace(userMessage)) {
		c.append(Signal{
			Timestamp: time.Now(),
			Type:      TypeUserCorrection,
			Error:     truncate(userMessage, maxContextLen),
			Session:   session,
		})
	}
}

// HandleToolEnd processes a tool execution end event.
// Logs error signals and increments per-turn tool counters.
func (c *Collector) HandleToolEnd(toolName string, isError bool, errorMsg, session string) {
	c.mu.Lock()
	c.turnToolCounts[toolName]++
	c.mu.Unlock()

	if isError {
		c.append(Signal{
			Timestamp: time.Now(),
			Type:      TypeToolError,
			Tool:      toolName,
			Error:     truncate(errorMsg, maxContextLen),
			Session:   session,
		})
	}
}

// HandleTurnEnd processes a turn end event.
// Emits repeated-tool signals for any tool called 3+ times this turn.
func (c *Collector) HandleTurnEnd(session string) {
	c.mu.Lock()
	counts := make(map[string]int, len(c.turnToolCounts))
	for k, v := range c.turnToolCounts {
		counts[k] = v
	}
	c.mu.Unlock()

	for tool, count := range counts {
		if count >= repeatedToolThreshold {
			c.append(Signal{
				Timestamp: time.Now(),
				Type:      TypeRepeatedTool,
				Tool:      tool,
				Count:     count,
				Session:   session,
			})
		}
	}
}

func (c *Collector) append(sig Signal) {
	f, err := os.OpenFile(c.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		logger.ErrorCF("struggles", "Failed to open struggle log", map[string]any{"error": err.Error()})
		return
	}
	defer f.Close()

	data, err := json.Marshal(sig)
	if err != nil {
		logger.ErrorCF("struggles", "Failed to marshal signal", map[string]any{"error": err.Error()})
		return
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		logger.ErrorCF("struggles", "Failed to write signal", map[string]any{"error": err.Error()})
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
