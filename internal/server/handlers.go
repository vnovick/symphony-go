package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vnovick/symphony-go/internal/domain"
)

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshot()
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	select {
	case s.refreshChan <- struct{}{}:
	default:
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"queued":    true,
		"queued_at": time.Now(),
	})
}

// handleEvents streams state snapshots as Server-Sent Events.
// Each event is a "data: <JSON>\n\n" frame carrying the full StateSnapshot.
// A keep-alive comment (": ping\n\n") is sent every 25 s to prevent proxy timeouts.
// GET /api/v1/events
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial snapshot immediately.
	if err := s.writeSSEEvent(w, flusher); err != nil {
		return
	}

	// Subscribe to state-change signals.
	sub := s.bc.subscribe()
	defer s.bc.unsubscribe(sub)

	// Keep-alive ticker (every 25s) to prevent proxy timeouts.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-sub:
			if err := s.writeSSEEvent(w, flusher); err != nil {
				return
			}
		case <-ticker.C:
			// Send SSE comment as heartbeat.
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher) error {
	snap := s.snapshot()
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// handleReanalyzeIssue moves a paused issue to the forced re-analysis queue,
// bypassing the open-PR guard on next dispatch.
// POST /api/v1/issues/{identifier}/reanalyze
func (s *Server) handleReanalyzeIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if !s.client.ReanalyzeIssue(identifier) {
		writeError(w, http.StatusNotFound, "not_paused", "issue "+identifier+" is not paused")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queued": true, "identifier": identifier})
}

// handleResumeIssue removes a paused issue from the pause set so it can be dispatched again.
func (s *Server) handleResumeIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if s.client.ResumeIssue(identifier) {
		writeJSON(w, http.StatusOK, map[string]any{"resumed": true, "identifier": identifier})
	} else {
		writeError(w, http.StatusNotFound, "not_paused", "issue "+identifier+" is not paused")
	}
}

// handleCancelIssue cancels the running worker for the given issue identifier.
func (s *Server) handleCancelIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if s.client.CancelIssue(identifier) {
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true, "identifier": identifier})
	} else {
		writeError(w, http.StatusNotFound, "not_running", "issue "+identifier+" is not running")
	}
}

// handleTerminateIssue hard-stops a running or paused issue without adding it to PausedIdentifiers.
func (s *Server) handleTerminateIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if s.client.TerminateIssue(identifier) {
		writeJSON(w, http.StatusOK, map[string]any{"terminated": true, "identifier": identifier})
	} else {
		writeError(w, http.StatusNotFound, "not_found", "issue "+identifier+" is not running or paused")
	}
}

// handleIssueDetail returns a single issue by identifier, enriched with orchestrator state.
func (s *Server) handleIssueDetail(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")

	// Fast path: use the single-item callback when available.
	if s.fetchIssue != nil {
		issue, err := s.fetchIssue(r.Context(), identifier)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
			return
		}
		if issue == nil {
			writeError(w, http.StatusNotFound, "not_found", "issue "+identifier+" not found")
			return
		}
		writeJSON(w, http.StatusOK, *issue)
		return
	}

	// Slow path: scan all issues.
	issues, err := s.client.FetchIssues(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
		return
	}
	for _, issue := range issues {
		if issue.Identifier == identifier {
			writeJSON(w, http.StatusOK, issue)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "issue "+identifier+" not found")
}

// handleIssues returns all project issues enriched with orchestrator state.
func (s *Server) handleIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := s.client.FetchIssues(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issues)
}

// handleLogs streams the symphony log file as Server-Sent Events.
// On connect it sends the last 16 KB of the file, then tails for new lines.
// Each SSE event is: event: log\ndata: <one log line>\n\n
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.logFile == "" {
		http.Error(w, "log file not configured", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	f, err := os.Open(s.logFile)
	if err != nil {
		_, _ = fmt.Fprintf(w, "event: log\ndata: [log file not yet available: %s]\n\n", err)
		flusher.Flush()
		return
	}
	defer func() { _ = f.Close() }()

	// Seek to last 16 KB for initial history.
	const tail = 16 * 1024
	if fi, err := f.Stat(); err == nil && fi.Size() > tail {
		_, _ = f.Seek(-tail, io.SeekEnd)
		// Skip to next newline so we don't send a partial line.
		buf := make([]byte, 1)
		for {
			n, err := f.Read(buf)
			if err != nil || n == 0 || buf[0] == '\n' {
				break
			}
		}
	}

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	// maxPending caps the incomplete-line carry buffer so a runaway log stream
	// without newlines cannot grow pending unboundedly.
	const maxPending = 256 * 1024 // 256 KB

	readBuf := make([]byte, 32*1024)
	var pending bytes.Buffer

	// Optional identifier filter: only emit lines belonging to this issue.
	// We parse each line as JSON and match on the issue_identifier field for
	// exactness — a substring match on raw bytes can produce false positives
	// (e.g. "PROJ-1" matches "PROJ-10"). Fall back to substring for non-JSON
	// lines so that legacy plain-text entries are still included (GO-R10-6).
	filterID := r.URL.Query().Get("identifier")

	lineMatchesFilter := func(line string) bool {
		if filterID == "" {
			return true
		}
		var entry struct {
			IssueIdentifier string `json:"issue_identifier"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			return entry.IssueIdentifier == filterID
		}
		// Non-JSON fallback: substring match.
		return strings.Contains(line, filterID)
	}

	flushPending := func() {
		for {
			idx := bytes.IndexByte(pending.Bytes(), '\n')
			if idx < 0 {
				break
			}
			line := string(pending.Next(idx + 1))
			line = strings.TrimRight(line, "\n")
			if line == "" {
				continue
			}
			if !lineMatchesFilter(line) {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", line)
		}
	}

	flush := func() {
		for {
			n, err := f.Read(readBuf)
			if n > 0 {
				if pending.Len()+n > maxPending {
					// Flush what we have before accepting more so data is not dropped.
					flushPending()
				}
				pending.Write(readBuf[:n])
			}
			// Send complete lines.
			flushPending()
			if err != nil || n == 0 {
				// n == 0 with err == nil means no new data (EOF on regular file);
				// break to avoid a busy-spin until the next ticker tick (GO-R10-5).
				break
			}
		}
		flusher.Flush()
	}

	flush() // send initial tail immediately
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			flush()
		}
	}
}

// handleIssueLogs returns parsed log entries for a specific issue identifier
// from the in-memory log buffer (only available for currently-running sessions).
func (s *Server) handleIssueLogs(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	lines := s.client.FetchLogs(identifier)
	entries := make([]IssueLogEntry, 0, len(lines))
	for _, line := range lines {
		entry, skip := parseLogLine(line)
		if skip {
			continue
		}
		entries = append(entries, entry)
	}
	writeJSON(w, http.StatusOK, entries)
}

// handleIssueLogStream streams parsed log entries for one issue as SSE.
// It tracks a cursor into the in-memory buffer and emits only new entries on
// each tick, so clients receive push notifications instead of polling.
// If the buffer is reset (cleared/issue removed) the cursor resets and all
// current entries are re-sent.
// GET /api/v1/issues/{identifier}/log-stream
func (s *Server) handleIssueLogStream(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// cursor tracks how many lines from the buffer have already been sent.
	sent := 0

	sendNew := func() bool {
		lines := s.client.FetchLogs(identifier)
		// Guard against buffer reset (cleared while streaming).
		if sent > len(lines) {
			sent = 0
		}
		for _, line := range lines[sent:] {
			sent++
			entry, skip := parseLogLine(line)
			if skip {
				continue
			}
			b, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: log\ndata: %s\n\n", b); err != nil {
				return false
			}
		}
		flusher.Flush()
		return true
	}

	if !sendNew() {
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !sendNew() {
				return
			}
		}
	}
}

// handleClearIssueLogs deletes the in-memory and on-disk log buffer for an issue.
// DELETE /api/v1/issues/{identifier}/logs
func (s *Server) handleClearIssueLogs(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if err := s.client.ClearLogs(identifier); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleClearIssueSubLogs deletes all JSONL session files for one issue.
// DELETE /api/v1/issues/{identifier}/sublogs
func (s *Server) handleClearIssueSubLogs(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if err := s.client.ClearIssueSubLogs(identifier); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleLogIdentifiers returns a list of issue identifiers that have log data
// (either in-memory or on-disk). Used by the Logs page sidebar to show only
// issues with actual log files, not all tracker issues.
// GET /api/v1/logs/identifiers
func (s *Server) handleLogIdentifiers(w http.ResponseWriter, r *http.Request) {
	ids := s.client.FetchLogIdentifiers()
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, ids)
}

// handleClearAllLogs deletes in-memory and on-disk log buffers for all issues.
// DELETE /api/v1/logs
func (s *Server) handleClearAllLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.client.ClearAllLogs(); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleClearSessionSublog deletes the JSONL file for a specific agent session run.
// DELETE /api/v1/issues/{identifier}/sublogs/{sessionId}
func (s *Server) handleClearSessionSublog(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	sessionID := chi.URLParam(r, "sessionId")
	if err := s.client.ClearSessionSublog(identifier, sessionID); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// skipEntry returns true for internal lifecycle events that are noise in the timeline.
// Operates on already-parsed BufLogEntry fields rather than string-prefix matching.
func skipEntry(e bufLogEntry) bool {
	if e.Level == "DEBUG" {
		return true
	}
	switch e.Msg {
	case "claude: session started", "claude: turn done",
		"codex: session started", "codex: turn done":
		return true
	}
	return false
}

// buildDetailJSON builds a compact JSON detail string for shell completions.
// Fields are omitted when empty so the Detail field stays minimal.
// Uses a struct for deterministic key ordering in the JSON output.
func buildDetailJSON(status, exitCode, outputSize string) string {
	type detail struct {
		Status     string `json:"status,omitempty"`
		ExitCode   *int   `json:"exit_code,omitempty"`
		OutputSize *int   `json:"output_size,omitempty"`
	}
	d := detail{Status: status}
	if exitCode != "" {
		if n, err := strconv.Atoi(exitCode); err == nil {
			d.ExitCode = &n
		}
	}
	if outputSize != "" {
		if n, err := strconv.Atoi(outputSize); err == nil {
			d.OutputSize = &n
		}
	}
	if d.Status == "" && d.ExitCode == nil && d.OutputSize == nil {
		return ""
	}
	b, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return string(b)
}

// bufLogEntry is a package-local alias for domain.BufLogEntry.
// The canonical definition lives in internal/domain, shared with the orchestrator.
type bufLogEntry = domain.BufLogEntry

// parseLogLine converts a JSON log buffer line into a structured IssueLogEntry.
// Returns (entry, false) for valid entries, (zero, true) to signal skip.
func parseLogLine(line string) (IssueLogEntry, bool) {
	var e bufLogEntry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		// Non-JSON line (e.g. legacy entry) — skip rather than panic.
		return IssueLogEntry{}, true
	}

	if skipEntry(e) {
		return IssueLogEntry{}, true
	}

	entry := IssueLogEntry{Level: e.Level, Time: e.Time, SessionID: e.SessionID}

	switch e.Msg {
	case "claude: text", "codex: text":
		entry.Event = "text"
		entry.Message = e.Text
	case "claude: subagent", "codex: subagent":
		entry.Event = "subagent"
		entry.Tool = e.Tool
		entry.Message = e.Description
		if entry.Message == "" {
			entry.Message = e.Tool + " (subagent)"
		}
	case "claude: action_started", "codex: action_started":
		entry.Event = "action"
		entry.Tool = e.Tool
		entry.Message = e.Tool + "…"
		if e.Description != "" {
			entry.Message = e.Tool + " — " + e.Description + "…"
		}
	case "claude: action_detail", "codex: action_detail":
		entry.Event = "action"
		entry.Tool = e.Tool
		entry.Detail = buildDetailJSON(e.Status, e.ExitCode, e.OutputSize)
		entry.Message = e.Tool + " completed"
		if e.ExitCode != "" && e.ExitCode != "0" {
			entry.Message = e.Tool + " failed (exit:" + e.ExitCode + ")"
		}
	case "claude: action", "codex: action":
		entry.Event = "action"
		entry.Tool = e.Tool
		entry.Message = e.Tool
		if e.Description != "" {
			entry.Message = e.Tool + " — " + e.Description
		}
	case "claude: todo", "codex: todo":
		entry.Event = "action"
		entry.Tool = "TodoWrite"
		task := e.Task
		if task == "" {
			task = e.Msg
		}
		entry.Message = "☐ " + task
	case "worker: pr_opened":
		entry.Event = "pr"
		entry.Message = "✓ PR opened: " + e.URL
	case "worker: turn_summary":
		entry.Event = "turn"
		entry.Message = e.Summary
	case "worker: turn failed":
		entry.Event = "error"
		if e.Detail != "" {
			entry.Message = e.Detail
		} else {
			entry.Message = "turn failed"
		}
	default:
		switch e.Level {
		case "ERROR":
			entry.Event = "error"
			entry.Message = e.Msg
		case "WARN":
			entry.Event = "warn"
			entry.Message = e.Msg
		default:
			entry.Event = "info"
			entry.Message = e.Msg
		}
	}

	return entry, false
}

// handleSubLogs returns parsed session log entries from CLAUDE_CODE_LOG_DIR files.
// This endpoint reads .jsonl stream-json files written by Claude Code when
// CLAUDE_CODE_LOG_DIR is set, covering all subagents spawned during the session.
// Returns an empty array when no logs exist (not an error).
// GET /api/v1/issues/{identifier}/sublogs
func (s *Server) handleSubLogs(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	entries, err := s.client.FetchSubLogs(identifier)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
		return
	}
	if entries == nil {
		entries = []domain.IssueLogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// handleAIReview dispatches a reviewer worker for the given issue identifier.
// POST /api/v1/issues/{identifier}/ai-review
func (s *Server) handleAIReview(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if err := s.client.DispatchReviewer(identifier); err != nil {
		writeError(w, http.StatusInternalServerError, "dispatch_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"queued":     true,
		"identifier": identifier,
	})
}

// handleListProjects returns all projects visible to the API key.
// Only available when a ProjectManager (Linear) is configured; returns 501 otherwise.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if s.projectManager == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "project listing is only available for Linear")
		return
	}
	projects, err := s.projectManager.FetchProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

// handleGetProjectFilter returns the current runtime project filter.
func (s *Server) handleGetProjectFilter(w http.ResponseWriter, r *http.Request) {
	if s.projectManager == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "project filter is only available for Linear")
		return
	}
	slugs := s.projectManager.GetProjectFilter()
	writeJSON(w, http.StatusOK, map[string]any{"filter": slugs})
}

// handleSetProjectFilter replaces the runtime project filter.
// Body: {"slugs": ["<slug>", ...]}  — empty array = all issues, omit/null = reset to WORKFLOW.md default.
func (s *Server) handleSetProjectFilter(w http.ResponseWriter, r *http.Request) {
	if s.projectManager == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "project filter is only available for Linear")
		return
	}
	var body struct {
		Slugs *[]string `json:"slugs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "expected JSON with optional 'slugs' array")
		return
	}
	if body.Slugs == nil {
		s.projectManager.SetProjectFilter(nil) // reset to WORKFLOW.md default
	} else {
		s.projectManager.SetProjectFilter(*body.Slugs)
	}
	filter := s.projectManager.GetProjectFilter()
	writeJSON(w, http.StatusOK, map[string]any{"filter": filter, "ok": true})
}

// handleUpdateIssueState transitions an issue to a new state in the upstream tracker.
func (s *Server) handleUpdateIssueState(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	var body struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.State == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "state field required")
		return
	}
	if err := s.client.UpdateIssueState(r.Context(), identifier, body.State); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	// Trigger an immediate re-poll so the orchestrator picks up the new state
	// without waiting for the next polling_interval_ms tick.
	select {
	case s.refreshChan <- struct{}{}:
	default:
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "identifier": identifier, "state": body.State})
}

// handleSetIssueProfile sets (or clears) the per-issue agent profile override.
// POST /api/v1/issues/{identifier}/profile
// Body: {"profile": "fast"} to set; {"profile": ""} to reset to default.
func (s *Server) handleSetIssueProfile(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	var body struct {
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	s.client.SetIssueProfile(identifier, body.Profile) // empty string = reset to default
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "identifier": identifier, "profile": body.Profile})
}

// handleSetIssueBackend sets (or clears) the per-issue backend override.
// POST /api/v1/issues/{identifier}/backend
// Body: {"backend": "codex"} to set; {"backend": ""} to reset to default.
func (s *Server) handleSetIssueBackend(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	var body struct {
		Backend string `json:"backend"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	s.client.SetIssueBackend(identifier, body.Backend)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "identifier": identifier, "backend": body.Backend})
}

func (s *Server) handleProvideInput(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if body.Message == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "message is required")
		return
	}
	if ok := s.client.ProvideInput(identifier, body.Message); !ok {
		writeError(w, http.StatusNotFound, "not_found", "issue not in input-required state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDismissInput(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if ok := s.client.DismissInput(identifier); !ok {
		writeError(w, http.StatusNotFound, "not_found", "issue not in input-required state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetInlineInput(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if err := s.client.SetInlineInput(body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleSetWorkers updates the max concurrent agents at runtime.
// POST /api/v1/settings/workers
// Body: {"workers": 5} for absolute, {"delta": 1} or {"delta": -1} for relative.
func (s *Server) handleSetWorkers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Workers int `json:"workers"`
		Delta   int `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	var target int
	if body.Workers > 0 {
		// Absolute set: clamp and apply directly.
		target = max(1, min(body.Workers, 50))
		s.client.SetWorkers(target)
	} else {
		// Relative delta: use BumpMaxWorkers for an atomic read-modify-write.
		target = s.client.BumpWorkers(body.Delta)
	}
	writeJSON(w, http.StatusOK, map[string]any{"workers": target})
}

// handleListProfiles returns the current profile definitions.
// GET /api/v1/settings/profiles
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	defs := s.client.ProfileDefs()
	writeJSON(w, http.StatusOK, map[string]any{"profiles": defs})
}

// handleListModels returns available models from the WORKFLOW.md config.
// GET /api/v1/settings/models
// handleGetReviewer returns the reviewer configuration.
// GET /api/v1/settings/reviewer
func (s *Server) handleGetReviewer(w http.ResponseWriter, _ *http.Request) {
	profile, autoReview := s.client.ReviewerConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"profile":     profile,
		"auto_review": autoReview,
	})
}

// handleSetReviewer updates the reviewer configuration.
// PUT /api/v1/settings/reviewer
// Body: {"profile": "reviewer", "auto_review": true}
func (s *Server) handleSetReviewer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Profile    string `json:"profile"`
		AutoReview bool   `json:"auto_review"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := s.client.SetReviewerConfig(body.Profile, body.AutoReview); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListModels(w http.ResponseWriter, _ *http.Request) {
	models := s.client.AvailableModels()
	if models == nil {
		models = make(map[string][]ModelOption)
	}
	writeJSON(w, http.StatusOK, models)
}

// handleUpsertProfile creates or updates a named agent profile.
// PUT /api/v1/settings/profiles/{name}
// Body: {"command": "claude --model ...", "prompt": "...", "backend": "codex"}
func (s *Server) handleUpsertProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var body struct {
		Command string `json:"command"`
		Prompt  string `json:"prompt"`
		Backend string `json:"backend"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Command == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "command field required")
		return
	}
	def := ProfileDef{
		Command: body.Command,
		Prompt:  body.Prompt,
		Backend: body.Backend,
	}
	if err := s.client.UpsertProfile(name, def); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleDeleteProfile removes a named agent profile.
// DELETE /api/v1/settings/profiles/{name}
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.client.DeleteProfile(name); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleSetAgentMode sets the agent collaboration mode.
// POST /api/v1/settings/agent-mode
// Body: {"mode": "" | "subagents" | "teams"}
func (s *Server) handleSetAgentMode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "mode field required")
		return
	}
	if body.Mode != "" && body.Mode != "teams" && body.Mode != "subagents" {
		writeError(w, http.StatusBadRequest, "invalid_mode", `mode must be "", "subagents", or "teams"`)
		return
	}
	if err := s.client.SetAgentMode(body.Mode); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "agentMode": body.Mode})
}

// handleClearAllWorkspaces removes all workspace directories under workspace.root.
// Responds 202 immediately and performs deletion in a background goroutine so
// the UI does not hang on large workspace trees.
// DELETE /api/v1/workspaces
func (s *Server) handleClearAllWorkspaces(w http.ResponseWriter, r *http.Request) {
	go func() {
		if err := s.client.ClearAllWorkspaces(); err != nil {
			slog.Error("clear all workspaces failed", "error", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// handleSetAutoClearWorkspace toggles automatic workspace cleanup after task success.
// POST /api/v1/settings/workspace/auto-clear
// Body: {"enabled": true|false}
func (s *Server) handleSetAutoClearWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if body.Enabled == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "enabled field is required")
		return
	}
	if err := s.client.SetAutoClearWorkspace(*body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "autoClearWorkspace": *body.Enabled})
}

// handleUpdateTrackerStates updates active/terminal/completion states in-memory and in WORKFLOW.md.
// PUT /api/v1/settings/tracker/states
// Body: {"activeStates": [...], "terminalStates": [...], "completionState": "..."}
func (s *Server) handleUpdateTrackerStates(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ActiveStates    []string `json:"activeStates"`
		TerminalStates  []string `json:"terminalStates"`
		CompletionState string   `json:"completionState"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if len(body.ActiveStates) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "activeStates must not be empty")
		return
	}
	if err := s.client.UpdateTrackerStates(body.ActiveStates, body.TerminalStates, body.CompletionState); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAddSSHHost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host        string `json:"host"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if strings.TrimSpace(body.Host) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "host is required")
		return
	}
	if err := s.client.AddSSHHost(strings.TrimSpace(body.Host), body.Description); err != nil {
		writeError(w, http.StatusInternalServerError, "add_ssh_host_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRemoveSSHHost(w http.ResponseWriter, r *http.Request) {
	// chi v5 extracts params from RawPath when set, so the value may still be
	// percent-encoded (e.g. "user%40host" instead of "user@host"). Decode before
	// comparing against the stored host string.
	host, err := url.PathUnescape(chi.URLParam(r, "host"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_host", "malformed host encoding in URL")
		return
	}
	if err := s.client.RemoveSSHHost(host); err != nil {
		writeError(w, http.StatusInternalServerError, "remove_ssh_host_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetDispatchStrategy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Strategy string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	switch body.Strategy {
	case "round-robin", "least-loaded":
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "strategy must be round-robin or least-loaded")
		return
	}
	if err := s.client.SetDispatchStrategy(body.Strategy); err != nil {
		writeError(w, http.StatusInternalServerError, "set_strategy_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("writeJSON: marshal failed", "type", fmt.Sprintf("%T", v), "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
