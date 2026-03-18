package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeRunner spawns a real claude subprocess and streams its output.
type ClaudeRunner struct{}

// NewClaudeRunner constructs a ClaudeRunner.
func NewClaudeRunner() *ClaudeRunner {
	return &ClaudeRunner{}
}

// RunTurn runs a single claude turn as a subprocess.
//
// First turn (sessionID == nil): claude -p <prompt> --output-format stream-json
// Continuation (sessionID != nil): claude --resume <sessionID> --output-format stream-json
//
// readTimeoutMs is the per-line idle deadline; if no output arrives within that
// window the turn is aborted as a stall. turnTimeoutMs is the hard wall-clock
// limit for the entire turn.
func (c *ClaudeRunner) RunTurn(
	ctx context.Context,
	log Logger,
	onProgress func(TurnResult),
	sessionID *string,
	prompt, workspacePath, command, workerHost string,
	readTimeoutMs, turnTimeoutMs int,
) (TurnResult, error) {
	turnCtx, cancel := context.WithTimeout(ctx, time.Duration(turnTimeoutMs)*time.Millisecond)
	defer cancel()

	var cmd *exec.Cmd
	if workerHost != "" {
		// Remote execution: SSH to host and run command in a login shell.
		// The workspace path is expected to exist on the remote host (e.g. NFS share).
		shellCmd := buildShellCmd(command, sessionID, prompt)
		if workspacePath != "" {
			shellCmd = "cd " + shellQuote(workspacePath) + " && " + shellCmd
		}
		cmd = exec.CommandContext(turnCtx, "ssh",
			"-T",
			"-o", "BatchMode=yes",
			workerHost,
			"bash", "-lc", shellCmd,
		)
	} else if filepath.IsAbs(command) && !strings.Contains(command, " ") {
		// Clean absolute path with no flags — run the binary directly, no shell needed.
		cmd = exec.CommandContext(turnCtx, command, buildDirectArgs(sessionID, prompt)...)
	} else {
		// Bare name — wrap in login shell so PATH is resolved at runtime.
		cmd = exec.CommandContext(turnCtx, loginShell(), "-lc", buildShellCmd(command, sessionID, prompt))
	}
	if workspacePath != "" && workerHost == "" {
		cmd.Dir = workspacePath
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("agent: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("agent: start: %w", err)
	}

	result, readErr := readLines(turnCtx, log, onProgress, stdout, readTimeoutMs)

	// Wait regardless of readErr so we don't leave zombie processes.
	if waitErr := cmd.Wait(); waitErr != nil && readErr == nil {
		result.Failed = true
	}

	// Attach stderr to FailureText when it adds information.
	if stderr := strings.TrimSpace(stderrBuf.String()); stderr != "" && result.Failed {
		if result.FailureText != "" {
			result.FailureText = result.FailureText + " | stderr: " + stderr
		} else {
			result.FailureText = stderr
		}
	}

	if readErr != nil {
		result.Failed = true
		return result, readErr
	}
	result.TotalTokens = result.InputTokens + result.OutputTokens
	return result, nil
}

// sharedFlags are the CLI flags used by every claude invocation regardless of
// execution mode (direct binary or shell). Centralised here so adding a new
// flag only requires one edit.
const sharedFlagsStr = " --output-format stream-json --verbose --dangerously-skip-permissions"

var sharedFlagsSlice = []string{"--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"}

// buildDirectArgs returns CLI args for direct (non-shell) invocation.
func buildDirectArgs(sessionID *string, prompt string) []string {
	base := append([]string{}, sharedFlagsSlice...)
	if sessionID != nil && *sessionID != "" {
		return append(base, "--resume", *sessionID)
	}
	return append(base, "-p", prompt)
}

// buildShellCmd returns the full shell command string for bash/zsh -lc.
// The prompt is passed via a shell variable to avoid quoting issues with
// special characters (backticks, $, !, quotes) in the rendered template.
func buildShellCmd(command string, sessionID *string, prompt string) string {
	base := command + sharedFlagsStr
	if sessionID != nil && *sessionID != "" {
		return base + " --resume " + shellQuote(*sessionID)
	}
	return base + " -p " + shellQuote(prompt)
}

// todoItems parses a TodoWrite input and returns the content of each todo.
func todoItems(rawInput json.RawMessage) []string {
	if len(rawInput) == 0 {
		return nil
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return nil
	}
	v, ok := input["todos"]
	if !ok {
		return nil
	}
	var todos []struct {
		Content string `json:"content"`
	}
	if json.Unmarshal(v, &todos) != nil {
		return nil
	}
	items := make([]string, 0, len(todos))
	for _, t := range todos {
		if t.Content != "" {
			items = append(items, t.Content)
		}
	}
	return items
}

// toolDescription extracts a short human-readable summary from a tool's JSON input,
// making log lines informative — e.g. "Glob — *.go in src/" instead of just "Glob".
func toolDescription(name string, rawInput json.RawMessage) string {
	if len(rawInput) == 0 {
		return ""
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ""
	}
	str := func(key string) string {
		v, ok := input[key]
		if !ok {
			return ""
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return ""
		}
		return s
	}
	trunc := func(s string, n int) string {
		if len([]rune(s)) <= n {
			return s
		}
		return string([]rune(s)[:n]) + "…"
	}
	switch strings.ToLower(name) {
	case "bash":
		return trunc(str("command"), 600)
	case "read":
		return str("file_path")
	case "write":
		return str("file_path")
	case "edit", "multiedit":
		return str("file_path")
	case "glob":
		p := str("pattern")
		if d := str("path"); d != "" {
			return p + " in " + d
		}
		return p
	case "grep":
		p := str("pattern")
		if d := str("path"); d != "" {
			return p + " in " + d
		}
		return p
	case "agent", "task":
		if d := str("description"); d != "" {
			return trunc(d, 300)
		}
		return trunc(str("prompt"), 200)
	case "webfetch":
		return trunc(str("url"), 200)
	case "websearch":
		return trunc(str("query"), 200)
	case "todowrite":
		var todos []struct {
			Content string `json:"content"`
		}
		if v, ok := input["todos"]; ok {
			if json.Unmarshal(v, &todos) == nil && len(todos) > 0 {
				if len(todos) == 1 {
					return trunc(todos[0].Content, 100)
				}
				return fmt.Sprintf("%d tasks: %s", len(todos), trunc(todos[0].Content, 60))
			}
		}
		return ""
	case "todoread":
		return ""
	default:
		// Fall back to first non-empty string field value.
		for _, v := range input {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				return trunc(s, 120)
			}
		}
		return ""
	}
}

// loginShell returns the user's login shell from $SHELL, falling back to bash.
func loginShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "bash"
}

// shellQuote wraps s in single quotes, escaping any single quotes within it.
// This is the POSIX-safe way to pass arbitrary strings to bash -c.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// readLines reads stream-json lines from r, accumulating a TurnResult.
// log is used for INFO-level output (assistant messages, turn result) so that
// Claude's live activity appears in the log stream with the caller's context.
// Returns on EOF, context cancellation, or readTimeoutMs idle expiry.
func readLines(ctx context.Context, log Logger, onProgress func(TurnResult), r io.Reader, readTimeoutMs int) (TurnResult, error) {
	type scanResult struct {
		line []byte
		err  error
		done bool
	}
	lineCh := make(chan scanResult, 1)

	// This goroutine exits when the stdout pipe is closed. When the turn
	// context is cancelled, exec.CommandContext sends SIGKILL to the subprocess,
	// which closes the pipe and unblocks scanner.Scan — no explicit goroutine
	// teardown is needed here.
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max line to handle large prompts
		for scanner.Scan() {
			b := make([]byte, len(scanner.Bytes()))
			copy(b, scanner.Bytes())
			lineCh <- scanResult{line: b}
		}
		lineCh <- scanResult{done: true, err: scanner.Err()}
	}()

	readDeadline := time.Duration(readTimeoutMs) * time.Millisecond
	var result TurnResult

	for {
		timer := time.NewTimer(readDeadline)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, ctx.Err()

		case <-timer.C:
			return result, fmt.Errorf("agent: read timeout after %dms idle", readTimeoutMs)

		case sr := <-lineCh:
			timer.Stop()
			if sr.done {
				return result, sr.err
			}
			ev, err := ParseLine(sr.line)
			if err != nil {
				slog.Debug("agent: raw line", "data", string(sr.line))
				continue
			}
			switch ev.Type {
			case "assistant":
				for _, text := range ev.TextBlocks {
					log.Info("claude: text", "session_id", ev.SessionID, "text", text)
				}
				for _, tc := range ev.ToolCalls {
					desc := toolDescription(tc.Name, tc.Input)
					nameLower := strings.ToLower(tc.Name)
					if nameLower == "agent" || nameLower == "task" {
						// Subagent launch — log separately so TUI/web can highlight it.
						log.Info("claude: subagent", "session_id", ev.SessionID, "tool", tc.Name, "description", desc)
					} else {
						log.Info("claude: action", "session_id", ev.SessionID, "tool", tc.Name, "description", desc)
					}
					// For TodoWrite, emit one log line per task so the full list is visible.
					if nameLower == "todowrite" {
						for _, item := range todoItems(tc.Input) {
							log.Info("claude: todo", "session_id", ev.SessionID, "task", item)
						}
					}
					log.Debug("claude: tool_input", "session_id", ev.SessionID, "tool", tc.Name, "input", string(tc.Input))
				}
			case "result":
				if ev.IsError {
					log.Warn("claude: result error", "session_id", ev.SessionID, "text", ev.ResultText)
				} else {
					log.Info("claude: turn done", "session_id", ev.SessionID,
						"input_tokens", ev.Usage.InputTokens, "output_tokens", ev.Usage.OutputTokens)
				}
			case "system":
				log.Info("claude: session started", "session_id", ev.SessionID)
			}
			result = ApplyEvent(result, ev)
			if onProgress != nil && (ev.Type == "assistant" || ev.Type == "system") {
				onProgress(result)
			}
		}
	}
}
