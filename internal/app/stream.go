package app

import (
	"fmt"
	"strings"

	"github.com/Ceinl/plums/capabilities"
)

func displayTextForStreamEvent(event capabilities.StreamEvent, emittedTools map[string]bool) string {
	if event.Text != "" {
		return event.Text
	}
	if event.Tool == nil {
		return ""
	}
	return displayTextForToolEvent(*event.Tool, emittedTools)
}

func displayTextForPart(part capabilities.Part, emittedTools map[string]bool) string {
	if part.Tool != nil {
		return displayTextForToolEvent(*part.Tool, emittedTools)
	}
	return displayTextForPartText(part.Type, part.Text)
}

func displayTextForToolEvent(tool capabilities.ToolCall, emittedTools map[string]bool) string {
	var chunks []string
	if tool.ID == "" || !emittedTools[tool.ID] {
		chunks = append(chunks, fmt.Sprintf("<tool_call name=%q>%s</tool_call>", tool.Name, tool.Input))
		if tool.ID != "" {
			emittedTools[tool.ID] = true
		}
	}
	if tool.Output != "" {
		chunks = append(chunks, "<tool_output>"+tool.Output+"</tool_output>")
	}
	if tool.Error != "" {
		chunks = append(chunks, "<tool_output>"+tool.Error+"</tool_output>")
	}
	if len(chunks) == 0 {
		return ""
	}
	return "\n" + strings.Join(chunks, "\n") + "\n"
}

func displayTextForPartText(partType, text string) string {
	if text == "" {
		return ""
	}
	switch partType {
	case "text":
		return text
	case "reasoning", "thinking":
		return "<thinking>" + text + "</thinking>"
	default:
		return ""
	}
}
