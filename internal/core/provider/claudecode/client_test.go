package claudecode

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestNewUUIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := 0; i < 10; i++ {
		id, err := newUUID()
		if err != nil {
			t.Fatalf("newUUID() error: %v", err)
		}
		if !re.MatchString(id) {
			t.Errorf("newUUID() = %q, not a v4 UUID", id)
		}
	}
}

func TestCreateAndGetSession(t *testing.T) {
	c := NewClient()
	ctx := context.Background()

	session, err := c.CreateSession(ctx, "/tmp/project")
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if session.Directory != "/tmp/project" {
		t.Errorf("Directory = %q", session.Directory)
	}

	got, err := c.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("GetSession ID = %q, want %q", got.ID, session.ID)
	}

	if _, err := c.GetSession(ctx, "missing"); err == nil {
		t.Error("GetSession(missing) should fail")
	}
}

func TestBuildArgsSessionLifecycle(t *testing.T) {
	c := NewClient()

	first := strings.Join(c.buildArgs("abc", "opus"), " ")
	if !strings.Contains(first, "--session-id abc") {
		t.Errorf("first turn should use --session-id, got: %s", first)
	}
	if !strings.Contains(first, "--model opus") {
		t.Errorf("model flag missing, got: %s", first)
	}

	second := strings.Join(c.buildArgs("abc", defaultModelID), " ")
	if !strings.Contains(second, "--resume abc") {
		t.Errorf("second turn should use --resume, got: %s", second)
	}
	if strings.Contains(second, "--model") {
		t.Errorf("default model should not add --model, got: %s", second)
	}
}

func TestRecordExchangeSetsTitleAndHistory(t *testing.T) {
	c := NewClient()
	ctx := context.Background()
	session, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}

	c.recordExchange(session.ID, "fix the build\nplease", "done", "sonnet")

	got, err := c.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if got.Title != "fix the build" {
		t.Errorf("Title = %q, want first line of message", got.Title)
	}
	if got.Model == nil || got.Model.ID != "sonnet" {
		t.Errorf("Model = %+v, want sonnet", got.Model)
	}

	messages, err := c.ListMessages(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListMessages error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if messages[0].Info.Role != "user" || messages[1].Info.Role != "assistant" {
		t.Errorf("unexpected roles: %s, %s", messages[0].Info.Role, messages[1].Info.Role)
	}
}

func TestStreamLineParsing(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want func(t *testing.T, line streamLine)
	}{
		{
			name: "text delta",
			raw:  `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}}`,
			want: func(t *testing.T, line streamLine) {
				if line.Event.Delta.Text != "hi" {
					t.Errorf("Delta.Text = %q", line.Event.Delta.Text)
				}
			},
		},
		{
			name: "tool use",
			raw:  `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls"}}]}}`,
			want: func(t *testing.T, line streamLine) {
				block := line.Message.Content[0]
				if block.Name != "Bash" || block.ID != "t1" {
					t.Errorf("block = %+v", block)
				}
				if !strings.Contains(string(block.Input), "ls") {
					t.Errorf("Input = %s", block.Input)
				}
			},
		},
		{
			name: "result error",
			raw:  `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"boom"}`,
			want: func(t *testing.T, line streamLine) {
				if !line.IsError || line.Result != "boom" {
					t.Errorf("line = %+v", line)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var line streamLine
			if err := json.Unmarshal([]byte(tc.raw), &line); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			tc.want(t, line)
		})
	}
}

func TestToolResultText(t *testing.T) {
	if got := toolResultText(json.RawMessage(`"plain"`)); got != "plain" {
		t.Errorf("string content = %q", got)
	}
	blocks := json.RawMessage(`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`)
	if got := toolResultText(blocks); got != "a\nb" {
		t.Errorf("block content = %q", got)
	}
	if got := toolResultText(nil); got != "" {
		t.Errorf("empty content = %q", got)
	}
}
