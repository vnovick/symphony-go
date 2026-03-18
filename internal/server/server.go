package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

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
	// "" (off/solo): Claude runs alone.
	// "subagents":   Claude may spawn helpers via the Task tool.
	// "teams":       Task tool + profile role context injected into the prompt.
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
}

// ProfileDef is the JSON representation of one named agent profile.
type ProfileDef struct {
	Command string `json:"command"`
	Prompt  string `json:"prompt,omitempty"`
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

// Server is an HTTP server exposing orchestrator state.
type Server struct {
	router              *chi.Mux
	snapshot            func() StateSnapshot
	refreshChan         chan struct{}
	logFile             string // path to rotating log file; empty = logs endpoint disabled
	fetchIssues         func(ctx context.Context) ([]TrackerIssue, error)
	cancelIssue         func(identifier string) bool                                  // nil = cancel not supported
	resumeIssue         func(identifier string) bool                                  // nil = resume not supported
	terminateIssue      func(identifier string) bool                                  // nil = terminate not supported
	fetchLogs           func(identifier string) []string                              // nil = issue logs endpoint disabled
	clearLogs           func(identifier string) error                                 // nil = clear logs not supported
	dispatchReviewer    func(identifier string) error                                 // nil = ai-review not supported
	updateIssueState    func(ctx context.Context, identifier, stateName string) error // nil = state update not supported
	setWorkers          func(n int)                                                   // nil = worker control not available
	setIssueProfile     func(identifier, profile string)                              // nil = profiles not supported
	profileDefs         func() map[string]ProfileDef                                  // returns current name→ProfileDef map; nil = not supported
	upsertProfile       func(name string, def ProfileDef) error                       // create or update
	deleteProfile       func(name string) error                                       // delete
	setAgentMode        func(mode string) error                                       // nil = agent mode toggle not supported
	updateTrackerStates func(active, terminal []string, completion string) error      // nil = state update not supported
	reanalyzeIssue      func(identifier string) bool                                  // nil = reanalyze not supported
	projectManager      ProjectManager                                                // nil for GitHub (no project support)
	bc                  *broadcaster
}

// New constructs a Server with the given snapshot function and refresh channel.
// logFile is the path to the rotating log file for the /api/v1/logs endpoint.
// fetchIssues is called by GET /api/v1/issues; nil disables the endpoint.
// cancelIssue is called by DELETE /api/v1/issues/{identifier}; nil disables cancellation.
// resumeIssue is called by POST /api/v1/issues/{identifier}/resume; nil disables resume.
// fetchLogs is called by GET /api/v1/issues/{identifier}/logs; nil disables the endpoint.
func New(snapshot func() StateSnapshot, refreshChan chan struct{}, logFile string, fetchIssues func(ctx context.Context) ([]TrackerIssue, error), cancelIssue func(string) bool, resumeIssue func(string) bool, fetchLogs func(string) []string) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		snapshot:    snapshot,
		refreshChan: refreshChan,
		logFile:     logFile,
		fetchIssues: fetchIssues,
		cancelIssue: cancelIssue,
		resumeIssue: resumeIssue,
		fetchLogs:   fetchLogs,
		bc:          newBroadcaster(),
	}

	s.routes()
	return s
}

// SetProjectManager attaches a project manager (Linear only).
// Must be called before the server starts accepting requests.
func (s *Server) SetProjectManager(pm ProjectManager) {
	s.projectManager = pm
}

// SetDispatchReviewer attaches the AI reviewer dispatch callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetDispatchReviewer(fn func(identifier string) error) {
	s.dispatchReviewer = fn
}

// SetUpdateIssueState attaches the issue state update callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetUpdateIssueState(fn func(ctx context.Context, identifier, stateName string) error) {
	s.updateIssueState = fn
}

// SetWorkerSetter attaches the runtime max-workers setter callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetWorkerSetter(fn func(n int)) {
	s.setWorkers = fn
}

// SetIssueProfileSetter attaches the per-issue agent profile setter callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetIssueProfileSetter(fn func(identifier, profile string)) {
	s.setIssueProfile = fn
}

// SetProfileReader attaches the profile definitions reader callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetProfileReader(fn func() map[string]ProfileDef) {
	s.profileDefs = fn
}

// SetProfileUpserter attaches the profile create/update callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetProfileUpserter(fn func(name string, def ProfileDef) error) {
	s.upsertProfile = fn
}

// SetProfileDeleter attaches the profile delete callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetProfileDeleter(fn func(name string) error) {
	s.deleteProfile = fn
}

// SetTerminateIssue attaches the hard-terminate callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetTerminateIssue(fn func(identifier string) bool) {
	s.terminateIssue = fn
}

// SetClearLogs attaches the log-clear callback for DELETE /api/v1/issues/{identifier}/logs.
func (s *Server) SetClearLogs(fn func(identifier string) error) {
	s.clearLogs = fn
}

// SetAgentModeSetter attaches the agent mode setter callback.
// mode is one of "" (off/solo), "subagents", or "teams".
// Must be called before the server starts accepting requests.
func (s *Server) SetAgentModeSetter(fn func(mode string) error) {
	s.setAgentMode = fn
}

// SetTrackerStateUpdater attaches the tracker state update callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetTrackerStateUpdater(fn func(active, terminal []string, completion string) error) {
	s.updateTrackerStates = fn
}

// SetReanalyzeIssue attaches the forced re-analysis callback.
// Must be called before the server starts accepting requests.
func (s *Server) SetReanalyzeIssue(fn func(identifier string) bool) {
	s.reanalyzeIssue = fn
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
