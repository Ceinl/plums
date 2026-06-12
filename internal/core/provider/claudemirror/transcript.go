package claudemirror

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	"github.com/Ceinl/plums/internal/core/adapter"
)

const maxTranscriptLine = 16 * 1024 * 1024

// transcriptEntry is one JSONL line of a Claude Code session transcript
// (~/.claude/projects/<encoded-cwd>/<session-id>.jsonl).
type transcriptEntry struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	SessionID   string `json:"sessionId"`
	Cwd         string `json:"cwd"`
	Timestamp   string `json:"timestamp"`
	IsMeta      bool   `json:"isMeta"`
	IsSidechain bool   `json:"isSidechain"`
	Message     struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		StopReason string          `json:"stop_reason"`
	} `json:"message"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// contentBlocks decodes a message content field, which is either a plain
// string (typed user prompts) or a list of content blocks.
func contentBlocks(raw json.RawMessage) []contentBlock {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []contentBlock{{Type: "text", Text: s}}
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}
	return nil
}

// isHarnessText reports whether a user text is harness-injected scaffolding
// (slash-command markers, caveats) rather than something the user typed.
func isHarnessText(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, "<command-name>") ||
		strings.HasPrefix(t, "<local-command-") ||
		strings.HasPrefix(t, "<system-reminder>")
}

// entryParts converts a transcript entry into displayable message parts.
func entryParts(entry transcriptEntry) []adapter.Part {
	var parts []adapter.Part
	for _, block := range contentBlocks(entry.Message.Content) {
		switch block.Type {
		case "text":
			if block.Text != "" && !(entry.Message.Role == "user" && isHarnessText(block.Text)) {
				parts = append(parts, adapter.Part{Type: "text", Text: block.Text})
			}
		case "thinking":
			if block.Thinking != "" {
				parts = append(parts, adapter.Part{Type: "thinking", Text: block.Thinking})
			}
		case "tool_use":
			parts = append(parts, adapter.Part{Type: "tool", Tool: &adapter.ToolEvent{
				ID:    block.ID,
				Name:  block.Name,
				Input: string(block.Input),
			}})
		case "tool_result":
			tool := &adapter.ToolEvent{ID: block.ToolUseID}
			output := toolResultText(block.Content)
			if block.IsError {
				tool.Error = output
			} else {
				tool.Output = output
			}
			parts = append(parts, adapter.Part{Type: "tool", Tool: tool})
		}
	}
	return parts
}

// toolResultText extracts readable text from a tool_result content field,
// which may be a plain string or a list of content blocks.
func toolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var texts []string
	for _, b := range contentBlocks(raw) {
		if b.Type == "text" && b.Text != "" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// readTranscript parses every entry in a transcript file.
func readTranscript(path string) ([]transcriptEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []transcriptEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxTranscriptLine)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry transcriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// conversationEntry reports whether an entry is part of the main
// conversation (not meta, not a subagent sidechain).
func conversationEntry(entry transcriptEntry) bool {
	if entry.IsMeta || entry.IsSidechain {
		return false
	}
	return entry.Type == "user" || entry.Type == "assistant"
}

// transcriptTitle derives a session title from the first typed user message.
func transcriptTitle(entries []transcriptEntry, fallback string) string {
	for _, entry := range entries {
		if !conversationEntry(entry) || entry.Message.Role != "user" {
			continue
		}
		for _, part := range entryParts(entry) {
			if part.Type == "text" && part.Text != "" {
				return adapter.TitleFromMessage(part.Text, fallback)
			}
		}
	}
	return fallback
}
