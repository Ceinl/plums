package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Ceinl/plums/internal/debuglog"
)

const (
	maxLineSize = 1024 * 1024
	bufferSize  = 64 * 1024
)

// AppServer manages a `codex app-server` child process and speaks the Codex
// App Server Protocol (JSON-RPC over stdio) with it.
type AppServer struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	scanner   *bufio.Scanner
	stderrBuf bytes.Buffer
	stderrMu  sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan *rpcResponse

	listeners   map[string]chan CodexEvent
	listenersMu sync.Mutex

	done     chan struct{}
	doneOnce sync.Once
	stopOnce sync.Once
	wg       sync.WaitGroup
}

type rpcMessage struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Result json.RawMessage
	Error  *rpcError
}

// CodexEvent is a parsed notification from the Codex app-server.
type CodexEvent struct {
	Method   string
	Params   json.RawMessage
	ThreadID string
	TurnID   string
	ItemID   string
	Delta    string
	Message  string
	Status   string
}

// NewAppServer spawns `codex app-server`, initializes the JSON-RPC protocol,
// and returns a ready AppServer.
func NewAppServer(ctx context.Context, directory string) (*AppServer, error) {
	if _, err := exec.LookPath("codex"); err != nil {
		return nil, fmt.Errorf("codex executable not found: %w", err)
	}

	cmd := exec.CommandContext(ctx, "codex", "app-server")
	cmd.Dir = directory

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}
	debuglog.Printf("codex: app-server started pid=%d", cmd.Process.Pid)

	s := &AppServer{
		cmd:       cmd,
		stdin:     stdin,
		scanner:   bufio.NewScanner(stdout),
		pending:   make(map[int64]chan *rpcResponse),
		listeners: make(map[string]chan CodexEvent),
		done:      make(chan struct{}),
	}
	s.scanner.Buffer(make([]byte, 0, bufferSize), maxLineSize)

	// Drain stderr in the background so the pipe does not block.
	s.wg.Add(1)
	go s.stderrLoop(stderr)

	// Read stdout JSON-RPC messages.
	s.wg.Add(1)
	go s.readLoop()

	// Initialize protocol.
	if err := s.initialize(ctx); err != nil {
		s.Stop()
		return nil, fmt.Errorf("codex app-server initialization failed: %w", err)
	}

	return s, nil
}

func (s *AppServer) initialize(ctx context.Context) error {
	_, err := s.Request(ctx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{
			"name":    "plums",
			"version": "0.1.0",
		},
		"capabilities": map[string]interface{}{},
	})
	if err != nil {
		return err
	}
	return s.Notify(ctx, "initialized", nil)
}

// Request sends a JSON-RPC request and waits for the matching response.
func (s *AppServer) Request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&s.nextID, 1)
	ch := make(chan *rpcResponse, 1)

	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	msg := rpcMessage{ID: &id, Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		msg.Params = json.RawMessage(b)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	if err := s.write(data); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("codex rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// Notify sends a JSON-RPC notification (no response expected).
func (s *AppServer) Notify(ctx context.Context, method string, params interface{}) error {
	msg := rpcMessage{Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		msg.Params = json.RawMessage(b)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.write(data)
}

func (s *AppServer) write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
		return fmt.Errorf("app-server closed")
	default:
	}
	_, err := s.stdin.Write(data)
	return err
}

func (s *AppServer) readLoop() {
	defer s.wg.Done()
	defer s.markDone()
	defer s.closeAllListeners()

	for s.scanner.Scan() {
		line := bytes.TrimSpace(s.scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			debuglog.Printf("codex: unmarshal rpc line: %v", err)
			continue
		}

		if msg.ID != nil && msg.Method == "" {
			// Response to a pending client request.
			s.mu.Lock()
			ch, ok := s.pending[*msg.ID]
			s.mu.Unlock()
			if ok {
				ch <- &rpcResponse{Result: msg.Result, Error: msg.Error}
			}
			continue
		}

		if msg.ID != nil && msg.Method != "" {
			// Server-initiated request.
			s.handleServerRequest(*msg.ID, msg.Method, msg.Params)
			continue
		}

		if msg.Method != "" {
			// Notification from server.
			event := parseEvent(msg.Method, msg.Params)
			s.listenersMu.Lock()
			if ch, ok := s.listeners[event.ThreadID]; ok {
				select {
				case ch <- event:
				default:
				}
			}
			s.listenersMu.Unlock()
		}
	}

	if err := s.scanner.Err(); err != nil {
		debuglog.Printf("codex: app-server stdout scanner error: %v", err)
	}
	debuglog.Printf("codex: app-server read loop exited")
}

func (s *AppServer) handleServerRequest(id int64, method string, params json.RawMessage) {
	switch method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		_ = s.respond(id, map[string]string{"decision": "approve"})
	case "item/tool/requestUserInput":
		_ = s.respond(id, map[string]interface{}{"answers": map[string]interface{}{}})
	default:
		_ = s.respondError(id, -32601, "Method not found: "+method)
	}
}

func (s *AppServer) respond(id int64, result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	msg := rpcMessage{ID: &id, Result: json.RawMessage(b)}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.write(data)
}

func (s *AppServer) respondError(id int64, code int, message string) error {
	msg := rpcMessage{
		ID:    &id,
		Error: &rpcError{Code: code, Message: message},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.write(data)
}

func (s *AppServer) stderrLoop(r io.Reader) {
	defer s.wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		debuglog.Printf("codex: app-server stderr: %s", line)
		s.stderrMu.Lock()
		s.stderrBuf.WriteString(line)
		s.stderrBuf.WriteByte('\n')
		s.stderrMu.Unlock()
	}
}

// Subscribe registers a channel to receive CodexEvents for the given threadID.
func (s *AppServer) Subscribe(threadID string) <-chan CodexEvent {
	ch := make(chan CodexEvent, 64)
	s.listenersMu.Lock()
	s.listeners[threadID] = ch
	s.listenersMu.Unlock()
	return ch
}

// Unsubscribe removes the listener for threadID and closes its channel.
func (s *AppServer) Unsubscribe(threadID string) {
	s.listenersMu.Lock()
	if ch, ok := s.listeners[threadID]; ok {
		close(ch)
		delete(s.listeners, threadID)
	}
	s.listenersMu.Unlock()
}

func (s *AppServer) closeAllListeners() {
	s.listenersMu.Lock()
	for id, ch := range s.listeners {
		close(ch)
		delete(s.listeners, id)
	}
	s.listenersMu.Unlock()
}

// Stop kills the app-server process and cleans up goroutines.
func (s *AppServer) Stop() {
	s.stopOnce.Do(func() {
		s.markDone()
		_ = s.stdin.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		s.wg.Wait()
		debuglog.Printf("codex: app-server stopped")
	})
}

func (s *AppServer) markDone() {
	s.doneOnce.Do(func() { close(s.done) })
}

// Done returns a channel that is closed when the read loop exits.
func (s *AppServer) Done() <-chan struct{} {
	return s.done
}

// Stderr returns the accumulated stderr output.
func (s *AppServer) Stderr() string {
	s.stderrMu.Lock()
	defer s.stderrMu.Unlock()
	return strings.TrimSpace(s.stderrBuf.String())
}

// parseEvent converts a raw JSON-RPC notification into a typed CodexEvent.
func parseEvent(method string, params json.RawMessage) CodexEvent {
	event := CodexEvent{Method: method, Params: params}

	var common struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		ItemID   string `json:"itemId"`
		Delta    string `json:"delta"`
		Message  string `json:"message"`
	}
	_ = json.Unmarshal(params, &common)
	event.ThreadID = common.ThreadID
	event.TurnID = common.TurnID
	event.ItemID = common.ItemID
	event.Delta = common.Delta
	event.Message = common.Message

	if method == "thread/status/changed" {
		var status struct {
			Status struct {
				Type string `json:"type"`
			} `json:"status"`
		}
		_ = json.Unmarshal(params, &status)
		event.Status = status.Status.Type
	}

	if method == "item/completed" {
		var item struct {
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		_ = json.Unmarshal(params, &item)
		event.Status = item.Item.Type
		event.Message = item.Item.Text
	}

	if method == "error" {
		var payload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(params, &payload)
		if payload.Error.Message != "" {
			event.Message = payload.Error.Message
		}
	}

	return event
}
