package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

type TaskStore struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	filePath string
	warning  string
}

type taskFile struct {
	Tasks []*Task `json:"tasks"`
}

func NewTaskStore(filePath string) *TaskStore {
	s := &TaskStore{
		tasks:    make(map[string]*Task),
		filePath: filePath,
	}
	s.Load()
	return s
}

func (s *TaskStore) Warning() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.warning
}

func (s *TaskStore) Create(title, description, priority string) (*Task, error) {
	task := &Task{
		ID:          GenerateID(),
		Title:       title,
		Description: description,
		Priority:    priority,
	}
	task.ApplyDefaults()
	if err := task.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()
	if err := s.Save(); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskStore) Get(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil
	}
	cp := *t
	return &cp
}

func (s *TaskStore) List(status string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Task
	for _, t := range s.tasks {
		if status == "" || t.Status == status {
			cp := *t
			result = append(result, &cp)
		}
	}
	return result
}

func (s *TaskStore) Update(id string, fields map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	if IsTerminal(task.Status) {
		return fmt.Errorf("cannot update %s task %q — create a new task instead", task.Status, id)
	}
	for key, val := range fields {
		switch key {
		case "status":
			if !validStatuses[val] {
				return fmt.Errorf("invalid status %q. Valid: pending, in_progress, completed, failed", val)
			}
			task.Status = val
		case "description":
			task.Description = val
		case "result":
			task.Result = val
		case "agent_type":
			task.AgentType = val
		case "priority":
			if !validPriorities[val] {
				return fmt.Errorf("invalid priority %q. Valid: low, medium, high", val)
			}
			task.Priority = val
		}
	}
	task.UpdatedAt = time.Now().UnixMilli()
	return s.saveLocked()
}

func (s *TaskStore) Complete(id, result string) error {
	return s.Update(id, map[string]string{"status": StatusCompleted, "result": result})
}

func (s *TaskStore) Fail(id, result string) error {
	return s.Update(id, map[string]string{"status": StatusFailed, "result": result})
}

func (s *TaskStore) Load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.warning = fmt.Sprintf("tasks.json failed to load: %s", err)
			logger.WarnCF("tasks", "Failed to load tasks.json", map[string]any{"error": err.Error()})
		}
		return
	}
	var tf taskFile
	if err := json.Unmarshal(data, &tf); err != nil {
		s.warning = fmt.Sprintf("tasks.json failed to load: %s", err)
		logger.WarnCF("tasks", "Failed to parse tasks.json", map[string]any{"error": err.Error()})
		return
	}
	cleaned := false
	for _, t := range tf.Tasks {
		if IsExpired(t) {
			cleaned = true
			continue
		}
		s.tasks[t.ID] = t
	}
	if cleaned {
		s.Save()
	}
}

func (s *TaskStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

func (s *TaskStore) saveLocked() error {
	var tf taskFile
	for _, t := range s.tasks {
		tf.Tasks = append(tf.Tasks, t)
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks: %w", err)
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		return fmt.Errorf("failed to rename tasks file: %w", err)
	}
	return nil
}
