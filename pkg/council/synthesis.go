package council

import (
	"regexp"
	"strings"
)

var twoOrMoreSpaces = regexp.MustCompile(`\s{2,}`)

// parseSynthesisOutput parses the structured output from generateSynthesis.
// Expected format:
//
//	Summary:
//	<summary text>
//
//	Tasks:
//	- agent: <name>  task: <description>  priority: high|medium|low
//
// If the format is not followed, the full raw text is returned as summary
// with an empty task list.
func parseSynthesisOutput(raw string) (summary string, tasks []CouncilTask) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	const tasksMarker = "\nTasks:\n"
	idx := strings.Index(raw, tasksMarker)
	if idx < 0 {
		return raw, nil
	}

	summaryPart := strings.TrimSpace(raw[:idx])
	summaryPart = strings.TrimPrefix(summaryPart, "Summary:")
	summaryPart = strings.TrimSpace(summaryPart)

	tasksPart := raw[idx+len(tasksMarker):]
	for _, line := range strings.Split(tasksPart, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		t := parseTaskLine(strings.TrimPrefix(line, "- "))
		if t.Agent != "" && t.Task != "" {
			tasks = append(tasks, t)
		}
	}

	return summaryPart, tasks
}

// parseTaskLine parses a single task line of the form:
// agent: <name>  task: <description>  priority: <level>
// Fields are separated by two or more spaces.
func parseTaskLine(line string) CouncilTask {
	var t CouncilTask
	for _, part := range twoOrMoreSpaces.Split(line, -1) {
		i := strings.Index(part, ": ")
		if i < 0 {
			continue
		}
		key := strings.TrimSpace(part[:i])
		val := strings.TrimSpace(part[i+2:])
		switch key {
		case "agent":
			t.Agent = val
		case "task":
			t.Task = val
		case "priority":
			t.Priority = val
		}
	}
	return t
}
