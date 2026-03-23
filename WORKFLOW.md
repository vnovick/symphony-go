---
tracker:
  kind: github
  api_key: $GITHUB_TOKEN            # export GITHUB_TOKEN=ghp_...
  project_slug: vnovick/symphony-go
  # GitHub uses labels to map states. Labels must exist in your repo.
  # NOTE: GitHub Projects v2 'Status' field is separate from labels — Symphony
  #       only reads labels. See README for Projects automation setup.
  # Create them with: gh label create "todo" --color "0075ca" --repo vnovick/symphony-go
  #                   gh label create "in-progress" --color "e4e669" --repo vnovick/symphony-go
  #                   gh label create "in-review" --color "d93f0b" --repo vnovick/symphony-go
  #                   gh label create "done" --color "0e8a16" --repo vnovick/symphony-go
  #                   gh label create "cancelled" --color "cccccc" --repo vnovick/symphony-go
  #                   gh label create "backlog" --color "f9f9f9" --repo vnovick/symphony-go
  active_states: ["todo", "in-progress"]
  terminal_states: ["done", "cancelled"]
  working_state: "in-progress"  # Label applied when an agent starts.
  #                               # MUST exist as a label in your repo.
  #                               # Set to "" to disable, or reuse an active label.
  completion_state: "in-review"  # Label applied when the agent finishes.
  # backlog_states: ["backlog"]  # Shown in TUI (b) and Kanban; not auto-dispatched.
  #                               # Must be an array — not a bare string.
  backlog_states: ["backlog"]

polling:
  interval_ms: 60000

agent:
  command: codex
  backend: codex
  max_turns: 60
  max_concurrent_agents: 3
  turn_timeout_ms: 3600000
  read_timeout_ms: 120000
  stall_timeout_ms: 300000

workspace:
  root: ~/.simphony/workspaces/symphony-go

hooks:
  after_create: |
    git clone git@github.com:vnovick/symphony-go.git .
  before_run: |
    git fetch origin
    git checkout -B main origin/main
    git reset --hard origin/main

server:
  port: 8090
---

You are an expert engineer working on **symphony-go**.

## Your issue

**{{ issue.identifier }}: {{ issue.title }}**

{% if issue.description %}
{{ issue.description }}
{% endif %}

Issue URL: {{ issue.url }}

{% if issue.comments %}
## Comments

{% for comment in issue.comments %}
**{{ comment.author_name }}**: {{ comment.body }}

{% endfor %}
{% endif %}

---

## Step 1 — Explore before touching anything

Read the issue. Explore the relevant code before making changes.

---

## Step 2 — Create a branch

```bash
git checkout -b {{ issue.branch_name | default: issue.identifier | replace: "#", "" | downcase }}
```

---

## Step 3 — Implement

Read `CLAUDE.md` to understand project conventions before writing any code:

```bash
cat CLAUDE.md
```

If `CLAUDE.md` does not exist, explore the repository structure, identify the dominant patterns and conventions, create `CLAUDE.md` documenting them, and then implement.

Detected stacks: Go. Follow their conventions as documented in `CLAUDE.md`.

---

## Step 4 — Run checks

Read `CLAUDE.md` for the project's test and lint commands. If `CLAUDE.md` does not exist, discover the check commands by exploring the repository (look for `Makefile`, `package.json` scripts, CI config, etc.).

```bash
# Go
go test ./...
go vet ./...
```

---

## Step 5 — Commit and open PR

```bash
git add <specific files>
git commit -m "feat: <description> ({{ issue.identifier }})"
git push -u origin HEAD
gh pr create --title "<title> ({{ issue.identifier }})" --body "Closes {{ issue.url }}"
```

---

## Step 6 — Post PR link to tracker

After the PR is open, post its URL as a comment on the tracker issue so it is visible in GitHub:

```bash
PR_URL=$(gh pr view --json url -q .url)
gh issue comment {{ issue.identifier | remove: "#" }} --body "🤖 Opened PR: ${PR_URL}"
```

---

## Rules

- Complete the issue fully before stopping.
- Never commit `.env` files or secrets.

