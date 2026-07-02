package opencode

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

	"github.com/Ceinl/plums/internal/debuglog"
)

// Client communicates with a running `opencode serve` process.
type Client struct {
	baseURL    string
	httpClient *http.Client // used for regular requests (300 s timeout)
	sseClient  *http.Client // used for SSE streams (no timeout)
}

// ── Constructors ──────────────────────────────────────────────────────────────

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

func (c *Client) BaseURL() string {
	if c == nil || c.baseURL == "" {
		return DefaultBaseURL
	}
	return c.baseURL
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

func (c *Client) CreateSession(ctx context.Context, directory string) (*Session, error) {
	body := createSessionBody{Directory: strings.TrimSpace(directory)}
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

func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]MessageResponse, error) {
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
	var raw []messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	messages := make([]MessageResponse, 0, len(raw))
	for _, msg := range raw {
		messages = append(messages, convertMessageResponse(msg))
	}
	return messages, nil
}

func (c *Client) AbortSession(ctx context.Context, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session/"+url.PathEscape(sessionID)+"/abort", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	return nil
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
	all := providers.All
	if len(all) == 0 {
		all = providers.Providers
	}
	return all, providers.Connected, nil
}

// streamBufferSize is the slack between the SSE reader and the UI consumer.
// Generous enough to ride out short consumer stalls without unbounded memory.
const streamBufferSize = 256

// SendMessage streams the assistant's reply token-by-token via the returned
// channel. It first tries the SSE-based approach (prompt_async + /event); if
// that fails (404 or connection error) it falls back to the synchronous
// /session/{id}/message endpoint.
func (c *Client) SendMessage(ctx context.Context, sessionID, text, providerID, modelID, agent string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		events := make(chan StreamEvent, streamBufferSize)
		go func() {
			defer close(events)
			result, err := c.sendWithSSE(ctx, sessionID, text, providerID, modelID, agent, events)
			if err == nil {
				return
			}
			if ctx.Err() != nil {
				return
			}
			if !result.emitted {
				c.sendSync(ctx, sessionID, text, providerID, modelID, agent, events)
				return
			}
			select {
			case <-ctx.Done():
			case out <- fmt.Sprintf("\n⚠️  SSE stream ended before completion: %v\n", err):
			}
		}()
		for event := range events {
			if event.Text == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- event.Text:
			}
		}
	}()
	return out
}

func (c *Client) SendMessageEvents(ctx context.Context, sessionID, text, providerID, modelID, agent string) <-chan StreamEvent {
	// Buffered so a brief stall in the UI consumer (e.g. a synchronous question
	// reply or post-stream model refresh) does not block the SSE reader. If the
	// reader blocks, opencode's /event buffer fills and the agent's tool calls
	// stall — the symptom that looks like "tool calls aborting".
	out := make(chan StreamEvent, streamBufferSize)
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
		case out <- StreamEvent{Text: fmt.Sprintf("\n⚠️  SSE stream ended before completion: %v\n", err)}:
		}
	}()
	return out
}

func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	body := questionReplyBody{Answers: answers}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal question reply: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/question/"+url.PathEscape(requestID)+"/reply",
		bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("question reply: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("question reply returned status %d", resp.StatusCode)
	}
	return nil
}

// grantPermission approves an opencode tool permission request so the agent can
// proceed. opencode 1.17 exposes a global reply endpoint; older builds use the
// session-scoped one, so try the former and fall back to the latter.
func (c *Client) grantPermission(ctx context.Context, sessionID, permissionID string) {
	if c.replyPermission(ctx, "/permission/"+url.PathEscape(permissionID)+"/reply", `{"reply":"once"}`) {
		return
	}
	c.replyPermission(ctx, "/session/"+url.PathEscape(sessionID)+"/permissions/"+url.PathEscape(permissionID), `{"response":"once"}`)
}

// replyPermission POSTs a permission reply and reports whether it was accepted.
func (c *Client) replyPermission(ctx context.Context, path, body string) bool {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, strings.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			debuglog.Printf("opencode: permission reply %s failed: %v", path, err)
		}
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
}

// ── Private helpers ───────────────────────────────────────────────────────────

// sendWithSSE opens a long-lived GET /event SSE connection, kicks off
// generation via POST /session/{id}/prompt_async, then reads events until
// session.idle arrives. It reports whether tokens were emitted so the caller
// only falls back to sendSync when doing so cannot duplicate output.
func (c *Client) sendWithSSE(ctx context.Context, sessionID, text, providerID, modelID, agent string, out chan<- StreamEvent) (sseResult, error) {
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
	// partTypes tracks part IDs confirmed to belong to our session.
	partTypes := make(map[string]string)
	// pendingDeltas holds deltas that arrived before their message.part.updated
	// registration event (a known opencode race – see issue #26924).
	pendingDeltas := make(map[string][]string)
	toolCallsEmitted := make(map[string]bool)

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
				if props.Part.SessionID == sessionID {
					if props.Part.Type == "tool" {
						tool := convertToolPart(props.Part)
						if tool == nil {
							continue
						}
						if toolCallsEmitted[props.Part.ID] {
							tool.Input = ""
						} else {
							toolCallsEmitted[props.Part.ID] = true
						}
						select {
						case <-ctx.Done():
							return result, ctx.Err()
						case out <- StreamEvent{Tool: tool}:
							result.emitted = true
						}
						continue
					}
					partType := props.Part.Type
					partTypes[props.Part.ID] = partType
					// Flush any deltas that arrived before this registration.
					for _, d := range pendingDeltas[props.Part.ID] {
						text := DisplayTextForPart(Part{Type: partType, Text: d})
						if text == "" {
							continue
						}
						select {
						case <-ctx.Done():
							return result, ctx.Err()
						case out <- StreamEvent{Text: text}:
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
				if partType := partTypes[props.PartID]; partType != "" {
					text := DisplayTextForPart(Part{Type: partType, Text: props.Delta})
					if text == "" {
						continue
					}
					select {
					case <-ctx.Done():
						return result, ctx.Err()
					case out <- StreamEvent{Text: text}:
						result.emitted = true
					}
				} else {
					// Delta arrived before its part.updated; queue it until we know
					// whether it is answer text, thinking text, or a non-display part.
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

			case "question.asked":
				var props questionAskedProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.SessionID == sessionID {
					select {
					case <-ctx.Done():
						return result, ctx.Err()
					case out <- StreamEvent{Question: &props}:
					}
				}

			case "permission.asked", "permission.v2.asked":
				// opencode pauses tool execution until the client replies to a
				// permission request. plums has no interactive approval UI for
				// these, so without a reply every tool call hangs forever (the
				// "tool calls aborting" symptom). Auto-grant so the agent can run.
				var props permissionAskedProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.SessionID == sessionID && props.ID != "" {
					go c.grantPermission(ctx, sessionID, props.ID)
				}

			case "message.updated":
				var props messageUpdatedProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				// Only finish == "stop" ends the turn; "tool-calls" (and its
				// completed timestamp) just closes one step of a tool loop while
				// the session keeps working toward session.idle.
				if props.SessionID == sessionID && props.Info.Role == "assistant" && props.Info.Finish == "stop" {
					result.completed = true
					return result, nil
				}

			case "session.error":
				var props sessionErrorProperties
				if err := json.Unmarshal(env.Properties, &props); err != nil {
					continue
				}
				if props.SessionID == sessionID {
					message := props.Error.Data.Message
					if message == "" {
						message = props.Error.Name
					}
					if message == "" {
						message = "unknown error"
					}
					select {
					case <-ctx.Done():
						return result, ctx.Err()
					case out <- StreamEvent{Text: fmt.Sprintf("\n⚠️  opencode error: %s\n", message)}:
						result.emitted = true
					}
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
func (c *Client) sendSync(ctx context.Context, sessionID, text, providerID, modelID, agent string, out chan<- StreamEvent) {
	b := newSendMessageBody(text, providerID, modelID, agent)
	data, err := json.Marshal(b)
	if err != nil {
		select {
		case <-ctx.Done():
		case out <- StreamEvent{Text: fmt.Sprintf("\n⚠️  marshal error: %v\n", err)}:
		}
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/session/"+url.PathEscape(sessionID)+"/message",
		bytes.NewReader(data))
	if err != nil {
		select {
		case <-ctx.Done():
		case out <- StreamEvent{Text: fmt.Sprintf("\n⚠️  request error: %v\n", err)}:
		}
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return
		case out <- StreamEvent{Text: "\n⚠️  Opencode server not available (is `opencode serve` running?)\n"}:
		}
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		select {
		case <-ctx.Done():
		case out <- StreamEvent{Text: fmt.Sprintf("\n⚠️  opencode server returned status %d\n", resp.StatusCode)}:
		}
		return
	}

	var msgResp messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		select {
		case <-ctx.Done():
		case out <- StreamEvent{Text: fmt.Sprintf("\n⚠️  decode error: %v\n", err)}:
		}
		return
	}

	for _, part := range convertMessageResponse(msgResp).Parts {
		if part.Tool != nil {
			select {
			case <-ctx.Done():
				return
			case out <- StreamEvent{Tool: part.Tool}:
			}
			continue
		}
		text := DisplayTextForPart(part)
		if text != "" {
			for _, r := range text {
				select {
				case <-ctx.Done():
					return
				case out <- StreamEvent{Text: string(r)}:
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

// ── Request / response types ──────────────────────────────────────────────────

type questionReplyBody struct {
	Answers [][]string `json:"answers"`
}

type createSessionBody struct {
	Title     string `json:"title,omitempty"`
	Directory string `json:"directory,omitempty"`
}

type sendMessageBody struct {
	Model *MessageModelRef `json:"model,omitempty"`
	Agent string           `json:"agent,omitempty"`
	Parts []Part           `json:"parts"`
}

type providerListResponse struct {
	All       []Provider `json:"all"`
	Providers []Provider `json:"providers"`
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
	Part opencodePart `json:"part"`
}

type opencodePart struct {
	ID        string            `json:"id"`
	SessionID string            `json:"sessionID"`
	Type      string            `json:"type"`
	Text      string            `json:"text"`
	Tool      string            `json:"tool"`
	State     opencodeToolState `json:"state"`
}

type opencodeToolState struct {
	Status string         `json:"status"`
	Input  map[string]any `json:"input,omitempty"`
	Raw    string         `json:"raw,omitempty"`
	Output string         `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
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

type questionAskedProperties = QuestionRequest

// permissionAskedProperties is the shared subset of "permission.asked" and
// "permission.v2.asked" event payloads that plums needs to send a reply.
type permissionAskedProperties struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
}

type messageUpdatedProperties struct {
	SessionID string `json:"sessionID"`
	Info      struct {
		Role   string `json:"role"`
		Finish string `json:"finish"`
		Time   struct {
			Completed int64 `json:"completed"`
		} `json:"time"`
	} `json:"info"`
}

type sessionErrorProperties struct {
	SessionID string `json:"sessionID"`
	Error     struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
}

type sseResult struct {
	emitted   bool
	completed bool
}

type messageResponse struct {
	Info  MessageInfo    `json:"info"`
	Parts []opencodePart `json:"parts"`
}

func convertMessageResponse(msg messageResponse) MessageResponse {
	parts := make([]Part, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		if part.Type == "tool" {
			if tool := convertToolPart(part); tool != nil {
				parts = append(parts, Part{Type: "tool", Tool: tool})
			}
			continue
		}
		parts = append(parts, Part{Type: part.Type, Text: part.Text})
	}
	return MessageResponse{Info: msg.Info, Parts: parts}
}

func convertToolPart(part opencodePart) *ToolEvent {
	input := part.State.Raw
	if input == "" && part.State.Input != nil {
		if data, err := json.Marshal(part.State.Input); err == nil {
			input = string(data)
		}
	}
	tool := &ToolEvent{
		ID:    part.ID,
		Name:  part.Tool,
		Input: input,
	}
	switch part.State.Status {
	case "completed":
		tool.Output = part.State.Output
	case "error":
		tool.Error = part.State.Error
	}
	if tool.Name == "" && tool.Input == "" && tool.Output == "" && tool.Error == "" {
		return nil
	}
	return tool
}
