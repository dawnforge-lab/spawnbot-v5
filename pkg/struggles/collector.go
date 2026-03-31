package struggles

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
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
type Collector struct {
	logPath string
}

// NewCollector creates a new struggle signal collector.
func NewCollector(logPath string) *Collector {
	return &Collector{logPath: logPath}
}

// OnToolResult logs a signal when a tool call returns an error.
func (c *Collector) OnToolResult(toolName string, args map[string]any, isError bool, errorMsg, session string) {
	if !isError {
		return
	}

	context := truncate(formatArgs(args), maxContextLen)
	c.append(Signal{
		Timestamp: time.Now(),
		Type:      TypeToolError,
		Tool:      toolName,
		Error:     truncate(errorMsg, maxContextLen),
		Session:   session,
		Context:   context,
	})
}

// OnUserMessage logs a signal when the user's message matches correction patterns.
func (c *Collector) OnUserMessage(userMsg, prevAssistant, session string) {
	if !correctionPatterns.MatchString(strings.TrimSpace(userMsg)) {
		return
	}

	c.append(Signal{
		Timestamp: time.Now(),
		Type:      TypeUserCorrection,
		Error:     truncate(userMsg, maxContextLen),
		Context:   truncate(prevAssistant, maxContextLen),
		Session:   session,
	})
}

// OnTurnEnd logs signals for tools called 3+ times in a single turn.
func (c *Collector) OnTurnEnd(toolCallCounts map[string]int, session string) {
	for tool, count := range toolCallCounts {
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

func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	data, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(data)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
