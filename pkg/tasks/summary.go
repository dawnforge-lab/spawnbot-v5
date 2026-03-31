package tasks

import (
	"fmt"
	"sort"
	"strings"
)

var statusOrder = map[string]int{
	StatusInProgress: 0, StatusPending: 1, StatusFailed: 2, StatusCompleted: 3,
}

var priorityOrder = map[string]int{
	PriorityHigh: 0, PriorityMedium: 1, PriorityLow: 2,
}

func (s *TaskStore) Summary(maxFull int) string {
	s.mu.RLock()
	warning := s.warning
	tasks := s.sortedTasks()
	s.mu.RUnlock()

	var lines []string
	if warning != "" {
		lines = append(lines, fmt.Sprintf("WARNING: %s. The file is at %s — inspect and fix it.", warning, s.filePath))
	}
	if len(tasks) == 0 {
		if len(lines) == 0 {
			return ""
		}
		return strings.Join(lines, "\n")
	}
	if len(tasks) <= maxFull {
		lines = append(lines, "Active tasks:")
		for _, t := range tasks {
			lines = append(lines, formatTaskLine(t))
		}
	} else {
		counts := make(map[string]int)
		for _, t := range tasks {
			counts[t.Status]++
		}
		lines = append(lines, fmt.Sprintf("You have %d tasks (%s).", len(tasks), formatCounts(counts)))
		lines = append(lines, "")
		lines = append(lines, "Top 5 by priority:")
		limit := 5
		if len(tasks) < limit {
			limit = len(tasks)
		}
		for _, t := range tasks[:limit] {
			lines = append(lines, formatTaskLine(t))
		}
		lines = append(lines, "")
		lines = append(lines, "Use tasks list for full details.")
	}
	return strings.Join(lines, "\n")
}

func (s *TaskStore) PendingSummary() string {
	s.mu.RLock()
	tasks := s.sortedTasks()
	s.mu.RUnlock()

	var lines []string
	for _, t := range tasks {
		if IsTerminal(t.Status) {
			continue
		}
		lines = append(lines, formatTaskLine(t))
	}
	return strings.Join(lines, "\n")
}

func (s *TaskStore) sortedTasks() []*Task {
	all := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool {
		si, sj := statusOrder[all[i].Status], statusOrder[all[j].Status]
		if si != sj {
			return si < sj
		}
		return priorityOrder[all[i].Priority] < priorityOrder[all[j].Priority]
	})
	return all
}

func formatTaskLine(t *Task) string {
	return fmt.Sprintf("- [%s] %s %s (%s)", t.ID, strings.ToUpper(t.Priority), t.Title, t.Status)
}

func formatCounts(counts map[string]int) string {
	var parts []string
	for _, status := range []string{StatusInProgress, StatusPending, StatusFailed, StatusCompleted} {
		if c, ok := counts[status]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, status))
		}
	}
	return strings.Join(parts, ", ")
}
