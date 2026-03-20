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

// GeminiRunner spawns a Google Gemini CLI subprocess.
// Gemini CLI is invoked as: gemini -p "<prompt>"
// Output is captured as plain text (no stream-json support).
type GeminiRunner struct{}

// NewGeminiRunner constructs a GeminiRunner.
func NewGeminiRunner() *GeminiRunner {
	return &GeminiRunner{}
}

// RunTurn runs a single gemini turn as a subprocess.
// The command parameter allows overriding the binary name (default "gemini").
func (g *GeminiRunner) RunTurn(
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
		command = "gemini"
	}

	// Gemini CLI doesn't support session resumption natively.
	effectivePrompt := prompt
	if sessionID != nil && *sessionID != "" {
		effectivePrompt = "Continue working on the previous task. " + prompt
	}

	shellCmd := buildGeminiShellCmd(command, effectivePrompt)
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

	cmd.Env = append(os.Environ(), "GEMINI_PROMPT="+effectivePrompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("gemini: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	log.Info("gemini: session started", "command", command)

	if err := cmd.Start(); err != nil {
		return TurnResult{Failed: true}, fmt.Errorf("gemini: start: %w", err)
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
			log.Info("gemini: text", "text", line)
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

	log.Info("gemini: turn done")
	return result, nil
}

// buildGeminiShellCmd builds the shell command for gemini invocation.
func buildGeminiShellCmd(command, prompt string) string {
	return command + " --prompt " + shellQuote(prompt)
}
