# Configuration Reference

Symphony is configured via a `WORKFLOW.md` file in your project root (or wherever you point `--workflow`). The file contains a YAML front matter block followed by a free-form agent prompt.

## File format

```markdown
---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY
  # ... other tracker fields
agent:
  command: claude
  # ... other agent fields
---

Your agent prompt goes here. This text is rendered as a Liquid template
and passed to the agent on every turn.
```

---

## `tracker`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `kind` | string | yes | — | Tracker backend: `linear` or `github` |
| `api_key` | string | yes | — | API key. Use `$ENV_VAR` for env var substitution |
| `project_slug` | string | github: yes | — | GitHub: `owner/repo`. Linear: optional team key filter |
| `endpoint` | string | no | provider default | Override the API endpoint (rarely needed) |
| `active_states` | []string | no | `["Todo","In Progress"]` | Issue states considered "ready to work" |
| `terminal_states` | []string | no | `["Closed","Cancelled","Done",...]` | Issue states considered permanently done |
| `backlog_states` | []string | no | `[]` | Issue states to move back to on pause |
| `working_state` | string | no | `""` | State to assign when an agent starts (e.g. `"In Progress"`) |
| `completion_state` | string | no | `""` | State to assign on successful agent completion |
| `polling_interval_ms` | int | no | `30000` | How often to poll for new issues (milliseconds) |

---

## `agent`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `command` | string | no | `claude` | The agent command to run (e.g. `claude`, `/path/to/agent`) |
| `max_concurrent_agents` | int | no | `10` | Maximum number of issues processed in parallel |
| `max_turns` | int | no | `20` | Maximum agent turns per issue before aborting |
| `turn_timeout_ms` | int | no | `3600000` | Per-turn timeout in milliseconds (1 hour) |
| `read_timeout_ms` | int | no | `30000` | Inactivity timeout — aborts turn if no output for this long |
| `stall_timeout_ms` | int | no | `300000` | Hard stall timeout in milliseconds |
| `max_retry_backoff_ms` | int | no | `300000` | Maximum back-off between retries (5 minutes) |
| `agent_mode` | string | no | `""` | Multi-agent mode: `""` (single), `"subagents"`, or `"teams"` |
| `reviewer_prompt` | string | no | `""` | Liquid template for the reviewer agent prompt |
| `ssh_hosts` | []string | no | `[]` | SSH worker hosts for remote agent execution |
| `profiles` | map | no | `{}` | Named agent profiles (see below) |

### Agent profiles

Profiles allow per-issue command overrides. Reference a profile by name from the dashboard.

```yaml
agent:
  profiles:
    fast:
      command: claude --model claude-haiku-4-5
      prompt: "Fix this quickly with minimal changes."
    thorough:
      command: claude --model claude-opus-4-6
```

---

## `hooks`

```yaml
hooks:
  timeout_ms: 60000   # default 60s per hook
  before_run:
    - ./scripts/setup-workspace.sh
  after_run:
    - ./scripts/notify.sh
```

| Field | Type | Default | Description |
|---|---|---|---|
| `timeout_ms` | int | `60000` | Per-hook execution timeout |
| `before_run` | []string | `[]` | Commands run before each agent turn |
| `after_run` | []string | `[]` | Commands run after each agent turn |

---

## `server`

```yaml
server:
  port: 3000   # default: random available port
```

---

## `workspace`

```yaml
workspace:
  root: ~/.simphony/workspaces   # default
```

Each issue gets an isolated subdirectory under `root` that persists across runs.

---

## Environment variable substitution

Any field value starting with `$` is replaced with the corresponding environment variable at load time:

```yaml
api_key: $LINEAR_API_KEY
```

If the variable is unset, the field is set to an empty string.

---

