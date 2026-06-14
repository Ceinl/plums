package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
)

func TestSendMessageBodyIncludesAgentMode(t *testing.T) {
	body := newSendMessageBody("hello", "openai", "gpt-5.5", "plan")
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["agent"] != "plan" {
		t.Fatalf("expected agent mode plan, got %#v", got["agent"])
	}
	model, ok := got["model"].(map[string]any)
	if !ok {
		t.Fatalf("expected model object, got %#v", got["model"])
	}
	if model["providerID"] != "openai" || model["modelID"] != "gpt-5.5" {
		t.Fatalf("unexpected model: %#v", model)
	}
}

func TestSendMessageBodyOmitsEmptyAgentMode(t *testing.T) {
	body := newSendMessageBody("hello", "", "", "")
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := got["agent"]; ok {
		t.Fatalf("expected empty agent mode to be omitted, got %#v", got["agent"])
	}
	if _, ok := got["model"]; ok {
		t.Fatalf("expected empty model to be omitted, got %#v", got["model"])
	}
}

func TestWaitForHealthOrExitReportsStartupExit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	proc := &ServerProcess{done: make(chan struct{})}
	proc.stderr.WriteString("startup boom")
	proc.waitErr = errors.New("exit status 42")
	close(proc.done)

	client := NewClientWithURL(server.URL)
	err := WaitForHealthOrExit(context.Background(), client, proc, time.Second)
	if err == nil {
		t.Fatalf("expected startup exit error")
	}
	if got := err.Error(); !strings.Contains(got, "startup boom") {
		t.Fatalf("expected stderr in error, got %q", got)
	}
}

func TestServerCommandArgsUsesConfiguredLocalURL(t *testing.T) {
	got, err := serverCommandArgs("http://localhost:9999")
	if err != nil {
		t.Fatalf("server args: %v", err)
	}
	want := []string{"serve", "--hostname", "localhost", "--port", "9999"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestServerCommandArgsRejectsRemoteURL(t *testing.T) {
	_, err := serverCommandArgs("http://opencode.example.com:4096")
	if err == nil {
		t.Fatalf("expected remote URL error")
	}
	if got := err.Error(); !strings.Contains(got, "non-local URL") {
		t.Fatalf("expected non-local URL error, got %q", got)
	}
}

func TestCreateSessionPostsDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/session" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		var body createSessionBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Directory != "/tmp/project" {
			t.Fatalf("expected directory %q, got %q", "/tmp/project", body.Directory)
		}
		w.WriteHeader(http.StatusCreated)
		if _, err := fmt.Fprintf(w, `{"id":"s1","directory":"%s"}`, body.Directory); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	session, err := client.CreateSession(context.Background(), " /tmp/project ")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.Directory != "/tmp/project" {
		t.Fatalf("expected session directory %q, got %q", "/tmp/project", session.Directory)
	}
}

func TestReplyQuestionPostsAnswers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/question/req-1/reply" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		var body questionReplyBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, want := body.Answers[0][0], "Yes"; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	if err := client.ReplyQuestion(context.Background(), "req-1", [][]string{{"Yes"}}); err != nil {
		t.Fatalf("reply question: %v", err)
	}
}

func TestAbortSessionPostsAbortEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/session/s1/abort" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if _, err := fmt.Fprint(w, `true`); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	if err := client.AbortSession(context.Background(), "s1"); err != nil {
		t.Fatalf("abort session: %v", err)
	}
}

func TestAbortSessionReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusConflict)
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	err := client.AbortSession(context.Background(), "s1")
	if err == nil {
		t.Fatalf("expected abort error")
	}
	if got := err.Error(); !strings.Contains(got, "status 409") {
		t.Fatalf("expected status in error, got %q", got)
	}
}

func TestListProvidersDecodesCurrentProvidersKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/provider" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"providers":[{"id":"openai","name":"OpenAI","models":{"gpt-5.5":{"id":"gpt-5.5","providerID":"openai","name":"GPT-5.5"}}}],"connected":["openai"]}`)
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	providers, connected, err := client.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if len(providers) != 1 || providers[0].ID != "openai" {
		t.Fatalf("expected openai provider, got %#v", providers)
	}
	if _, ok := providers[0].Models["gpt-5.5"]; !ok {
		t.Fatalf("expected decoded model, got %#v", providers[0].Models)
	}
	if !reflect.DeepEqual(connected, []string{"openai"}) {
		t.Fatalf("expected connected openai, got %#v", connected)
	}
}

func TestListProvidersDecodesLegacyAllKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"all":[{"id":"anthropic","name":"Anthropic","models":{}}],"connected":["anthropic"]}`)
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	providers, connected, err := client.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if len(providers) != 1 || providers[0].ID != "anthropic" {
		t.Fatalf("expected anthropic provider, got %#v", providers)
	}
	if !reflect.DeepEqual(connected, []string{"anthropic"}) {
		t.Fatalf("expected connected anthropic, got %#v", connected)
	}
}

func TestSendMessageEventsEmitsReasoningDeltasRaw(t *testing.T) {
	promptSeen := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			select {
			case <-promptSeen:
			case <-r.Context().Done():
				return
			}

			writeSSE(t, w, "message.part.delta", partDeltaProperties{SessionID: "s1", PartID: "r1", Field: "text", Delta: "secret"})
			writeSSE(t, w, "message.part.updated", partUpdatedProperties{Part: opencodePart{ID: "r1", SessionID: "s1", Type: "reasoning"}})
			writeSSE(t, w, "message.part.updated", partUpdatedProperties{Part: opencodePart{ID: "t1", SessionID: "s1", Type: "text"}})
			writeSSE(t, w, "message.part.delta", partDeltaProperties{SessionID: "s1", PartID: "t1", Field: "text", Delta: " answer"})
			writeSSE(t, w, "session.idle", sessionIdleProperties{SessionID: "s1"})
			flusher.Flush()

		case "/session/s1/prompt_async":
			close(promptSeen)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	stream := client.SendMessageEvents(context.Background(), "s1", "hello", "", "", "")
	var got strings.Builder
	for event := range stream {
		got.WriteString(event.Text)
	}

	if got.String() != "<thinking>secret</thinking> answer" {
		t.Fatalf("unexpected stream text %q", got.String())
	}
}

func TestSendMessageEventsCompletesOnAssistantMessageFinish(t *testing.T) {
	promptSeen := make(chan struct{})
	releaseSSE := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			select {
			case <-promptSeen:
			case <-r.Context().Done():
				return
			}

			writeSSE(t, w, "message.part.updated", partUpdatedProperties{Part: opencodePart{ID: "t1", SessionID: "s1", Type: "text"}})
			writeSSE(t, w, "message.part.delta", partDeltaProperties{SessionID: "s1", PartID: "t1", Field: "text", Delta: "done"})
			writeSSE(t, w, "message.updated", messageUpdatedProperties{
				SessionID: "s1",
				Info: struct {
					Role   string `json:"role"`
					Finish string `json:"finish"`
					Time   struct {
						Completed int64 `json:"completed"`
					} `json:"time"`
				}{Role: "assistant", Finish: "stop"},
			})
			flusher.Flush()
			<-releaseSSE

		case "/session/s1/prompt_async":
			close(promptSeen)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(releaseSSE)

	client := NewClientWithURL(server.URL)
	stream := client.SendMessageEvents(context.Background(), "s1", "hello", "", "", "")
	var got strings.Builder
	for event := range stream {
		got.WriteString(event.Text)
	}

	if got.String() != "done" {
		t.Fatalf("unexpected stream text %q", got.String())
	}
}

func TestSendMessageEventsAutoGrantsPermission(t *testing.T) {
	promptSeen := make(chan struct{})
	granted := make(chan struct{})
	var grantOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			select {
			case <-promptSeen:
			case <-r.Context().Done():
				return
			}

			writeSSE(t, w, "permission.asked", permissionAskedProperties{ID: "per1", SessionID: "s1"})
			flusher.Flush()

			// Hold the turn open until the grant arrives so the reply is sent
			// before the stream closes, then finish.
			select {
			case <-granted:
			case <-r.Context().Done():
				return
			}
			writeSSE(t, w, "session.idle", sessionIdleProperties{SessionID: "s1"})
			flusher.Flush()

		case "/session/s1/prompt_async":
			close(promptSeen)
			w.WriteHeader(http.StatusNoContent)

		case "/permission/per1/reply":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"reply":"once"`) {
				t.Errorf("unexpected permission reply body %q", body)
			}
			grantOnce.Do(func() { close(granted) })
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	stream := client.SendMessageEvents(context.Background(), "s1", "hello", "", "", "")
	for range stream { //nolint:revive // drain to completion
	}

	select {
	case <-granted:
	default:
		t.Fatal("expected a permission grant POST to /permission/per1/reply")
	}
}

func TestSendMessageEventsContinuesPastToolCallsFinish(t *testing.T) {
	promptSeen := make(chan struct{})
	releaseSSE := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			select {
			case <-promptSeen:
			case <-r.Context().Done():
				return
			}

			// A tool-call step ends with finish "tool-calls" and a completed
			// timestamp, but the turn is still in progress.
			writeSSE(t, w, "message.updated", messageUpdatedProperties{
				SessionID: "s1",
				Info: struct {
					Role   string `json:"role"`
					Finish string `json:"finish"`
					Time   struct {
						Completed int64 `json:"completed"`
					} `json:"time"`
				}{Role: "assistant", Finish: "tool-calls", Time: struct {
					Completed int64 `json:"completed"`
				}{Completed: 123}},
			})
			writeSSE(t, w, "message.part.updated", partUpdatedProperties{Part: opencodePart{ID: "t1", SessionID: "s1", Type: "text"}})
			writeSSE(t, w, "message.part.delta", partDeltaProperties{SessionID: "s1", PartID: "t1", Field: "text", Delta: "after tools"})
			writeSSE(t, w, "session.idle", sessionIdleProperties{SessionID: "s1"})
			flusher.Flush()
			<-releaseSSE

		case "/session/s1/prompt_async":
			close(promptSeen)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(releaseSSE)

	client := NewClientWithURL(server.URL)
	stream := client.SendMessageEvents(context.Background(), "s1", "hello", "", "", "")
	var got strings.Builder
	for event := range stream {
		got.WriteString(event.Text)
	}

	if got.String() != "after tools" {
		t.Fatalf("expected text after tool-calls finish, got %q", got.String())
	}
}

func TestSendMessageEventsSurfacesSessionError(t *testing.T) {
	promptSeen := make(chan struct{})
	releaseSSE := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			select {
			case <-promptSeen:
			case <-r.Context().Done():
				return
			}

			var props sessionErrorProperties
			props.SessionID = "s1"
			props.Error.Name = "UnknownError"
			props.Error.Data.Message = "SQLiteError: NOT NULL constraint failed"
			writeSSE(t, w, "session.error", props)
			flusher.Flush()
			<-releaseSSE

		case "/session/s1/prompt_async":
			close(promptSeen)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer close(releaseSSE)

	client := NewClientWithURL(server.URL)
	stream := client.SendMessageEvents(context.Background(), "s1", "hello", "", "", "")
	var got strings.Builder
	for event := range stream {
		got.WriteString(event.Text)
	}

	want := "\n⚠️  opencode error: SQLiteError: NOT NULL constraint failed\n"
	if got.String() != want {
		t.Fatalf("unexpected stream text %q, want %q", got.String(), want)
	}
}

func TestDisplayTextForPartWrapsReasoning(t *testing.T) {
	if got := adapter.DisplayTextForPart(adapter.Part{Type: "reasoning", Text: "secret"}); got != "<thinking>secret</thinking>" {
		t.Fatalf("expected wrapped reasoning, got %q", got)
	}
	if got := adapter.DisplayTextForPart(adapter.Part{Type: "text", Text: "answer"}); got != "answer" {
		t.Fatalf("expected text part, got %q", got)
	}
}

func TestSendMessageEventsEmitsToolPartUpdates(t *testing.T) {
	promptSeen := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("expected flusher")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher.Flush()

			select {
			case <-promptSeen:
			case <-r.Context().Done():
				return
			}

			writeSSE(t, w, "message.part.updated", partUpdatedProperties{Part: opencodePart{
				ID:        "tool1",
				SessionID: "s1",
				Type:      "tool",
				Tool:      "read",
				State: opencodeToolState{
					Status: "running",
					Input:  map[string]any{"filePath": "/tmp/example.go"},
				},
			}})
			writeSSE(t, w, "message.part.updated", partUpdatedProperties{Part: opencodePart{
				ID:        "tool1",
				SessionID: "s1",
				Type:      "tool",
				Tool:      "read",
				State: opencodeToolState{
					Status: "completed",
					Input:  map[string]any{"filePath": "/tmp/example.go"},
					Output: "package main",
				},
			}})
			writeSSE(t, w, "session.idle", sessionIdleProperties{SessionID: "s1"})
			flusher.Flush()

		case "/session/s1/prompt_async":
			close(promptSeen)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL)
	stream := client.SendMessageEvents(context.Background(), "s1", "hello", "", "", "")
	var got []adapter.ToolEvent
	for event := range stream {
		if event.Tool != nil {
			got = append(got, *event.Tool)
		}
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 tool events, got %#v", got)
	}
	if got[0].ID != "tool1" || got[0].Name != "read" || got[0].Input != `{"filePath":"/tmp/example.go"}` {
		t.Fatalf("unexpected tool call event %#v", got[0])
	}
	if got[1].ID != "tool1" || got[1].Input != "" || got[1].Output != "package main" {
		t.Fatalf("unexpected tool output event %#v", got[1])
	}
}

func writeSSE(t *testing.T, w http.ResponseWriter, eventType string, properties any) {
	t.Helper()
	rawProps, err := json.Marshal(properties)
	if err != nil {
		t.Fatalf("marshal properties: %v", err)
	}
	rawEvent, err := json.Marshal(sseEnvelope{Type: eventType, Properties: rawProps})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", rawEvent); err != nil {
		t.Fatalf("write event: %v", err)
	}
}
