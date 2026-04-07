# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| latest  | ✅        |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Report security issues privately via [GitHub Security Advisories](https://github.com/vnovick/itervox/security/advisories/new).

Include:
- A description of the vulnerability and its impact
- Steps to reproduce or a proof-of-concept
- Affected versions
- Any suggested mitigations

You can expect an initial response within **72 hours** and a fix or mitigation plan within **14 days** for confirmed issues.

## Scope

Security concerns relevant to this project include:

- **API token exposure** — Itervox reads `LINEAR_API_KEY` / `GITHUB_TOKEN` from environment variables. These should never be committed to version control or logged at non-debug levels.
- **Workspace path traversal** — The SSH agent runner validates workspace paths with `filepath.EvalSymlinks` to prevent escape outside the configured root.
- **HTTP API access** — The dashboard HTTP API binds to `127.0.0.1` by default. Do not expose it to the public internet without authentication.
- **Prompt injection** — Issue titles and descriptions are included in agent prompts. Malicious issue content could attempt to influence agent behaviour.

## Protocol Specification

The agent communication protocol is specified in the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md).
