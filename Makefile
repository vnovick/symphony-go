.PHONY: all build verify dev test lint lint-go fmt vet web-build web-test web-spelling coverage clean benchmark tui-golden

# Pin to the toolchain declared in go.mod so `go tool cover` and other tools
# always use go1.25.8, even on machines where /usr/local/go is an older version.
export GOTOOLCHAIN := go1.25.8

all: build verify

# IMPORTANT: Always use `make build` instead of bare `go build ./cmd/symphony`.
# The Go binary embeds web/dist via //go:embed. If web/dist is missing, the binary
# compiles but panics at runtime with "embed: failed to sub web/dist".
# `make build` runs web-build first to ensure the frontend assets exist.
build: web-build
	go build ./...

verify: fmt vet lint-go test web-test web-spelling

fmt:
	gofmt -l -w .

vet:
	go vet ./...

lint-go:
	golangci-lint run ./...

lint: lint-go

test:
	go test -race ./... -count=1

# Run tests with coverage and generate an HTML report (coverage.html).
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@go tool cover -func=coverage.out | tail -1

# Remove build artifacts and generated coverage files.
clean:
	rm -f symphony coverage.out coverage.html
	go clean ./...

# Regenerate catwalk golden files after intentional TUI render changes.
tui-golden:
	go test ./internal/statusui/... -args -rewrite

# Run benchmarks with memory allocation stats.
benchmark:
	go test -bench=. -benchmem ./...

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build

web-test:
	cd web && pnpm install --frozen-lockfile && pnpm test

# Guard against the common misspelling "Symphony" slipping into web source.
web-spelling:
	@if grep -r "Symphony" web/src/ --include="*.ts" --include="*.tsx" -l 2>/dev/null | grep -q .; then \
		echo "ERROR: 'Symphony' (typo) found in web/src/ — run: grep -r Symphony web/src/"; \
		exit 1; \
	fi

dev:
	cd web && pnpm dev
