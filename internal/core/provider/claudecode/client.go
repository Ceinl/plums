package claudecode

import (
	"context"
	"crypto/rand"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
)

const (
	providerID     = "claude"
	defaultModelID = "default"
)

// Client implements adapter.Backend by running the `claude` CLI in headless
// streaming mode (one process per turn, conversations resumed by session ID).
type Client struct {
	mu       sync.Mutex
	sessions []adapter.Session
	messages map[string][]adapter.MessageResponse
	resumed  map[string]bool
}

func NewBackend() adapter.Backend {
	return &Client{
		messages: make(map[string][]adapter.MessageResponse),
		resumed:  make(map[string]bool),
	}
}

func (c *Client) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude executable not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, "claude", "--version")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude health check failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, directory string) (*adapter.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	now := time.Now().UnixMilli()
	session := adapter.Session{
		ID:        id,
		Title:     "Claude session",
		Directory: directory,
		Model:     &adapter.ModelRef{ID: defaultModelID, ProviderID: providerID},
		Time:      adapter.SessionTime{Created: now, Updated: now},
	}
	c.mu.Lock()
	c.sessions = append([]adapter.Session{session}, c.sessions...)
	c.mu.Unlock()
	return copySession(session), nil
}

func (c *Client) ListSessions(ctx context.Context) ([]adapter.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]adapter.Session(nil), c.sessions...), nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*adapter.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, session := range c.sessions {
		if session.ID == sessionID {
			return copySession(session), nil
		}
	}
	return nil, fmt.Errorf("claude session %q not found", sessionID)
}

func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]adapter.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]adapter.MessageResponse(nil), c.messages[sessionID]...), nil
}

func (c *Client) ListProviders(ctx context.Context) ([]adapter.Provider, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	models := map[string]adapter.Model{
		defaultModelID: {ID: defaultModelID, ProviderID: providerID, Name: "Claude default"},
		"opus":         {ID: "opus", ProviderID: providerID, Name: "Claude Opus"},
		"sonnet":       {ID: "sonnet", ProviderID: providerID, Name: "Claude Sonnet"},
		"haiku":        {ID: "haiku", ProviderID: providerID, Name: "Claude Haiku"},
	}
	provider := adapter.Provider{ID: providerID, Name: "Claude Code", Models: models}
	return []adapter.Provider{provider}, []string{providerID}, nil
}

func (c *Client) SendMessageEvents(ctx context.Context, sessionID, text, _ string, modelID, _ string) <-chan adapter.StreamEvent {
	out := make(chan adapter.StreamEvent)
	go func() {
		defer close(out)
		if strings.TrimSpace(text) == "" {
			return
		}
		assistantText, err := c.runTurn(ctx, out, sessionID, text, modelID)
		if err != nil {
			emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf("\nClaude error: %v\n", err)})
		}
		c.recordExchange(sessionID, text, assistantText, modelID)
	}()
	return out
}

func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	return fmt.Errorf("claude backend does not support interactive questions")
}

func (c *Client) BaseURL() string {
	return "claude://local"
}

func (c *Client) sessionDirectory(sessionID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, session := range c.sessions {
		if session.ID == sessionID {
			return session.Directory
		}
	}
	return ""
}

func (c *Client) markResumed(sessionID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resumed[sessionID] {
		return true
	}
	c.resumed[sessionID] = true
	return false
}

func (c *Client) recordExchange(sessionID, userText, assistantText, modelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages[sessionID] = append(c.messages[sessionID], adapter.MessageResponse{
		Info:  adapter.MessageInfo{Role: "user"},
		Parts: []adapter.Part{{Type: "text", Text: userText}},
	}, adapter.MessageResponse{
		Info:  adapter.MessageInfo{Role: "assistant"},
		Parts: []adapter.Part{{Type: "text", Text: assistantText}},
	})
	now := time.Now().UnixMilli()
	for i := range c.sessions {
		if c.sessions[i].ID == sessionID {
			c.sessions[i].Time.Updated = now
			if c.sessions[i].Title == "Claude session" {
				c.sessions[i].Title = sessionTitle(userText)
			}
			if modelID != "" {
				c.sessions[i].Model = &adapter.ModelRef{ID: modelID, ProviderID: providerID}
			}
			return
		}
	}
}

func sessionTitle(text string) string {
	title := strings.TrimSpace(text)
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = title[:i]
	}
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60]) + "…"
	}
	if title == "" {
		return "Claude session"
	}
	return title
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func copySession(session adapter.Session) *adapter.Session {
	cp := session
	if session.Model != nil {
		model := *session.Model
		cp.Model = &model
	}
	return &cp
}

func emit(ctx context.Context, out chan<- adapter.StreamEvent, event adapter.StreamEvent) {
	select {
	case <-ctx.Done():
	case out <- event:
	}
}
