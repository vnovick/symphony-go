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

// OpenCodeRunner spawns an OpenCode CLI subprocess.
// OpenCode (https://opencode.ai) is a terminal-based AI coding agent.
// This is a stub implementation — the CLI interface may change.
// Invoked as: opencode --prompt "<prompt>"
type OpenCodeRunner struct{}

// NewOpenCodeRunner constructs an OpenCodeRunner.
func NewOpenCodeRunner() *OpenCodeRunner {
	return &OpenCodeRunner{}
}

// RunTurn runs a single opencode turn as a subprocess.
// The command parameter allows overriding the binary name (default "opencode").
func (o *OpenCodeRunner) RunTurn(
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
		command = "opencode"
	}

	effectivePrompt := prompt
	if sessionID != nil && *sessionID != "" {
		effectivePrompt = "Continue working on the previous task. " + prompt
	}

	shellCmd := buildOpenCodeShellCmd(command, effectivePrompt)
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
		args := []string{"--prompt", effectivePrompt}
		cmd = exec.CommandContext(turnCtx, command, args...)
	} else {
		cmd = exec.CommandContext(turnCtx, loginShell(), "-lc", shellCmd)
	}
	if workspacePath != "" && workerHost == "" {
		cmd.Dir = workspacePath
	}

	cmd.Env = append(os.Environ(), "OPENCODE_PROMPT="+effectivePrompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("opencode: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	log.Info("opencode: session started", "command", command)

	if err := cmd.Start(); err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("opencode: start: %w", err)
	}

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
			log.Info("opencode: text", "text", line)
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

	log.Info("opencode: turn done")
	return result, nil
}

// buildOpenCodeShellCmd builds the shell command for opencode invocation.
func buildOpenCodeShellCmd(command, prompt string) string {
	return command + " --prompt " + shellQuote(prompt)
}
