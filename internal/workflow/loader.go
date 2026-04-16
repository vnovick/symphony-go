package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrorCode identifies the category of a workflow load/parse failure.
type ErrorCode string

// Workflow error code constants returned by Load.
const (
	ErrMissingFile        ErrorCode = "missing_workflow_file"
	ErrParseError         ErrorCode = "workflow_parse_error"
	ErrFrontMatterNotAMap ErrorCode = "workflow_front_matter_not_a_map"
)

// Error is a typed workflow error carrying a code and optional cause.
type Error struct {
	Code  ErrorCode
	Path  string
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Path, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Path)
}

func (e *Error) Unwrap() error { return e.Cause }

// Workflow holds the parsed front matter and prompt template from a WORKFLOW.md file.
type Workflow struct {
	Config         map[string]any
	PromptTemplate string
}

// Load reads and parses a WORKFLOW.md file at the given path.
func Load(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: ErrMissingFile, Path: path, Cause: err}
	}
	return parse(path, string(data))
}

func parse(path, content string) (*Workflow, error) {
	frontLines, promptLines := splitFrontMatter(content)

	config, err := parseFrontMatter(path, frontLines)
	if err != nil {
		return nil, err
	}

	prompt := strings.TrimSpace(strings.Join(promptLines, "\n"))
	return &Workflow{Config: config, PromptTemplate: prompt}, nil
}

// splitFrontMatter splits content on --- delimiters.
// Returns front matter lines and prompt body lines.
func splitFrontMatter(content string) (front []string, body []string) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, lines
	}
	// Skip the opening "---"
	rest := lines[1:]
	for i, line := range rest {
		if line == "---" {
			return rest[:i], rest[i+1:]
		}
	}
	// Opening --- but no closing ---: treat all as front matter, empty body
	return rest, nil
}

func parseFrontMatter(path string, lines []string) (map[string]any, error) {
	if len(lines) == 0 {
		return map[string]any{}, nil
	}
	raw := strings.Join(lines, "\n")
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var decoded any
	if err := yaml.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, &Error{Code: ErrParseError, Path: path, Cause: err}
	}

	switch v := decoded.(type) {
	case map[string]any:
		return v, nil
	case nil:
		return map[string]any{}, nil
	default:
		return nil, &Error{Code: ErrFrontMatterNotAMap, Path: path}
	}
}

// keyLineRE matches a YAML key-value line like "  max_concurrent_agents: 3"
// and captures the leading whitespace, the key, and the value.
var keyLineRE = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_]*)(\s*:\s*)(.*)$`)

// PatchIntField rewrites the first occurrence of `key: <int>` inside the
// YAML front matter of the file at path, replacing the integer value with n.
// The rest of the file (comments, formatting, body) is preserved byte-for-byte.
// Returns an error if the key is not found in the front matter or the file
// cannot be read/written.
func PatchIntField(path, key string, n int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Only patch within the front matter (between the first pair of --- markers).
	frontEnd := -1
	if strings.HasPrefix(content, "---\n") {
		idx := strings.Index(content[4:], "\n---")
		if idx >= 0 {
			frontEnd = 4 + idx + 1 // index of the closing '\n---'
		}
	}
	searchRegion := content
	if frontEnd > 0 {
		searchRegion = content[:frontEnd]
	}

	lines := strings.Split(searchRegion, "\n")
	replaced := false
	for i, line := range lines {
		m := keyLineRE.FindStringSubmatch(line)
		if m == nil || m[2] != key {
			continue
		}
		// Preserve everything except the value; strip inline comment from old value.
		oldVal := m[4]
		comment := ""
		if ci := strings.Index(oldVal, " #"); ci >= 0 {
			comment = " " + strings.TrimSpace(oldVal[ci+1:])
			comment = " #" + comment[2:]
		}
		lines[i] = m[1] + m[2] + m[3] + strconv.Itoa(n) + comment
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("workflow patch: key %q not found in front matter of %s", key, path)
	}

	patched := strings.Join(lines, "\n")
	if frontEnd > 0 {
		patched = patched + content[frontEnd:]
	}
	return os.WriteFile(path, []byte(patched), 0o644)
}

// PatchAgentBoolField sets a boolean key under the agent: block of the YAML front matter.
// If the key already exists it is updated in place; if it does not exist it is appended
// inside the agent: block. Setting enabled=false removes the key entirely.
func PatchAgentBoolField(path, key string, enabled bool) error {
	return patchBlockBoolField(path, "agent", key, enabled)
}

// PatchWorkspaceBoolField sets a boolean key under the workspace: block of the YAML front matter.
// Behaves identically to PatchAgentBoolField but targets the workspace: block.
func PatchWorkspaceBoolField(path, key string, enabled bool) error {
	return patchBlockBoolField(path, "workspace", key, enabled)
}

// patchBlockBoolField is the shared implementation used by PatchAgentBoolField
// and PatchWorkspaceBoolField.
func patchBlockBoolField(path, block, key string, enabled bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch bool: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	frontLines, bodyLines := splitFrontMatter(content)
	if frontLines == nil {
		return fmt.Errorf("workflow patch bool: no front matter in %s", path)
	}

	keyLine := "  " + key + ": "
	keyFound := -1
	blockLine := -1
	// Find the target block header, then search for the key only within that
	// block (i.e. lines after the header that start with two-space indent,
	// stopping at the next top-level key or end of front matter).
	for i, l := range frontLines {
		if l == block+":" {
			blockLine = i
			// Scan forward within this block for the key.
			for j := i + 1; j < len(frontLines); j++ {
				line := frontLines[j]
				// A line with no leading spaces is the start of the next block.
				if len(line) > 0 && line[0] != ' ' {
					break
				}
				if strings.HasPrefix(line, keyLine) {
					keyFound = j
					break
				}
			}
			break
		}
	}

	if keyFound >= 0 {
		if !enabled {
			frontLines = append(frontLines[:keyFound], frontLines[keyFound+1:]...)
		} else {
			frontLines[keyFound] = keyLine + "true"
		}
	} else if enabled {
		insertAt := len(frontLines)
		if blockLine >= 0 {
			insertAt = blockLine + 1
		}
		newLines := make([]string, 0, len(frontLines)+1)
		newLines = append(newLines, frontLines[:insertAt]...)
		newLines = append(newLines, keyLine+"true")
		newLines = append(newLines, frontLines[insertAt:]...)
		frontLines = newLines
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(strings.Join(frontLines, "\n"))
	b.WriteString("\n---\n")
	b.WriteString(strings.Join(bodyLines, "\n"))
	if len(bodyLines) > 0 && bodyLines[len(bodyLines)-1] != "" {
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// PatchAgentStringField sets or removes a string key under the agent: block of the YAML front matter.
// If the key already exists it is updated in place; if value == "" the key is removed.
func PatchAgentStringField(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch string: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	frontLines, bodyLines := splitFrontMatter(content)
	if frontLines == nil {
		return fmt.Errorf("workflow patch string: no front matter in %s", path)
	}

	keyPrefix := "  " + key + ": "
	keyFound := -1
	agentLine := -1
	for i, l := range frontLines {
		if l == "agent:" {
			agentLine = i
		}
		if strings.HasPrefix(l, keyPrefix) {
			keyFound = i
			break
		}
	}

	if keyFound >= 0 {
		if value == "" {
			// Remove the key line.
			frontLines = append(frontLines[:keyFound], frontLines[keyFound+1:]...)
		} else {
			frontLines[keyFound] = keyPrefix + strconv.Quote(value)
		}
	} else if value != "" {
		insertAt := len(frontLines)
		if agentLine >= 0 {
			insertAt = agentLine + 1
		}
		newLines := make([]string, 0, len(frontLines)+1)
		newLines = append(newLines, frontLines[:insertAt]...)
		newLines = append(newLines, keyPrefix+strconv.Quote(value))
		newLines = append(newLines, frontLines[insertAt:]...)
		frontLines = newLines
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(strings.Join(frontLines, "\n"))
	b.WriteString("\n---\n")
	b.WriteString(strings.Join(bodyLines, "\n"))
	if len(bodyLines) > 0 && bodyLines[len(bodyLines)-1] != "" {
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ProfileEntry describes one named agent profile for PatchProfilesBlock.
type ProfileEntry struct {
	// Command is the CLI command string (e.g. "claude --model claude-haiku-4-5-20251001").
	// Any leading "command: " prefix typed by the user is stripped automatically.
	Command string
	// Prompt is an optional role description for this sub-agent, shown to the
	// orchestrating agent when agent teams are enabled.
	Prompt string
	// Backend is an optional explicit runner selection override.
	Backend string
	// Enabled controls whether the profile is selectable and dispatchable.
	// Nil means omit the field from WORKFLOW.md, which defaults to true.
	Enabled *bool
	// AllowedActions is the optional allowlist of daemon-backed agent actions.
	AllowedActions []string
	// CreateIssueState is the target tracker state/column for the create_issue action.
	CreateIssueState string
}

type AutomationTriggerEntry struct {
	Type     string
	Cron     string
	Timezone string
	State    string
}

type AutomationFilterEntry struct {
	MatchMode         string
	States            []string
	LabelsAny         []string
	IdentifierRegex   string
	Limit             int
	InputContextRegex string
}

type AutomationPolicyEntry struct {
	AutoResume bool
}

type AutomationEntry struct {
	ID           string
	Enabled      bool
	Profile      string
	Instructions string
	Trigger      AutomationTriggerEntry
	Filter       AutomationFilterEntry
	Policy       AutomationPolicyEntry
}

// PatchProfilesBlock replaces (or inserts) the agent.profiles block in the YAML
// front matter of the file at path. profiles maps profile name → ProfileEntry.
// Passing nil or an empty map removes the profiles block entirely.
// The rest of the file (other keys, comments, prompt body) is preserved byte-for-byte.
func PatchProfilesBlock(path string, profiles map[string]ProfileEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch profiles: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Split into front matter and body.
	frontLines, bodyLines := splitFrontMatter(content)
	if frontLines == nil {
		return fmt.Errorf("workflow patch profiles: no front matter found in %s", path)
	}

	// Find the profiles block: lines[profilesStart:profilesEnd]
	profilesStart := -1
	profilesEnd := -1
	for i, line := range frontLines {
		if line == "  profiles:" {
			profilesStart = i
			// Scan forward to find the end of the block.
			j := i + 1
			for j < len(frontLines) {
				l := frontLines[j]
				// Blank lines inside the block are included.
				if l == "" {
					j++
					continue
				}
				// Count leading spaces.
				trimmed := strings.TrimLeft(l, " ")
				indent := len(l) - len(trimmed)
				if indent > 2 {
					j++
				} else {
					break
				}
			}
			profilesEnd = j
			break
		}
	}

	// Build replacement lines.
	var replacement []string
	if len(profiles) > 0 {
		replacement = append(replacement, "  profiles:")
		names := make([]string, 0, len(profiles))
		for n := range profiles {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			entry := profiles[name]
			// Strip accidental "command: " prefix users may have typed in the UI.
			cmd := strings.TrimPrefix(entry.Command, "command: ")
			cmd = strings.TrimPrefix(cmd, "command:")
			replacement = append(replacement, "    "+name+":")
			replacement = append(replacement, "      command: "+cmd)
			if entry.Backend != "" {
				replacement = append(replacement, "      backend: "+entry.Backend)
			}
			if entry.Enabled != nil && !*entry.Enabled {
				replacement = append(replacement, "      enabled: false")
			}
			if len(entry.AllowedActions) > 0 {
				replacement = append(replacement, "      allowed_actions:")
				for _, action := range entry.AllowedActions {
					if action == "" {
						continue
					}
					replacement = append(replacement, "        - "+action)
				}
			}
			if entry.CreateIssueState != "" {
				replacement = append(replacement, "      create_issue_state: "+strconv.Quote(entry.CreateIssueState))
			}
			if entry.Prompt != "" {
				replacement = append(replacement, "      prompt: "+strconv.Quote(entry.Prompt))
			}
		}
	}

	var newFrontLines []string
	if profilesStart >= 0 {
		// Block found: splice it out and insert replacement.
		newFrontLines = append(newFrontLines, frontLines[:profilesStart]...)
		newFrontLines = append(newFrontLines, replacement...)
		newFrontLines = append(newFrontLines, frontLines[profilesEnd:]...)
	} else if len(profiles) > 0 {
		// Block not found; find the agent: key and insert after its block.
		agentEnd := len(frontLines)
		agentFound := false
		for i, line := range frontLines {
			if line == "agent:" {
				agentFound = true
				// Find first line after agent: block (indent 0, non-blank, not part of agent block).
				for j := i + 1; j < len(frontLines); j++ {
					l := frontLines[j]
					if l == "" {
						continue
					}
					trimmed := strings.TrimLeft(l, " ")
					indent := len(l) - len(trimmed)
					if indent == 0 {
						agentEnd = j
						break
					}
				}
				break
			}
		}
		if !agentFound {
			// No agent: key; just append profiles at end of front matter.
			newFrontLines = append(frontLines, replacement...)
		} else {
			newFrontLines = append(newFrontLines, frontLines[:agentEnd]...)
			newFrontLines = append(newFrontLines, replacement...)
			newFrontLines = append(newFrontLines, frontLines[agentEnd:]...)
		}
	} else {
		// Block not found and profiles is empty: nothing to do.
		return nil
	}

	// Reassemble file.
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(strings.Join(newFrontLines, "\n"))
	sb.WriteString("\n---")
	if len(bodyLines) > 0 {
		sb.WriteString("\n")
		sb.WriteString(strings.Join(bodyLines, "\n"))
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// PatchAutomationsBlock replaces (or inserts) the top-level automations block in
// the YAML front matter of the file at path. Passing nil or an empty slice
// removes the automations block entirely. Legacy schedules blocks are removed
// when writing automations so the file has a single source of truth.
func PatchAutomationsBlock(path string, automations []AutomationEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch automations: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	frontLines, bodyLines := splitFrontMatter(content)
	if frontLines == nil {
		return fmt.Errorf("workflow patch automations: no front matter found in %s", path)
	}

	automationsStart := -1
	automationsEnd := -1
	legacySchedulesStart := -1
	legacySchedulesEnd := -1
	for i, line := range frontLines {
		if line != "automations:" && line != "schedules:" {
			continue
		}
		if line == "automations:" {
			automationsStart = i
		} else {
			legacySchedulesStart = i
		}
		j := i + 1
		for j < len(frontLines) {
			l := frontLines[j]
			if l == "" {
				j++
				continue
			}
			trimmed := strings.TrimLeft(l, " ")
			indent := len(l) - len(trimmed)
			if indent > 0 {
				j++
			} else {
				break
			}
		}
		if line == "automations:" {
			automationsEnd = j
		} else {
			legacySchedulesEnd = j
		}
	}

	var replacement []string
	if len(automations) > 0 {
		replacement = append(replacement, "automations:")
		for _, automation := range automations {
			replacement = append(replacement, "  - id: "+automation.ID)
			replacement = append(replacement, "    enabled: "+strconv.FormatBool(automation.Enabled))
			replacement = append(replacement, "    profile: "+automation.Profile)
			if automation.Instructions != "" {
				replacement = append(replacement, "    instructions: "+strconv.Quote(automation.Instructions))
			}
			replacement = append(replacement, "    trigger:")
			replacement = append(replacement, "      type: "+automation.Trigger.Type)
			if automation.Trigger.Cron != "" {
				replacement = append(replacement, "      cron: "+strconv.Quote(automation.Trigger.Cron))
			}
			if automation.Trigger.Timezone != "" {
				replacement = append(replacement, "      timezone: "+strconv.Quote(automation.Trigger.Timezone))
			}
			if automation.Trigger.State != "" {
				replacement = append(replacement, "      state: "+strconv.Quote(automation.Trigger.State))
			}
			filterLines := buildAutomationFilterLines(automation.Filter)
			if len(filterLines) > 0 {
				replacement = append(replacement, "    filter:")
				replacement = append(replacement, filterLines...)
			}
			policyLines := buildAutomationPolicyLines(automation.Policy)
			if len(policyLines) > 0 {
				replacement = append(replacement, "    policy:")
				replacement = append(replacement, policyLines...)
			}
		}
	}

	// Remove either existing block if present, preferring the new automations block.
	var newFrontLines []string
	switch {
	case automationsStart >= 0:
		newFrontLines = append(newFrontLines, frontLines[:automationsStart]...)
		newFrontLines = append(newFrontLines, replacement...)
		newFrontLines = append(newFrontLines, frontLines[automationsEnd:]...)
	case legacySchedulesStart >= 0:
		newFrontLines = append(newFrontLines, frontLines[:legacySchedulesStart]...)
		newFrontLines = append(newFrontLines, replacement...)
		newFrontLines = append(newFrontLines, frontLines[legacySchedulesEnd:]...)
	case len(automations) > 0:
		newFrontLines = append(frontLines, replacement...)
	default:
		return nil
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(strings.Join(newFrontLines, "\n"))
	sb.WriteString("\n---\n")
	sb.WriteString(strings.Join(bodyLines, "\n"))
	if len(bodyLines) > 0 && bodyLines[len(bodyLines)-1] != "" {
		sb.WriteString("\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func buildAutomationFilterLines(filter AutomationFilterEntry) []string {
	var lines []string
	if filter.MatchMode != "" && filter.MatchMode != "all" {
		lines = append(lines, "      match_mode: "+strconv.Quote(filter.MatchMode))
	}
	if len(filter.States) > 0 {
		lines = append(lines, "      states: "+marshalStringSliceInline(filter.States))
	}
	if len(filter.LabelsAny) > 0 {
		lines = append(lines, "      labels_any: "+marshalStringSliceInline(filter.LabelsAny))
	}
	if filter.IdentifierRegex != "" {
		lines = append(lines, "      identifier_regex: "+strconv.Quote(filter.IdentifierRegex))
	}
	if filter.Limit > 0 {
		lines = append(lines, "      limit: "+strconv.Itoa(filter.Limit))
	}
	if filter.InputContextRegex != "" {
		lines = append(lines, "      input_context_regex: "+strconv.Quote(filter.InputContextRegex))
	}
	return lines
}

func buildAutomationPolicyLines(policy AutomationPolicyEntry) []string {
	if !policy.AutoResume {
		return nil
	}
	return []string{"      auto_resume: true"}
}

func marshalStringSliceInline(values []string) string {
	data, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// PatchStringSliceField rewrites a YAML key whose value is an inline sequence
// (e.g. `active_states: ["Todo", "In Progress"]`) inside the front matter.
// values is marshalled as a JSON-style inline YAML sequence.
// Returns error if the key is not found.
func PatchStringSliceField(path, key string, values []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Only patch within the front matter.
	frontEnd := -1
	if strings.HasPrefix(content, "---\n") {
		idx := strings.Index(content[4:], "\n---")
		if idx >= 0 {
			frontEnd = 4 + idx + 1
		}
	}
	searchRegion := content
	if frontEnd > 0 {
		searchRegion = content[:frontEnd]
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("workflow patch: marshal %q: %w", key, err)
	}

	lines := strings.Split(searchRegion, "\n")
	replaced := false
	for i, line := range lines {
		m := keyLineRE.FindStringSubmatch(line)
		if m == nil || m[2] != key {
			continue
		}
		// Preserve inline comment, strip from old value.
		oldVal := m[4]
		comment := ""
		if ci := strings.Index(oldVal, " #"); ci >= 0 {
			comment = " " + strings.TrimSpace(oldVal[ci+1:])
			comment = " #" + comment[2:]
		}
		lines[i] = m[1] + m[2] + m[3] + string(encoded) + comment
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("workflow patch: key %q not found in front matter of %s", key, path)
	}

	patched := strings.Join(lines, "\n")
	if frontEnd > 0 {
		patched = patched + content[frontEnd:]
	}
	return os.WriteFile(path, []byte(patched), 0o644)
}

// PatchStringField rewrites a YAML key whose value is a plain string
// (e.g. `completion_state: "In Review"`) inside the front matter.
// Returns error if the key is not found.
func PatchStringField(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workflow patch: read %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Only patch within the front matter.
	frontEnd := -1
	if strings.HasPrefix(content, "---\n") {
		idx := strings.Index(content[4:], "\n---")
		if idx >= 0 {
			frontEnd = 4 + idx + 1
		}
	}
	searchRegion := content
	if frontEnd > 0 {
		searchRegion = content[:frontEnd]
	}

	// Use strconv.Quote to produce a quoted string value.
	quoted := strconv.Quote(value)

	lines := strings.Split(searchRegion, "\n")
	replaced := false
	for i, line := range lines {
		m := keyLineRE.FindStringSubmatch(line)
		if m == nil || m[2] != key {
			continue
		}
		oldVal := m[4]
		comment := ""
		if ci := strings.Index(oldVal, " #"); ci >= 0 {
			comment = " " + strings.TrimSpace(oldVal[ci+1:])
			comment = " #" + comment[2:]
		}
		lines[i] = m[1] + m[2] + m[3] + quoted + comment
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("workflow patch: key %q not found in front matter of %s", key, path)
	}

	patched := strings.Join(lines, "\n")
	if frontEnd > 0 {
		patched = patched + content[frontEnd:]
	}
	return os.WriteFile(path, []byte(patched), 0o644)
}
