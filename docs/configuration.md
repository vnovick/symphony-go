# Configuration Reference

Itervox is configured via a single `WORKFLOW.md` file in your project root
(or wherever you point `--workflow`). The file contains a YAML front matter
block followed by a Liquid-templated agent prompt.

**Note:** `server.port` is required for the dashboard. If omitted, no HTTP server is started.

```markdown
---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY
agent:
  command: claude
workspace:
  root: ~/.itervox/workspaces
server:
  port: 8090
---

You are working on {{ issue.identifier }} — {{ issue.title }}.
{{ issue.description }}
```

The prompt template is re-rendered on every agent turn. It has access to
`issue.*` (identifier, title, description, state, priority, labels,
blocked_by, branch_name, …) and the `attempt` counter on retries.

Any string value of the form `$VAR_NAME` is substituted with the corresponding
environment variable at load time. Unset variables resolve to an empty string.

The canonical schema lives in `internal/config/config.go`. Runtime-editable
fields are also mutable via the dashboard Settings page and persist back to
`WORKFLOW.md` automatically.

---

## `tracker`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `kind` | string | yes | — | Tracker backend: `linear` or `github` |
| `api_key` | string | yes | — | API key. Use `$ENV_VAR` for env var substitution |
| `project_slug` | string | github: yes | `""` | GitHub: `owner/repo`. Linear: optional project slug filter |
| `endpoint` | string | no | Linear: `https://api.linear.app/graphql`; GitHub: provider default | Override the API endpoint |
| `active_states` | []string | no | `["Todo","In Progress"]` | Issue states considered ready to work |
| `terminal_states` | []string | no | `["Closed","Cancelled","Canceled","Duplicate","Done"]` | States treated as permanently done |
| `backlog_states` | []string | no | Linear: `["Backlog"]`, GitHub: `[]` | Always fetched; shown as leftmost Kanban column(s) |
| `working_state` | string | no | `"In Progress"` | State assigned when an agent starts. Empty string disables the transition |
| `completion_state` | string | no | `""` | State assigned on successful completion. When set, the issue leaves `active_states` so it is not re-dispatched |
| `failed_state` | string | no | `""` | State assigned when max retries are exhausted. When empty, failed issues are paused instead |

---

## `polling`

| Field | Type | Default | Description |
|---|---|---|---|
| `interval_ms` | int | `30000` | How often to poll the tracker for new issues (milliseconds) |

---

## `agent`

| Field | Type | Default | Description |
|---|---|---|---|
| `command` | string | `"claude"` | Agent CLI command (e.g. `claude`, `codex`, `/abs/path/to/wrapper`) |
| `backend` | string | `""` | Explicit backend override when `command` is a wrapper. One of `claude`, `codex`. Inferred from `command` when empty |
| `max_concurrent_agents` | int | `10` | Global cap on parallel agents |
| `max_concurrent_agents_by_state` | map[string]int | `{}` | Per-state concurrency cap (state keys lowercased), e.g. `{"in progress": 3}` |
| `max_turns` | int | `20` | Maximum turns per issue before aborting |
| `turn_timeout_ms` | int | `3600000` | Hard wall-clock limit for the entire agent session (ms). `0` disables |
| `read_timeout_ms` | int | `30000` | Per-read timeout on subprocess stdout. Aborts if no bytes for this long |
| `stall_timeout_ms` | int | `300000` | Orchestrator-level inactivity timeout. `≤ 0` disables stall detection |
| `max_retry_backoff_ms` | int | `300000` | Exponential back-off cap between retries (10 s × 2^(n−1), capped here). Set to `0` to disable retries |
| `max_retries` | int | `5` | Maximum retry attempts before moving to `failed_state`. `0` means unlimited |
| `base_branch` | string | `""` (auto-detect) | Remote base branch for PR diff enrichment (e.g. `origin/main`). Auto-detected via `git symbolic-ref` when empty |
| `agent_mode` | string | `""` | Collaboration model: `""` (solo), `"subagents"`, or `"teams"`. Runtime-editable |
| `inline_input` | bool | `false` | When `true`, agent input-required signals post as tracker comments instead of waiting in the dashboard UI |
| `ssh_hosts` | []string | `[]` | SSH worker hosts (`host` or `host:port`). Empty = run locally. Runtime-editable |
| `dispatch_strategy` | string | `"round-robin"` | Routing for SSH hosts. One of `round-robin`, `least-loaded`. Runtime-editable |
| `reviewer_profile` | string | `""` | Name of the profile used for AI code review. Required if `auto_review: true` |
| `auto_review` | bool | `false` | When `true`, dispatches a reviewer worker after every successful worker completion |
| `reviewer_prompt` | string | Built-in default | **Deprecated** — prefer `reviewer_profile`. Liquid template used when no reviewer profile is set |
| `profiles` | map | `{}` | Named agent profiles — see below. Runtime-editable |
| `available_models` | map | `{}` | Backend → model-option list used by the dashboard model picker |

### Agent profiles

Each entry under `profiles:` is a named role selectable per-issue from the
dashboard or the agent queue view. Profile names with empty `command` are
silently dropped at load time. Commands must not contain shell metacharacters
(`;|&\`$()><`) — use a wrapper script.

| Field | Description |
|---|---|
| `command` | CLI command for this profile (required) |
| `backend` | Explicit backend override (`claude` or `codex`); inferred from `command` when absent |
| `prompt` | Role description appended to the rendered template when `agent_mode: teams` |
| `enabled` | Optional boolean. Disabled profiles stay in config but are hidden from normal selection and dispatch. |
| `allowed_actions` | Optional list of daemon-backed actions: `comment`, `create_issue`, `move_state`, `provide_input`. |
| `create_issue_state` | Required when `allowed_actions` includes `create_issue`; the tracker state/column for follow-up issues. |

```yaml
agent:
  reviewer_profile: code-reviewer
  auto_review: true
  profiles:
    fast:
      command: claude --model claude-haiku-4-5
      prompt: "Fix this quickly with minimal changes."
    thorough:
      command: claude --model claude-opus-4-6
    code-reviewer:
      command: claude --model claude-opus-4-6
      prompt: "You are a senior code reviewer. Focus on correctness and test coverage."
      allowed_actions: [comment, move_state]
    codex-research:
      command: run-codex-wrapper --json
      backend: codex
      prompt: "You are a long-horizon investigation agent."
    input-responder:
      command: claude --model claude-sonnet-4-6
      enabled: true
      allowed_actions: [provide_input]
    qa:
      command: claude --model claude-sonnet-4-6
      allowed_actions: [comment, create_issue, move_state]
      create_issue_state: Todo
```

---

## `automations`

Automations dispatch a selected profile when a trigger fires, then add a small
instruction overlay on top of that profile.

Supported triggers:

- `cron`
- `input_required`
- `tracker_comment_added`
- `issue_entered_state`
- `issue_moved_to_backlog`
- `run_failed`

| Field | Type | Description |
|---|---|---|
| `id` | string | Stable automation identifier |
| `enabled` | bool | Whether the automation is active |
| `profile` | string | Name of the agent profile to dispatch |
| `instructions` | string | Markdown/Liquid instruction overlay appended after the selected profile prompt |
| `trigger.type` | string | Trigger type |
| `trigger.cron` | string | Five-field cron expression for `cron` triggers |
| `trigger.timezone` | string | Optional timezone for `cron` triggers |
| `trigger.state` | string | Required for `issue_entered_state`; the state that must be entered |
| `filter.match_mode` | string | How populated filters combine: `all` or `any` |
| `filter.states` | []string | Issue-state filter. For cron automations, leave empty to use backlog and active states |
| `filter.labels_any` | []string | Match issues with at least one listed label |
| `filter.identifier_regex` | string | Regex matched against issue identifiers like `ENG-42` |
| `filter.limit` | int | Maximum issues to queue from one cron tick or event poll batch |
| `filter.input_context_regex` | string | Only for `input_required`; matched against the blocked-agent question text |
| `policy.auto_resume` | bool | Only for `input_required`; allows the helper to resume the blocked run via `provide_input` |

```yaml
automations:
  - id: qa-ready
    enabled: true
    trigger:
      type: issue_entered_state
      state: "Ready for QA"
    profile: qa
    instructions: |
      Run the QA routine for this issue.
      Comment the results.
      If any required check fails, move the issue to Todo.

  - id: pm-backlog-review
    enabled: true
    trigger:
      type: cron
      cron: "0 9 * * 1-5"
      timezone: "Asia/Jerusalem"
    profile: pm
    instructions: |
      Review backlog issues for missing clarity and acceptance criteria.
      Leave one concise comment summarising what is unclear.
    filter:
      states: ["Backlog"]
      limit: 20
```

For a more detailed guide, including trigger semantics, filter behavior, prompt
variables, and worked examples, see `site/src/content/docs/guides/automations.mdx`.

---

## `workspace`

| Field | Type | Default | Description |
|---|---|---|---|
| `root` | string | `~/.itervox/workspaces` | Root directory for per-issue workspaces. Supports `~` and `$ENV_VAR` |
| `auto_clear` | bool | `false` | Delete the workspace directory after a task reaches the completion state. Logs are preserved separately. Runtime-editable |
| `worktree` | bool | `false` | Enable git-worktree mode: per-issue worktrees inside `root` instead of plain directories. Requires a git repo at `root` |
| `clone_url` | string | `""` | Remote URL used to initialise the bare clone when `worktree: true` and `root` is empty |
| `base_branch` | string | `"main"` | Branch worktrees are created from |

---

## `hooks`

Lifecycle scripts run via `bash -lc` inside each workspace. `after_create` and
`before_run` are fatal on non-zero exit; `after_run` and `before_remove`
failures are logged and ignored.

| Field | Type | Default | Description |
|---|---|---|---|
| `timeout_ms` | int | `60000` | Per-hook execution timeout (ms) |
| `after_create` | string | `""` | Shell script run once, right after the workspace directory is created |
| `before_run` | string | `""` | Shell script run before every agent turn |
| `after_run` | string | `""` | Shell script run after every agent turn |
| `before_remove` | string | `""` | Shell script run before the workspace is removed (auto-clear) |

```yaml
hooks:
  timeout_ms: 60000
  after_create: |
    git clone git@github.com:org/repo.git .
  before_run: |
    git fetch origin && git reset --hard origin/main
```

---

## `server`

| Field | Type | Default | Description |
|---|---|---|---|
| `host` | string | `"127.0.0.1"` | HTTP bind address. Change to `0.0.0.0` to expose to LAN |
| `port` | int | unset → no HTTP server | HTTP listen port. The scaffolded `WORKFLOW.md` defaults to `8090`; if the port is in use, Itervox tries up to 10 successors |

---

## Input-required sentinel

Agents request human input by emitting a literal sentinel token in their
output: `<!-- itervox:needs-input -->`. The orchestrator detects this and
either pauses for a dashboard reply (`agent.inline_input: false`, default) or
posts the question as a tracker comment (`inline_input: true`). The prompt
template that teaches agents how to emit the sentinel is appended
automatically — see `internal/templates/human_input.md`. The canonical
constant is `agent.InputRequiredSentinel` in `internal/agent/events.go`; the
contract is documented in `CONTRIBUTING.md` and `docs/architecture.md`.

---

## Environment variable substitution

Any field value of the form `$VAR_NAME` is replaced with `os.Getenv("VAR_NAME")`
at load time. Unset variables resolve to an empty string. Itervox also
auto-loads `.itervox/.env` and `.env` from the current working directory at
startup (existing env vars are never overwritten).

```yaml
tracker:
  api_key: $LINEAR_API_KEY
workspace:
  root: $ITERVOX_WORKSPACES
```
