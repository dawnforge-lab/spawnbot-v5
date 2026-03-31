package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/skills"
)

// SkillCommand creates a command definition for /skill that lists or activates skills.
func SkillCommand(loader *skills.SkillsLoader) Definition {
	return Definition{
		Name:        "skill",
		Description: "List or activate a skill",
		Usage:       "/skill [name [args...]]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			args := strings.Fields(strings.TrimSpace(req.Text))
			// Strip the leading "/skill" token if present.
			if len(args) > 0 {
				if name, ok := trimCommandPrefix(args[0]); ok && strings.ToLower(name) == "skill" {
					args = args[1:]
				}
			}

			if len(args) == 0 {
				return req.Reply(formatSkillList(loader))
			}

			skillName := args[0]
			skillArgs := strings.Join(args[1:], " ")
			return req.Reply(formatSkillActivation(skillName, skillArgs))
		},
	}
}

func formatSkillList(loader *skills.SkillsLoader) string {
	all := loader.ListSkills()

	var invocable []skills.SkillInfo
	for _, s := range all {
		if s.UserInvocable {
			invocable = append(invocable, s)
		}
	}

	if len(invocable) == 0 {
		return "No skills available."
	}

	lines := make([]string, 0, len(invocable))
	for _, s := range invocable {
		usage := "/" + s.Name
		if s.ArgumentHint != "" {
			usage += " " + s.ArgumentHint
		}
		desc := s.Description
		if desc == "" {
			desc = "No description"
		}
		lines = append(lines, fmt.Sprintf("%s - %s", usage, desc))
	}
	return strings.Join(lines, "\n")
}

func formatSkillActivation(name, args string) string {
	if args != "" {
		return fmt.Sprintf("Activating skill: %s %s", name, args)
	}
	return fmt.Sprintf("Activating skill: %s", name)
}
