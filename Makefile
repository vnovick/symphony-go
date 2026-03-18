.PHONY: all build verify dev test lint lint-go fmt vet web-build web-test coverage clean benchmark

all: build verify

build: web-build
	go build ./...

verify: fmt vet lint-go test web-test

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

# Run benchmarks with memory allocation stats.
benchmark:
	go test -bench=. -benchmem ./...

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build

web-test:
	cd web && pnpm install --frozen-lockfile && pnpm test

dev:
	cd web && pnpm dev
