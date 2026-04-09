package council

import (
	"os"
	"testing"
	"time"
)

func TestStore_CreateAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	meta := &CouncilMeta{
		Title:       "Test Council",
		Description: "A test council session",
		Roster:      []string{"agent-1", "agent-2"},
		Status:      StatusActive,
		Rounds:      0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	id, err := s.Create(meta)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	if meta.ID != id {
		t.Fatalf("expected meta.ID=%q, got %q", id, meta.ID)
	}

	loaded, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID != id {
		t.Fatalf("loaded ID=%q, want %q", loaded.ID, id)
	}
	if loaded.Title != "Test Council" {
		t.Fatalf("loaded Title=%q, want %q", loaded.Title, "Test Council")
	}
	if loaded.Description != "A test council session" {
		t.Fatalf("loaded Description=%q, want %q", loaded.Description, "A test council session")
	}
	if len(loaded.Roster) != 2 {
		t.Fatalf("loaded Roster len=%d, want 2", len(loaded.Roster))
	}
	if loaded.Status != StatusActive {
		t.Fatalf("loaded Status=%q, want %q", loaded.Status, StatusActive)
	}
}

func TestStore_AppendAndGetTranscript(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	meta := &CouncilMeta{
		Title:     "Transcript Test",
		Roster:    []string{"agent-1"},
		Status:    StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	id, err := s.Create(meta)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	entries := []TranscriptEntry{
		{Role: RoleUser, AgentID: "user-1", Content: "Hello", Round: 1, Timestamp: time.Now()},
		{Role: RoleAgent, AgentID: "agent-1", AgentType: "claude", Content: "Hi there", Round: 1, Timestamp: time.Now()},
		{Role: RoleModerator, AgentID: "mod", Content: "Summarizing", Round: 1, Timestamp: time.Now()},
	}
	for _, e := range entries {
		if err := s.AppendMessage(id, e); err != nil {
			t.Fatalf("AppendMessage failed: %v", err)
		}
	}

	transcript, err := s.GetTranscript(id)
	if err != nil {
		t.Fatalf("GetTranscript failed: %v", err)
	}
	if len(transcript) != 3 {
		t.Fatalf("transcript len=%d, want 3", len(transcript))
	}
	if transcript[0].Content != "Hello" {
		t.Fatalf("transcript[0].Content=%q, want %q", transcript[0].Content, "Hello")
	}
	if transcript[1].Role != RoleAgent {
		t.Fatalf("transcript[1].Role=%q, want %q", transcript[1].Role, RoleAgent)
	}
	if transcript[2].AgentID != "mod" {
		t.Fatalf("transcript[2].AgentID=%q, want %q", transcript[2].AgentID, "mod")
	}
}

func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	now := time.Now()
	for i, title := range []string{"First", "Second", "Third"} {
		meta := &CouncilMeta{
			Title:     title,
			Roster:    []string{"a"},
			Status:    StatusActive,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if _, err := s.Create(meta); err != nil {
			t.Fatalf("Create %q failed: %v", title, err)
		}
		// Small sleep to ensure distinct millisecond IDs
		time.Sleep(2 * time.Millisecond)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list len=%d, want 3", len(list))
	}
	// Should be sorted by UpdatedAt desc — "Third" first
	if list[0].Title != "Third" {
		t.Fatalf("list[0].Title=%q, want %q", list[0].Title, "Third")
	}
	if list[2].Title != "First" {
		t.Fatalf("list[2].Title=%q, want %q", list[2].Title, "First")
	}
}

func TestStore_UpdateMeta(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	meta := &CouncilMeta{
		Title:     "Update Test",
		Roster:    []string{"a"},
		Status:    StatusActive,
		Rounds:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	id, err := s.Create(meta)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update status and rounds
	meta.Status = StatusClosed
	meta.Rounds = 5
	meta.UpdatedAt = time.Now()
	if err := s.SaveMeta(meta); err != nil {
		t.Fatalf("SaveMeta failed: %v", err)
	}

	loaded, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Status != StatusClosed {
		t.Fatalf("loaded Status=%q, want %q", loaded.Status, StatusClosed)
	}
	if loaded.Rounds != 5 {
		t.Fatalf("loaded Rounds=%d, want 5", loaded.Rounds)
	}
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	meta := &CouncilMeta{
		Title:     "Delete Test",
		Roster:    []string{"a"},
		Status:    StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	id, err := s.Create(meta)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := s.Delete(id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify the directory is gone
	councilDir := dir + "/" + id
	if _, err := os.Stat(councilDir); !os.IsNotExist(err) {
		t.Fatalf("expected council dir to be gone, got err: %v", err)
	}

	// Load should fail
	_, err = s.Load(id)
	if err == nil {
		t.Fatal("expected Load to fail after Delete")
	}
}
