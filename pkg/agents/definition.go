package agents

import (
	"fmt"
	"regexp"
	"time"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-]*$`)

const (
	maxNameLength        = 64
	maxDescriptionLength = 1024
	defaultMaxIterations = 20
	defaultTimeout       = 5 * time.Minute
)

type AgentDefinition struct {
	Name          string
	Description   string
	SystemPrompt  string
	Model         string
	ToolsAllow    []string
	ToolsDeny     []string
	MaxIterations int
	Timeout       time.Duration
	Source        string // "builtin" | "workspace"
	BaseDir       string
}

func (d *AgentDefinition) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if len(d.Name) > maxNameLength {
		return fmt.Errorf("agent name must be at most %d characters, got %d", maxNameLength, len(d.Name))
	}
	if !namePattern.MatchString(d.Name) {
		return fmt.Errorf("agent name must be alphanumeric + hyphens, got %q", d.Name)
	}
	if d.Description == "" {
		return fmt.Errorf("agent description is required")
	}
	if len(d.Description) > maxDescriptionLength {
		return fmt.Errorf("agent description must be at most %d characters", maxDescriptionLength)
	}
	return nil
}

func (d *AgentDefinition) ApplyDefaults() {
	if d.MaxIterations <= 0 {
		d.MaxIterations = defaultMaxIterations
	}
	if d.Timeout <= 0 {
		d.Timeout = defaultTimeout
	}
}

func (d *AgentDefinition) FilterTools(allTools []string) []string {
	deny := make(map[string]bool)
	for _, t := range d.ToolsDeny {
		deny[t] = true
	}

	hasExplicitAllow := len(d.ToolsAllow) > 0
	if !hasExplicitAllow {
		deny["spawn"] = true
		deny["subagent"] = true
	}

	var pool []string
	if hasExplicitAllow {
		allow := make(map[string]bool)
		for _, t := range d.ToolsAllow {
			allow[t] = true
		}
		for _, t := range allTools {
			if allow[t] {
				pool = append(pool, t)
			}
		}
	} else {
		pool = allTools
	}

	var result []string
	for _, t := range pool {
		if !deny[t] {
			result = append(result, t)
		}
	}
	return result
}
