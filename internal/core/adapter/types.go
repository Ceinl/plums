package adapter

import (
	"fmt"
	"hash/fnv"
	"time"
)

// Session represents an opencode session.
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

type MessageModelRef struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
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

// Part is a content part within a message.
type Part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type StreamEvent struct {
	Text     string
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

type MessageInfo struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type MessageResponse struct {
	Info  MessageInfo `json:"info"`
	Parts []Part      `json:"parts"`
}

// DisplayTextForPart returns the displayable text for a part, wrapping
// reasoning/thinking content when appropriate.
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

// DefaultBaseURL is the default opencode server URL.
const DefaultBaseURL = "http://127.0.0.1:4096"

// DefaultPortForDir returns a deterministic registered port (1024–49151)
// derived from dir so different projects / worktrees don't collide on the
// same opencode server.
func DefaultPortForDir(dir string) int {
	if dir == "" {
		return 4096
	}
	h := fnv.New32a()
	h.Write([]byte(dir))
	// Registered port range: 1024 – 49151 = 48128 ports.
	return 1024 + int(h.Sum32()%48128)
}

// DefaultBaseURLForDir returns an opencode server URL using a port
// deterministically derived from dir.
func DefaultBaseURLForDir(dir string) string {
	return fmt.Sprintf("http://127.0.0.1:%d", DefaultPortForDir(dir))
}

// DefaultConfig holds application-level defaults that can be overridden.
type DefaultConfig struct {
	AppVersion           string
	DefaultBaseURL       string
	GlobalConfigDir      string
	LocalConfigDir       string
	PlumsConfigPath      string
	ClipboardCommand     string
	SpinnerInterval      time.Duration
	HealthTimeout        time.Duration
	QuestionReplyTimeout time.Duration
	RecentModelTimeout   time.Duration
	ListTimeout          time.Duration
	DefaultLayoutType    string
	DefaultMode          string
	DefaultOutputPercent int
}

func NewDefaultConfig() *DefaultConfig {
	return &DefaultConfig{
		AppVersion:           "0.1.0-dev",
		DefaultBaseURL:       DefaultBaseURL,
		GlobalConfigDir:      "~/.config/plums/config",
		LocalConfigDir:       "./.agents/plums/config",
		PlumsConfigPath:      ".agents/plums/config/config.toml",
		ClipboardCommand:     "pbcopy",
		SpinnerInterval:      80 * time.Millisecond,
		HealthTimeout:        10 * time.Second,
		QuestionReplyTimeout: 5 * time.Second,
		RecentModelTimeout:   2 * time.Second,
		ListTimeout:          3 * time.Second,
		DefaultLayoutType:    "split",
		DefaultMode:          "build",
		DefaultOutputPercent: 50,
	}
}
