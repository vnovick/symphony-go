package agent

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vnovick/symphony-go/internal/domain"
)

const maxSubLogLines = 5000

// SublogFetcher retrieves parsed session log entries for one issue.
// The dir argument is the per-issue log directory on whichever host holds the files.
// Implementations: LocalSublogFetcher (disk), SSHSublogFetcher (remote tar-over-SSH),
// DockerSublogFetcher (planned — docker cp / exec cat).
type SublogFetcher interface {
	FetchSubLogs(ctx context.Context, dir string) ([]domain.IssueLogEntry, error)
}

// LocalSublogFetcher reads session logs from the local filesystem.
type LocalSublogFetcher struct{}

func (LocalSublogFetcher) FetchSubLogs(_ context.Context, dir string) ([]domain.IssueLogEntry, error) {
	return ParseSessionLogsMulti(dir)
}

// SSHSublogFetcher fetches session logs from a remote host over SSH.
type SSHSublogFetcher struct{ Host string }

func (s SSHSublogFetcher) FetchSubLogs(_ context.Context, dir string) ([]domain.IssueLogEntry, error) {
	return SSHFetchLogs(s.Host, dir)
}

// ParseSessionLogs reads all *.jsonl files in dir, parses each line as a
// Claude Code stream-json event, and returns the converted IssueLogEntry slice.
// Returns nil (not an error) when dir does not exist or contains no files.
func ParseSessionLogs(dir string) ([]domain.IssueLogEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("logtailer: read dir %s: %w", dir, err)
	}

	var all []domain.IssueLogEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		lines, err := readJSONLFile(path)
		if err != nil {
			continue // skip unreadable files
		}
		all = append(all, lines...)
	}
	if len(all) > maxSubLogLines {
		all = all[len(all)-maxSubLogLines:]
	}
	return all, nil
}

// SSHFetchLogs fetches and parses session logs from a remote host using
// short-lived ssh exec calls. Returns nil when the remote directory is absent.
// Files named "codex-session.jsonl" are parsed with ParseCodexLine; all other
// .jsonl files are parsed with ParseLine (Claude Code stream-json format).
//
// Session IDs are derived from filenames so each log entry is stamped with the
// session that produced it — matching the behaviour of the local ParseSessionLogs
// path and enabling per-run log isolation in the Timeline view.
func SSHFetchLogs(host, dir string) ([]domain.IssueLogEntry, error) {
	claudeEntries := sshFetchClaude(host, dir)
	codexEntries := sshFetchWithParserMulti(host, dir,
		`cat %s/codex-session.jsonl 2>/dev/null || true`,
		ParseCodexLine,
	)

	all := append(claudeEntries, codexEntries...)
	if len(all) > maxSubLogLines {
		all = all[len(all)-maxSubLogLines:]
	}
	return all, nil
}

// sshFetchClaude fetches all Claude .jsonl session files from dir on host in a
// single SSH connection. The remote produces a tar archive; the Go client reads
// it with archive/tar so session IDs come from tar header filenames — the same
// source as the local readJSONLFile path. No sentinel string parsing required.
//
// Remote requirement: tar must be available on the SSH host (standard on Linux/macOS).
func sshFetchClaude(host, dir string) []domain.IssueLogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Collect matching files into a variable first so we can skip the tar call
	// entirely when none exist (tar with zero arguments is an error on some platforms).
	script := `files=$(find ` + shellQuote(dir) + ` -maxdepth 1 -name '*.jsonl' ! -name 'codex-session.jsonl' 2>/dev/null | sort); ` +
		`[ -n "$files" ] && tar -cf - -C ` + shellQuote(dir) + ` $(basename -a $files) 2>/dev/null || true`

	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		host,
		"bash", "-c", script,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	defer func() { _ = cmd.Wait() }()

	var entries []domain.IssueLogEntry
	tr := tar.NewReader(stdout)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // truncated archive or SSH error — return what we have
		}
		sessionID := strings.TrimSuffix(filepath.Base(hdr.Name), ".jsonl")
		scanner := bufio.NewScanner(tr)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			entries = append(entries, streamLineToEntriesWith(scanner.Bytes(), ParseLine, sessionID)...)
		}
	}
	return entries
}

// sshFetchWithParserMulti runs a shell script on host and converts each output line
// using parseFn (which may produce multiple entries per line).
func sshFetchWithParserMulti(host, dir, scriptFmt string, parseFn func([]byte) (StreamEvent, error)) []domain.IssueLogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	script := fmt.Sprintf(scriptFmt, shellQuote(dir))
	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		host,
		"bash", "-c", script,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	var entries []domain.IssueLogEntry
	scanner := bufio.NewScanner(&out)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		entries = append(entries, streamLineToEntriesWith(scanner.Bytes(), parseFn, "")...)
	}
	return entries
}

// readJSONLFile reads a single .jsonl file and converts each line.
// The session ID is derived from the filename (without .jsonl extension).
func readJSONLFile(path string) ([]domain.IssueLogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	var entries []domain.IssueLogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		entry, ok := streamLineToEntry(scanner.Bytes(), sessionID)
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, scanner.Err()
}

// streamLineToEntry converts one stream-json line to an IssueLogEntry.
// sessionID is stamped on every returned entry.
// Returns (entry, false) when the line should be skipped.
func streamLineToEntry(line []byte, sessionID string) (domain.IssueLogEntry, bool) {
	ev, err := ParseLine(line)
	if err != nil {
		return domain.IssueLogEntry{}, false
	}

	switch ev.Type {
	case EventAssistant:
		if ev.InProgress {
			return domain.IssueLogEntry{}, false
		}
		var entries []domain.IssueLogEntry
		// Emit one entry per text block.
		for _, text := range ev.TextBlocks {
			if strings.TrimSpace(text) == "" {
				continue
			}
			entries = append(entries, domain.IssueLogEntry{
				Level:     "INFO",
				Event:     "text",
				Message:   text,
				SessionID: sessionID,
			})
		}
		// Emit one action entry per tool call.
		for _, tc := range ev.ToolCalls {
			name := tc.Name
			desc := toolDescription(name, tc.Input)
			msg := name
			if desc != "" {
				msg = name + " — " + desc
			}
			entries = append(entries, domain.IssueLogEntry{
				Level:     "INFO",
				Event:     "action",
				Message:   msg,
				Tool:      name,
				SessionID: sessionID,
			})
		}
		if len(entries) == 0 {
			return domain.IssueLogEntry{}, false
		}
		return entries[0], true // caller only takes one; multiple handled below

	case EventResult:
		if ev.IsError {
			return domain.IssueLogEntry{
				Level:     "ERROR",
				Event:     "error",
				Message:   ev.ResultText,
				SessionID: sessionID,
			}, true
		}
		return domain.IssueLogEntry{}, false

	case EventSystem:
		return domain.IssueLogEntry{}, false // skip session-start metadata

	default:
		return domain.IssueLogEntry{}, false
	}
}

// ParseSessionLogsMulti is like ParseSessionLogs but returns all entries from
// a stream event that produces multiple entries (e.g., a turn with both text
// blocks and tool calls). This is the full-fidelity version used by the API.
// Files named "codex-session.jsonl" are parsed with ParseCodexLine; all other
// .jsonl files are parsed with ParseLine (Claude Code stream-json format).
func ParseSessionLogsMulti(dir string) ([]domain.IssueLogEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("logtailer: read dir %s: %w", dir, err)
	}

	var all []domain.IssueLogEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		parseFn := ParseLine
		if e.Name() == "codex-session.jsonl" {
			parseFn = ParseCodexLine
		}
		lines, err := readJSONLFileMultiWith(path, parseFn)
		if err != nil {
			continue
		}
		all = append(all, lines...)
	}
	if len(all) > maxSubLogLines {
		all = all[len(all)-maxSubLogLines:]
	}
	return all, nil
}

// readJSONLFileMultiWith reads a .jsonl file using the provided parse function and
// converts each line to zero or more IssueLogEntry.
// The session ID is derived from the filename (without .jsonl extension).
func readJSONLFileMultiWith(path string, parseFn func([]byte) (StreamEvent, error)) ([]domain.IssueLogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	var entries []domain.IssueLogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		entries = append(entries, streamLineToEntriesWith(scanner.Bytes(), parseFn, sessionID)...)
	}
	return entries, scanner.Err()
}

// streamLineToEntriesWith converts one JSONL line to zero or more IssueLogEntry using parseFn.
// sessionID is stamped on every returned entry.
// Supports both Claude Code (ParseLine) and Codex (ParseCodexLine) formats since both
// normalize to the same StreamEvent type.
func streamLineToEntriesWith(line []byte, parseFn func([]byte) (StreamEvent, error), sessionID string) []domain.IssueLogEntry {
	ev, err := parseFn(line)
	if err != nil {
		return nil
	}

	switch ev.Type {
	case EventAssistant:
		if ev.InProgress {
			return nil
		}
		var entries []domain.IssueLogEntry
		for _, text := range ev.TextBlocks {
			if strings.TrimSpace(text) == "" {
				continue
			}
			entries = append(entries, domain.IssueLogEntry{
				Level:     "INFO",
				Event:     "text",
				Message:   text,
				SessionID: sessionID,
			})
		}
		for _, tc := range ev.ToolCalls {
			name := tc.Name
			desc := toolDescription(name, tc.Input)
			msg := name
			if desc != "" {
				msg = name + " — " + desc
			}
			entries = append(entries, domain.IssueLogEntry{
				Level:     "INFO",
				Event:     "action",
				Message:   msg,
				Tool:      name,
				SessionID: sessionID,
			})
		}
		return entries

	case EventResult:
		if ev.IsError {
			return []domain.IssueLogEntry{{
				Level:     "ERROR",
				Event:     "error",
				Message:   ev.ResultText,
				SessionID: sessionID,
			}}
		}
		return nil

	default:
		return nil
	}
}
