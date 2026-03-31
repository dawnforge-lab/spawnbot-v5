package skills

import (
	"fmt"
	"strings"
)

// SubstituteArgs replaces variable placeholders in a skill body.
// Supported variables: ${ARGUMENTS}, ${ARG1}..${ARGN}, ${SKILL_DIR}, ${WORKSPACE}.
// Undefined positional args are left as-is.
func SubstituteArgs(body string, args []string, skillDir, workspace string) string {
	body = strings.ReplaceAll(body, "${ARGUMENTS}", strings.Join(args, " "))
	for i, arg := range args {
		placeholder := fmt.Sprintf("${ARG%d}", i+1)
		body = strings.ReplaceAll(body, placeholder, arg)
	}
	body = strings.ReplaceAll(body, "${SKILL_DIR}", skillDir)
	body = strings.ReplaceAll(body, "${WORKSPACE}", workspace)
	return body
}
