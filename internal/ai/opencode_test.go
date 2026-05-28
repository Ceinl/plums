package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
