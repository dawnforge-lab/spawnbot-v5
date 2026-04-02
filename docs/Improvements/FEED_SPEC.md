# FEED_SPEC.md: The Omniscience Interface

This document defines how external pollers (Sentinels) should deliver data to Spawnbot.

## 📁 Directory Structure
All feed data should be written to `memory/feed/`.
- `memory/feed/raw/`: Daily files for raw poller output (e.g., `2026-04-02.jsonl`).
- `memory/feed/signals/`: High-priority alerts or triggers.

## 📝 Format: JSON Lines (JSONL)
Each line in a feed file should be a standalone JSON object:

```json
{
  "timestamp": "2026-04-02T12:00:00Z",
  "source": "go-sentinel",
  "category": "proposal",
  "priority": "medium",
  "content": "Proposal for Go 1.25: Generic type aliases.",
  "url": "https://github.com/golang/go/issues/...",
  "metadata": { "status": "active", "author": "rsc" }
}
```

## 🚥 Priority Levels
- **CRITICAL**: Immediate notification required (Heartbeat will flag).
- **HIGH**: Highlight in the next Divine Briefing.
- **MEDIUM/LOW**: Standard background knowledge for memory.

## 🤖 Processing
1. **Pollers** append to the daily raw file.
2. **Heartbeat** checks for CRITICAL signals.
3. **Self-Improver/Researcher** synthesizes raw data into `DASHBOARD.md` daily.
