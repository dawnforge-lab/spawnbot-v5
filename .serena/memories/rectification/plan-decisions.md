# Rectification Plan — Decisions (2026-04-02)

Based on agent self-reported struggles in docs/Improvements/RECTIFICATION_PLAN.md, reviewed and refined with user.

## Work Items (in order)

### 1. Tool Loop Active Intervention
**Problem:** Agent repeats same tool 3+ times per turn; detection exists (pkg/struggles/collector.go) but is monitoring-only.
**Decision:** Add active intervention in agent loop — inject a message mid-turn when repeated tool threshold is hit, forcing the agent to explain why it's repeating. Code change in loop.go, not a skill.
**Priority:** High

### 2. Researcher Agent — Allow Writes
**Problem:** Researcher agent denies write_file/edit_file/append_file, making it useless for documenting findings.
**Decision:** Remove write_file from researcher's deny list. Agent runs in separate environment so restrictions are unnecessary.
**Priority:** High (trivial effort)

### 3. Capabilities Transparency — Bring Back agents.md
**Problem:** Agent doesn't know its own autonomous capabilities (heartbeat, self-improvement, cron triggers).
**Decision:** Bring back agents.md file, inject it into the system prompt. Move everything under "Rules" in SOUL.md into agents.md, plus add capabilities summary. This separates identity (SOUL.md) from operational instructions (agents.md).
**Priority:** High

### 4. Permissions Review — agents.md + Freedom
**Problem:** Overly restrictive rules scattered across SOUL.md.
**Decision:** Part of the agents.md work (#3). Review all rules, allow more freedom since agent sits in a separate environment. Instructions belong in agents.md, not SOUL.md.
**Priority:** High (bundled with #3)

### 5. Path Safety — Better Prompt Guidance
**Problem:** Agent uses exec when read_file/list_dir would work, triggering deny patterns.
**Decision:** Add guidance in system prompt (or agents.md) telling the agent to prefer read_file/list_dir over exec for filesystem inspection. Not a new skill.
**Priority:** Medium

### 6. Context Compaction & Review
**Problem:** Blunt compression drops entire turns. Summarization before dropping would preserve more context.
**Decision:** Deferred — will tackle last. Current system works, improvement is high-effort.
**Priority:** Low (last)

## Not Pursuing
- "system-triage" skill (workaround, not a fix)
- "indexing" skill for memory (overengineered)
- "summarize-and-pivot" skill (deferred to compaction work)
- "capabilities" skill (replaced by agents.md injection)
