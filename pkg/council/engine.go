package council

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agent"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers/protocoltypes"
)

const maxRounds = 10

// Engine orchestrates multi-agent council discussions.
type Engine struct {
	store         *Store
	agentRegistry *agents.Registry
	provider      providers.LLMProvider
	eventBus      *agent.EventBus
}

// NewEngine creates a new council Engine.
func NewEngine(store *Store, agentRegistry *agents.Registry, provider providers.LLMProvider, eventBus *agent.EventBus) *Engine {
	return &Engine{
		store:         store,
		agentRegistry: agentRegistry,
		provider:      provider,
		eventBus:      eventBus,
	}
}

// Run executes a council session according to the given config.
func (e *Engine) Run(ctx context.Context, cfg CouncilConfig) (*CouncilResult, error) {
	meta, transcript, err := e.initSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("init session: %w", err)
	}

	e.emitEvent(agent.Event{
		Kind: agent.EventKindCouncilStart,
		Payload: agent.CouncilStartPayload{
			CouncilID:   meta.ID,
			Title:       meta.Title,
			Description: meta.Description,
			Roster:      meta.Roster,
		},
	})

	// If there's a topic, append it as a user message
	if cfg.Topic != "" {
		entry := TranscriptEntry{
			Role:      RoleUser,
			Content:   cfg.Topic,
			Round:     meta.Rounds + 1,
			Timestamp: time.Now(),
		}
		if err := e.store.AppendMessage(meta.ID, entry); err != nil {
			return nil, fmt.Errorf("append topic: %w", err)
		}
		transcript = append(transcript, entry)
	}

	// Round loop
	for round := meta.Rounds + 1; round <= meta.Rounds+maxRounds; round++ {
		e.emitEvent(agent.Event{
			Kind: agent.EventKindCouncilRoundStart,
			Payload: agent.CouncilRoundStartPayload{
				CouncilID: meta.ID,
				Round:     round,
			},
		})

		// Each agent speaks
		for _, agentName := range meta.Roster {
			agentDef := e.agentRegistry.Get(agentName)
			if agentDef == nil {
				return nil, fmt.Errorf("agent %q not found in registry", agentName)
			}

			response, err := e.callAgent(ctx, meta, agentDef, transcript, round)
			if err != nil {
				return nil, fmt.Errorf("call agent %s: %w", agentName, err)
			}

			entry := TranscriptEntry{
				Role:      RoleAgent,
				AgentID:   agentName,
				AgentType: agentName,
				Content:   response,
				Round:     round,
				Timestamp: time.Now(),
			}
			if err := e.store.AppendMessage(meta.ID, entry); err != nil {
				return nil, fmt.Errorf("append agent message: %w", err)
			}
			transcript = append(transcript, entry)
		}

		// Moderator decision
		decision, err := e.moderatorDecision(ctx, meta, transcript, round)
		if err != nil {
			return nil, fmt.Errorf("moderator decision: %w", err)
		}

		e.emitEvent(agent.Event{
			Kind: agent.EventKindCouncilRoundEnd,
			Payload: agent.CouncilRoundEndPayload{
				CouncilID: meta.ID,
				Round:     round,
				Decision:  decision,
			},
		})

		meta.Rounds = round

		// Always append moderator note to transcript
		modEntry := TranscriptEntry{
			Role:      RoleModerator,
			Content:   decision,
			Round:     round,
			Timestamp: time.Now(),
		}
		if err := e.store.AppendMessage(meta.ID, modEntry); err != nil {
			return nil, fmt.Errorf("append moderator note: %w", err)
		}
		transcript = append(transcript, modEntry)

		if decision == "conclude" {
			break
		}
	}

	// Generate synthesis
	synthesis, err := e.generateSynthesis(ctx, meta, transcript)
	if err != nil {
		return nil, fmt.Errorf("generate synthesis: %w", err)
	}

	// Persist synthesis
	synthEntry := TranscriptEntry{
		Role:      RoleSynthesis,
		Content:   synthesis,
		Round:     meta.Rounds,
		Timestamp: time.Now(),
	}
	if err := e.store.AppendMessage(meta.ID, synthEntry); err != nil {
		return nil, fmt.Errorf("append synthesis: %w", err)
	}

	// Close council
	meta.Status = StatusClosed
	meta.UpdatedAt = time.Now()
	if err := e.store.SaveMeta(meta); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}

	result := &CouncilResult{
		ID:        meta.ID,
		Title:     meta.Title,
		Rounds:    meta.Rounds,
		Synthesis: synthesis,
		Status:    StatusClosed,
	}

	e.emitEvent(agent.Event{
		Kind: agent.EventKindCouncilEnd,
		Payload: agent.CouncilEndPayload{
			CouncilID: meta.ID,
			Rounds:    meta.Rounds,
			Synthesis: synthesis,
			Status:    StatusClosed,
		},
	})

	return result, nil
}

// initSession creates or loads a council session.
func (e *Engine) initSession(cfg CouncilConfig) (*CouncilMeta, []TranscriptEntry, error) {
	if cfg.ID != "" {
		// Resume existing council
		meta, err := e.store.Load(cfg.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("load council %s: %w", cfg.ID, err)
		}
		transcript, err := e.store.GetTranscript(cfg.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("load transcript %s: %w", cfg.ID, err)
		}
		// Reopen if closed
		meta.Status = StatusActive
		meta.UpdatedAt = time.Now()
		if err := e.store.SaveMeta(meta); err != nil {
			return nil, nil, fmt.Errorf("save meta: %w", err)
		}
		return meta, transcript, nil
	}

	// New council
	now := time.Now()
	meta := &CouncilMeta{
		Title:       cfg.Title,
		Description: cfg.Description,
		Roster:      cfg.Roster,
		Status:      StatusActive,
		Rounds:      0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := e.store.Create(meta); err != nil {
		return nil, nil, fmt.Errorf("create council: %w", err)
	}

	return meta, nil, nil
}

// callAgent builds messages and calls the LLM for a single agent.
func (e *Engine) callAgent(ctx context.Context, meta *CouncilMeta, agentDef *agents.AgentDefinition, transcript []TranscriptEntry, round int) (string, error) {
	e.emitEvent(agent.Event{
		Kind: agent.EventKindCouncilAgentStart,
		Payload: agent.CouncilAgentStartPayload{
			CouncilID: meta.ID,
			AgentID:   agentDef.Name,
			AgentType: agentDef.Name,
			Round:     round,
		},
	})

	messages := e.buildAgentMessages(agentDef, meta, transcript)

	model := agentDef.Model
	if model == "" {
		model = e.provider.GetDefaultModel()
	}

	var response string

	// Try streaming if provider supports it
	if sp, ok := e.provider.(providers.StreamingProvider); ok {
		var lastAccumulated string
		resp, err := sp.ChatStream(ctx, messages, nil, model, nil, func(accumulated string) {
			delta := accumulated[len(lastAccumulated):]
			lastAccumulated = accumulated
			if delta != "" {
				e.emitEvent(agent.Event{
					Kind: agent.EventKindCouncilAgentDelta,
					Payload: agent.CouncilAgentDeltaPayload{
						CouncilID: meta.ID,
						AgentID:   agentDef.Name,
						Delta:     delta,
					},
				})
			}
		})
		if err != nil {
			return "", err
		}
		response = resp.Content
	} else {
		resp, err := e.provider.Chat(ctx, messages, nil, model, nil)
		if err != nil {
			return "", err
		}
		response = resp.Content
	}

	e.emitEvent(agent.Event{
		Kind: agent.EventKindCouncilAgentEnd,
		Payload: agent.CouncilAgentEndPayload{
			CouncilID: meta.ID,
			AgentID:   agentDef.Name,
			Content:   response,
			Round:     round,
		},
	})

	return response, nil
}

// buildAgentMessages constructs the message list for an agent LLM call.
func (e *Engine) buildAgentMessages(agentDef *agents.AgentDefinition, meta *CouncilMeta, transcript []TranscriptEntry) []protocoltypes.Message {
	var messages []protocoltypes.Message

	// System message with agent's prompt and council context
	systemContent := fmt.Sprintf("%s\n\nYou are participating in a council discussion titled %q with agents: %s. Provide your perspective based on your expertise.",
		agentDef.SystemPrompt, meta.Title, strings.Join(meta.Roster, ", "))
	messages = append(messages, protocoltypes.Message{
		Role:    "system",
		Content: systemContent,
	})

	// Map transcript to user/assistant roles
	for _, entry := range transcript {
		switch {
		case entry.Role == RoleAgent && entry.AgentID == agentDef.Name:
			// Agent's own messages become assistant
			messages = append(messages, protocoltypes.Message{
				Role:    "assistant",
				Content: entry.Content,
			})
		case entry.Role == RoleAgent:
			// Other agents' messages become user with prefix
			messages = append(messages, protocoltypes.Message{
				Role:    "user",
				Content: fmt.Sprintf("[%s]: %s", entry.AgentID, entry.Content),
			})
		case entry.Role == RoleUser:
			messages = append(messages, protocoltypes.Message{
				Role:    "user",
				Content: entry.Content,
			})
		case entry.Role == RoleModerator:
			messages = append(messages, protocoltypes.Message{
				Role:    "user",
				Content: fmt.Sprintf("[Moderator]: %s", entry.Content),
			})
		case entry.Role == RoleSynthesis:
			messages = append(messages, protocoltypes.Message{
				Role:    "user",
				Content: fmt.Sprintf("[Synthesis]: %s", entry.Content),
			})
		}
	}

	return messages
}

// moderatorDecision asks the LLM whether to continue or conclude the council.
func (e *Engine) moderatorDecision(ctx context.Context, meta *CouncilMeta, transcript []TranscriptEntry, round int) (string, error) {
	var messages []protocoltypes.Message

	messages = append(messages, protocoltypes.Message{
		Role:    "system",
		Content: "You are a council moderator. Review the discussion transcript and decide whether the council should continue discussing or conclude. If the agents have provided sufficient input and reached useful conclusions, respond with 'CONCLUDE' followed by your reasoning. Otherwise, provide a brief note on what should be discussed further.",
	})

	// Include transcript summary
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Council: %s\nRound: %d\n\nTranscript:\n", meta.Title, round))
	for _, entry := range transcript {
		switch entry.Role {
		case RoleUser:
			sb.WriteString(fmt.Sprintf("User: %s\n", entry.Content))
		case RoleAgent:
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", entry.AgentID, entry.Content))
		case RoleModerator:
			sb.WriteString(fmt.Sprintf("[Moderator]: %s\n", entry.Content))
		}
	}

	messages = append(messages, protocoltypes.Message{
		Role:    "user",
		Content: sb.String(),
	})

	model := e.provider.GetDefaultModel()
	resp, err := e.provider.Chat(ctx, messages, nil, model, nil)
	if err != nil {
		return "", err
	}

	content := strings.TrimSpace(resp.Content)
	if strings.HasPrefix(strings.ToUpper(content), "CONCLUDE") {
		return "conclude", nil
	}

	return content, nil
}

// generateSynthesis summarizes the full transcript into actionable output.
func (e *Engine) generateSynthesis(ctx context.Context, meta *CouncilMeta, transcript []TranscriptEntry) (string, error) {
	var messages []protocoltypes.Message

	messages = append(messages, protocoltypes.Message{
		Role:    "system",
		Content: "You are a council synthesizer. Summarize the full discussion transcript into a clear, actionable synthesis that captures the key insights, recommendations, and action items from all agents.",
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Council: %s\n\nFull Transcript:\n", meta.Title))
	for _, entry := range transcript {
		switch entry.Role {
		case RoleUser:
			sb.WriteString(fmt.Sprintf("User: %s\n", entry.Content))
		case RoleAgent:
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", entry.AgentID, entry.Content))
		case RoleModerator:
			sb.WriteString(fmt.Sprintf("[Moderator]: %s\n", entry.Content))
		}
	}

	messages = append(messages, protocoltypes.Message{
		Role:    "user",
		Content: sb.String(),
	})

	model := e.provider.GetDefaultModel()
	resp, err := e.provider.Chat(ctx, messages, nil, model, nil)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// emitEvent safely emits an event, checking if eventBus is nil.
func (e *Engine) emitEvent(evt agent.Event) {
	if e.eventBus == nil {
		return
	}
	e.eventBus.Emit(evt)
}
