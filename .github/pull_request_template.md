## What does this PR do?

<!-- One paragraph explaining the change and the motivation. Focus on WHY, not what. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactoring (no behaviour change)
- [ ] Documentation
- [ ] CI / tooling

## Testing

<!-- How did you test this? What edge cases did you consider? -->

## Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes (or `make test` if touching Go)
- [ ] `cd web && pnpm test` passes (or `make web-test` if touching frontend)
- [ ] `golangci-lint run ./cmd/... ./internal/...` passes (if touching Go)
- [ ] `cd web && pnpm lint` passes (if touching frontend)
- [ ] New behaviour is covered by tests
- [ ] No API tokens, secrets, or credentials in the diff
- [ ] Exported Go symbols have doc comments

## Related issues

<!-- Closes #NNN -->
