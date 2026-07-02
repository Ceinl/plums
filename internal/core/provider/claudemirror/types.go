package claudemirror

import "strings"

type Session struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Directory string      `json:"directory"`
	Model     *ModelRef   `json:"model"`
	Time      SessionTime `json:"time"`
}

type SessionTime struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
}

type ModelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant"`
}

type Provider struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Models map[string]Model `json:"models"`
}

type Model struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

type Part struct {
	Type string     `json:"type"`
	Text string     `json:"text,omitempty"`
	Tool *ToolEvent `json:"tool,omitempty"`
}

type ToolEvent struct {
	ID     string
	Name   string
	Input  string
	Output string
	Error  string
}

type MessageInfo struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type MessageResponse struct {
	Info  MessageInfo `json:"info"`
	Parts []Part      `json:"parts"`
}

type StreamEvent struct {
	Text     string
	Tool     *ToolEvent
	Question *QuestionRequest
}

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type QuestionInfo struct {
	Question string           `json:"question"`
	Header   string           `json:"header"`
	Options  []QuestionOption `json:"options"`
	Multiple bool             `json:"multiple"`
	Custom   *bool            `json:"custom,omitempty"`
}

type QuestionRequest struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionID"`
	Questions []QuestionInfo `json:"questions"`
}

func DisplayTextForPart(part Part) string {
	return streamTextForKind(ssePartKind(part.Type), part.Text)
}

type partKind int

const (
	partKindUnknown partKind = iota
	partKindText
	partKindThinking
	partKindIgnored
)

func ssePartKind(partType string) partKind {
	switch partType {
	case "text":
		return partKindText
	case "reasoning", "thinking":
		return partKindThinking
	default:
		return partKindIgnored
	}
}

func streamTextForKind(kind partKind, text string) string {
	if text == "" {
		return ""
	}
	switch kind {
	case partKindText:
		return text
	case partKindThinking:
		return "<thinking>" + text + "</thinking>"
	default:
		return ""
	}
}

func TitleFromMessage(text, fallback string) string {
	title := strings.TrimSpace(text)
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = strings.TrimSpace(title[:i])
	}
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60]) + "..."
	}
	if title == "" {
		return fallback
	}
	return title
}
