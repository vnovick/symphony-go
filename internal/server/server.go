package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// errNotConfigured is returned by no-op callback stubs installed in New()
// for optional Config fields that were left nil by the caller.
var errNotConfigured = errors.New("not configured")

// RunningRow is a single row in the active sessions table.
type RunningRow struct {
	Identifier   string    `json:"identifier"`
	State        string    `json:"state"`
	TurnCount    int       `json:"turnCount"`
	LastEvent    string    `json:"lastEvent,omitempty"`
	LastEventAt  string    `json:"lastEventAt,omitempty"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	Tokens       int       `json:"tokens"`
	ElapsedMs    int64     `json:"elapsedMs"`
	StartedAt    time.Time `json:"startedAt"`
	SessionID    string    `json:"sessionId,omitempty"`
	WorkerHost   string    `json:"workerHost,omitempty"`
	Backend      string    `json:"backend,omitempty"`
}

// HistoryRow is one completed agent session in the run-history list.
type HistoryRow struct {
	Identifier   string    `json:"identifier"`
	Title        string    `json:"title,omitempty"`
	StartedAt    time.Time `json:"startedAt"`
	FinishedAt   time.Time `json:"finishedAt"`
	ElapsedMs    int64     `json:"elapsedMs"`
	TurnCount    int       `json:"turnCount"`
	TotalTokens  int       `json:"tokens"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	Status       string    `json:"status"` // "succeeded" | "failed" | "cancelled"
	WorkerHost   string    `json:"workerHost,omitempty"`
	Backend      string    `json:"backend,omitempty"`
	SessionID    string    `json:"sessionId,omitempty"`
}

// RateLimitInfo holds the last observed API rate limit snapshot.
type RateLimitInfo struct {
	RequestsLimit       int        `json:"requestsLimit"`
	RequestsRemaining   int        `json:"requestsRemaining"`
	RequestsReset       *time.Time `json:"requestsReset,omitempty"`
	ComplexityLimit     int        `json:"complexityLimit,omitempty"`
	ComplexityRemaining int        `json:"complexityRemaining,omitempty"`
}

// RetryRow is a single row in the retry queue table.
type RetryRow struct {
	Identifier string    `json:"identifier"`
	Attempt    int       `json:"attempt"`
	DueAt      time.Time `json:"dueAt"`
	Error      string    `json:"error,omitempty"`
}

// Counts holds summary counts for the state snapshot.
type Counts struct {
	Running  int `json:"running"`
	Retrying int `json:"retrying"`
	Paused   int `json:"paused"`
}

// Project is one item in the interactive project picker.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ProjectManager is implemented by tracker adapters that support project
// filtering (currently only the Linear adapter). The server registers project
// endpoints only when a non-nil ProjectManager is provided.
type ProjectManager interface {
	FetchProjects(ctx context.Context) ([]Project, error)
	SetProjectFilter(slugs []string)
	GetProjectFilter() []string
}

// StateSnapshot is the payload returned by GET /api/v1/state.
type StateSnapshot struct {
	GeneratedAt         time.Time      `json:"generatedAt"`
	Counts              Counts         `json:"counts"`
	Running             []RunningRow   `json:"running"`
	History             []HistoryRow   `json:"history,omitempty"`
	Retrying            []RetryRow     `json:"retrying"`
	Paused              []string       `json:"paused"`
	MaxConcurrentAgents int            `json:"maxConcurrentAgents"`
	RateLimits          *RateLimitInfo `json:"rateLimits"`
	// TrackerKind is "linear" or "github" — lets the web UI decide whether to
	// show the project picker.
	TrackerKind string `json:"trackerKind,omitempty"`
	// ActiveProjectFilter is the current runtime project filter slugs.
	// nil/absent means "using WORKFLOW.md default"; empty array means "all issues".
	ActiveProjectFilter []string `json:"activeProjectFilter,omitempty"`
	// AvailableProfiles is the list of named agent profile names defined in WORKFLOW.md.
	// Empty/absent means no profiles are configured.
	AvailableProfiles []string `json:"availableProfiles,omitempty"`
	// ProfileDefs is the map of named agent profile definitions from WORKFLOW.md.
	ProfileDefs map[string]ProfileDef `json:"profileDefs,omitempty"`
	// AgentMode is the active agent collaboration mode.
	// "" (off/solo): agent runs alone.
	// "subagents":   agent may spawn helpers via its native delegation tool.
	// "teams":       delegation tool + profile role context injected into the prompt.
	AgentMode string `json:"agentMode,omitempty"`
	// ActiveStates is the list of tracker states the orchestrator will pick up.
	ActiveStates []string `json:"activeStates,omitempty"`
	// TerminalStates is the list of tracker states treated as done/closed.
	TerminalStates []string `json:"terminalStates,omitempty"`
	// CompletionState is the state the agent moves an issue to when it finishes (may be empty).
	CompletionState string `json:"completionState,omitempty"`
	// BacklogStates are always-fetched states shown as the leftmost board column.
	BacklogStates []string `json:"backlogStates,omitempty"`
	// PausedWithPR is the subset of paused identifiers that were auto-paused due to an open PR.
	// Value is the PR URL.
	PausedWithPR map[string]string `json:"pausedWithPR,omitempty"`
	// PollIntervalMs is the configured tracker poll interval in milliseconds.
	// The TUI uses this to derive a safe background refresh rate.
	PollIntervalMs int `json:"pollIntervalMs,omitempty"`
	// AutoClearWorkspace indicates whether workspace directories are
	// automatically deleted after a task succeeds.
	AutoClearWorkspace bool `json:"autoClearWorkspace,omitempty"`
}

// ProfileDef is the JSON representation of one named agent profile.
type ProfileDef struct {
	Command string `json:"command"`
	Prompt  string `json:"prompt,omitempty"`
	Backend string `json:"backend,omitempty"`
}

// CommentRow is one comment entry in a TrackerIssue response.
type CommentRow struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt,omitempty"` // RFC3339; "" when nil
}

// TrackerIssue is a single issue row returned by /api/v1/issues.
type TrackerIssue struct {
	Identifier        string `json:"identifier"`
	Title             string `json:"title"`
	State             string `json:"state"`
	Description       string `json:"description,omitempty"`
	URL               string `json:"url,omitempty"`
	OrchestratorState string `json:"orchestratorState"` // running, retrying, paused, idle
	TurnCount         int    `json:"turnCount,omitempty"`
	Tokens            int    `json:"tokens,omitempty"`
	ElapsedMs         int64  `json:"elapsedMs,omitempty"`
	LastMessage       string `json:"lastMessage,omitempty"`
	Error             string `json:"error,omitempty"`
	// Enriched fields
	Labels           []string     `json:"labels,omitempty"`
	Priority         *int         `json:"priority,omitempty"`
	BranchName       *string      `json:"branchName,omitempty"`
	BlockedBy        []string     `json:"blockedBy,omitempty"`
	Comments         []CommentRow `json:"comments,omitempty"`
	IneligibleReason string       `json:"ineligibleReason,omitempty"`
	// AgentProfile is the name of the per-issue agent profile override, if any.
	AgentProfile string `json:"agentProfile,omitempty"`
}

// IssueLogEntry is one parsed log event for /api/v1/issues/{id}/logs.
type IssueLogEntry struct {
	Level   string `json:"level"`
	Event   string `json:"event"` // "text", "action", "subagent", "info", "warn", "pr", "turn"
	Message string `json:"message"`
	Tool    string `json:"tool,omitempty"`
	Time    string `json:"time,omitempty"` // HH:MM:SS wall-clock time of the event
	// Detail carries backend-specific structured metadata as a JSON string.
	// Populated for Codex shell completions (exit_code, status, output_size).
	Detail string `json:"detail,omitempty"`
}

// broadcaster fans out state-change notifications to multiple SSE clients.
type broadcaster struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{clients: make(map[chan struct{}]struct{})}
}

func (b *broadcaster) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broadcaster) unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func (b *broadcaster) notify() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Config holds all constructor parameters for a Server.
// Required fields: Snapshot, RefreshChan.
// Optional callback fields left nil are replaced with no-op stubs in New() so that
// handler bodies never need to nil-check before calling a function field.
// Exceptions: ProjectManager (interface; nil = GitHub tracker) and FetchIssue
// (fast-path optimisation; nil falls back to FetchIssues).
type Config struct {
	// Required
	Snapshot    func() StateSnapshot
	RefreshChan chan struct{}
	// LogFile is the path to the rotating log file for /api/v1/logs; empty disables it.
	LogFile string

	// Optional – nil fields are replaced with no-op stubs by New()
	FetchIssues           func(ctx context.Context) ([]TrackerIssue, error)
	FetchIssue            func(ctx context.Context, identifier string) (*TrackerIssue, error)
	CancelIssue           func(identifier string) bool
	ResumeIssue           func(identifier string) bool
	TerminateIssue        func(identifier string) bool
	ReanalyzeIssue        func(identifier string) bool
	FetchLogs             func(identifier string) []string
	ClearLogs             func(identifier string) error
	DispatchReviewer      func(identifier string) error
	UpdateIssueState      func(ctx context.Context, identifier, stateName string) error
	SetWorkers            func(n int)
	SetIssueProfile       func(identifier, profile string)
	ProfileDefs           func() map[string]ProfileDef
	UpsertProfile         func(name string, def ProfileDef) error
	DeleteProfile         func(name string) error
	SetAgentMode          func(mode string) error
	SetAutoClearWorkspace func(enabled bool) error
	UpdateTrackerStates   func(active, terminal []string, completion string) error
	ProjectManager        ProjectManager
}

// Server is an HTTP server exposing orchestrator state.
type Server struct {
	router               *chi.Mux
	snapshot             func() StateSnapshot
	refreshChan          chan struct{}
	logFile              string
	fetchIssues          func(ctx context.Context) ([]TrackerIssue, error)
	fetchIssue           func(ctx context.Context, identifier string) (*TrackerIssue, error)
	cancelIssue          func(identifier string) bool
	resumeIssue          func(identifier string) bool
	terminateIssue       func(identifier string) bool
	fetchLogs            func(identifier string) []string
	clearLogs            func(identifier string) error
	dispatchReviewer     func(identifier string) error
	updateIssueState     func(ctx context.Context, identifier, stateName string) error
	setWorkers           func(n int)
	setIssueProfile      func(identifier, profile string)
	profileDefs          func() map[string]ProfileDef
	upsertProfile        func(name string, def ProfileDef) error
	deleteProfile        func(name string) error
	setAgentMode         func(mode string) error
	setAutoClearWorkspace func(enabled bool) error
	updateTrackerStates  func(active, terminal []string, completion string) error
	reanalyzeIssue       func(identifier string) bool
	projectManager       ProjectManager
	bc                   *broadcaster
}

// New constructs a Server from a Config. Snapshot and RefreshChan must be non-nil.
func New(cfg Config) *Server {
	s := &Server{
		router:               chi.NewRouter(),
		snapshot:             cfg.Snapshot,
		refreshChan:          cfg.RefreshChan,
		logFile:              cfg.LogFile,
		fetchIssues:          cfg.FetchIssues,
		fetchIssue:           cfg.FetchIssue,
		cancelIssue:          cfg.CancelIssue,
		resumeIssue:          cfg.ResumeIssue,
		terminateIssue:       cfg.TerminateIssue,
		reanalyzeIssue:       cfg.ReanalyzeIssue,
		fetchLogs:            cfg.FetchLogs,
		clearLogs:            cfg.ClearLogs,
		dispatchReviewer:     cfg.DispatchReviewer,
		updateIssueState:     cfg.UpdateIssueState,
		setWorkers:           cfg.SetWorkers,
		setIssueProfile:      cfg.SetIssueProfile,
		profileDefs:          cfg.ProfileDefs,
		upsertProfile:        cfg.UpsertProfile,
		deleteProfile:        cfg.DeleteProfile,
		setAgentMode:         cfg.SetAgentMode,
		setAutoClearWorkspace: cfg.SetAutoClearWorkspace,
		updateTrackerStates:  cfg.UpdateTrackerStates,
		projectManager:       cfg.ProjectManager,
		bc:                   newBroadcaster(),
	}
	// Install no-op stubs for nil optional callbacks so handler bodies can call
	// function fields unconditionally without nil guards.
	// projectManager and fetchIssue are deliberately excluded:
	//   - projectManager is a legitimate optional (nil = GitHub tracker, no project API)
	//   - fetchIssue is a fast-path optimisation; nil simply falls back to fetchIssues
	if s.cancelIssue == nil {
		s.cancelIssue = func(string) bool { return false }
	}
	if s.resumeIssue == nil {
		s.resumeIssue = func(string) bool { return false }
	}
	if s.terminateIssue == nil {
		s.terminateIssue = func(string) bool { return false }
	}
	if s.reanalyzeIssue == nil {
		s.reanalyzeIssue = func(string) bool { return false }
	}
	if s.fetchIssues == nil {
		s.fetchIssues = func(context.Context) ([]TrackerIssue, error) { return nil, errNotConfigured }
	}
	if s.fetchLogs == nil {
		s.fetchLogs = func(string) []string { return nil }
	}
	if s.clearLogs == nil {
		s.clearLogs = func(string) error { return errNotConfigured }
	}
	if s.dispatchReviewer == nil {
		s.dispatchReviewer = func(string) error { return errNotConfigured }
	}
	if s.updateIssueState == nil {
		s.updateIssueState = func(context.Context, string, string) error { return errNotConfigured }
	}
	if s.setWorkers == nil {
		s.setWorkers = func(int) {}
	}
	if s.setIssueProfile == nil {
		s.setIssueProfile = func(string, string) {}
	}
	if s.profileDefs == nil {
		s.profileDefs = func() map[string]ProfileDef { return nil }
	}
	if s.upsertProfile == nil {
		s.upsertProfile = func(string, ProfileDef) error { return errNotConfigured }
	}
	if s.deleteProfile == nil {
		s.deleteProfile = func(string) error { return errNotConfigured }
	}
	if s.setAgentMode == nil {
		s.setAgentMode = func(string) error { return errNotConfigured }
	}
	if s.setAutoClearWorkspace == nil {
		s.setAutoClearWorkspace = func(bool) error { return errNotConfigured }
	}
	if s.updateTrackerStates == nil {
		s.updateTrackerStates = func([]string, []string, string) error { return errNotConfigured }
	}
	s.routes()
	return s
}

// Validate checks that all required Config fields are set.
// Call before starting the HTTP listener.
func (s *Server) Validate() error {
	var missing []string
	if s.snapshot == nil {
		missing = append(missing, "Snapshot")
	}
	if s.refreshChan == nil {
		missing = append(missing, "RefreshChan")
	}
	if len(missing) > 0 {
		return fmt.Errorf("server: missing required Config fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Notify signals all active SSE clients to push the current state immediately.
func (s *Server) Notify() {
	s.bc.notify()
}

func spaHandler() http.Handler {
	fs := spaFS()
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// index.html must never be cached: it references hashed JS/CSS assets,
		// and a stale copy would load old bundles after a binary rebuild.
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			// File not found — serve index.html for React Router client-side routing.
			u := *r.URL
			u.Path = "/"
			r2 := *r
			r2.URL = &u
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			fileServer.ServeHTTP(w, &r2)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// API routes are nested under /api so method-not-allowed works correctly
	// even when the SPA catch-all is registered at the root level.
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/state", s.handleState)
		r.Get("/events", s.handleEvents)
		r.Get("/issues", s.handleIssues)
		r.Get("/issues/{identifier}", s.handleIssueDetail)
		r.Get("/issues/{identifier}/logs", s.handleIssueLogs)
		r.Delete("/issues/{identifier}/logs", s.handleClearIssueLogs)
		r.Delete("/issues/{identifier}", s.handleCancelIssue)
		r.Post("/issues/{identifier}/cancel", s.handleCancelIssue)
		r.Post("/issues/{identifier}/resume", s.handleResumeIssue)
		r.Post("/issues/{identifier}/reanalyze", s.handleReanalyzeIssue)
		r.Post("/issues/{identifier}/terminate", s.handleTerminateIssue)
		r.Post("/issues/{identifier}/ai-review", s.handleAIReview)
		r.Patch("/issues/{identifier}/state", s.handleUpdateIssueState)
		r.Post("/issues/{identifier}/profile", s.handleSetIssueProfile)
		r.Get("/logs", s.handleLogs)
		r.Post("/refresh", s.handleRefresh)
		r.Get("/projects", s.handleListProjects)
		r.Get("/projects/filter", s.handleGetProjectFilter)
		r.Put("/projects/filter", s.handleSetProjectFilter)
		r.Post("/settings/workers", s.handleSetWorkers)
		r.Post("/settings/agent-mode", s.handleSetAgentMode)
		r.Post("/settings/workspace/auto-clear", s.handleSetAutoClearWorkspace)
		r.Get("/settings/profiles", s.handleListProfiles)
		r.Put("/settings/profiles/{name}", s.handleUpsertProfile)
		r.Delete("/settings/profiles/{name}", s.handleDeleteProfile)
		r.Put("/settings/tracker/states", s.handleUpdateTrackerStates)
		r.Get("/{identifier}", s.handleIssue)

		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		})
	})

	// React SPA: serves all non-API paths from the embedded web/dist.
	// Falls back to index.html so React Router client-side routing works.
	s.router.Handle("/*", spaHandler())
}
