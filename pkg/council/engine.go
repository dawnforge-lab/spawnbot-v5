package council

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/agents"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/providers/protocoltypes"
)

const maxRounds = 10

// Engine orchestrates multi-agent council discussions.
type Engine struct {
	store         *Store
	agentRegistry *agents.Registry
	provider      providers.LLMProvider
	emitter       EventEmitter
	defaultModel  string
}

// NewEngine creates a new council Engine. defaultModel is the fallback model
// used when an agent definition doesn't specify one.
func NewEngine(store *Store, agentRegistry *agents.Registry, provider providers.LLMProvider, emitter EventEmitter, defaultModel string) *Engine {
	return &Engine{
		store:         store,
		agentRegistry: agentRegistry,
		provider:      provider,
		emitter:       emitter,
		defaultModel:  defaultModel,
	}
}

// Run executes a council session according to the given config.
func (e *Engine) Run(ctx context.Context, cfg CouncilConfig) (*CouncilResult, error) {
	meta, transcript, err := e.initSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("init session: %w", err)
	}

	// Default roster to all registered agents if not specified
	if len(meta.Roster) == 0 {
		for _, def := range e.agentRegistry.List() {
			meta.Roster = append(meta.Roster, def.Name)
		}
		if err := e.store.SaveMeta(meta); err != nil {
			return nil, fmt.Errorf("save default roster: %w", err)
		}
	}

	e.emitEvent(Event{
		Kind: EventCouncilStart,
		Payload: CouncilStartPayload{
			CouncilID:   meta.ID,
			Title:       meta.Title,
			Description: meta.Description,
			Roster:      meta.Roster,
		},
	})

	// If there's a topic, append it as the moderator's agenda
	if cfg.Topic != "" {
		entry := TranscriptEntry{
			Role:      RoleModerator,
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
		e.emitEvent(Event{
			Kind: EventCouncilRoundStart,
			Payload: CouncilRoundStartPayload{
				CouncilID: meta.ID,
				Round:     round,
			},
		})

		// Each agent speaks — skip agents that fail after retry
		for _, agentName := range meta.Roster {
			agentDef := e.agentRegistry.Get(agentName)
			if agentDef == nil {
				logger.WarnCF("council", "Agent not found in registry, skipping",
					map[string]any{"agent": agentName, "council_id": meta.ID})
				continue
			}

			response, err := e.callAgentWithRetry(ctx, meta, agentDef, transcript, round)
			if err != nil {
				logger.ErrorCF("council", "Agent failed after retries, skipping",
					map[string]any{"agent": agentName, "error": err.Error(), "council_id": meta.ID})
				entry := TranscriptEntry{
					Role:      RoleAgent,
					AgentID:   agentName,
					AgentType: agentName,
					Content:   fmt.Sprintf("[Agent %s was unable to respond this round]", agentName),
					Round:     round,
					Timestamp: time.Now(),
				}
				if storeErr := e.store.AppendMessage(meta.ID, entry); storeErr != nil {
					return nil, fmt.Errorf("append agent skip message: %w", storeErr)
				}
				transcript = append(transcript, entry)
				continue
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

		// Moderator decision — retry on failure
		decision, err := e.callWithRetry("moderator", func() (string, error) {
			return e.moderatorDecision(ctx, meta, transcript, round)
		})
		if err != nil {
			return nil, fmt.Errorf("moderator decision: %w", err)
		}

		e.emitEvent(Event{
			Kind: EventCouncilRoundEnd,
			Payload: CouncilRoundEndPayload{
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

	// Generate synthesis — retry on failure
	synthesis, tasks, err := e.callWithRetrySynthesis("synthesis", func() (string, []CouncilTask, error) {
		return e.generateSynthesis(ctx, meta, transcript)
	})
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
		Tasks:     tasks,
		Status:    StatusClosed,
	}

	e.emitEvent(Event{
		Kind: EventCouncilEnd,
		Payload: CouncilEndPayload{
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
		Title:        cfg.Title,
		Description:  cfg.Description,
		AgentContext: cfg.AgentContext,
		Roster:       cfg.Roster,
		Status:       StatusActive,
		Rounds:       0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := e.store.Create(meta); err != nil {
		return nil, nil, fmt.Errorf("create council: %w", err)
	}

	return meta, nil, nil
}

// callAgent builds messages and calls the LLM for a single agent.
func (e *Engine) callAgent(ctx context.Context, meta *CouncilMeta, agentDef *agents.AgentDefinition, transcript []TranscriptEntry, round int) (string, error) {
	e.emitEvent(Event{
		Kind: EventCouncilAgentStart,
		Payload: CouncilAgentStartPayload{
			CouncilID: meta.ID,
			AgentID:   agentDef.Name,
			AgentType: agentDef.Name,
			Round:     round,
		},
	})

	messages := e.buildAgentMessages(agentDef, meta, transcript)

	model := agentDef.Model
	if model == "" {
		model = e.defaultModel
	}
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
				e.emitEvent(Event{
					Kind: EventCouncilAgentDelta,
					Payload: CouncilAgentDeltaPayload{
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

	e.emitEvent(Event{
		Kind: EventCouncilAgentEnd,
		Payload: CouncilAgentEndPayload{
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

	// System message: tell the agent to speak ONLY for itself, not others.
	agentContextBlock := ""
	if meta.AgentContext != "" {
		agentContextBlock = fmt.Sprintf("\n\nYou were convened by the main agent, who introduces themselves as:\n%s\nKeep this context in mind when contributing to the discussion.", meta.AgentContext)
	}

	systemContent := fmt.Sprintf(`You are %s, participating in a council discussion titled %q.
Other participants: %s.%s

CRITICAL RULES:
- You speak ONLY for yourself. Do NOT write responses for other agents.
- Do NOT format your response with [Agent Name] headers or simulate a multi-person discussion.
- Just give YOUR perspective in YOUR voice. Other agents will speak in their own turns.
- Be concise and substantive. 2-4 paragraphs max.
- Build on what others have said in the transcript. Agree or disagree with specifics.
- Focus on what's true and useful, not on defending a position.`,
		agentDef.Name, meta.Title, strings.Join(meta.Roster, ", "), agentContextBlock)
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
		Content: `You are a council moderator. Review the discussion and decide: CONTINUE or CONCLUDE.

CONCLUDE when:
- The council has identified clear points of agreement
- Actionable conclusions or decisions have emerged
- Further rounds would just repeat what's been said

CONTINUE when:
- Key disagreements remain unresolved with no path forward
- Important angles haven't been explored yet
- The discussion is still producing new, useful insights

If continuing, identify specific points of agreement so far and direct the next round toward resolving remaining gaps. Push for convergence — ask agents to commit to positions rather than hedging.

Respond with either "CONCLUDE" or a brief directive for the next round.`,
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

	model := e.defaultModel
	if model == "" {
		model = e.provider.GetDefaultModel()
	}
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
func (e *Engine) generateSynthesis(ctx context.Context, meta *CouncilMeta, transcript []TranscriptEntry) (string, []CouncilTask, error) {
	var messages []protocoltypes.Message

	messages = append(messages, protocoltypes.Message{
		Role: "system",
		Content: `You are a council synthesizer. Summarize the full discussion into this exact format:

Summary:
<2-3 sentence summary of key conclusions and agreements>

Tasks:
- agent: <agent name>  task: <one sentence describing what to do>  priority: high|medium|low
- agent: <agent name>  task: <one sentence describing what to do>  priority: high|medium|low

Rules:
- Only assign tasks to agents who participated in the council, or "main" for the calling agent.
- Each task must be one clear, actionable sentence.
- Separate agent/task/priority fields with two spaces on each task line.
- If there are no actionable tasks, omit the Tasks section entirely.`,
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

	model := e.defaultModel
	if model == "" {
		model = e.provider.GetDefaultModel()
	}
	resp, err := e.provider.Chat(ctx, messages, nil, model, nil)
	if err != nil {
		return "", nil, err
	}

	summary, tasks := parseSynthesisOutput(resp.Content)
	return summary, tasks, nil
}

const maxRetries = 2

// callAgentWithRetry calls an agent with retry on transient failures.
func (e *Engine) callAgentWithRetry(ctx context.Context, meta *CouncilMeta, agentDef *agents.AgentDefinition, transcript []TranscriptEntry, round int) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.InfoCF("council", "Retrying agent call",
				map[string]any{"agent": agentDef.Name, "attempt": attempt + 1, "council_id": meta.ID})
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		response, err := e.callAgent(ctx, meta, agentDef, transcript, round)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", lastErr
}

// callWithRetrySynthesis retries a synthesis-returning function on failure.
func (e *Engine) callWithRetrySynthesis(label string, fn func() (string, []CouncilTask, error)) (string, []CouncilTask, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.InfoCF("council", "Retrying "+label,
				map[string]any{"attempt": attempt + 1})
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		summary, tasks, err := fn()
		if err == nil {
			return summary, tasks, nil
		}
		lastErr = err
	}
	return "", nil, lastErr
}

// callWithRetry retries a string-returning function on failure.
func (e *Engine) callWithRetry(label string, fn func() (string, error)) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.InfoCF("council", "Retrying "+label,
				map[string]any{"attempt": attempt + 1})
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// emitEvent safely emits an event, checking if emitter is nil.
func (e *Engine) emitEvent(evt Event) {
	if e.emitter == nil {
		return
	}
	e.emitter.EmitCouncilEvent(evt)
}
