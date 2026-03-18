---
tracker:
  kind: github
  api_key: $GITHUB_TOKEN            # export GITHUB_TOKEN=ghp_...
  project_slug: owner/repo          # e.g. vnovick/StoriesAI
  # GitHub uses labels to simulate workflow states.
  # Create labels with these exact names in your repo settings.
  active_states: ["todo"]           # label names (lowercase)
  terminal_states: ["done", "cancelled"]
  completion_state: "in-review"     # label added when PR is created

polling:
  interval_ms: 60000

agent:
  max_turns: 60
  max_concurrent_agents: 3
  turn_timeout_ms: 3600000
  read_timeout_ms: 120000
  stall_timeout_ms: 300000

workspace:
  root: ~/.simphony/workspaces/my-project

hooks:
  after_create: |
    git clone git@github.com:owner/repo.git .
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
git checkout -b {{ issue.branch_name | default: issue.identifier | replace: "#", "" | downcase }}
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
