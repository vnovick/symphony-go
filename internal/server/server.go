package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vnovick/itervox/internal/domain"

	"github.com/go-chi/chi/v5"
)

// errNotConfigured is returned by no-op callback stubs installed in New()
// for optional Config fields that were left nil by the caller.
var errNotConfigured = errors.New("not configured")

// RunningRow is a single row in the active sessions table.
type RunningRow struct {
	Identifier    string    `json:"identifier"`
	State         string    `json:"state"`
	TurnCount     int       `json:"turnCount"`
	LastEvent     string    `json:"lastEvent,omitempty"`
	LastEventAt   string    `json:"lastEventAt,omitempty"`
	InputTokens   int       `json:"inputTokens"`
	OutputTokens  int       `json:"outputTokens"`
	Tokens        int       `json:"tokens"`
	ElapsedMs     int64     `json:"elapsedMs"`
	StartedAt     time.Time `json:"startedAt"`
	SessionID     string    `json:"sessionId,omitempty"`
	WorkerHost    string    `json:"workerHost,omitempty"`
	Backend       string    `json:"backend,omitempty"`
	Kind          string    `json:"kind,omitempty"` // "worker" (default) | "reviewer"
	SubagentCount int       `json:"subagentCount,omitempty"`
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
	Status       string    `json:"status"` // "succeeded" | "failed" | "cancelled" | "stalled" | "input_required"
	WorkerHost   string    `json:"workerHost,omitempty"`
	Backend      string    `json:"backend,omitempty"`
	SessionID    string    `json:"sessionId,omitempty"`
	AppSessionID string    `json:"appSessionId,omitempty"`
	Kind         string    `json:"kind,omitempty"` // "worker" (default) | "reviewer"
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

// OrchestratorClient abstracts the orchestrator and workflow operations called
// by HTTP handlers. A nil value in Config is replaced with noopClient.
type OrchestratorClient interface {
	FetchIssues(ctx context.Context) ([]TrackerIssue, error)
	CancelIssue(identifier string) bool
	ResumeIssue(identifier string) bool
	TerminateIssue(identifier string) bool
	ReanalyzeIssue(identifier string) bool
	FetchLogs(identifier string) []string
	ClearLogs(identifier string) error
	ClearAllLogs() error
	ClearIssueSubLogs(identifier string) error
	ClearSessionSublog(identifier, sessionID string) error
	FetchSubLogs(identifier string) ([]domain.IssueLogEntry, error)
	DispatchReviewer(identifier string) error
	UpdateIssueState(ctx context.Context, identifier, stateName string) error
	SetWorkers(n int)
	BumpWorkers(delta int) int
	SetIssueProfile(identifier, profile string)
	SetIssueBackend(identifier, backend string)
	ProfileDefs() map[string]ProfileDef
	AvailableModels() map[string][]ModelOption
	ReviewerConfig() (profile string, autoReview bool)
	SetReviewerConfig(profile string, autoReview bool) error
	UpsertProfile(name string, def ProfileDef) error
	DeleteProfile(name string) error
	SetAgentMode(mode string) error
	SetAutoClearWorkspace(enabled bool) error
	ClearAllWorkspaces() error
	FetchLogIdentifiers() []string
	UpdateTrackerStates(active, terminal []string, completion string) error
	AddSSHHost(host, description string) error
	RemoveSSHHost(host string) error
	SetDispatchStrategy(strategy string) error
	ProvideInput(identifier, message string) bool
	DismissInput(identifier string) bool
	SetInlineInput(enabled bool) error
}

// noopClient implements OrchestratorClient with harmless defaults.
// Boolean methods return false; error methods return errNotConfigured.
type noopClient struct{}

func (noopClient) FetchIssues(context.Context) ([]TrackerIssue, error)    { return nil, errNotConfigured }
func (noopClient) CancelIssue(string) bool                                { return false }
func (noopClient) ResumeIssue(string) bool                                { return false }
func (noopClient) TerminateIssue(string) bool                             { return false }
func (noopClient) ReanalyzeIssue(string) bool                             { return false }
func (noopClient) FetchLogs(string) []string                              { return nil }
func (noopClient) ClearLogs(string) error                                 { return errNotConfigured }
func (noopClient) ClearAllLogs() error                                    { return errNotConfigured }
func (noopClient) ClearIssueSubLogs(string) error                         { return errNotConfigured }
func (noopClient) ClearSessionSublog(string, string) error                { return errNotConfigured }
func (noopClient) FetchSubLogs(string) ([]domain.IssueLogEntry, error)    { return nil, nil }
func (noopClient) DispatchReviewer(string) error                          { return errNotConfigured }
func (noopClient) UpdateIssueState(context.Context, string, string) error { return errNotConfigured }
func (noopClient) SetWorkers(int)                                         {}
func (noopClient) BumpWorkers(int) int                                    { return 0 }
func (noopClient) SetIssueProfile(string, string)                         {}
func (noopClient) SetIssueBackend(string, string)                         {}
func (noopClient) ProfileDefs() map[string]ProfileDef                     { return nil }
func (noopClient) AvailableModels() map[string][]ModelOption              { return nil }
func (noopClient) ReviewerConfig() (string, bool)                         { return "", false }
func (noopClient) SetReviewerConfig(string, bool) error                   { return nil }
func (noopClient) UpsertProfile(string, ProfileDef) error                 { return errNotConfigured }
func (noopClient) DeleteProfile(string) error                             { return errNotConfigured }
func (noopClient) SetAgentMode(string) error                              { return errNotConfigured }
func (noopClient) SetAutoClearWorkspace(bool) error                       { return errNotConfigured }
func (noopClient) ClearAllWorkspaces() error                              { return errNotConfigured }
func (noopClient) FetchLogIdentifiers() []string                          { return nil }
func (noopClient) UpdateTrackerStates([]string, []string, string) error   { return errNotConfigured }
func (noopClient) AddSSHHost(string, string) error                        { return errNotConfigured }
func (noopClient) RemoveSSHHost(string) error                             { return errNotConfigured }
func (noopClient) SetDispatchStrategy(string) error                       { return errNotConfigured }
func (noopClient) ProvideInput(string, string) bool                       { return false }
func (noopClient) DismissInput(string) bool                               { return false }
func (noopClient) SetInlineInput(bool) error                              { return errNotConfigured }

// FuncClient builds an OrchestratorClient from individual function fields.
// Any nil field falls back to the noopClient default. Intended for tests.
type FuncClient struct {
	FetchIssuesFn           func(context.Context) ([]TrackerIssue, error)
	CancelIssueFn           func(string) bool
	ResumeIssueFn           func(string) bool
	TerminateIssueFn        func(string) bool
	ReanalyzeIssueFn        func(string) bool
	FetchLogsFn             func(string) []string
	ClearLogsFn             func(string) error
	ClearAllLogsFn          func() error
	ClearIssueSubLogsFn     func(string) error
	ClearSessionSublogFn    func(string, string) error
	DispatchReviewerFn      func(string) error
	UpdateIssueStateFn      func(context.Context, string, string) error
	SetWorkersFn            func(int)
	BumpWorkersFn           func(int) int
	SetIssueProfileFn       func(string, string)
	SetIssueBackendFn       func(string, string)
	ProfileDefsFn           func() map[string]ProfileDef
	AvailableModelsFn       func() map[string][]ModelOption
	ReviewerConfigFn        func() (string, bool)
	SetReviewerConfigFn     func(string, bool) error
	UpsertProfileFn         func(string, ProfileDef) error
	DeleteProfileFn         func(string) error
	SetAgentModeFn          func(string) error
	SetAutoClearWorkspaceFn func(bool) error
	ClearAllWorkspacesFn    func() error
	FetchLogIdentifiersFn   func() []string
	UpdateTrackerStatesFn   func([]string, []string, string) error
	FetchSubLogsFn          func(string) ([]domain.IssueLogEntry, error)
	AddSSHHostFn            func(string, string) error
	RemoveSSHHostFn         func(string) error
	SetDispatchStrategyFn   func(string) error
}

func (c *FuncClient) FetchIssues(ctx context.Context) ([]TrackerIssue, error) {
	if c.FetchIssuesFn != nil {
		return c.FetchIssuesFn(ctx)
	}
	return nil, errNotConfigured
}
func (c *FuncClient) CancelIssue(id string) bool {
	if c.CancelIssueFn != nil {
		return c.CancelIssueFn(id)
	}
	return false
}
func (c *FuncClient) ResumeIssue(id string) bool {
	if c.ResumeIssueFn != nil {
		return c.ResumeIssueFn(id)
	}
	return false
}
func (c *FuncClient) TerminateIssue(id string) bool {
	if c.TerminateIssueFn != nil {
		return c.TerminateIssueFn(id)
	}
	return false
}
func (c *FuncClient) ReanalyzeIssue(id string) bool {
	if c.ReanalyzeIssueFn != nil {
		return c.ReanalyzeIssueFn(id)
	}
	return false
}
func (c *FuncClient) FetchLogs(id string) []string {
	if c.FetchLogsFn != nil {
		return c.FetchLogsFn(id)
	}
	return nil
}
func (c *FuncClient) ClearLogs(id string) error {
	if c.ClearLogsFn != nil {
		return c.ClearLogsFn(id)
	}
	return errNotConfigured
}
func (c *FuncClient) ClearAllLogs() error {
	if c.ClearAllLogsFn != nil {
		return c.ClearAllLogsFn()
	}
	return errNotConfigured
}
func (c *FuncClient) ClearIssueSubLogs(id string) error {
	if c.ClearIssueSubLogsFn != nil {
		return c.ClearIssueSubLogsFn(id)
	}
	return errNotConfigured
}
func (c *FuncClient) ClearSessionSublog(id, sessionID string) error {
	if c.ClearSessionSublogFn != nil {
		return c.ClearSessionSublogFn(id, sessionID)
	}
	return errNotConfigured
}
func (c *FuncClient) FetchSubLogs(id string) ([]domain.IssueLogEntry, error) {
	if c.FetchSubLogsFn != nil {
		return c.FetchSubLogsFn(id)
	}
	return nil, nil
}
func (c *FuncClient) DispatchReviewer(id string) error {
	if c.DispatchReviewerFn != nil {
		return c.DispatchReviewerFn(id)
	}
	return errNotConfigured
}
func (c *FuncClient) UpdateIssueState(ctx context.Context, id, state string) error {
	if c.UpdateIssueStateFn != nil {
		return c.UpdateIssueStateFn(ctx, id, state)
	}
	return errNotConfigured
}
func (c *FuncClient) SetWorkers(n int) {
	if c.SetWorkersFn != nil {
		c.SetWorkersFn(n)
	}
}
func (c *FuncClient) BumpWorkers(delta int) int {
	if c.BumpWorkersFn != nil {
		return c.BumpWorkersFn(delta)
	}
	return 0
}
func (c *FuncClient) SetIssueProfile(id, profile string) {
	if c.SetIssueProfileFn != nil {
		c.SetIssueProfileFn(id, profile)
	}
}
func (c *FuncClient) SetIssueBackend(id, backend string) {
	if c.SetIssueBackendFn != nil {
		c.SetIssueBackendFn(id, backend)
	}
}
func (c *FuncClient) ProfileDefs() map[string]ProfileDef {
	if c.ProfileDefsFn != nil {
		return c.ProfileDefsFn()
	}
	return nil
}
func (c *FuncClient) AvailableModels() map[string][]ModelOption {
	if c.AvailableModelsFn != nil {
		return c.AvailableModelsFn()
	}
	return nil
}
func (c *FuncClient) ReviewerConfig() (string, bool) {
	if c.ReviewerConfigFn != nil {
		return c.ReviewerConfigFn()
	}
	return "", false
}
func (c *FuncClient) SetReviewerConfig(profile string, autoReview bool) error {
	if c.SetReviewerConfigFn != nil {
		return c.SetReviewerConfigFn(profile, autoReview)
	}
	return nil
}
func (c *FuncClient) UpsertProfile(name string, def ProfileDef) error {
	if c.UpsertProfileFn != nil {
		return c.UpsertProfileFn(name, def)
	}
	return errNotConfigured
}
func (c *FuncClient) DeleteProfile(name string) error {
	if c.DeleteProfileFn != nil {
		return c.DeleteProfileFn(name)
	}
	return errNotConfigured
}
func (c *FuncClient) SetAgentMode(mode string) error {
	if c.SetAgentModeFn != nil {
		return c.SetAgentModeFn(mode)
	}
	return errNotConfigured
}
func (c *FuncClient) SetAutoClearWorkspace(enabled bool) error {
	if c.SetAutoClearWorkspaceFn != nil {
		return c.SetAutoClearWorkspaceFn(enabled)
	}
	return errNotConfigured
}
func (c *FuncClient) ClearAllWorkspaces() error {
	if c.ClearAllWorkspacesFn != nil {
		return c.ClearAllWorkspacesFn()
	}
	return errNotConfigured
}
func (c *FuncClient) FetchLogIdentifiers() []string {
	if c.FetchLogIdentifiersFn != nil {
		return c.FetchLogIdentifiersFn()
	}
	return nil
}
func (c *FuncClient) UpdateTrackerStates(active, terminal []string, completion string) error {
	if c.UpdateTrackerStatesFn != nil {
		return c.UpdateTrackerStatesFn(active, terminal, completion)
	}
	return errNotConfigured
}
func (c *FuncClient) AddSSHHost(host, description string) error {
	if c.AddSSHHostFn != nil {
		return c.AddSSHHostFn(host, description)
	}
	return errNotConfigured
}
func (c *FuncClient) RemoveSSHHost(host string) error {
	if c.RemoveSSHHostFn != nil {
		return c.RemoveSSHHostFn(host)
	}
	return errNotConfigured
}
func (c *FuncClient) SetDispatchStrategy(strategy string) error {
	if c.SetDispatchStrategyFn != nil {
		return c.SetDispatchStrategyFn(strategy)
	}
	return errNotConfigured
}
func (c *FuncClient) ProvideInput(identifier, message string) bool { return false }
func (c *FuncClient) DismissInput(identifier string) bool          { return false }
func (c *FuncClient) SetInlineInput(bool) error                    { return errNotConfigured }

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
	ProfileDefs     map[string]ProfileDef    `json:"profileDefs,omitempty"`
	AvailableModels map[string][]ModelOption `json:"availableModels,omitempty"`
	ReviewerProfile string                   `json:"reviewerProfile,omitempty"`
	AutoReview      bool                     `json:"autoReview,omitempty"`
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
	// CurrentAppSessionID is the ID of the current daemon invocation.
	// All history rows produced during this run share this ID.
	CurrentAppSessionID string `json:"currentAppSessionId,omitempty"`
	// SSHHosts is the configured SSH worker host pool with optional descriptions.
	// Empty/absent means all work runs locally.
	SSHHosts []SSHHostInfo `json:"sshHosts,omitempty"`
	// DispatchStrategy is the active SSH host dispatch strategy.
	// "round-robin" (default) | "least-loaded"
	DispatchStrategy string `json:"dispatchStrategy,omitempty"`
	// DefaultBackend is the configured default runner backend ("claude" or "codex").
	// Used by the frontend to show the correct badge on non-running issues.
	DefaultBackend string `json:"defaultBackend,omitempty"`
	// InlineInput indicates whether agent input-required signals are posted as
	// tracker comments (true) or queued in the dashboard UI (false).
	InlineInput bool `json:"inlineInput,omitempty"`
	// InputRequired lists issues whose agent is blocked waiting for human input.
	InputRequired []InputRequiredRow `json:"inputRequired,omitempty"`
}

// InputRequiredRow is one issue waiting for human input in the snapshot.
type InputRequiredRow struct {
	Identifier string `json:"identifier"`
	SessionID  string `json:"sessionId"`
	Context    string `json:"context"`
	Backend    string `json:"backend,omitempty"`
	Profile    string `json:"profile,omitempty"`
	QueuedAt   string `json:"queuedAt"`
}

// SSHHostInfo is one entry in the configured SSH host pool.
type SSHHostInfo struct {
	Host        string `json:"host"`
	Description string `json:"description,omitempty"`
}

// ProfileDef is the JSON representation of one named agent profile.
type ProfileDef struct {
	Command string `json:"command"`
	Prompt  string `json:"prompt,omitempty"`
	Backend string `json:"backend,omitempty"`
}

// ModelOption represents an available model for a backend (mirrors config.ModelOption for JSON).
type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
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
	// AgentBackend is the per-issue backend override, if any ("claude" or "codex").
	AgentBackend string `json:"agentBackend,omitempty"`
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
	Detail    string `json:"detail,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
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
// Client provides orchestrator operations; nil → noopClient.
// FetchIssue is an optional fast-path for single-issue detail lookups; nil falls back to Client.FetchIssues.
// ProjectManager is optional: nil means GitHub tracker (no project API).
type Config struct {
	// Required
	Snapshot    func() StateSnapshot
	RefreshChan chan struct{}
	// LogFile is the path to the rotating log file for /api/v1/logs; empty disables it.
	LogFile string

	// Client provides all orchestrator operations. Nil → noopClient (no-ops).
	Client OrchestratorClient
	// FetchIssue is an optional fast-path for single-issue lookups.
	// Nil falls back to Client.FetchIssues scanning all issues.
	FetchIssue func(ctx context.Context, identifier string) (*TrackerIssue, error)
	// ProjectManager supports project filtering (Linear only). Nil = no project API.
	ProjectManager ProjectManager
	// APIToken, when non-empty, enables bearer-token authentication on all
	// /api/ routes except /api/v1/health. Requests must include the header
	// "Authorization: Bearer <token>".
	APIToken string
}

// Server is an HTTP server exposing orchestrator state.
type Server struct {
	router         *chi.Mux
	snapshot       func() StateSnapshot
	refreshChan    chan struct{}
	logFile        string
	client         OrchestratorClient
	fetchIssue     func(ctx context.Context, identifier string) (*TrackerIssue, error)
	projectManager ProjectManager
	bc             *broadcaster
	apiToken       string
}

// New constructs a Server from a Config. Snapshot and RefreshChan must be non-nil.
func New(cfg Config) *Server {
	client := cfg.Client
	if client == nil {
		client = noopClient{}
	}
	s := &Server{
		router:         chi.NewRouter(),
		snapshot:       cfg.Snapshot,
		refreshChan:    cfg.RefreshChan,
		logFile:        cfg.LogFile,
		client:         client,
		fetchIssue:     cfg.FetchIssue,
		projectManager: cfg.ProjectManager,
		bc:             newBroadcaster(),
		apiToken:       cfg.APIToken,
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
		// Health check is unauthenticated so load balancers can reach it.
		r.Get("/health", s.handleHealth)

		// If an API token is configured, all remaining routes require it.
		// Use r.Group to create a sub-router so middleware is applied only to
		// authenticated routes without violating chi's "middleware before routes" rule.
		r.Group(func(r chi.Router) {
			if s.apiToken != "" {
				r.Use(s.bearerAuthMiddleware)
			}

			r.Get("/state", s.handleState)
			r.Get("/events", s.handleEvents)
			r.Get("/issues", s.handleIssues)
			r.Get("/issues/{identifier}", s.handleIssueDetail)
			r.Get("/issues/{identifier}/logs", s.handleIssueLogs)
			r.Get("/issues/{identifier}/log-stream", s.handleIssueLogStream)
			r.Get("/issues/{identifier}/sublogs", s.handleSubLogs)
			r.Delete("/issues/{identifier}/logs", s.handleClearIssueLogs)
			r.Delete("/issues/{identifier}/sublogs", s.handleClearIssueSubLogs)
			r.Delete("/issues/{identifier}/sublogs/{sessionId}", s.handleClearSessionSublog)
			r.Get("/logs/identifiers", s.handleLogIdentifiers)
			r.Delete("/logs", s.handleClearAllLogs)
			r.Delete("/issues/{identifier}", s.handleCancelIssue)
			r.Post("/issues/{identifier}/cancel", s.handleCancelIssue)
			r.Post("/issues/{identifier}/resume", s.handleResumeIssue)
			r.Post("/issues/{identifier}/reanalyze", s.handleReanalyzeIssue)
			r.Post("/issues/{identifier}/terminate", s.handleTerminateIssue)
			r.Post("/issues/{identifier}/ai-review", s.handleAIReview)
			r.Patch("/issues/{identifier}/state", s.handleUpdateIssueState)
			r.Post("/issues/{identifier}/profile", s.handleSetIssueProfile)
			r.Post("/issues/{identifier}/backend", s.handleSetIssueBackend)
			r.Post("/issues/{identifier}/provide-input", s.handleProvideInput)
			r.Post("/issues/{identifier}/dismiss-input", s.handleDismissInput)
			r.Post("/settings/inline-input", s.handleSetInlineInput)
			r.Get("/logs", s.handleLogs)
			r.Post("/refresh", s.handleRefresh)
			r.Get("/projects", s.handleListProjects)
			r.Get("/projects/filter", s.handleGetProjectFilter)
			r.Put("/projects/filter", s.handleSetProjectFilter)
			r.Post("/settings/workers", s.handleSetWorkers)
			r.Post("/settings/agent-mode", s.handleSetAgentMode)
			r.Delete("/workspaces", s.handleClearAllWorkspaces)
			r.Post("/settings/workspace/auto-clear", s.handleSetAutoClearWorkspace)
			r.Get("/settings/models", s.handleListModels)
			r.Get("/settings/reviewer", s.handleGetReviewer)
			r.Put("/settings/reviewer", s.handleSetReviewer)
			r.Get("/settings/profiles", s.handleListProfiles)
			r.Put("/settings/profiles/{name}", s.handleUpsertProfile)
			r.Delete("/settings/profiles/{name}", s.handleDeleteProfile)
			r.Put("/settings/tracker/states", s.handleUpdateTrackerStates)
			r.Post("/settings/ssh-hosts", s.handleAddSSHHost)
			r.Delete("/settings/ssh-hosts/{host}", s.handleRemoveSSHHost)
			r.Put("/settings/dispatch-strategy", s.handleSetDispatchStrategy)

			r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			})
		})
	})

	// React SPA: serves all non-API paths from the embedded web/dist.
	// Falls back to index.html so React Router client-side routing works.
	s.router.Handle("/*", spaHandler())
}

// handleHealth returns a lightweight 200 OK for load balancer probes.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// bearerAuthMiddleware rejects requests that do not carry a valid
// "Authorization: Bearer <token>" header matching s.apiToken.
func (s *Server) bearerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) || strings.TrimPrefix(auth, prefix) != s.apiToken {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
