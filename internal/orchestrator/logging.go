package orchestrator

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vnovick/itervox/internal/domain"
	"github.com/vnovick/itervox/internal/logbuffer"
)

// bufLogger wraps a slog.Logger and also writes INFO/WARN lines to a LogBuffer.
type bufLogger struct {
	base       *slog.Logger
	buf        *logbuffer.Buffer
	identifier string
	// sessionID is prepended to every log buffer entry so that all entries for
	// a run — including early hook/worker messages written before the agent
	// subprocess starts — share the same session ID and are attributable to the
	// correct run in the Timeline.
	sessionID string
}

// Info logs at INFO level and writes the message to the log buffer.
func (l *bufLogger) Info(msg string, args ...any) {
	l.base.Info(msg, args...)
	if l.buf != nil {
		l.buf.Add(l.identifier, formatBufLine("INFO", msg, l.withSessionID(args)))
	}
}

// Debug logs at DEBUG level (not written to log buffer).
func (l *bufLogger) Debug(msg string, args ...any) {
	l.base.Debug(msg, args...)
}

// Warn logs at WARN level and writes the message to the log buffer.
func (l *bufLogger) Warn(msg string, args ...any) {
	l.base.Warn(msg, args...)
	if l.buf != nil {
		l.buf.Add(l.identifier, formatBufLine("WARN", msg, l.withSessionID(args)))
	}
}

// withSessionID appends "session_id", l.sessionID to args so it overrides any
// session_id already present (formatBufLine uses last-write-wins for duplicate keys).
func (l *bufLogger) withSessionID(args []any) []any {
	if l.sessionID == "" {
		return args
	}
	return append(args, "session_id", l.sessionID)
}

// bufLogEntry is a package-local alias for domain.BufLogEntry.
// The canonical definition lives in internal/domain so it can be shared
// with the server package without an import cycle.
type bufLogEntry = domain.BufLogEntry

// formatBufLine serialises a log buffer entry as a single JSON line.
// The schema is stable and parseable without string scanning.
func formatBufLine(level, msg string, args []any) string {
	e := bufLogEntry{
		Level: level,
		Msg:   msg,
		Time:  time.Now().Format("15:04:05"),
	}
	// Map well-known attribute keys into the struct; unknown keys are ignored.
	for i := 0; i+1 < len(args); i += 2 {
		key := fmt.Sprintf("%v", args[i])
		val := fmt.Sprintf("%v", args[i+1])
		switch key {
		case "text":
			e.Text = val
		case "tool":
			e.Tool = val
		case "description":
			e.Description = val
		case "status":
			e.Status = val
		case "exit_code":
			e.ExitCode = val
		case "output_size":
			e.OutputSize = val
		case "task":
			e.Task = val
		case "url":
			e.URL = val
		case "summary":
			e.Summary = val
		case "detail":
			e.Detail = val
		case "session_id":
			e.SessionID = val //nolint:govet
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		// Fallback: return a minimal JSON object so the parser always gets valid JSON.
		// Use json.Marshal for the message to properly escape quotes and backslashes.
		escapedMsg, _ := json.Marshal(msg)
		return `{"level":"` + level + `","msg":` + string(escapedMsg) + `,"time":"` + e.Time + `"}`
	}
	return string(b)
}

// makeBufLine builds a timestamped JSON log buffer line for direct (non-slog) entries.
func makeBufLine(level, msg string) string {
	return formatBufLine(level, msg, nil)
}

// makeBufLineWithSession is like makeBufLine but stamps the entry with sessionID so
// early worker/hook log lines are attributable to the correct run in the Timeline.
func makeBufLineWithSession(level, msg, sessionID string) string {
	if sessionID == "" {
		return makeBufLine(level, msg)
	}
	return formatBufLine(level, msg, []any{"session_id", sessionID})
}

// formatSessionComment builds a Markdown comment summarising the full agent session.
// allText is every assistant text block emitted across all turns.
// Returns empty string if there is nothing worth posting.
func formatSessionComment(allText []string, identifier string) string {
	var nonEmpty []string
	for _, t := range allText {
		if strings.TrimSpace(t) != "" {
			nonEmpty = append(nonEmpty, t)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Itervox Agent Session — ")
	sb.WriteString(identifier)
	sb.WriteString("\n\n")
	for _, t := range nonEmpty {
		sb.WriteString(t)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
