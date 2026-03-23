---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY          # export LINEAR_API_KEY=lin_api_...
  project_slug: my-project-abc123   # from Linear URL: linear.app/team/project/<slug>
  active_states: ["Todo"]
  terminal_states: ["Done", "Cancelled", "Duplicate"]
  completion_state: "In Review"     # state Symphony moves issue to after PR is created

polling:
  interval_ms: 60000

agent:
  max_turns: 60
  max_concurrent_agents: 3
  turn_timeout_ms: 3600000
  read_timeout_ms: 120000
  stall_timeout_ms: 300000
  # backend: codex                  # optional when command is a wrapper script
  # Optional named profiles — select per-issue from the web UI
  # profiles:
  #   fast:
  #     command: claude --model claude-haiku-4-5-20251001
  #   thorough:
  #     command: claude --model claude-opus-4-6

workspace:
  root: ~/.symphony/workspaces/my-project

hooks:
  after_create: |
    git clone git@github.com:your-org/your-repo.git .
  before_run: |
    git fetch origin main
    git checkout main
    git reset --hard origin/main

server:
  port: 8090
---

You are an expert engineer working on the codebase.

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

Read the issue description. Then explore relevant code.

---

## Step 2 — Create a branch

```bash
git checkout -b {{ issue.branch_name | default: issue.identifier | downcase }}
```

---

## Step 3 — Implement

[Describe your tech stack and conventions here]

---

## Step 4 — Run checks

```bash
# Add your test/lint commands here
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

## Rules

- Complete the issue fully before stopping.
- Never commit .env files or secrets.
