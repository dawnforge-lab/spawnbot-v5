---
name: bitwarden
description: "Bitwarden CLI vault operations: login, unlock, search, retrieve, create, edit, and generate passwords. Use when the user needs passwords, secrets, API keys, credentials, vault lookups, or password generation. Also use when injecting secrets into environment variables, config files, or .env files."
argument-hint: "[operation or query]"
metadata: {"nanobot":{"emoji":"🔐","requires":{"bins":["bw"]},"install":[{"id":"snap","kind":"shell","command":"sudo snap install bw","bins":["bw"],"label":"Install Bitwarden CLI (snap)"},{"id":"npm","kind":"shell","command":"npm install -g @bitwarden/cli","bins":["bw"],"label":"Install Bitwarden CLI (npm)"}]}}
---

# Bitwarden CLI

Manage credentials and secrets through the Bitwarden CLI (`bw`). This skill covers authentication, vault lookups, item management, and password generation.

## Security Rules

These rules are non-negotiable and apply to every operation below.

- **Never print secrets in conversation.** Do not echo, log, or display passwords, API keys, tokens, TOTP codes, or any sensitive field values in messages.
- **Never write secrets to plain text files.** No dumping to `.txt`, `.log`, `.json`, or any unencrypted file.
- **Use pipes and env vars.** When the user needs a secret injected somewhere, pipe `bw get` output directly into the target command or export it as an environment variable in the same shell invocation.
- **Session keys are sensitive.** Treat `BW_SESSION` the same as a password — export it in-shell, never echo it.

### Safe patterns

Inject into env var (single shell context):
```bash
export DB_PASSWORD=$(bw get password "database-prod")
```

Pipe into a command:
```bash
bw get password "deploy-key" | ssh-add -
```

Write to `.env` via pipe (no intermediate variable in output):
```bash
echo "API_KEY=$(bw get password 'my-api')" >> .env
```

### Unsafe patterns (never do these)

```bash
# DO NOT — prints secret to conversation
bw get password "my-item"

# DO NOT — stores in plain text
bw get item "my-item" > item.json
```

## 1. Authentication

The Bitwarden CLI requires login then unlock before vault access.

### Check status

```bash
bw status
```

Returns JSON with `status` field: `unauthenticated`, `locked`, or `unlocked`.

### Login

Ask the user which method they prefer:

**Email + master password** (interactive — ask user to run it themselves):
```bash
bw login
```

**API key** (non-interactive, good for automation):
```bash
export BW_CLIENTID="<client_id>"
export BW_CLIENTSECRET="<client_secret>"
bw login --apikey
```

The user gets their API key from the Bitwarden web vault under Settings > Security > Keys.

### Unlock

After login, the vault is locked. Unlocking produces a session key:

```bash
export BW_SESSION=$(bw unlock --raw)
```

The `--raw` flag outputs only the session key (no extra text). The session key stays valid until `bw lock` or `bw logout`.

If the session has expired or a command returns "Vault is locked", re-run the unlock flow.

### Sync

Force a fresh pull from the server:
```bash
bw sync
```

Run this if the user says they just added or changed something in the web vault.

## 2. Search and Retrieve

### List items with search

```bash
bw list items --search "github"
```

Returns JSON array. Filter with `--folderid`, `--collectionid`, or `--organizationid` if needed.

### Get a specific item

By name (returns first match):
```bash
bw get item "my-item-name"
```

By ID:
```bash
bw get item "a1b2c3d4-..."
```

### Get specific fields

Password only:
```bash
bw get password "item-name"
```

Username only:
```bash
bw get username "item-name"
```

TOTP code:
```bash
bw get totp "item-name"
```

URI:
```bash
bw get uri "item-name"
```

Notes:
```bash
bw get notes "item-name"
```

Custom fields (use `jq` to extract):
```bash
bw get item "item-name" | jq -r '.fields[] | select(.name=="fieldname") | .value'
```

Remember: never print these outputs in conversation. Always pipe or assign to a variable.

### List folders

```bash
bw list folders
```

### List collections

```bash
bw list collections
```

## 3. Create Items

Use `bw get template` to get the JSON structure, modify with `jq`, encode, and create.

### Create a login item

```bash
bw get template item | jq '.type = 1 | .name = "New Service" | .login = {"username": "user@example.com", "password": "'"$(bw generate -ulns --length 32)"'", "uris": [{"uri": "https://service.example.com"}]}' | bw encode | bw create item
```

### Create a secure note

```bash
bw get template item | jq '.type = 2 | .name = "Deploy Notes" | .notes = "content here" | .secureNote = {"type": 0}' | bw encode | bw create item
```

### Create a folder

```bash
bw get template folder | jq '.name = "MyFolder"' | bw encode | bw create folder
```

### Move item to folder

Get the folder ID first with `bw list folders`, then:
```bash
bw get item "item-name" | jq '.folderId = "folder-id-here"' | bw encode | bw edit item <item-id>
```

## 4. Edit Items

Fetch the item, modify with `jq`, encode, and edit.

### Update a password

```bash
bw get item "item-name" | jq '.login.password = "'"$(bw generate -ulns --length 32)"'"' | bw encode | bw edit item <item-id>
```

### Update username

```bash
bw get item "item-name" | jq '.login.username = "new-user@example.com"' | bw encode | bw edit item <item-id>
```

### Add a custom field

```bash
bw get item "item-name" | jq '.fields += [{"name": "env", "value": "production", "type": 0}]' | bw encode | bw edit item <item-id>
```

### Update notes

```bash
bw get item "item-name" | jq '.notes = "Updated notes content"' | bw encode | bw edit item <item-id>
```

After editing, run `bw sync` to push changes.

## 5. Generate Passwords

### Random password

```bash
bw generate --length 24 -ulns
```

Flags: `-u` uppercase, `-l` lowercase, `-n` numbers, `-s` special characters.

### Passphrase

```bash
bw generate --passphrase --words 4 --separator "-"
```

### Generate and assign directly

```bash
export NEW_PASSWORD=$(bw generate -ulns --length 32)
```

Password generation output is the one exception where displaying a generated (not stored) password is acceptable if the user explicitly asks to see it — but prefer piping it directly to where it's needed.

## 6. Common Workflows

### Inject secrets into .env file

```bash
cat > .env << 'ENVEOF'
DATABASE_URL=$(bw get password "db-prod-url")
API_KEY=$(bw get password "api-key-prod")
SECRET_KEY=$(bw get password "app-secret")
ENVEOF
```

Better approach using subshell evaluation:
```bash
{
  echo "DATABASE_URL=$(bw get password 'db-prod-url')"
  echo "API_KEY=$(bw get password 'api-key-prod')"
  echo "SECRET_KEY=$(bw get password 'app-secret')"
} > .env
```

### Rotate a password

```bash
NEW_PW=$(bw generate -ulns --length 32)
bw get item "service-name" | jq --arg pw "$NEW_PW" '.login.password = $pw' | bw encode | bw edit item <item-id>
# Then update the service with $NEW_PW
```

### Export vault (encrypted)

```bash
bw export --format encrypted_json --output vault-backup.json
```

The user will be prompted for an encryption password. Never use `--format json` or `--format csv` as those produce unencrypted exports.

## 7. Bitwarden Secrets Manager (bws)

For machine-to-machine secret access using service account tokens. Requires `bws` CLI (separate from `bw`).

### Authenticate

```bash
export BWS_ACCESS_TOKEN="<machine-account-token>"
```

### List secrets

```bash
bws secret list
```

Filter by project:
```bash
bws secret list <project-id>
```

### Get a secret

```bash
bws secret get <secret-id>
```

### Create a secret

```bash
bws secret create <KEY> <VALUE> <PROJECT_ID> --note "description"
```

### Edit a secret

```bash
bws secret edit <secret-id> --value "new-value"
```

The same security rules apply to `bws` — never print secret values in conversation.

## Error Reference

| Error | Cause | Fix |
|-------|-------|-----|
| "Vault is locked" | Session expired or missing | Re-run `export BW_SESSION=$(bw unlock --raw)` |
| "You are not logged in" | Not authenticated | Run `bw login` |
| "Not found" | Item name/ID doesn't match | Use `bw list items --search "..."` to find it |
| "More than one result" | Ambiguous name | Use the item ID instead of name |
| "Session key is invalid" | Stale session | `bw logout` then `bw login` + `bw unlock` |
