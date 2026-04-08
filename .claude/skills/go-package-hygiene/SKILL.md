---
name: go-package-hygiene
description: Use when adding a new `.go` file to any `internal/` package, when an existing file grows past ~400 lines, when introducing a helper function, or when considering a new package. Enforces itervox's package dependency order, file-size discipline, reuse of stdlib builtins, and the no-premature-utils-package rule.
---

# Go Package Hygiene (itervox)

Apply these rules whenever you touch Go code under `internal/` or `cmd/`.

## 1. Respect the package dependency order

```
domain → tracker, prompt, logbuffer, prdetector
workflow → config → workspace
agent           (imports domain, config)
orchestrator    (imports agent, config, domain, logbuffer, prdetector, prompt, tracker, workspace)
app             (imports domain, tracker)
server          (imports domain, config)
cmd/itervox     (wires everything)
```

Never introduce a cycle. Before adding an import, trace it: does it reverse the arrow? If `domain` starts importing `tracker`, stop. Move the type, not the import.

## 2. No `utils/` or `helpers/` package — ever

Shared code lives in the most specific package that owns the concept. Function used only inside `orchestrator/`? It belongs there. Type shared by `orchestrator/` and `app/`? It belongs in `domain/`. A `utils` package is where architecture goes to die.

## 3. File size discipline (~400 lines)

When a file grows past ~400 lines, split by **responsibility**, not alphabet. The canonical example is `internal/orchestrator/`: `event_loop.go`, `worker.go`, `snapshot.go`, `dispatch.go`, `reconcile.go`, `retry.go`, `reviewer.go`, `issue_control.go`, `ssh_host.go`, `logging.go`, `state.go`. Each file name names a concern.

## 4. Reuse stdlib builtins — do not reinvent

- `maps.Copy(dst, src)` — not a `for k, v := range` loop
- `max(a, b)` / `min(a, b)` — not if/else clamping
- `slices.Contains`, `slices.Index`, `slices.Sort` — not handwritten loops
- `cmp.Or(a, b, c)` — not nested fallback ifs
- `strings.Cut` — not `strings.Index` + manual slicing

## 5. Grep before writing a new helper

Before adding `func somethingX(...)`, search the current package for an existing helper. Then the parent package. Extend or reuse — do not create a second helper with a similar name. Common targets: `internal/agent/helpers*.go`, the bottom of `internal/config/config.go`, `internal/orchestrator/*.go`.

## 6. Error conventions

- `fmt.Errorf("package: lowercase message: %w", err)` — always wrap with `%w`
- `errors.New("package: static message")` for static strings
- `errors.Is` / `errors.As` for checking wrapped errors — never string matching

## 7. Logging via `log/slog`

Structured key/value, not `log.Printf`. Canonical:

```go
slog.Info("orchestrator: dispatched", "identifier", issue.Identifier, "profile", profileName)
```

Message is a short sentence; details are key/value pairs.

## 8. Tests in the same package (whitebox)

Unexported helpers are tested via `helpers_test.go` in the same package. Use `package foo_test` only when deliberately testing the public surface from outside.

## 9. Exported symbols need doc comments

`golangci-lint` enforces this. Describe **behavior**, not type:

- Good: `// RunTurn executes one claude turn as a subprocess and streams progress to onProgress.`
- Useless: `// RunTurn is a method.`

## 10. New package creation requires justification

Adding `internal/<name>/` requires that the concept (a) fits in no existing package and (b) has multiple consumers. Single-use code stays in its consumer.

## Verification

```bash
go vet ./...
golangci-lint run ./...
go test -race ./...
make verify
```

## Common mistakes

- Duplicating a helper across two files in the same package instead of grepping first
- Creating `internal/util/` or `internal/helpers/` as a "temporary" dumping ground
- Letting `domain` import `tracker` and creating a cycle
- Handwritten `for k, v := range src { dst[k] = v }` instead of `maps.Copy`
- `log.Printf("dispatched %s", id)` instead of `slog.Info("orchestrator: dispatched", "identifier", id)`
- Splitting a 600-line file into `a.go` / `b.go` instead of by responsibility
- Adding a new `internal/<name>/` package for a single-call-site helper
