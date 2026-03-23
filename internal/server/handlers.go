package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshot()
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	snap := s.snapshot()
	for _, row := range snap.Running {
		if row.Identifier == identifier {
			writeJSON(w, http.StatusOK, row)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "issue "+identifier+" not found")
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
	if !s.reanalyzeIssue(identifier) {
		writeError(w, http.StatusNotFound, "not_paused", "issue "+identifier+" is not paused")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queued": true, "identifier": identifier})
}

// handleResumeIssue removes a paused issue from the pause set so it can be dispatched again.
func (s *Server) handleResumeIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if s.resumeIssue(identifier) {
		writeJSON(w, http.StatusOK, map[string]any{"resumed": true, "identifier": identifier})
	} else {
		writeError(w, http.StatusNotFound, "not_paused", "issue "+identifier+" is not paused")
	}
}

// handleCancelIssue cancels the running worker for the given issue identifier.
func (s *Server) handleCancelIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if s.cancelIssue(identifier) {
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true, "identifier": identifier})
	} else {
		writeError(w, http.StatusNotFound, "not_running", "issue "+identifier+" is not running")
	}
}

// handleTerminateIssue hard-stops a running or paused issue without adding it to PausedIdentifiers.
func (s *Server) handleTerminateIssue(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if s.terminateIssue(identifier) {
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
	issues, err := s.fetchIssues(r.Context())
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
	issues, err := s.fetchIssues(r.Context())
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

	// Optional identifier filter: only emit lines containing this string.
	filterID := r.URL.Query().Get("identifier")

	flush := func() {
		for {
			n, err := f.Read(readBuf)
			if n > 0 {
				if pending.Len()+n <= maxPending {
					pending.Write(readBuf[:n])
				}
			}
			// Send complete lines.
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
				if filterID != "" && !strings.Contains(line, filterID) {
					continue
				}
				_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", line)
			}
			if err != nil {
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
	lines := s.fetchLogs(identifier)
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

// handleClearIssueLogs deletes the in-memory and on-disk log buffer for an issue.
// DELETE /api/v1/issues/{identifier}/logs
func (s *Server) handleClearIssueLogs(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if err := s.clearLogs(identifier); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// skipLine returns true for internal lifecycle events that are noise in the timeline.
func skipLine(line string) bool {
	return strings.HasPrefix(line, "INFO claude: session started") ||
		strings.HasPrefix(line, "INFO claude: turn done") ||
		strings.HasPrefix(line, "INFO codex: session started") ||
		strings.HasPrefix(line, "INFO codex: turn done") ||
		strings.HasPrefix(line, "DEBUG ")
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

// bufLogEntry mirrors orchestrator.bufLogEntry. Both sides must stay in sync.
// Using a local type avoids an import cycle between server and orchestrator.
type bufLogEntry struct {
	Level       string `json:"level"`
	Msg         string `json:"msg"`
	Time        string `json:"time"`
	Text        string `json:"text,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	ExitCode    string `json:"exit_code,omitempty"`
	OutputSize  string `json:"output_size,omitempty"`
	Task        string `json:"task,omitempty"`
	URL         string `json:"url,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

// parseLogLine converts a JSON log buffer line into a structured IssueLogEntry.
// Returns (entry, false) for valid entries, (zero, true) to signal skip.
func parseLogLine(line string) (IssueLogEntry, bool) {
	var e bufLogEntry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		// Non-JSON line (e.g. legacy entry) — skip rather than panic.
		return IssueLogEntry{}, true
	}

	if skipLine(e.Level + " " + e.Msg) {
		return IssueLogEntry{}, true
	}

	entry := IssueLogEntry{Level: e.Level, Time: e.Time}

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

// handleAIReview dispatches a reviewer worker for the given issue identifier.
// POST /api/v1/issues/{identifier}/ai-review
func (s *Server) handleAIReview(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if err := s.dispatchReviewer(identifier); err != nil {
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
	if err := s.updateIssueState(r.Context(), identifier, body.State); err != nil {
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
	s.setIssueProfile(identifier, body.Profile) // empty string = reset to default
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "identifier": identifier, "profile": body.Profile})
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
	snap := s.snapshot()
	target := snap.MaxConcurrentAgents
	if body.Workers > 0 {
		target = body.Workers
	} else {
		target += body.Delta
	}
	if target < 1 {
		target = 1
	}
	if target > 50 {
		target = 50
	}
	s.setWorkers(target)
	writeJSON(w, http.StatusOK, map[string]any{"workers": target})
}

// handleListProfiles returns the current profile definitions.
// GET /api/v1/settings/profiles
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	defs := s.profileDefs()
	writeJSON(w, http.StatusOK, map[string]any{"profiles": defs})
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
	if err := s.upsertProfile(name, def); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleDeleteProfile removes a named agent profile.
// DELETE /api/v1/settings/profiles/{name}
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.deleteProfile(name); err != nil {
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
	if err := s.setAgentMode(body.Mode); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "agentMode": body.Mode})
}

// handleSetAutoClearWorkspace toggles automatic workspace cleanup after task success.
// POST /api/v1/settings/workspace/auto-clear
// Body: {"enabled": true|false}
func (s *Server) handleSetAutoClearWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "enabled field required")
		return
	}
	if err := s.setAutoClearWorkspace(body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "autoClearWorkspace": body.Enabled})
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
	if err := s.updateTrackerStates(body.ActiveStates, body.TerminalStates, body.CompletionState); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
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
