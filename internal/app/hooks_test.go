package app

import (
	"context"
	"testing"
	"time"

	"github.com/Ceinl/plums/capabilities"
)

type messageHook struct {
	ch chan capabilities.Message
}

func (h messageHook) OnMessage(ctx capabilities.Ctx, message capabilities.Message) {
	h.ch <- message
}

type sessionHook struct {
	ch chan capabilities.Session
}

func (h sessionHook) OnSessionStart(ctx capabilities.Ctx, session capabilities.Session) {
	h.ch <- session
}

type toolHook struct {
	ch chan capabilities.ToolCall
}

func (h toolHook) OnToolCall(ctx capabilities.Ctx, tool capabilities.ToolCall) {
	h.ch <- tool
}

func TestRunMessageHooks(t *testing.T) {
	ch := make(chan capabilities.Message, 1)
	runMessageHooks(context.Background(), []capabilities.OnMessage{messageHook{ch: ch}}, newRuntimeCtx(NewState(80, 24), RunConfig{}, nil, nil), capabilities.Message{Role: "user", Content: "hello"})

	select {
	case got := <-ch:
		if got.Role != "user" || got.Content != "hello" {
			t.Fatalf("message = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("message hook did not run")
	}
}

func TestRunSessionStartHooks(t *testing.T) {
	ch := make(chan capabilities.Session, 1)
	runSessionStartHooks(context.Background(), []capabilities.OnSessionStart{sessionHook{ch: ch}}, newRuntimeCtx(NewState(80, 24), RunConfig{}, nil, nil), capabilities.Session{ID: "s1"})

	select {
	case got := <-ch:
		if got.ID != "s1" {
			t.Fatalf("session = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("session hook did not run")
	}
}

func TestRunToolCallHooks(t *testing.T) {
	ch := make(chan capabilities.ToolCall, 1)
	runToolCallHooks(context.Background(), []capabilities.OnToolCall{toolHook{ch: ch}}, newRuntimeCtx(NewState(80, 24), RunConfig{}, nil, nil), capabilities.ToolCall{ID: "tool1", Name: "read"})

	select {
	case got := <-ch:
		if got.ID != "tool1" || got.Name != "read" {
			t.Fatalf("tool = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("tool hook did not run")
	}
}
