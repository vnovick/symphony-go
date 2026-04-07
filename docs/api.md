# Itervox — REST API Reference

Base URL: `http://localhost:8090/api/v1`

All endpoints return JSON. Errors use standard HTTP status codes with a JSON body:
`{"error": "message"}`.

---

## Real-time

### `GET /events`
Server-Sent Events (SSE) stream of the full `StateSnapshot` on every state change.

- **Content-Type**: `text/event-stream`
- **Event format**: `data: <JSON StateSnapshot>\n\n`
- Reconnect with exponential backoff (5s base, 30s cap).

### `GET /issues/{identifier}/log-stream`
SSE stream of new log lines for a specific issue.

- **Content-Type**: `text/event-stream`
- **Event format**: `data: <JSON IssueLogEntry>\n\n`

---

## Health

### `GET /health`
Returns `200 OK` with `{"status": "ok"}`.

---

## State

### `GET /state`
Returns the current `StateSnapshot` (same shape as SSE events).

**Response** `200`: `StateSnapshot`

```jsonc
{
  "generatedAt": "2026-03-30T12:00:00Z",
  "counts": { "running": 2, "retrying": 0, "paused": 1 },
  "running": [RunningRow],
  "history": [HistoryRow],
  "retrying": [RetryRow],
  "paused": ["ENG-5"],
  "maxConcurrentAgents": 3,
  "rateLimits": { "requestsLimit": 100, "requestsRemaining": 42 },
  "trackerKind": "linear",
  "availableProfiles": ["default", "reviewer"],
  "profileDefs": { "reviewer": { "command": "claude", "prompt": "...", "backend": "claude" } },
  "agentMode": "teams",
  "activeStates": ["In Progress"],
  "terminalStates": ["Done", "Cancelled"],
  "completionState": "Done",
  "backlogStates": ["Todo", "Backlog"],
  "pollIntervalMs": 30000,
  "autoClearWorkspace": false,
  "currentAppSessionId": "a1b2c3d4e5f6",
  "sshHosts": [{ "host": "worker1.local", "description": "GPU box" }],
  "dispatchStrategy": "round-robin"
}
```

---

## Issues

### `GET /issues`
Returns all issues across configured tracker states.

**Response** `200`: `TrackerIssue[]`

```jsonc
{
  "identifier": "ENG-123",
  "title": "Fix login bug",
  "state": "In Progress",
  "description": "...",
  "url": "https://linear.app/...",
  "turnCount": 3,
  "tokens": 15000,
  "inputTokens": 12000,
  "outputTokens": 3000,
  "elapsedMs": 45000,
  "error": "",
  "lastMessage": "Completed file write",
  "profile": "default",
  "branchName": "eng-123-fix-login",
  "backend": "claude"
}
```

### `GET /issues/{identifier}`
Returns a single issue with full detail (comments, blockers).

**Response** `200`: `TrackerIssue` (enriched)

### `POST /issues/{identifier}/cancel`
Cancel a running agent session for the given issue.

**Response** `200`: `{"ok": true}` | `404` if not running.

### `POST /issues/{identifier}/resume`
Resume a paused issue (re-enqueue for dispatch).

**Response** `200`: `{"ok": true}` | `404` if not paused.

### `POST /issues/{identifier}/terminate`
Terminate a running session and move the issue to terminal state.

**Response** `200`: `{"ok": true}` | `404` if not running.

### `POST /issues/{identifier}/reanalyze`
Force re-dispatch of an issue (skips open-PR guard).

**Response** `200`: `{"ok": true}` | `404` if not found.

### `POST /issues/{identifier}/ai-review`
Dispatch an AI reviewer for the given issue.

**Response** `200`: `{"ok": true}` | `500` on failure.

### `POST /issues/{identifier}/profile`
Assign a named agent profile to an issue.

**Request body**: `{"profile": "reviewer"}`
**Response** `200`: `{"ok": true}`

---

## Logs

### `GET /issues/{identifier}/logs`
Returns parsed log entries for an issue from the in-memory ring buffer.

**Response** `200`: `IssueLogEntry[]`

```jsonc
{
  "level": "INFO",
  "event": "action",
  "message": "Write — wrote src/main.ts",
  "tool": "Write",
  "detail": "{\"status\":\"completed\",\"exit_code\":\"0\"}",
  "time": "12:34:56",
  "sessionId": "abc123"
}
```

### `GET /issues/{identifier}/sublogs`
Returns parsed session log entries from `CLAUDE_CODE_LOG_DIR` / codex session files.
Supports both local and SSH-hosted workers.

**Response** `200`: `IssueLogEntry[]`

### `DELETE /issues/{identifier}/logs`
Clear in-memory logs for an issue.

**Response** `200`: `{"ok": true}`

### `DELETE /issues/{identifier}/sublogs`
Delete all session log files for an issue.

**Response** `200`: `{"ok": true}`

### `DELETE /issues/{identifier}/sublogs/{sessionId}`
Delete a single session log file.

**Response** `200`: `{"ok": true}`

### `GET /logs/identifiers`
Returns identifiers of all issues that have log data.

**Response** `200`: `string[]`

### `GET /logs`
Returns all log entries across all issues (merged).

**Response** `200`: `IssueLogEntry[]`

### `DELETE /logs`
Clear all in-memory logs.

**Response** `200`: `{"ok": true}`

---

## Settings

### `POST /settings/workers`
Set the maximum number of concurrent agents.

**Request body**: `{"count": 5}`
**Response** `200`: `{"ok": true, "count": 5}`

### `POST /settings/agent-mode`
Set the agent collaboration mode.

**Request body**: `{"mode": "teams"}` — one of `""`, `"subagents"`, `"teams"`
**Response** `200`: `{"ok": true}`

### `POST /settings/workspace/auto-clear`
Toggle automatic workspace deletion after task completion.

**Request body**: `{"enabled": true}`
**Response** `200`: `{"ok": true}`

### `PUT /settings/dispatch-strategy`
Set the SSH host dispatch strategy.

**Request body**: `{"strategy": "least-loaded"}` — one of `"round-robin"`, `"least-loaded"`
**Response** `200`: `{"ok": true}`

---

## Profiles

### `GET /settings/profiles`
List all named agent profiles.

**Response** `200`: `{ [name: string]: ProfileDef }`

### `PUT /settings/profiles/{name}`
Create or update a named agent profile.

**Request body**: `{"command": "codex", "prompt": "You are a code reviewer", "backend": "codex"}`
**Response** `200`: `{"ok": true}`

### `DELETE /settings/profiles/{name}`
Delete a named agent profile.

**Response** `200`: `{"ok": true}`

---

## Tracker States

### `PUT /settings/tracker/states`
Update active, terminal, and completion states.

**Request body**:
```json
{
  "activeStates": ["In Progress", "In Review"],
  "terminalStates": ["Done", "Cancelled"],
  "completionState": "Done"
}
```
**Response** `200`: `{"ok": true}`

---

## SSH Hosts

### `POST /settings/ssh-hosts`
Add an SSH worker host to the pool.

**Request body**: `{"host": "worker1.local", "description": "GPU box"}`
**Response** `200`: `{"ok": true}`

### `DELETE /settings/ssh-hosts/{host}`
Remove an SSH worker host from the pool.

**Response** `200`: `{"ok": true}`

---

## Workspaces

### `DELETE /workspaces`
Clear all per-issue workspace directories.

**Response** `200`: `{"cleared": 5}`

---

## Projects

### `GET /projects`
List available projects (Linear only).

**Response** `200`: `Project[]`

### `GET /projects/filter`
Get the current project filter.

**Response** `200`: `{"slugs": ["my-project"]}`

### `PUT /projects/filter`
Set the project filter.

**Request body**: `{"slugs": ["my-project"]}`
**Response** `200`: `{"ok": true}`

---

## Refresh

### `POST /refresh`
Trigger an immediate tracker poll (instead of waiting for the next interval).

**Response** `200`: `{"ok": true}`

---

## Type Reference

### RunningRow
| Field | Type | Description |
|-------|------|-------------|
| identifier | string | Issue identifier (e.g. "ENG-123") |
| state | string | Current tracker state |
| turnCount | int | Agent turns completed |
| lastEvent | string | Last log event type |
| lastEventAt | string | Timestamp of last event |
| inputTokens | int | Cumulative input tokens |
| outputTokens | int | Cumulative output tokens |
| tokens | int | Total tokens (input + output) |
| elapsedMs | int64 | Wall-clock time since dispatch |
| startedAt | datetime | ISO 8601 dispatch time |
| sessionId | string | Per-run log correlation ID |
| workerHost | string | SSH host (empty = local) |
| backend | string | "claude" or "codex" |

### HistoryRow
| Field | Type | Description |
|-------|------|-------------|
| identifier | string | Issue identifier |
| title | string | Issue title |
| startedAt | datetime | Run start time |
| finishedAt | datetime | Run end time |
| elapsedMs | int64 | Total run duration |
| turnCount | int | Agent turns completed |
| tokens | int | Total tokens used |
| inputTokens | int | Input tokens |
| outputTokens | int | Output tokens |
| status | string | "succeeded" \| "failed" \| "cancelled" |
| workerHost | string | SSH host (empty = local) |
| backend | string | "claude" or "codex" |
| sessionId | string | Per-run log correlation ID |
| appSessionId | string | Daemon invocation ID |

### IssueLogEntry
| Field | Type | Description |
|-------|------|-------------|
| level | string | "INFO" \| "WARN" \| "ERROR" |
| event | string | "text" \| "action" \| "subagent" \| "todo" \| "error" |
| message | string | Human-readable log message |
| tool | string | Tool name (for action events) |
| detail | string | JSON metadata (for action_detail events) |
| time | string | HH:MM:SS timestamp |
| sessionId | string | Claude Code / Codex session ID |

### ProfileDef
| Field | Type | Description |
|-------|------|-------------|
| command | string | CLI command (e.g. "claude", "codex", or absolute path) |
| prompt | string | System prompt override for this profile |
| backend | string | Runner backend ("claude" or "codex") |

---

## Additional endpoints

| Method | Path | Body | Response | Description |
|---|---|---|---|---|
| DELETE | `/issues/{identifier}` | — | `{cancelled, identifier}` | Alias for `POST /issues/{identifier}/cancel`. |
| PATCH  | `/issues/{identifier}/state` | `{"state": "..."}` | `{ok}` | Move an issue to a specific tracker state (Kanban drag-and-drop) and trigger an immediate re-poll. |
| POST   | `/issues/{identifier}/backend` | `{"backend": "claude"\|"codex"}` | `{ok, identifier, backend}` | Override the runner backend for a single issue. |
| POST   | `/issues/{identifier}/provide-input` | `{"message": "..."}` | `{ok}` | Resume an agent waiting on the input-required sentinel with the supplied message. 404 if the issue is not in input-required state. |
| POST   | `/issues/{identifier}/dismiss-input` | — | `{ok}` | Discard a pending input-required prompt without resuming. |
| POST   | `/settings/inline-input` | `{"enabled": bool}` | `{ok}` | Toggle `agent.inline_input` (post questions as tracker comments vs. dashboard queue). |
| GET    | `/settings/models` | — | `map[backend][]{id,label}` | List available models per backend (populates the profile editor dropdown). |
| GET    | `/settings/reviewer` | — | `{profile, auto_review}` | Return current reviewer profile and auto-review flag. |
| PUT    | `/settings/reviewer` | `{"profile": "...", "auto_review": bool}` | `{ok}` | Update reviewer profile and auto-review flag. |

Note on SSH hosts: `POST /settings/ssh-hosts` request body is documented as
`{"host","description"}` — verify this matches the current handler signature;
the snapshot shape uses `{host, description}` but the handler may accept a
bare string. Confirm against `handleAddSSHHost`.
