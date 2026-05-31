package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/Ceinl/plums/internal/core/adapter"
)

type writeCloser struct {
	io.Writer
}

func (w writeCloser) Close() error {
	return nil
}

func TestParseEventThreadStatus(t *testing.T) {
	params := json.RawMessage(`{"threadId":"t1","status":{"type":"idle"}}`)
	event := parseEvent("thread/status/changed", params)
	if event.ThreadID != "t1" {
		t.Fatalf("expected threadId t1, got %q", event.ThreadID)
	}
	if event.Status != "idle" {
		t.Fatalf("expected status idle, got %q", event.Status)
	}
}

func TestParseEventAgentMessageDelta(t *testing.T) {
	params := json.RawMessage(`{"threadId":"t1","turnId":"turn1","itemId":"item1","delta":"hello"}`)
	event := parseEvent("item/agentMessage/delta", params)
	if event.Delta != "hello" {
		t.Fatalf("expected delta hello, got %q", event.Delta)
	}
	if event.TurnID != "turn1" {
		t.Fatalf("expected turnId turn1, got %q", event.TurnID)
	}
}

func TestParseEventItemCompletedAgentMessage(t *testing.T) {
	params := json.RawMessage(`{"item":{"type":"agentMessage","text":"world"},"threadId":"t1"}`)
	event := parseEvent("item/completed", params)
	if event.Status != "agentMessage" {
		t.Fatalf("expected type agentMessage, got %q", event.Status)
	}
	if event.Message != "world" {
		t.Fatalf("expected text world, got %q", event.Message)
	}
}

func TestParseEventError(t *testing.T) {
	params := json.RawMessage(`{"error":{"message":"not logged in"}}`)
	event := parseEvent("error", params)
	if event.Message != "not logged in" {
		t.Fatalf("expected message 'not logged in', got %q", event.Message)
	}
}

func TestParseModelList(t *testing.T) {
	raw := json.RawMessage(`{"data":[{"id":"gpt-5.5","displayName":"GPT-5.5","hidden":false},{"id":"gpt-5.4","displayName":"GPT-5.4","hidden":false},{"id":"hidden-model","hidden":true}]}`)
	models, err := parseModelList(raw)
	if err != nil {
		t.Fatalf("parseModelList error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if m, ok := models["gpt-5.5"]; !ok || m.Name != "GPT-5.5" {
		t.Fatalf("expected GPT-5.5 model")
	}
}

func TestCopySession(t *testing.T) {
	original := adapter.Session{ID: "s1", Title: "Test"}
	original.Model = &adapter.ModelRef{ID: "m1", ProviderID: "p1"}
	cp := copySession(original)
	if cp.ID != original.ID {
		t.Fatalf("copy ID mismatch")
	}
	if cp.Model == original.Model {
		t.Fatalf("copy should not share Model pointer")
	}
}

func TestReadLoopDispatchesServerRequestWithCollidingID(t *testing.T) {
	id := int64(1)
	pending := make(chan *rpcResponse, 1)
	var stdout bytes.Buffer

	s := &AppServer{
		stdin:     writeCloser{Writer: &stdout},
		scanner:   bufio.NewScanner(bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"item/tool/requestUserInput","params":{}}` + "\n"))),
		pending:   map[int64]chan *rpcResponse{id: pending},
		listeners: make(map[string]chan CodexEvent),
		done:      make(chan struct{}),
	}
	s.wg.Add(1)

	s.readLoop()

	select {
	case resp := <-pending:
		t.Fatalf("server request was incorrectly delivered as client response: %#v", resp)
	default:
	}

	var reply rpcMessage
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &reply); err != nil {
		t.Fatalf("unmarshal server request reply: %v", err)
	}
	if reply.ID == nil || *reply.ID != id {
		t.Fatalf("expected reply id %d, got %#v", id, reply.ID)
	}
	if reply.Result == nil {
		t.Fatalf("expected server request response result")
	}
}

func TestStderrIsSafeDuringStderrLoop(t *testing.T) {
	s := &AppServer{}
	r := strings.NewReader(strings.Repeat("stderr line\n", 100))

	var reads sync.WaitGroup
	reads.Add(1)
	go func() {
		defer reads.Done()
		for i := 0; i < 100; i++ {
			_ = s.Stderr()
		}
	}()

	s.wg.Add(1)
	s.stderrLoop(r)
	reads.Wait()

	if got := s.Stderr(); got == "" {
		t.Fatalf("expected stderr output")
	}
}
