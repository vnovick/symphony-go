package agent

import "context"

// Event type constants emitted by a supported agent CLI stream.
const (
	EventSystem    = "system"
	EventAssistant = "assistant"
	EventResult    = "result"
)

// TurnResult holds the outcome of a single agent subprocess turn.
type TurnResult struct {
	SessionID         string
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	TotalTokens       int
	LastText      string   // most recent assistant text block
	AllTextBlocks []string // all assistant text blocks across the turn, for tracker comments
	Failed        bool
	InputRequired bool
	FailureText   string // result field from the error event, or stderr output
	ResultText    string // result field from a successful result event
}

// Logger is a minimal structured logging interface, satisfied by *slog.Logger.
type Logger interface {
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

// Runner is the interface for executing a single agent turn.
// Real implementations spawn an agent subprocess; FakeRunner is used in tests.
// log should be pre-seeded with issue context (e.g. issue_identifier) so that
// Claude's live output appears in the log stream with filterable attributes.
// workerHost: if non-empty, the command is executed on that SSH host.
// onProgress, if non-nil, is called after each assistant event with the partial
// TurnResult so callers can stream live token/message updates to the dashboard.
type Runner interface {
	RunTurn(ctx context.Context, log Logger, onProgress func(TurnResult), sessionID *string, prompt, workspacePath, command, workerHost string, readTimeoutMs, turnTimeoutMs int) (TurnResult, error)
}

// ApplyEvent merges a StreamEvent into the accumulated TurnResult.
func ApplyEvent(r TurnResult, ev StreamEvent) TurnResult {
	switch ev.Type {
	case EventSystem:
		if r.SessionID == "" {
			r.SessionID = ev.SessionID
		}
	case EventAssistant:
		// InProgress events (item.started) carry no token counts or text; skip
		// accumulation to avoid polluting AllTextBlocks if the parser ever adds text.
		if ev.InProgress {
			break
		}
		r.InputTokens += ev.Usage.InputTokens
		r.CachedInputTokens += ev.Usage.CachedInputTokens
		r.OutputTokens += ev.Usage.OutputTokens
		r.TotalTokens = r.InputTokens + r.OutputTokens
		if len(ev.TextBlocks) > 0 {
			r.LastText = ev.TextBlocks[len(ev.TextBlocks)-1]
			r.AllTextBlocks = append(r.AllTextBlocks, ev.TextBlocks...)
		}
	case EventResult:
		r.InputTokens += ev.Usage.InputTokens
		r.CachedInputTokens += ev.Usage.CachedInputTokens
		r.OutputTokens += ev.Usage.OutputTokens
		r.TotalTokens = r.InputTokens + r.OutputTokens
		if ev.SessionID != "" {
			r.SessionID = ev.SessionID
		}
		if ev.IsError {
			r.Failed = true
			r.FailureText = ev.ResultText
		} else {
			r.ResultText = ev.ResultText
		}
		if ev.IsInputRequired {
			r.InputRequired = true
		}
	}
	return r
}
