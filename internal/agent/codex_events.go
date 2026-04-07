package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

type codexRawEvent struct {
	Type     string        `json:"type"`
	ThreadID string        `json:"thread_id"`
	Item     *codexRawItem `json:"item"`
	Usage    *codexUsage   `json:"usage"`
	Error    *codexError   `json:"error"`
}

type codexError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type codexRawItem struct {
	ID                string                 `json:"id"`
	Type              string                 `json:"type"`
	Text              string                 `json:"text"`
	Command           string                 `json:"command"`
	AggregatedOutput  string                 `json:"aggregated_output"`
	ExitCode          *int                   `json:"exit_code"`
	Status            string                 `json:"status"`
	Tool              string                 `json:"tool"`
	Prompt            string                 `json:"prompt"`
	SenderThreadID    string                 `json:"sender_thread_id"`
	ReceiverThreadIDs []string               `json:"receiver_thread_ids"`
	AgentsStates      map[string]codexThread `json:"agents_states"`
}

type codexUsage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

type codexThread struct {
	Status  string  `json:"status"`
	Message *string `json:"message"`
}

// ParseCodexLine maps a single Codex --json JSONL line to a StreamEvent.
// Irrelevant line types return an error so callers can skip them.
func ParseCodexLine(line []byte) (StreamEvent, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return StreamEvent{}, fmt.Errorf("codex: empty line")
	}

	var raw codexRawEvent
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return StreamEvent{}, fmt.Errorf("codex: parse line: %w", err)
	}

	switch raw.Type {
	case "thread.started":
		return StreamEvent{Type: EventSystem, SessionID: raw.ThreadID}, nil

	case "item.started":
		if raw.Item == nil {
			return StreamEvent{}, fmt.Errorf("codex: item.started missing item")
		}
		switch raw.Item.Type {
		case "command_execution":
			input, err := json.Marshal(map[string]any{
				"command":   raw.Item.Command,
				"status":    "in_progress",
				"item_id":   raw.Item.ID,
				"item_type": raw.Item.Type,
			})
			if err != nil {
				return StreamEvent{}, fmt.Errorf("codex: marshal item.started command: %w", err)
			}
			return StreamEvent{
				Type:       EventAssistant,
				InProgress: true,
				ToolCalls:  []ToolCall{{Name: "shell", Input: input}},
			}, nil
		case "collab_tool_call":
			name := raw.Item.Tool
			if name == "" {
				name = raw.Item.Type
			}
			input, err := json.Marshal(map[string]any{
				"tool":                raw.Item.Tool,
				"sender_thread_id":    raw.Item.SenderThreadID,
				"receiver_thread_ids": raw.Item.ReceiverThreadIDs,
				"status":              "in_progress",
				"item_id":             raw.Item.ID,
				"item_type":           raw.Item.Type,
			})
			if err != nil {
				return StreamEvent{}, fmt.Errorf("codex: marshal item.started collab: %w", err)
			}
			return StreamEvent{
				Type:       EventAssistant,
				InProgress: true,
				ToolCalls:  []ToolCall{{Name: name, Input: input}},
			}, nil
		default:
			return StreamEvent{}, fmt.Errorf("codex: skip item.started type %q", raw.Item.Type)
		}

	case "item.completed":
		if raw.Item == nil {
			return StreamEvent{}, fmt.Errorf("codex: item.completed missing item")
		}
		switch raw.Item.Type {
		case "agent_message":
			ev := StreamEvent{Type: EventAssistant}
			if raw.Item.Text != "" {
				ev.Message = raw.Item.Text
				ev.TextBlocks = []string{raw.Item.Text}
			}
			return ev, nil
		case "command_execution":
			input, err := json.Marshal(map[string]any{
				"command":   raw.Item.Command,
				"output":    raw.Item.AggregatedOutput,
				"exit_code": raw.Item.ExitCode,
				"status":    raw.Item.Status,
				"item_id":   raw.Item.ID,
				"item_type": raw.Item.Type,
			})
			if err != nil {
				return StreamEvent{}, fmt.Errorf("codex: marshal command_execution: %w", err)
			}
			return StreamEvent{
				Type: EventAssistant,
				ToolCalls: []ToolCall{{
					Name:  "shell",
					Input: input,
				}},
			}, nil
		case "collab_tool_call":
			name := raw.Item.Tool
			if name == "" {
				name = raw.Item.Type
			}
			input, err := json.Marshal(map[string]any{
				"tool":                raw.Item.Tool,
				"prompt":              raw.Item.Prompt,
				"sender_thread_id":    raw.Item.SenderThreadID,
				"receiver_thread_ids": raw.Item.ReceiverThreadIDs,
				"agents_states":       raw.Item.AgentsStates,
				"status":              raw.Item.Status,
				"item_id":             raw.Item.ID,
				"item_type":           raw.Item.Type,
			})
			if err != nil {
				return StreamEvent{}, fmt.Errorf("codex: marshal collab_tool_call: %w", err)
			}
			return StreamEvent{
				Type: EventAssistant,
				ToolCalls: []ToolCall{{
					Name:  name,
					Input: input,
				}},
			}, nil
		default:
			return StreamEvent{}, fmt.Errorf("codex: unknown item type %q", raw.Item.Type)
		}

	case "turn.completed":
		ev := StreamEvent{Type: EventResult}
		if raw.Usage != nil {
			ev.Usage = UsageSnapshot{
				InputTokens:       raw.Usage.InputTokens,
				CachedInputTokens: raw.Usage.CachedInputTokens,
				OutputTokens:      raw.Usage.OutputTokens,
			}
		}
		return ev, nil

	case "turn.failed":
		ev := StreamEvent{Type: EventResult, IsError: true}
		if raw.Error != nil {
			ev.ResultText = raw.Error.Message
			ev.IsInputRequired = isInputRequiredMsg(raw.Error.Message)
		}
		if raw.Usage != nil {
			ev.Usage = UsageSnapshot{
				InputTokens:       raw.Usage.InputTokens,
				CachedInputTokens: raw.Usage.CachedInputTokens,
				OutputTokens:      raw.Usage.OutputTokens,
			}
		}
		return ev, nil

	default:
		return StreamEvent{}, fmt.Errorf("codex: skip event type %q", raw.Type)
	}
}
