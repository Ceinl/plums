package claudecode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/debuglog"
)

const maxStreamLine = 16 * 1024 * 1024

// streamLine is one JSON line from `claude --output-format stream-json`.
type streamLine struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
	Message struct {
		Content []contentBlock `json:"content"`
	} `json:"message"`
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	} `json:"event"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

func (c *Client) buildArgs(sessionID, modelID string) []string {
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--permission-mode", "bypassPermissions",
	}
	if c.markResumed(sessionID) {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}
	if modelID != "" && modelID != defaultModelID {
		args = append(args, "--model", modelID)
	}
	return args
}

// runTurn executes one conversation turn and emits stream events. It returns
// the accumulated assistant text for history recording.
func (c *Client) runTurn(ctx context.Context, out chan<- adapter.StreamEvent, sessionID, text, modelID string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", c.buildArgs(sessionID, modelID)...)
	if dir := c.sessionDirectory(sessionID); dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader(text)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	debuglog.Printf("claude: turn started pid=%d session=%s", cmd.Process.Pid, sessionID)

	assistantText := ""
	sawDeltas := false
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxStreamLine)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var line streamLine
		if err := json.Unmarshal(raw, &line); err != nil {
			debuglog.Printf("claude: skipping unparseable line: %v", err)
			continue
		}
		switch line.Type {
		case "stream_event":
			if line.Event.Type != "content_block_delta" {
				continue
			}
			switch line.Event.Delta.Type {
			case "text_delta":
				if line.Event.Delta.Text != "" {
					sawDeltas = true
					assistantText += line.Event.Delta.Text
					emit(ctx, out, adapter.StreamEvent{Text: line.Event.Delta.Text})
				}
			case "thinking_delta":
				if line.Event.Delta.Thinking != "" {
					sawDeltas = true
					emit(ctx, out, adapter.StreamEvent{Text: "<thinking>" + line.Event.Delta.Thinking + "</thinking>"})
				}
			}
		case "assistant":
			for _, block := range line.Message.Content {
				switch block.Type {
				case "text":
					// Without partial-message support text arrives only here.
					if !sawDeltas && block.Text != "" {
						assistantText += block.Text
						emit(ctx, out, adapter.StreamEvent{Text: block.Text})
					}
				case "tool_use":
					emit(ctx, out, adapter.StreamEvent{Tool: &adapter.ToolEvent{
						ID:    block.ID,
						Name:  block.Name,
						Input: string(block.Input),
					}})
				}
			}
		case "user":
			for _, block := range line.Message.Content {
				if block.Type != "tool_result" {
					continue
				}
				tool := &adapter.ToolEvent{ID: block.ToolUseID}
				output := toolResultText(block.Content)
				if block.IsError {
					tool.Error = output
				} else {
					tool.Output = output
				}
				emit(ctx, out, adapter.StreamEvent{Tool: tool})
			}
		case "result":
			if line.IsError && line.Result != "" {
				emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf("\nClaude error: %s\n", line.Result)})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Wait()
		return assistantText, fmt.Errorf("reading claude output: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return assistantText, fmt.Errorf("%w: %s", err, msg)
		}
		return assistantText, err
	}
	return assistantText, nil
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
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}
