package agents

import (
	"fmt"
	_ "embed"
)

//go:embed builtin/researcher/AGENT.md
var researcherAgentMD string

//go:embed builtin/coder/AGENT.md
var coderAgentMD string

//go:embed builtin/reviewer/AGENT.md
var reviewerAgentMD string

//go:embed builtin/planner/AGENT.md
var plannerAgentMD string

func (r *Registry) LoadBuiltins() error {
	builtins := map[string]string{
		"researcher": researcherAgentMD,
		"coder":      coderAgentMD,
		"reviewer":   reviewerAgentMD,
		"planner":    plannerAgentMD,
	}

	for name, content := range builtins {
		def, err := ParseAgentMD(content, "builtin", "")
		if err != nil {
			return fmt.Errorf("failed to parse builtin agent %q: %w", name, err)
		}
		r.Register(def)
	}

	return nil
}
