package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/debuglog"
)

const (
	providerID     = "codex"
	defaultModelID = "default"
)

// Client implements adapter.Backend by communicating with a persistent
// `codex app-server` process over JSON-RPC stdio.
type Client struct {
	mu        sync.Mutex
	sessions  []adapter.Session
	threads   map[string]string
	messages  map[string][]adapter.MessageResponse
	appServer *AppServer
}

// appServerProcess wraps an AppServer so it satisfies app.ServerProcess.
type appServerProcess struct {
	client    *Client
	appServer *AppServer
}

func (p *appServerProcess) Stop() {
	if p.appServer != nil {
		p.appServer.Stop()
	}
	if p.client != nil {
		p.client.clearAppServer(p.appServer)
	}
}

func (p *appServerProcess) Done() <-chan struct{} {
	if p.appServer != nil {
		return p.appServer.Done()
	}
	return nil
}

func NewBackend() adapter.Backend {
	return &Client{
		threads:  make(map[string]string),
		messages: make(map[string][]adapter.MessageResponse),
	}
}

// ServerProcess returns the managed process for lifecycle control.
func (c *Client) ServerProcess() interface {
	Stop()
	Done() <-chan struct{}
} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.appServer == nil {
		return nil
	}
	return &appServerProcess{client: c, appServer: c.appServer}
}

func (c *Client) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := exec.LookPath("codex"); err != nil {
		return fmt.Errorf("codex executable not found: %w", err)
	}
	c.mu.Lock()
	as := c.appServer
	c.mu.Unlock()
	if as != nil {
		// A lightweight ping: model/list is quick and safe.
		_, err := as.Request(ctx, "model/list", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("codex app-server health check failed: %w", err)
		}
		return nil
	}
	// No app-server yet; just verify codex --version works.
	cmd := exec.CommandContext(ctx, "codex", "--version")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("codex health check failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, directory string) (*adapter.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := c.ensureAppServer(ctx, directory); err != nil {
		return nil, err
	}

	result, err := c.appServer.Request(ctx, "thread/start", map[string]interface{}{
		"cwd":            directory,
		"approvalPolicy": "never",
		"sandbox":        "danger-full-access",
	})
	if err != nil {
		return nil, fmt.Errorf("codex thread/start failed: %w", err)
	}

	var threadResp struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(result, &threadResp); err != nil {
		return nil, fmt.Errorf("parse thread/start response: %w", err)
	}

	now := time.Now().UnixMilli()
	modelID := threadResp.Model
	if modelID == "" {
		modelID = defaultModelID
	}
	session := adapter.Session{
		ID:        fmt.Sprintf("codex-%d", now),
		Title:     "Codex session",
		Directory: directory,
		Model:     &adapter.ModelRef{ID: modelID, ProviderID: providerID},
		Time:      adapter.SessionTime{Created: now, Updated: now},
	}

	c.mu.Lock()
	c.sessions = append([]adapter.Session{session}, c.sessions...)
	c.threads[session.ID] = threadResp.Thread.ID
	c.mu.Unlock()

	return copySession(session), nil
}

func (c *Client) ListSessions(ctx context.Context) ([]adapter.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	sessions := append([]adapter.Session(nil), c.sessions...)
	return sessions, nil
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
	return nil, fmt.Errorf("codex session %q not found", sessionID)
}

func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]adapter.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	messages := append([]adapter.MessageResponse(nil), c.messages[sessionID]...)
	return messages, nil
}

func (c *Client) ListProviders(ctx context.Context) ([]adapter.Provider, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	as := c.appServer
	c.mu.Unlock()

	if as != nil {
		result, err := as.Request(ctx, "model/list", map[string]interface{}{})
		if err == nil {
			models, err := parseModelList(result)
			if err == nil && len(models) > 0 {
				provider := adapter.Provider{ID: providerID, Name: "Codex", Models: models}
				return []adapter.Provider{provider}, []string{providerID}, nil
			}
		}
		debuglog.Printf("codex: model/list failed, using static fallback: %v", err)
	}

	// Static fallback when app-server is not running or model/list fails.
	models := map[string]adapter.Model{
		defaultModelID:  {ID: defaultModelID, ProviderID: providerID, Name: "Codex default"},
		"gpt-5.5":       {ID: "gpt-5.5", ProviderID: providerID, Name: "GPT-5.5"},
		"gpt-5.4":       {ID: "gpt-5.4", ProviderID: providerID, Name: "GPT-5.4"},
		"gpt-5.4-mini":  {ID: "gpt-5.4-mini", ProviderID: providerID, Name: "GPT-5.4-Mini"},
		"gpt-5.3-codex": {ID: "gpt-5.3-codex", ProviderID: providerID, Name: "GPT-5.3-Codex"},
		"o3":            {ID: "o3", ProviderID: providerID, Name: "o3"},
	}
	provider := adapter.Provider{ID: providerID, Name: "Codex", Models: models}
	return []adapter.Provider{provider}, []string{providerID}, nil
}

func parseModelList(raw json.RawMessage) (map[string]adapter.Model, error) {
	var resp struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Hidden      bool   `json:"hidden"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	models := make(map[string]adapter.Model, len(resp.Data))
	for _, m := range resp.Data {
		if m.Hidden {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		models[m.ID] = adapter.Model{ID: m.ID, ProviderID: providerID, Name: name}
	}
	return models, nil
}

func (c *Client) SendMessageEvents(ctx context.Context, sessionID, text, providerID, modelID, agent string) <-chan adapter.StreamEvent {
	out := make(chan adapter.StreamEvent)
	go func() {
		defer close(out)
		if strings.TrimSpace(text) == "" {
			return
		}

		threadID := c.thread(sessionID)
		if threadID == "" {
			emit(ctx, out, adapter.StreamEvent{Text: "\nCodex error: no thread for session\n"})
			return
		}

		c.mu.Lock()
		as := c.appServer
		c.mu.Unlock()
		if as == nil {
			emit(ctx, out, adapter.StreamEvent{Text: "\nCodex error: app-server not running\n"})
			return
		}

		// Subscribe before starting the turn so we do not miss events.
		events := as.Subscribe(threadID)
		defer as.Unsubscribe(threadID)
		inputEvents := as.InputEvents()

		params := map[string]interface{}{
			"threadId": threadID,
			"input":    []map[string]string{{"type": "text", "text": text}},
		}
		if modelID != "" && modelID != defaultModelID {
			params["model"] = modelID
		}
		_, err := as.Request(ctx, "turn/start", params)
		if err != nil {
			emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf("\nCodex error: %v\n", err)})
			return
		}

		assistantText := ""
		turnID := ""
		for {
			select {
			case <-ctx.Done():
				c.recordExchange(sessionID, text, assistantText, modelID)
				return
			case event, ok := <-events:
				if !ok {
					c.recordExchange(sessionID, text, assistantText, modelID)
					return
				}
				switch event.Method {
				case "turn/started":
					turnID = event.TurnID
				case "item/agentMessage/delta":
					if event.Delta != "" {
						assistantText += event.Delta
						emit(ctx, out, adapter.StreamEvent{Text: event.Delta})
					}
				case "item/reasoning/textDelta":
					if event.Delta != "" {
						assistantText += event.Delta
						emit(ctx, out, adapter.StreamEvent{Text: "<thinking>" + event.Delta + "</thinking>"})
					}
				case "turn/completed":
					if event.TurnID == turnID {
						c.recordExchange(sessionID, text, assistantText, modelID)
						return
					}
				case "thread/status/changed":
					if event.Status == "idle" && turnID != "" {
						c.recordExchange(sessionID, text, assistantText, modelID)
						return
					}
				case "error":
					if event.Message != "" {
						emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf("\nCodex error: %s\n", event.Message)})
					}
					c.recordExchange(sessionID, text, assistantText, modelID)
					return
				}
			case inputEvent, ok := <-inputEvents:
				if !ok {
					continue
				}
				question := parseUserInputQuestion(inputEvent, sessionID)
				if question != nil {
					select {
					case <-ctx.Done():
						c.recordExchange(sessionID, text, assistantText, modelID)
						return
					case out <- adapter.StreamEvent{Question: question}:
					}
				}
			}
		}
	}()
	return out
}

func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := strconv.ParseInt(requestID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid request id: %w", err)
	}
	c.mu.Lock()
	as := c.appServer
	c.mu.Unlock()
	if as == nil {
		return fmt.Errorf("app-server not running")
	}
	// Codex expects a simple answer payload.  When there is a single
	// free-text answer we send it as a plain string; otherwise we forward
	// the raw [][]string.
	var reply interface{}
	if len(answers) == 1 && len(answers[0]) == 1 {
		reply = answers[0][0]
	} else {
		reply = answers
	}
	return as.ReplyUserInput(id, reply)
}

func parseUserInputQuestion(event UserInputEvent, sessionID string) *adapter.QuestionRequest {
	var params struct {
		Message  string `json:"message"`
		Prompt   string `json:"prompt"`
		Question string `json:"question"`
		Tool     string `json:"tool"`
	}
	_ = json.Unmarshal(event.Params, &params)

	text := params.Message
	if text == "" {
		text = params.Prompt
	}
	if text == "" {
		text = params.Question
	}
	if text == "" {
		text = "Codex is requesting input."
	}
	if params.Tool != "" {
		text = fmt.Sprintf("[%s] %s", params.Tool, text)
	}

	custom := true
	return &adapter.QuestionRequest{
		ID:        fmt.Sprintf("%d", event.ID),
		SessionID: sessionID,
		Questions: []adapter.QuestionInfo{
			{
				Question: text,
				Header:   "Input required",
				Custom:   &custom,
			},
		},
	}
}

func (c *Client) BaseURL() string {
	return "codex://local"
}

func (c *Client) ensureAppServer(ctx context.Context, directory string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.appServer != nil {
		select {
		case <-c.appServer.Done():
			c.appServer = nil
		default:
			return nil
		}
	}
	server, err := NewAppServer(ctx, directory)
	if err != nil {
		return err
	}
	c.appServer = server
	return nil
}

func (c *Client) clearAppServer(appServer *AppServer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.appServer == appServer {
		c.appServer = nil
	}
}

func (c *Client) thread(sessionID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.threads[sessionID]
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
			if c.sessions[i].Title == "Codex session" {
				c.sessions[i].Title = adapter.TitleFromMessage(userText, "Codex session")
			}
			if modelID != "" {
				c.sessions[i].Model = &adapter.ModelRef{ID: modelID, ProviderID: providerID}
			}
			return
		}
	}
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
