package qwen

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/entireio/external-agents/agents/entire-agent-qwen/internal/protocol"
)

const compactCLIVersion = "unknown"

type compactLine struct {
	V          int    `json:"v"`
	Agent      string `json:"agent"`
	CLIVersion string `json:"cli_version"`
	Type       string `json:"type"`
	TS         string `json:"ts,omitempty"`
	ID         string `json:"id,omitempty"`
	Content    any    `json:"content"`
}

type compactUserTextBlock struct {
	Text string `json:"text"`
}

type compactAssistantTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type compactToolUseBlock struct {
	Type   string             `json:"type"`
	ID     string             `json:"id,omitempty"`
	Name   string             `json:"name"`
	Input  any                `json:"input,omitempty"`
	Result *compactToolResult `json:"result,omitempty"`
}

type compactToolResult struct {
	Output string `json:"output"`
	Status string `json:"status"`
}

func (a *Agent) CompactTranscript(sessionRef string) (protocol.CompactTranscriptResponse, error) {
	data, err := os.ReadFile(sessionRef)
	if err != nil {
		return protocol.CompactTranscriptResponse{}, err
	}
	compacted, err := compactTranscriptBytes(data)
	if err != nil {
		return protocol.CompactTranscriptResponse{}, err
	}
	return protocol.CompactTranscriptResponse{Transcript: base64.StdEncoding.EncodeToString(compacted)}, nil
}

func compactTranscriptBytes(data []byte) ([]byte, error) {
	records, err := parseSidecarRecords(data)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	toolResults := make(map[string]json.RawMessage)
	for _, record := range records {
		if record.Event == "ToolResult" && len(record.ToolResponse) > 0 {
			toolResults[record.ToolUseID] = record.ToolResponse
		}
	}
	for _, record := range records {
		switch record.Event {
		case "UserPromptSubmit":
			if record.Prompt == "" {
				continue
			}
			if err := writeCompactLine(&buf, compactLine{
				V:          1,
				Agent:      AgentName,
				CLIVersion: compactCLIVersion,
				Type:       "user",
				TS:         record.TS,
				Content:    []compactUserTextBlock{{Text: record.Prompt}},
			}); err != nil {
				return nil, err
			}
		case "PostToolUse", "PostToolUseFailure", "ToolCall":
			block := compactToolUseBlock{
				Type:  "tool_use",
				ID:    record.ToolUseID,
				Name:  record.ToolName,
				Input: decodeRawObject(record.ToolInput),
			}
			if len(record.ToolResponse) > 0 {
				block.Result = &compactToolResult{Output: string(record.ToolResponse), Status: "success"}
			} else if result, ok := toolResults[record.ToolUseID]; ok {
				block.Result = &compactToolResult{Output: string(result), Status: "success"}
			}
			if record.Error != "" || record.ErrorDetails != "" {
				block.Result = &compactToolResult{Output: record.Error + " " + record.ErrorDetails, Status: "error"}
			}
			if err := writeCompactLine(&buf, compactLine{
				V:          1,
				Agent:      AgentName,
				CLIVersion: compactCLIVersion,
				Type:       "assistant",
				TS:         record.TS,
				ID:         record.ToolUseID,
				Content:    []any{block},
			}); err != nil {
				return nil, err
			}
		case "Stop", "StopFailure", "AgentResponse", "CheckpointCreated":
			text := record.LastAssistantMessage
			if text == "" {
				text = record.ErrorDetails
			}
			if text == "" {
				continue
			}
			if err := writeCompactLine(&buf, compactLine{
				V:          1,
				Agent:      AgentName,
				CLIVersion: compactCLIVersion,
				Type:       "assistant",
				TS:         record.TS,
				Content:    []compactAssistantTextBlock{{Type: "text", Text: text}},
			}); err != nil {
				return nil, err
			}
		}
	}
	if buf.Len() == 0 {
		return nil, errors.New("compact transcript produced no output")
	}
	return buf.Bytes(), nil
}

func parseSidecarRecords(data []byte) ([]sidecarRecord, error) {
	lines := bytes.Split(data, []byte("\n"))
	records := make([]sidecarRecord, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		record, ok := parseTranscriptRecord(line)
		if !ok {
			// A partially written final JSONL line must not discard prior session data.
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func parseTranscriptRecord(line []byte) (sidecarRecord, bool) {
	var envelope struct {
		Event     string          `json:"event"`
		Timestamp string          `json:"timestamp"`
		SessionID string          `json:"session_id"`
		Text      string          `json:"text"`
		Tool      string          `json:"tool"`
		CallID    string          `json:"call_id"`
		Input     json.RawMessage `json:"input"`
		Output    json.RawMessage `json:"output"`
		Path      string          `json:"path"`
		Summary   string          `json:"summary"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return sidecarRecord{}, false
	}

	if isNewTranscriptEvent(envelope.Event) {
		return normalizeNewTranscriptEvent(envelope), true
	}

	var record sidecarRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return sidecarRecord{}, false
	}
	return record, true
}

func isNewTranscriptEvent(event string) bool {
	switch event {
	case "session_started", "user_prompt", "agent_response", "tool_call", "tool_result", "file_read", "file_changed", "usage", "checkpoint_created", "session_ended":
		return true
	default:
		return strings.Contains(event, "_")
	}
}

func normalizeNewTranscriptEvent(raw struct {
	Event     string          `json:"event"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"session_id"`
	Text      string          `json:"text"`
	Tool      string          `json:"tool"`
	CallID    string          `json:"call_id"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output"`
	Path      string          `json:"path"`
	Summary   string          `json:"summary"`
}) sidecarRecord {
	record := sidecarRecord{V: 1, Agent: AgentName, Event: raw.Event, SessionID: raw.SessionID, TS: raw.Timestamp}
	switch raw.Event {
	case "user_prompt":
		record.Event, record.Prompt = "UserPromptSubmit", raw.Text
	case "agent_response":
		record.Event, record.LastAssistantMessage = "AgentResponse", raw.Text
	case "tool_call":
		record.Event, record.ToolName, record.ToolUseID, record.ToolInput = "ToolCall", raw.Tool, raw.CallID, copyRawMessage(raw.Input)
	case "tool_result":
		record.Event, record.ToolUseID, record.ToolResponse = "ToolResult", raw.CallID, copyRawMessage(raw.Output)
	case "file_changed":
		record.Event, record.FilePath, record.CompactSummary = "FileChanged", raw.Path, raw.Summary
	case "checkpoint_created":
		record.Event, record.CompactSummary = "CheckpointCreated", raw.Summary
	}
	return record
}

func writeCompactLine(buf *bytes.Buffer, line compactLine) error {
	data, err := json.Marshal(line)
	if err != nil {
		return err
	}
	buf.Write(data)
	buf.WriteByte('\n')
	return nil
}

func decodeRawObject(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}
