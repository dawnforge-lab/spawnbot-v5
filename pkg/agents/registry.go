package agents

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	agents   map[string]*AgentDefinition
	builtins map[string]bool
	warnings []string
}

func NewRegistry() *Registry {
	return &Registry{
		agents:   make(map[string]*AgentDefinition),
		builtins: make(map[string]bool),
	}
}

func (r *Registry) Register(def *AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[def.Name] = def
	if def.Source == "builtin" {
		r.builtins[def.Name] = true
	}
}

func (r *Registry) Get(name string) *AgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[name]
}

func (r *Registry) List() []*AgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*AgentDefinition, 0, len(r.agents))
	for _, def := range r.agents {
		list = append(list, def)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (r *Registry) IsBuiltin(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.builtins[name]
}

func (r *Registry) AddWarning(warning string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnings = append(r.warnings, warning)
}

func (r *Registry) Summary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.agents) == 0 && len(r.warnings) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "Available agents:")

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		def := r.agents[name]
		lines = append(lines, fmt.Sprintf("- %s: %s", def.Name, def.Description))
	}

	for _, w := range r.warnings {
		lines = append(lines, fmt.Sprintf("WARNING: %s", w))
	}

	return strings.Join(lines, "\n")
}

func (r *Registry) Reload(workspaceAgentsDir string) {
	agents, warnings := LoadFromDir(workspaceAgentsDir)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear workspace agents and warnings, keep builtins
	for name, def := range r.agents {
		if def.Source != "builtin" {
			delete(r.agents, name)
		}
	}
	r.warnings = nil

	for _, def := range agents {
		r.agents[def.Name] = def
	}
	r.warnings = warnings
}
