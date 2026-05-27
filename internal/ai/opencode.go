package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client communicates with a running `opencode serve` process.
type Client struct {
	baseURL    string
	httpClient *http.Client // used for regular requests (300 s timeout)
	sseClient  *http.Client // used for SSE streams (no timeout)
}

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

type messageResponse struct {
	Info  messageInfo `json:"info"`
	Parts []Part      `json:"parts"`
}

type messageInfo struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type createSessionBody struct {
	Title string `json:"title,omitempty"`
}

type sendMessageBody struct {
	Model *MessageModelRef `json:"model,omitempty"`
	Agent string           `json:"agent,omitempty"`
	Parts []Part           `json:"parts"`
}

type providerListResponse struct {
	All       []Provider `json:"all"`
	Connected []string   `json:"connected"`
}

// ── SSE event types ───────────────────────────────────────────────────────────

// sseEnvelope is the top-level wrapper for every SSE event.
type sseEnvelope struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

// partUpdatedProperties is the payload of a "message.part.updated" event.
type partUpdatedProperties struct {
	Part partDetail `json:"part"`
}

// partDetail describes a single message part.
type partDetail struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Type      string `json:"type"`
	Text      string `json:"text"`
}

// partDeltaProperties is the payload of a "message.part.delta" event.
type partDeltaProperties struct {
	SessionID string `json:"sessionID"`
	PartID    string `json:"partID"`
	Field     string `json:"field"`
	Delta     string `json:"delta"`
}

// sessionIdleProperties is the payload of a "session.idle" event.
type sessionIdleProperties struct {
	SessionID string `json:"sessionID"`
}

// sessionStatusProperties is the payload of a "session.status" event.
// The Status field can be "idle", "busy", or "retry".
type sessionStatusProperties struct {
	SessionID string `json:"sessionID"`
	Status    struct {
		Type string `json:"type"`
	} `json:"status"`
}

type sseResult struct {
	emitted   bool
	completed bool
}

const DefaultBaseURL = "http://127.0.0.1:4096"

// ── Constructors ──────────────────────────────────────────────────────────────

func NewClient() *Client {
	return NewClientWithURL(DefaultBaseURL)
}

func NewClientWithURL(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		sseClient: &http.Client{
			Timeout: 0,
		},
	}
}

// ── Public API ────────────────────────────────────────────────────────────────

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/global/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode server at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opencode server at %s returned status %d", c.baseURL, resp.StatusCode)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context) (*Session, error) {
	body := createSessionBody{}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal session body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/session", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/session/"+url.PathEscape(sessionID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]messageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/session/"+url.PathEscape(sessionID)+"/message", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	var messages []messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (c *Client) ListProviders(ctx context.Context) ([]Provider, []string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/provider", nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	var providers providerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, nil, err
	}
	return providers.All, providers.Connected, nil
}

// SendMessage streams the assistant's reply token-by-token via the returned
// channel. It first tries the SSE-based approach (prompt_async + /event); if
// that fails (404 or connection error) it falls back to the synchronous
// /session/{id}/message endpoint.
func (c *Client) SendMessage(ctx context.Context, sessionID, text, providerID, modelID, agent string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		result, err := c.sendWithSSE(ctx, sessionID, text, providerID, modelID, agent, out)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if !result.emitted {
			// SSE path unavailable before streaming tokens; fall back safely.
			c.sendSync(ctx, sessionID, text, providerID, modelID, agent, out)
			return
		}
		select {
		case <-ctx.Done():
		case out <- fmt.Sprintf("\n⚠️  SSE stream ended before completion: %v\n", err):
		}
	}()
	return out
}

// ── Private helpers ───────────────────────────────────────────────────────────

// sendWithSSE opens a long-lived GET /event SSE connection, kicks off
// generation via POST /session/{id}/prompt_async, then reads events until
// session.idle arrives. It reports whether tokens were emitted so the caller
// only falls back to sendSync when doing so cannot duplicate output.
func (c *Client) sendWithSSE(ctx context.Context, sessionID, text, providerID, modelID, agent string, out chan<- string) (sseResult, error) {
	var result sseResult

	// 1. Open the SSE stream first so we don't miss any events.
	sseReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/event", nil)
	if err != nil {
		return result, fmt.Errorf("build SSE request: %w", err)
	}
	sseReq.Header.Set("Accept", "text/event-stream")
	sseReq.Header.Set("Cache-Control", "no-cache")

	sseResp, err := c.sseClient.Do(sseReq)
	if err != nil {
		return result, fmt.Errorf("SSE connect: %w", err)
	}
	if sseResp.StatusCode != http.StatusOK {
		_ = sseResp.Body.Close()
		return result, fmt.Errorf("SSE endpoint returned status %d", sseResp.StatusCode)
	}
	defer func() { _ = sseResp.Body.Close() }()

	// 2. Send the prompt asynchronously.
	b := newSendMessageBody(text, providerID, modelID, agent)
	bodyData, err := json.Marshal(b)
	if err != nil {
		return result, fmt.Errorf("marshal prompt body: %w", err)
	}
	asyncReq, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/session/"+url.PathEscape(sessionID)+"/prompt_async",
		bytes.NewReader(bodyData))
	if err != nil {
		return result, fmt.Errorf("build prompt_async request: %w", err)
	}
	asyncReq.Header.Set("Content-Type", "application/json")

	asyncResp, err := c.httpClient.Do(asyncReq)
	if err != nil {
		return result, fmt.Errorf("prompt_async: %w", err)
	}
	defer func() { _ = asyncResp.Body.Close() }()

	if asyncResp.StatusCode == http.StatusNotFound {
		return result, fmt.Errorf("prompt_async: 404 – old server, falling back")
	}
	if asyncResp.StatusCode != http.StatusNoContent && asyncResp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("prompt_async returned status %d", asyncResp.StatusCode)
	}

	// 3. Consume SSE events.
	// textPartIDs tracks part IDs confirmed to belong to our session.
	textPartIDs := make(map[string]bool)
	// pendingDeltas holds deltas that arrived before their message.part.updated
	// registration event (a known opencode race – see issue #26924).
	pendingDeltas := make(map[string][]string)

	scanner := bufio.NewScanner(sseResp.Body)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	var dataBuilder strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "data:"):
			// Accumulate the data payload (trim the "data:" prefix and any
			// single leading space as per the SSE spec).
			payload := strings.TrimPrefix(line, "data:")
			payload = strings.TrimPrefix(payload, " ")
			dataBuilder.WriteString(payload)

		case line == "":
			// Empty line = end of event – process whatever we accumulated.
			raw := dataBuilder.String()
			dataBuilder.Reset()
			if raw == "" {
				continue
			}

			var env sseEnvelope
			if err := json.Unmarshal([]byte(raw), &env); err != nil {
				continue // skip malformed events
			}

			switch env.Type {
			case "message.part.updated":
				var props partUpdatedProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.Part.SessionID == sessionID && props.Part.Type == "text" {
					textPartIDs[props.Part.ID] = true
					// Flush any deltas that arrived before this registration.
					for _, d := range pendingDeltas[props.Part.ID] {
						select {
						case <-ctx.Done():
							return result, ctx.Err()
						case out <- d:
							result.emitted = true
						}
					}
					delete(pendingDeltas, props.Part.ID)
				}

			case "message.part.delta":
				var props partDeltaProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.Field != "text" || props.Delta == "" {
					continue
				}
				if props.SessionID != "" && props.SessionID != sessionID {
					continue
				}
				if props.SessionID == sessionID || textPartIDs[props.PartID] {
					// Newer opencode delta events include sessionID, so we can emit
					// immediately instead of waiting for a part registration event.
					select {
					case <-ctx.Done():
						return result, ctx.Err()
					case out <- props.Delta:
						result.emitted = true
					}
				} else {
					// Delta arrived before its part.updated – queue it.
					pendingDeltas[props.PartID] = append(pendingDeltas[props.PartID], props.Delta)
				}

			case "session.idle":
				var props sessionIdleProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.SessionID == sessionID {
					result.completed = true
					return result, nil // generation complete
				}

			case "session.status":
				// Only treat as completion when status.type == "idle".
				// "busy" and "retry" fire at the start of generation – treating
				// them as done would kill the stream before any tokens arrive.
				var props sessionStatusProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.SessionID == sessionID && props.Status.Type == "idle" {
					result.completed = true
					return result, nil
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("SSE scan: %w", err)
	}

	return result, fmt.Errorf("SSE stream ended before session idle")
}

// sendSync is the legacy fallback: POSTs to /session/{id}/message, waits for
// the full JSON response, then emits the text character by character.
func (c *Client) sendSync(ctx context.Context, sessionID, text, providerID, modelID, agent string, out chan<- string) {
	b := newSendMessageBody(text, providerID, modelID, agent)
	data, err := json.Marshal(b)
	if err != nil {
		select {
		case <-ctx.Done():
		case out <- fmt.Sprintf("\n⚠️  marshal error: %v\n", err):
		}
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/session/"+url.PathEscape(sessionID)+"/message",
		bytes.NewReader(data))
	if err != nil {
		select {
		case <-ctx.Done():
		case out <- fmt.Sprintf("\n⚠️  request error: %v\n", err):
		}
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return
		case out <- "\n⚠️  Opencode server not available (is `opencode serve` running?)\n":
		}
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		select {
		case <-ctx.Done():
		case out <- fmt.Sprintf("\n⚠️  opencode server returned status %d\n", resp.StatusCode):
		}
		return
	}

	var msgResp messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		select {
		case <-ctx.Done():
		case out <- fmt.Sprintf("\n⚠️  decode error: %v\n", err):
		}
		return
	}

	for _, part := range msgResp.Parts {
		if part.Type == "text" && part.Text != "" {
			for _, r := range part.Text {
				select {
				case <-ctx.Done():
					return
				case out <- string(r):
				}
				time.Sleep(3 * time.Millisecond)
			}
		}
	}
}

func newSendMessageBody(text, providerID, modelID, agent string) sendMessageBody {
	b := sendMessageBody{
		Agent: agent,
		Parts: []Part{{Type: "text", Text: text}},
	}
	if providerID != "" && modelID != "" {
		b.Model = &MessageModelRef{ProviderID: providerID, ModelID: modelID}
	}
	return b
}
