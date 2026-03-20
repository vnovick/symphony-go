package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodexRunner spawns an OpenAI Codex CLI subprocess (codex).
// Codex does not emit stream-json, so output is captured as plain text.
type CodexRunner struct{}

// NewCodexRunner constructs a CodexRunner.
func NewCodexRunner() *CodexRunner {
	return &CodexRunner{}
}

// RunTurn runs a single codex turn as a subprocess.
// Codex is invoked as: codex --approval-mode full-auto --quiet "<prompt>"
// The command parameter allows overriding the binary name (default "codex").
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

	if command == "" || command == "claude" {
		command = "codex"
	}

	// Codex doesn't support --resume; every turn is a fresh invocation.
	// If sessionID is set, we include context about continuation in the prompt.
	effectivePrompt := prompt
	if sessionID != nil && *sessionID != "" {
		effectivePrompt = "Continue working on the previous task. " + prompt
	}

	shellCmd := buildCodexShellCmd(command, effectivePrompt)
	var cmd *exec.Cmd
	if workerHost != "" {
		if workspacePath != "" {
			shellCmd = "cd " + shellQuote(workspacePath) + " && " + shellCmd
		}
		cmd = exec.CommandContext(turnCtx, "ssh",
			"-T", "-o", "BatchMode=yes",
			workerHost,
			"bash", "-lc", shellCmd,
		)
	} else if filepath.IsAbs(command) && !strings.Contains(command, " ") {
		args := []string{"--approval-mode", "full-auto", "--quiet", effectivePrompt}
		cmd = exec.CommandContext(turnCtx, command, args...)
	} else {
		cmd = exec.CommandContext(turnCtx, loginShell(), "-lc", shellCmd)
	}
	if workspacePath != "" && workerHost == "" {
		cmd.Dir = workspacePath
	}

	// Pass prompt via environment to avoid shell quoting issues.
	cmd.Env = append(os.Environ(), "CODEX_PROMPT="+effectivePrompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("codex: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	log.Info("codex: session started", "command", command)

	if err := cmd.Start(); err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("codex: start: %w", err)
	}

	// Read stdout line-by-line with idle timeout.
	var result TurnResult
	var textBuf strings.Builder

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	readDeadline := time.Duration(readTimeoutMs) * time.Millisecond

	lineCh := make(chan string, 1)
	doneCh := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
		doneCh <- scanner.Err()
	}()

loop:
	for {
		timer := time.NewTimer(readDeadline)
		select {
		case <-turnCtx.Done():
			timer.Stop()
			result.Failed = true
			break loop
		case <-timer.C:
			result.Failed = true
			break loop
		case line := <-lineCh:
			timer.Stop()
			textBuf.WriteString(line)
			textBuf.WriteString("\n")
			log.Info("codex: text", "text", line)
			result.LastText = line
			if onProgress != nil {
				onProgress(result)
			}
		case scanErr := <-doneCh:
			timer.Stop()
			if scanErr != nil {
				result.Failed = true
			}
			break loop
		}
	}

	if waitErr := cmd.Wait(); waitErr != nil && !result.Failed {
		result.Failed = true
	}

	if stderr := strings.TrimSpace(stderrBuf.String()); stderr != "" && result.Failed {
		if result.FailureText != "" {
			result.FailureText = result.FailureText + " | stderr: " + stderr
		} else {
			result.FailureText = stderr
		}
	}

	fullText := strings.TrimSpace(textBuf.String())
	if fullText != "" {
		result.AllTextBlocks = []string{fullText}
		result.LastText = fullText
		result.ResultText = fullText
	}

	log.Info("codex: turn done")
	return result, nil
}

// buildCodexShellCmd builds the shell command for codex invocation.
func buildCodexShellCmd(command, prompt string) string {
	return command + " --approval-mode full-auto --quiet " + shellQuote(prompt)
}
