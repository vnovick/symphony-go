package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodexRunner spawns a codex subprocess and streams its --json output.
type CodexRunner struct{}

// NewCodexRunner constructs a CodexRunner.
func NewCodexRunner() *CodexRunner {
	return &CodexRunner{}
}

// ValidateCodexCLI checks if the codex CLI is available and returns an error
// describing the problem if it cannot be found or executed.
func ValidateCodexCLI() error {
	return validateCLI("codex", "ensure 'codex' is installed and on PATH, or set OPENAI_API_KEY")
}

// RunTurn runs a single codex turn as a subprocess.
//
// Fresh turn (sessionID == nil):
//
//	codex [-C <workspace>] exec --json --dangerously-bypass-approvals-and-sandbox <prompt>
//
// Continuation (sessionID != nil):
//
//	codex [-C <workspace>] exec resume --json --dangerously-bypass-approvals-and-sandbox <sessionID> <prompt>
func (c *CodexRunner) RunTurn(
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
		shellCmd := buildCodexShellCmd(command, sessionID, prompt, workspacePath)
		if workspacePath != "" {
			shellCmd = "cd " + shellQuote(workspacePath) + " && " + shellCmd
		}
		cmd = exec.CommandContext(turnCtx, "ssh",
			"-t",
			"-o", "StrictHostKeyChecking=no",
			"-o", "BatchMode=yes",
			workerHost,
			"bash", "-lc", shellCmd,
		)
	} else if filepath.IsAbs(command) && !strings.Contains(command, " ") {
		cmd = exec.CommandContext(turnCtx, command, buildCodexDirectArgs(sessionID, prompt, workspacePath)...)
	} else {
		cmd = exec.CommandContext(turnCtx, loginShell(), "-lc", buildCodexShellCmd(command, sessionID, prompt, workspacePath))
	}
	if workspacePath != "" && workerHost == "" {
		cmd.Dir = workspacePath
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("codex: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("codex: start: %w", err)
	}

	result, readErr := readLines(turnCtx, log, onProgress, stdout, readTimeoutMs, "codex", ParseCodexLine)

	if waitErr := cmd.Wait(); waitErr != nil && readErr == nil {
		result.Failed = true
	}

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

func buildCodexDirectArgs(sessionID *string, prompt, workspacePath string) []string {
	args := make([]string, 0, 8)
	if workspacePath != "" {
		args = append(args, "-C", workspacePath)
	}
	args = append(args, "exec")
	if sessionID != nil && *sessionID != "" {
		args = append(args, "resume", "--json", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", *sessionID, prompt)
		return args
	}
	args = append(args, "--json", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", prompt)
	return args
}

func buildCodexShellCmd(command string, sessionID *string, prompt, workspacePath string) string {
	var b strings.Builder
	b.WriteString(command)
	if workspacePath != "" {
		b.WriteString(" -C ")
		b.WriteString(shellQuote(workspacePath))
	}
	b.WriteString(" exec")
	if sessionID != nil && *sessionID != "" {
		b.WriteString(" resume --json --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check ")
		b.WriteString(shellQuote(*sessionID))
		b.WriteString(" ")
		b.WriteString(shellQuote(prompt))
		return b.String()
	}
	b.WriteString(" --json --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check ")
	b.WriteString(shellQuote(prompt))
	return b.String()
}
