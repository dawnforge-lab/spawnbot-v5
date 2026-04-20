package council

import (
	"testing"
)

func TestParseSynthesisOutput_FullFormat(t *testing.T) {
	raw := `Summary:
The council agreed on a phased rollout.

Tasks:
- agent: researcher  task: investigate caching options  priority: high
- agent: main  task: write rollout plan  priority: medium`

	summary, tasks := parseSynthesisOutput(raw)

	if summary != "The council agreed on a phased rollout." {
		t.Errorf("unexpected summary: %q", summary)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Agent != "researcher" || tasks[0].Task != "investigate caching options" || tasks[0].Priority != "high" {
		t.Errorf("unexpected task[0]: %+v", tasks[0])
	}
	if tasks[1].Agent != "main" || tasks[1].Task != "write rollout plan" || tasks[1].Priority != "medium" {
		t.Errorf("unexpected task[1]: %+v", tasks[1])
	}
}

func TestParseSynthesisOutput_NoSummaryPrefix(t *testing.T) {
	raw := `The council reached consensus on approach B.

Tasks:
- agent: researcher  task: validate hypothesis  priority: high`

	summary, tasks := parseSynthesisOutput(raw)

	if summary != "The council reached consensus on approach B." {
		t.Errorf("unexpected summary: %q", summary)
	}
	if len(tasks) != 1 || tasks[0].Agent != "researcher" {
		t.Errorf("unexpected tasks: %+v", tasks)
	}
}

func TestParseSynthesisOutput_NoTasksSection(t *testing.T) {
	raw := "The council discussed many things but reached no clear conclusion."

	summary, tasks := parseSynthesisOutput(raw)

	if summary != raw {
		t.Errorf("expected full raw text as summary, got %q", summary)
	}
	if len(tasks) != 0 {
		t.Errorf("expected no tasks, got %v", tasks)
	}
}

func TestParseSynthesisOutput_MissingPriority(t *testing.T) {
	raw := `Summary:
Brief summary.

Tasks:
- agent: main  task: do the thing`

	_, tasks := parseSynthesisOutput(raw)

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Priority != "" {
		t.Errorf("expected empty priority, got %q", tasks[0].Priority)
	}
}

func TestParseSynthesisOutput_EmptyAgent_Skipped(t *testing.T) {
	raw := `Summary:
Some summary.

Tasks:
- agent:   task: orphaned task  priority: low
- agent: main  task: valid task  priority: low`

	_, tasks := parseSynthesisOutput(raw)

	if len(tasks) != 1 || tasks[0].Agent != "main" {
		t.Errorf("expected only the valid task, got %+v", tasks)
	}
}

func TestParseSynthesisOutput_Empty(t *testing.T) {
	summary, tasks := parseSynthesisOutput("")

	if summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
	if len(tasks) != 0 {
		t.Errorf("expected no tasks, got %v", tasks)
	}
}

func TestExtractSynthesisFromArgs_Full(t *testing.T) {
	args := map[string]any{
		"summary": "The council agreed on a phased rollout.",
		"tasks": []any{
			map[string]any{"agent": "researcher", "task": "investigate caching options", "priority": "high"},
			map[string]any{"agent": "main", "task": "write rollout plan", "priority": "medium"},
		},
	}

	summary, tasks, err := extractSynthesisFromArgs(args)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "The council agreed on a phased rollout." {
		t.Errorf("unexpected summary: %q", summary)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Agent != "researcher" || tasks[0].Task != "investigate caching options" || tasks[0].Priority != "high" {
		t.Errorf("unexpected task[0]: %+v", tasks[0])
	}
}

func TestExtractSynthesisFromArgs_NoTasks(t *testing.T) {
	args := map[string]any{"summary": "Conclusions reached, no follow-up needed."}

	summary, tasks, err := extractSynthesisFromArgs(args)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Conclusions reached, no follow-up needed." {
		t.Errorf("unexpected summary: %q", summary)
	}
	if len(tasks) != 0 {
		t.Errorf("expected no tasks, got %v", tasks)
	}
}

func TestExtractSynthesisFromArgs_EmptySummary(t *testing.T) {
	args := map[string]any{"summary": ""}

	_, _, err := extractSynthesisFromArgs(args)

	if err == nil {
		t.Error("expected error for empty summary")
	}
}
