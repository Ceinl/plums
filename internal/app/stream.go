package app

import (
	"fmt"
	"strings"

	"github.com/Ceinl/plums/internal/core/adapter"
)

func displayTextForStreamEvent(event adapter.StreamEvent, emittedTools map[string]bool) string {
	if event.Text != "" {
		return event.Text
	}
	if event.Tool == nil {
		return ""
	}
	return displayTextForToolEvent(*event.Tool, emittedTools)
}

func displayTextForPart(part adapter.Part, emittedTools map[string]bool) string {
	if part.Tool != nil {
		return displayTextForToolEvent(*part.Tool, emittedTools)
	}
	return adapter.DisplayTextForPart(part)
}

func displayTextForToolEvent(tool adapter.ToolEvent, emittedTools map[string]bool) string {
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
