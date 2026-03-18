package server

// Build the React SPA into web/dist (embedded via embed.go).
// Run: go generate ./internal/server/
//
//go:generate pnpm --dir ../../web install --frozen-lockfile
//go:generate pnpm --dir ../../web run build
