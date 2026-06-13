package components

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// ── Markdown/code highlighting ───────────────────────────────────────────────

type blockKind int

const (
	blockKindText blockKind = iota
	blockKindCode
	blockKindToolCall
	blockKindToolOutput
)

type chatBlock struct {
	kind     blockKind
	toolName string
	lang     string
	text     string
}

func parseChatBlocks(content string) []chatBlock {
	var blocks []chatBlock
	var currentText []string
	var currentCode []string
	var currentTool []string

	inCode := false
	inToolCall := false
	inToolOutput := false
	toolName := ""
	codeLang := ""

	flushText := func() {
		if len(currentText) == 0 {
			return
		}
		blocks = append(blocks, chatBlock{
			kind: blockKindText,
			text: strings.Join(currentText, "\n"),
		})
		currentText = nil
	}

	flushCode := func() {
		blocks = append(blocks, chatBlock{
			kind: blockKindCode,
			lang: codeLang,
			text: strings.Join(currentCode, "\n"),
		})
		currentCode = nil
		codeLang = ""
	}

	flushToolCall := func() {
		blocks = append(blocks, chatBlock{
			kind:     blockKindToolCall,
			toolName: toolName,
			text:     strings.Join(currentTool, "\n"),
		})
		currentTool = nil
		toolName = ""
	}

	flushToolOutput := func() {
		blocks = append(blocks, chatBlock{
			kind: blockKindToolOutput,
			text: strings.Join(currentTool, "\n"),
		})
		currentTool = nil
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Code Block boundary
		if strings.HasPrefix(trimmed, "```") && !inToolCall && !inToolOutput {
			if inCode {
				flushCode()
				inCode = false
			} else {
				flushText()
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				inCode = true
			}
			continue
		}

		if inCode {
			currentCode = append(currentCode, line)
			continue
		}

		if name, input, ok := parsePlainToolCall(trimmed); ok {
			flushText()
			blocks = append(blocks, chatBlock{
				kind:     blockKindToolCall,
				toolName: name,
				text:     input,
			})
			continue
		}

		// 2. Tool Call / Output boundary checks
		if strings.Contains(trimmed, "<tool_call") {
			flushText()

			// Extract tool name if present: name="xyz" or name='xyz'
			name := ""
			if idx := strings.Index(trimmed, "name=\""); idx != -1 {
				sub := trimmed[idx+6:]
				if endIdx := strings.Index(sub, "\""); endIdx != -1 {
					name = sub[:endIdx]
				}
			} else if idx := strings.Index(trimmed, "name='"); idx != -1 {
				sub := trimmed[idx+6:]
				if endIdx := strings.Index(sub, "'"); endIdx != -1 {
					name = sub[:endIdx]
				}
			}

			if strings.Contains(trimmed, "</tool_call>") {
				contentInside := extractTagContent(trimmed, "<tool_call", "</tool_call>")
				blocks = append(blocks, chatBlock{
					kind:     blockKindToolCall,
					toolName: name,
					text:     contentInside,
				})
			} else {
				inToolCall = true
				toolName = name
				if tagEnd := strings.Index(line, ">"); tagEnd != -1 && tagEnd+1 < len(line) {
					rem := strings.TrimSpace(line[tagEnd+1:])
					if rem != "" {
						currentTool = append(currentTool, rem)
					}
				}
			}
			continue
		}

		if inToolCall {
			if strings.Contains(trimmed, "</tool_call>") {
				if idx := strings.Index(line, "</tool_call>"); idx != -1 {
					before := strings.TrimSpace(line[:idx])
					if before != "" {
						currentTool = append(currentTool, before)
					}
				}
				flushToolCall()
				inToolCall = false
			} else {
				currentTool = append(currentTool, line)
			}
			continue
		}

		isOutputStart := strings.Contains(trimmed, "<tool_output") || strings.Contains(trimmed, "<tool_response")
		if isOutputStart {
			flushText()

			closingTag := "</tool_output>"
			if strings.Contains(trimmed, "<tool_response") {
				closingTag = "</tool_response>"
			}

			if strings.Contains(trimmed, closingTag) {
				contentInside := extractTagContent(trimmed, "<tool_output", closingTag)
				if contentInside == "" && closingTag == "</tool_response>" {
					contentInside = extractTagContent(trimmed, "<tool_response", closingTag)
				}
				blocks = append(blocks, chatBlock{
					kind: blockKindToolOutput,
					text: contentInside,
				})
			} else {
				inToolOutput = true
				if tagEnd := strings.Index(line, ">"); tagEnd != -1 && tagEnd+1 < len(line) {
					rem := strings.TrimSpace(line[tagEnd+1:])
					if rem != "" {
						currentTool = append(currentTool, rem)
					}
				}
			}
			continue
		}

		if inToolOutput {
			hasClosingOutput := strings.Contains(trimmed, "</tool_output>")
			hasClosingResponse := strings.Contains(trimmed, "</tool_response>")
			if hasClosingOutput || hasClosingResponse {
				closingTag := "</tool_output>"
				if hasClosingResponse {
					closingTag = "</tool_response>"
				}
				if idx := strings.Index(line, closingTag); idx != -1 {
					before := strings.TrimSpace(line[:idx])
					if before != "" {
						currentTool = append(currentTool, before)
					}
				}
				flushToolOutput()
				inToolOutput = false
			} else {
				currentTool = append(currentTool, line)
			}
			continue
		}

		// 3. Normal text line
		currentText = append(currentText, line)
	}

	if inCode {
		flushCode()
	} else if inToolCall {
		flushToolCall()
	} else if inToolOutput {
		flushToolOutput()
	} else {
		flushText()
	}

	return blocks
}

func extractTagContent(s, tagOpen, tagClose string) string {
	idxOpen := strings.Index(s, tagOpen)
	if idxOpen == -1 {
		return ""
	}
	sub := s[idxOpen:]
	idxEndOpen := strings.Index(sub, ">")
	if idxEndOpen == -1 {
		return ""
	}
	start := idxOpen + idxEndOpen + 1

	idxClose := strings.Index(s, tagClose)
	if idxClose == -1 || idxClose <= start {
		return strings.TrimSpace(s[start:])
	}
	return strings.TrimSpace(s[start:idxClose])
}

func compactToOneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	words := strings.Fields(s)
	return strings.Join(words, " ")
}

func parsePlainToolCall(line string) (name, input string, ok bool) {
	const prefix = "Called the "
	const marker = " tool with the following input:"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	idx := strings.Index(rest, marker)
	if idx == -1 {
		return "", "", false
	}
	name = strings.TrimSpace(rest[:idx])
	input = strings.TrimSpace(rest[idx+len(marker):])
	if name == "" {
		return "", "", false
	}
	return name, input, true
}

// toolArgPriority lists input keys whose value alone summarises a call well,
// most informative first.
var toolArgPriority = []string{
	"command", "file_path", "filePath", "path", "pattern", "query",
	"url", "prompt", "description", "skill", "content",
}

// toolCallArg reduces a tool's JSON input to a one-line human summary:
// the highest-priority string argument if one exists, otherwise compact
// "key: value" pairs. An empty input ("{}") yields "".
func toolCallArg(input string) string {
	input = strings.TrimSpace(input)
	if input == "" || input == "{}" {
		return ""
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil || len(args) == 0 {
		return compactToOneLine(input)
	}

	for _, key := range toolArgPriority {
		if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
			return compactToOneLine(v)
		}
	}

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+": "+compactToOneLine(fmt.Sprintf("%v", args[k])))
	}
	return strings.Join(pairs, ", ")
}

func truncateToolSummary(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func highlightCode(lang, code, fallbackFg string) [][]textSpan {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		return plainCodeLines(code, fallbackFg)
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return plainCodeLines(code, fallbackFg)
	}

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	lines := [][]textSpan{{}}
	for token := iterator(); token != chroma.EOF; token = iterator() {
		entry := style.Get(token.Type)
		fg := chromaColourToANSI(entry.Colour)
		if fg == "" {
			fg = fallbackFg
		}
		decor := ""
		if entry.Bold == chroma.Yes {
			decor = "\x1b[1m"
		}

		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, []textSpan{})
			}
			if part != "" {
				last := len(lines) - 1
				lines[last] = append(lines[last], textSpan{text: part, fg: fg, decor: decor})
			}
		}
	}

	return lines
}

func chromaColourToANSI(c chroma.Colour) string {
	if !c.IsSet() {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.Red(), c.Green(), c.Blue())
}

func plainCodeLines(code, fg string) [][]textSpan {
	parts := strings.Split(code, "\n")
	lines := make([][]textSpan, len(parts))
	for i, part := range parts {
		if part != "" {
			lines[i] = []textSpan{{text: part, fg: fg}}
		}
	}
	return lines
}

func wrapSpanLines(lines [][]textSpan, width int) [][]textSpan {
	if width <= 0 {
		return lines
	}

	var out [][]textSpan
	for _, line := range lines {
		out = append(out, wrapSpansHard(line, width)...)
	}
	return out
}

func wrapSpansHard(spans []textSpan, width int) [][]textSpan {
	if len(spans) == 0 {
		return [][]textSpan{{}}
	}

	var out [][]textSpan
	var cur []textSpan
	curWidth := 0
	appendRune := func(r rune, span textSpan) {
		if curWidth >= width {
			out = append(out, cur)
			cur = nil
			curWidth = 0
		}
		if len(cur) > 0 && cur[len(cur)-1].fg == span.fg && cur[len(cur)-1].decor == span.decor {
			cur[len(cur)-1].text += string(r)
		} else {
			cur = append(cur, textSpan{text: string(r), fg: span.fg, decor: span.decor})
		}
		curWidth++
	}

	for _, span := range spans {
		for _, r := range span.text {
			appendRune(r, span)
		}
	}
	if cur != nil {
		out = append(out, cur)
	}
	if len(out) == 0 {
		out = append(out, []textSpan{})
	}
	return out
}
