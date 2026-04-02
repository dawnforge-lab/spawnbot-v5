# Gateway Logging Config Fix (2026-04-02)

## Issue
Gateway produced no agent-level logs after restart — neither to `~/.spawnbot/logs/gateway.log` nor to stdout/journalctl. Only the startup banner (written via `fmt.Println`) appeared.

## Root Cause
`~/.spawnbot/config.json` had `gateway.log_level: "fatal"`. This called `logger.SetLevelFromString("fatal")` at gateway startup (gateway.go:114), which set `currentLevel = FATAL` and `zerolog.SetGlobalLevel(FatalLevel)`. The `logMessage()` function's guard `if level < currentLevel { return }` then silently discarded all INFO/WARN/ERROR messages before reaching either the console writer or file writer.

## Fix
Changed `config.json` gateway.log_level from `"fatal"` to `"info"`.

## Notes
- The startup banner uses `fmt.Println`, bypassing zerolog entirely — that's why it always showed
- The old `gateway.log` had entries from 01:15-01:19 because the config was changed to `"fatal"` after that session
- The web service (`spawnbot-web`) proxies chat to the gateway via HTTP — agent loop logs live in the gateway process
