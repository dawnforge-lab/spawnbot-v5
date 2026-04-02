---
name: poller
description: Create background pollers that watch external sources (RSS, email, social media, APIs, web pages) and notify on new events without spending LLM tokens.
argument-hint: "[source description]"
---

# Poller

Create background polling jobs that watch external sources and push notifications to a channel when new events appear. Polling runs via cron + shell scripts — no LLM tokens spent on checking.

## Pattern

Every poller follows three steps:

1. **Write a polling script** to `~/.spawnbot/poller-scripts/{name}.sh` (or `.py`)
2. **Create a cron job** with `deliver: true` pointing at the script
3. **Verify** by running the script once manually and confirming the cron job is active

## Script Structure

Every polling script must follow this structure:

```
1. Create ~/.spawnbot/poller-state/ if it doesn't exist
2. Read state from ~/.spawnbot/poller-state/{name}.json (create empty state if file missing)
3. Fetch the source
4. Extract item IDs and content
5. Compare against seen_ids from state — keep only new items
6. If new items: print them to stdout (this becomes the notification)
7. If nothing new: print nothing (cron sends no message)
8. Write updated state atomically: write to {name}.json.tmp, then mv to {name}.json
```

Make scripts executable: `chmod +x ~/.spawnbot/poller-scripts/{name}.sh`

## State File Format

Each poller has a state file at `~/.spawnbot/poller-state/{name}.json`:

```json
{
  "last_check": "2026-04-02T10:30:00Z",
  "seen_ids": ["item-guid-1", "item-guid-2"]
}
```

- `last_check`: ISO 8601 timestamp of the last successful poll
- `seen_ids`: array of unique identifiers for items already seen (GUIDs, message UIDs, hashes, etc.)

Always write state atomically to prevent corruption:
```bash
echo "$new_state" > "$STATE_FILE.tmp" && mv "$STATE_FILE.tmp" "$STATE_FILE"
```

## Cron Job Convention

Use the cron tool with these settings:

- **name**: `poller-{source-name}` (e.g., `poller-hackernews`, `poller-gmail`)
- **command**: absolute path to script (e.g., `~/.spawnbot/poller-scripts/rss-hackernews.sh`)
- **every_seconds**: interval appropriate for the source (see defaults below)
- **deliver**: `true` — stdout goes directly to channel, no LLM invoked

### Default Intervals

| Source type | Interval | Reason |
|---|---|---|
| RSS/Atom feeds | 300s (5 min) | Feeds update infrequently |
| Email (IMAP) | 120s (2 min) | Users expect near-real-time |
| Social media / APIs | 600s (10 min) | Rate limit friendly |
| Web page changes | 3600s (1 hr) | Pages change slowly |

Adjust based on the source's update frequency and rate limits.

## Template: RSS/Atom Feed

```bash
#!/usr/bin/env bash
set -euo pipefail

FEED_URL="https://example.com/feed.xml"
NAME="rss-example"
STATE_DIR="$HOME/.spawnbot/poller-state"
STATE_FILE="$STATE_DIR/$NAME.json"

mkdir -p "$STATE_DIR"

# Read state
if [ -f "$STATE_FILE" ]; then
  SEEN_IDS=$(cat "$STATE_FILE" | grep -o '"seen_ids":\[[^]]*\]' | grep -o '"[^"]*"' | tr -d '"' || true)
else
  SEEN_IDS=""
fi

# Fetch feed, extract items (guid + title)
FEED=$(curl -sf "$FEED_URL" || exit 0)
ITEMS=$(echo "$FEED" | grep -oP '<item>.*?</item>' | head -20 || true)

if [ -z "$ITEMS" ]; then
  exit 0
fi

NEW_OUTPUT=""
NEW_IDS=""
ALL_IDS="$SEEN_IDS"

while IFS= read -r item; do
  guid=$(echo "$item" | grep -oP '<guid[^>]*>\K[^<]+' || echo "$item" | grep -oP '<link>\K[^<]+' || continue)
  title=$(echo "$item" | grep -oP '<title>\K[^<]+' || echo "(no title)")

  if echo "$SEEN_IDS" | grep -qF "$guid"; then
    continue
  fi

  NEW_OUTPUT="${NEW_OUTPUT}${title}\n  ${guid}\n"
  ALL_IDS="${ALL_IDS}${guid}\n"
done <<< "$(echo "$ITEMS" | tr '>' '>\n'  | grep -oP '<item>.*?</item>' || echo "$ITEMS")"

# Output new items (becomes notification)
if [ -n "$NEW_OUTPUT" ]; then
  echo -e "New from $NAME:\n$NEW_OUTPUT"
fi

# Update state atomically
SEEN_JSON=$(echo -e "$ALL_IDS" | sort -u | grep -v '^$' | sed 's/.*/"&"/' | paste -sd, | sed 's/^/[/;s/$/]/')
cat > "$STATE_FILE.tmp" <<STATEOF
{
  "last_check": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "seen_ids": $SEEN_JSON
}
STATEOF
mv "$STATE_FILE.tmp" "$STATE_FILE"
```

## Template: IMAP Email

```python
#!/usr/bin/env python3
import imaplib, email, json, os, sys
from datetime import datetime, timezone

NAME = "gmail-inbox"
HOST = "imap.gmail.com"
USER = "user@gmail.com"
# Store credentials securely — use app password, not main password
PASS = os.environ.get("POLLER_IMAP_PASS", "")

STATE_DIR = os.path.expanduser("~/.spawnbot/poller-state")
STATE_FILE = os.path.join(STATE_DIR, f"{NAME}.json")

os.makedirs(STATE_DIR, exist_ok=True)

# Read state
state = {"last_check": "", "seen_ids": []}
if os.path.exists(STATE_FILE):
    with open(STATE_FILE) as f:
        state = json.load(f)

seen = set(state["seen_ids"])

try:
    conn = imaplib.IMAP4_SSL(HOST)
    conn.login(USER, PASS)
    conn.select("INBOX", readonly=True)
    _, data = conn.search(None, "UNSEEN")
    uids = data[0].split()
except Exception:
    sys.exit(0)

new_items = []
for uid in uids:
    uid_str = uid.decode()
    if uid_str in seen:
        continue
    _, msg_data = conn.fetch(uid, "(RFC822.HEADER)")
    msg = email.message_from_bytes(msg_data[0][1])
    sender = msg.get("From", "unknown")
    subject = msg.get("Subject", "(no subject)")
    new_items.append(f"From: {sender}\n  Subject: {subject}")
    seen.add(uid_str)

conn.logout()

if new_items:
    print(f"New emails ({len(new_items)}):")
    print("\n".join(new_items))

# Update state atomically
tmp = STATE_FILE + ".tmp"
with open(tmp, "w") as f:
    json.dump({
        "last_check": datetime.now(timezone.utc).isoformat(),
        "seen_ids": list(seen)
    }, f)
os.rename(tmp, STATE_FILE)
```

## Template: Web Page Change Detection

```bash
#!/usr/bin/env bash
set -euo pipefail

URL="https://example.com/status"
NAME="page-example-status"
STATE_DIR="$HOME/.spawnbot/poller-state"
STATE_FILE="$STATE_DIR/$NAME.json"

mkdir -p "$STATE_DIR"

# Fetch page
CONTENT=$(curl -sf "$URL" || exit 0)
HASH=$(echo "$CONTENT" | sha256sum | cut -d' ' -f1)

# Read previous hash
PREV_HASH=""
if [ -f "$STATE_FILE" ]; then
  PREV_HASH=$(grep -o '"hash":"[^"]*"' "$STATE_FILE" | cut -d'"' -f4 || true)
fi

# Compare
if [ "$HASH" != "$PREV_HASH" ] && [ -n "$PREV_HASH" ]; then
  echo "Page changed: $URL"
fi

# Update state atomically
cat > "$STATE_FILE.tmp" <<EOF
{
  "last_check": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "hash": "$HASH",
  "seen_ids": []
}
EOF
mv "$STATE_FILE.tmp" "$STATE_FILE"
```

## Adapting to Other Sources

These templates are starting points. For any source:

1. Identify what constitutes a unique item (the ID)
2. Identify how to fetch new items (API, scraping, CLI tool)
3. Follow the script structure: read state, fetch, compare, output new, write state
4. Respect rate limits — choose an appropriate interval
5. Handle auth credentials via environment variables, never hardcode in scripts

## Managing Pollers

- **List active pollers**: use `cron list` and look for `poller-*` entries
- **Stop a poller**: use `cron disable poller-{name}` or `cron remove poller-{name}`
- **Check state**: read `~/.spawnbot/poller-state/{name}.json`
- **Reset a poller**: delete its state file to re-trigger all items on next run
- **Debug**: run the script manually and check stdout
