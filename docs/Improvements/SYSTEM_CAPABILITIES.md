# SYSTEM_CAPABILITIES.md: Internal Logic & Autonomy

This document serves as the "Instruction Manual" for Spawnbot's internal mechanisms.

## 💓 Heartbeat (Autonomy)
- **File**: `HEARTBEAT.md`
- **Execution**: Triggered periodically by the system.
- **Logic**: Fast, read-only checks. It flags deadlines, summarizes memory, and checks for subagent results.
- **Modification**: Add new tasks to the bottom of the file to increase my proactive awareness.

## 🔄 Self-Improvement (Evolution)
- **File**: `struggles.jsonl`
- **Execution**: Daily via the `self-improver` agent.
- **Logic**: Analyzes tool failures and blocks to suggest (or implement) skill updates and workflow changes.
- **Modification**: Manually add struggles to `struggles.jsonl` to force me to address specific frustrations.

## 🤖 Subagent Orchestration (Delegation)
- **Tool**: `spawn` (async), `subagent` (sync).
- **Execution**: On-demand for complex tasks.
- **Logic**: Can run `coder`, `researcher`, `planner`, `reviewer`, or `self-improver`.
- **Modification**: Define new agent types via `create_agent` to expand my specialized workforce.

## 🛠️ Modifications (Self-Refactoring)
- **File**: `PLAYBOOK.md`
- **Logic**: Defines my operational rules and strategies.
- **Modification**: Update the "Rules" section in `PLAYBOOK.md` to permanently change my behavior or constraints.
