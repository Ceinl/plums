package claudecode

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
	Text string
	Tool *ToolEvent
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
