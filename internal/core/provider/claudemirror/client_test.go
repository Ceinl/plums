package claudemirror

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
)

func TestEncodeProjectDir(t *testing.T) {
	got := encodeProjectDir("/Users/c/code/plums")
	want := "-Users-c-code-plums"
	if got != want {
		t.Fatalf("encodeProjectDir = %q, want %q", got, want)
	}
}

const sampleTranscript = `{"type":"mode","mode":"normal","sessionId":"abc"}
{"type":"user","isMeta":true,"message":{"role":"user","content":"<local-command-caveat>ignored</local-command-caveat>"},"uuid":"u0","sessionId":"abc","cwd":"/tmp/p","timestamp":"2026-06-13T10:00:00.000Z"}
{"type":"user","message":{"role":"user","content":"fix the build"},"uuid":"u1","sessionId":"abc","cwd":"/tmp/p","timestamp":"2026-06-13T10:00:01.000Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Looking."},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"go build"}}],"stop_reason":"tool_use"},"uuid":"a1","sessionId":"abc","timestamp":"2026-06-13T10:00:02.000Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]},"uuid":"u2","sessionId":"abc","timestamp":"2026-06-13T10:00:03.000Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done."}],"stop_reason":"end_turn"},"uuid":"a2","sessionId":"abc","timestamp":"2026-06-13T10:00:04.000Z"}
`

func writeSample(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	if err := os.WriteFile(path, []byte(sampleTranscript), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadTranscriptAndTitle(t *testing.T) {
	entries, err := readTranscript(writeSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 6 {
		t.Fatalf("got %d entries, want 6", len(entries))
	}
	if title := transcriptTitle(entries, "fallback"); title != "fix the build" {
		t.Fatalf("title = %q, want %q", title, "fix the build")
	}
}

func TestEntryPartsSkipsHarnessText(t *testing.T) {
	entries, err := readTranscript(writeSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if parts := entryParts(entries[1]); len(parts) != 0 {
		t.Fatalf("harness caveat produced parts: %+v", parts)
	}
	parts := entryParts(entries[3])
	if len(parts) != 2 || parts[0].Text != "Looking." || parts[1].Tool == nil || parts[1].Tool.Name != "Bash" {
		t.Fatalf("unexpected assistant parts: %+v", parts)
	}
	parts = entryParts(entries[4])
	if len(parts) != 1 || parts[0].Tool == nil || parts[0].Tool.Output != "ok" {
		t.Fatalf("unexpected tool_result parts: %+v", parts)
	}
}

func TestReadNewEntriesHandlesPartialLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	full := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn"},"uuid":"a1"}` + "\n"
	if err := os.WriteFile(path, []byte(full+`{"type":"assist`), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, offset, err := readNewEntries(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].UUID != "a1" {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	if offset != int64(len(full)) {
		t.Fatalf("offset = %d, want %d (partial line must not advance offset)", offset, len(full))
	}
}

func TestIsInteractiveClaudeCommand(t *testing.T) {
	cases := map[string]bool{
		"claude":                             true,
		"/usr/local/bin/claude --resume abc": true,
		"node /opt/cc/claude":                true,
		"node /opt/node_modules/@anthropic-ai/claude-code/cli.js": true,
		"bun /opt/claude-code/cli.js":                             true,
		"claude -p --output-format json":                          false,
		"claude --version":                                        false,
		"claude --output-format=stream-json":                      false,
		"vim notes.md":                                            false,
		// Claude desktop app spawns headless stream-json instances.
		"/Applications/Claude.app/x/claude --output-format stream-json --input-format stream-json --permission-prompt-tool stdio": false,
	}
	for command, want := range cases {
		if got := isInteractiveClaudeCommand(command); got != want {
			t.Errorf("isInteractiveClaudeCommand(%q) = %t, want %t", command, got, want)
		}
	}
}

func TestPairGroupMatchesByStartOrder(t *testing.T) {
	t0 := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	procs := []claudeProc{
		{pid: 2, started: t0.Add(time.Hour), cwd: "/tmp/p"},
		{pid: 1, started: t0, cwd: "/tmp/p"},
	}
	transcripts := []transcriptFile{
		{path: "old.jsonl", sessionID: "old", born: t0.Add(time.Minute), modified: t0.Add(2 * time.Hour)},
		{path: "stale.jsonl", sessionID: "stale", born: t0.Add(-24 * time.Hour), modified: t0.Add(-23 * time.Hour)},
		{path: "new.jsonl", sessionID: "new", born: t0.Add(time.Hour + time.Minute), modified: t0.Add(90 * time.Minute)},
	}
	instances := pairGroup(procs, transcripts)
	if len(instances) != 2 {
		t.Fatalf("got %d instances, want 2", len(instances))
	}
	got := map[int]string{}
	for _, inst := range instances {
		got[inst.pid] = inst.sessionID
	}
	if got[1] != "old" || got[2] != "new" {
		t.Fatalf("pairing = %v, want pid1->old pid2->new (stale dropped)", got)
	}
}

func TestHasPendingQuestion(t *testing.T) {
	ask := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"q1","name":"AskUserQuestion","input":{"questions":[{"question":"Which?","options":[{"label":"A"},{"label":"B"}]}]}}],"stop_reason":"tool_use"},"uuid":"a1"}` + "\n"
	answer := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"q1","content":"A"}]},"uuid":"u1"}` + "\n"
	typed := `{"type":"user","message":{"role":"user","content":"moved on"},"uuid":"u2"}` + "\n"

	parse := func(raw string) []transcriptEntry {
		path := filepath.Join(t.TempDir(), "t.jsonl")
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		entries, err := readTranscript(path)
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}
	if !hasPendingQuestion(parse(ask)) {
		t.Error("unanswered question should be pending")
	}
	if hasPendingQuestion(parse(ask + answer)) {
		t.Error("answered question should not be pending")
	}
	if hasPendingQuestion(parse(ask + typed)) {
		t.Error("dismissed question should not be pending")
	}
	if got := pendingQuestionID(parse(ask)); got != "q1" {
		t.Fatalf("pendingQuestionID = %q, want q1", got)
	}
}

// TestTailTurnSurvivesLeakedEndTurn reproduces the "question tool not always
// mirrored on the second prompt" bug: the previous turn's trailing end_turn
// line flushes late and shares a poll batch with the new typed prompt, so the
// tail starts at an offset that includes it. Without resetting turnEnded on the
// fresh prompt, the next empty poll ends the turn before the assistant's
// AskUserQuestion is written, and Plums never sees the question.
func TestTailTurnSurvivesLeakedEndTurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	// Stale end_turn from the previous turn, immediately followed by the new
	// typed prompt — both already on disk when the tail begins at offset 0.
	leaked := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"earlier answer"}],"stop_reason":"end_turn"},"uuid":"a0","sessionId":"abc"}` + "\n"
	prompt := `{"type":"user","message":{"role":"user","content":"using question tool ask me 1 or 2"},"uuid":"u1","sessionId":"abc"}` + "\n"
	if err := os.WriteFile(path, []byte(leaked+prompt), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := make(chan adapter.StreamEvent)
	done := make(chan error, 1)
	go func() { done <- tailTurn(ctx, out, path, 0, "", 0) }()

	// After the tail has consumed the initial batch and polled at least once
	// more (where the buggy version returns early), the assistant asks.
	go func() {
		time.Sleep(3 * tailPollInterval)
		ask := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"q1","name":"AskUserQuestion","input":{"questions":[{"question":"1 or 2?","options":[{"label":"1"},{"label":"2"}]}]}}],"stop_reason":"tool_use"},"uuid":"a1","sessionId":"abc"}` + "\n"
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		f.WriteString(ask)
		f.Close()
	}()

	for {
		select {
		case ev := <-out:
			if ev.Question != nil {
				if ev.Question.ID != "abc:q1" {
					t.Fatalf("question id = %q, want abc:q1", ev.Question.ID)
				}
				return // success: the question reached Plums despite the leak
			}
		case err := <-done:
			t.Fatalf("tailTurn returned before mirroring the question (err=%v)", err)
		case <-ctx.Done():
			t.Fatal("timed out waiting for the question event")
		}
	}
}

func TestAskQuestionID(t *testing.T) {
	parse := func(raw string) transcriptEntry {
		path := filepath.Join(t.TempDir(), "t.jsonl")
		if err := os.WriteFile(path, []byte(raw+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		entries, err := readTranscript(path)
		if err != nil || len(entries) != 1 {
			t.Fatalf("readTranscript: %v (%d entries)", err, len(entries))
		}
		return entries[0]
	}
	ask := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"q1","name":"AskUserQuestion","input":{"questions":[{"question":"?"}]}}]},"uuid":"a1"}`
	if got := askQuestionID(parse(ask)); got != "q1" {
		t.Fatalf("askQuestionID = %q, want q1", got)
	}
	bash := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"b1","name":"Bash","input":{}}]},"uuid":"a2"}`
	if got := askQuestionID(parse(bash)); got != "" {
		t.Fatalf("askQuestionID(non-question) = %q, want empty", got)
	}
	result := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"q1","content":"declined"}]},"uuid":"u1"}`
	if got := askQuestionID(parse(result)); got != "" {
		t.Fatalf("askQuestionID(user entry) = %q, want empty", got)
	}
}

// TestEmitEntrySuppressesDeclinedResult verifies the synthetic "user declined"
// tool_result from a question the mirror auto-declined is dropped, while the
// same result is mirrored normally when it is not the suppressed question.
func TestEmitEntrySuppressesDeclinedResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	raw := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"q1","content":"The user doesn't want to proceed","is_error":true}]},"uuid":"u1"}` + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	collect := func(suppress string) []adapter.StreamEvent {
		out := make(chan adapter.StreamEvent, 8)
		emitEntry(context.Background(), out, entries[0], suppress)
		close(out)
		var got []adapter.StreamEvent
		for ev := range out {
			got = append(got, ev)
		}
		return got
	}
	if got := collect("q1"); len(got) != 0 {
		t.Fatalf("suppressed result still emitted %d events: %+v", len(got), got)
	}
	if got := collect("other"); len(got) != 1 || got[0].Tool == nil || got[0].Tool.Error == "" {
		t.Fatalf("unrelated result should pass through as a tool error: %+v", got)
	}
}

func TestTranscriptPermissionMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	raw := `{"type":"permission-mode","permissionMode":"default","sessionId":"s"}` + "\n" +
		`{"type":"user","message":{"role":"user","content":"hi"},"uuid":"u1"}` + "\n" +
		`{"type":"permission-mode","permissionMode":"bypassPermissions","sessionId":"s"}` + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := transcriptPermissionMode(entries); got != "bypassPermissions" {
		t.Fatalf("transcriptPermissionMode = %q, want bypassPermissions (latest)", got)
	}
	if got := transcriptPermissionMode(nil); got != "" {
		t.Fatalf("transcriptPermissionMode(nil) = %q, want empty", got)
	}
}

func TestReadSessionState(t *testing.T) {
	if !(sessionState{Status: "waiting"}).isWaitingForInput() {
		t.Error(`status "waiting" should be waiting for input`)
	}
	if (sessionState{Status: "busy"}).isWaitingForInput() {
		t.Error(`status "busy" should not be waiting for input`)
	}
	// A non-existent pid must error rather than report a bogus state.
	if _, err := readSessionState(0); err == nil {
		t.Error("readSessionState(0) should error")
	}
}

func TestParseQuestionRequest(t *testing.T) {
	req := parseQuestionRequest("s1", "q1", `{"questions":[{"question":"Which?","header":"Pick","options":[{"label":"A","description":"first"},{"label":"B"}],"multiple":true}]}`)
	if req == nil {
		t.Fatal("expected question request")
	}
	if req.ID != "s1:q1" || req.SessionID != "s1" {
		t.Fatalf("unexpected ids: %+v", req)
	}
	q := req.Questions[0]
	if q.Question != "Which?" || q.Header != "Pick" || !q.Multiple || len(q.Options) != 2 || q.Options[0].Description != "first" {
		t.Fatalf("unexpected question: %+v", q)
	}
	if parseQuestionRequest("", "q1", `{"questions":[{"question":"Which?"}]}`) != nil {
		t.Fatal("missing session id must not produce an answerable request")
	}
}

func TestQuestionRequestIDRoundTrip(t *testing.T) {
	sessionID, toolUseID, ok := splitQuestionRequestID(questionRequestID("live-123", "tool-1"))
	if !ok || sessionID != "live-123" || toolUseID != "tool-1" {
		t.Fatalf("split = %q, %q, %t", sessionID, toolUseID, ok)
	}
	if _, _, ok := splitQuestionRequestID("bad"); ok {
		t.Fatal("invalid request id should not parse")
	}
}

func TestQuestionAnswerText(t *testing.T) {
	got := questionAnswerText([][]string{{" A ", "B"}, {"custom"}, {" "}})
	if got != "A, B\ncustom" {
		t.Fatalf("questionAnswerText = %q", got)
	}
}

func TestAwaitActiveTranscriptLocksOnNewFile(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "stale.jsonl")
	if err := os.WriteFile(stale, []byte(`{"type":"user","message":{"role":"user","content":"old"},"uuid":"u0"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := snapshotSizes(dir)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Unrelated growth in the stale file must not be locked onto.
		f, _ := os.OpenFile(stale, os.O_APPEND|os.O_WRONLY, 0o644)
		f.WriteString(`{"type":"user","message":{"role":"user","content":"other session"},"uuid":"u1"}` + "\n")
		f.Close()
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "fresh.jsonl"),
			[]byte(`{"type":"user","message":{"role":"user","content":"hello mirror"},"uuid":"u2"}`+"\n"), 0o644)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path, offset, err := awaitActiveTranscript(ctx, dir, baseline, "hello mirror")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "fresh.jsonl" || offset != 0 {
		t.Fatalf("locked onto %s@%d, want fresh.jsonl@0", path, offset)
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("own pid should be reported alive")
	}
	// A pid far above any plausible live process should be dead.
	if processAlive(1 << 30) {
		t.Error("nonexistent pid should be reported dead")
	}
	if processAlive(0) {
		t.Error("pid 0 should be reported dead")
	}
}

func TestTranscriptModel(t *testing.T) {
	entries, err := readTranscript(writeSample(t))
	if err != nil {
		t.Fatal(err)
	}
	// The sample has no model field; absence must not panic and returns "".
	if got := transcriptModel(entries); got != "" {
		t.Fatalf("transcriptModel = %q, want empty", got)
	}

	path := filepath.Join(t.TempDir(), "m.jsonl")
	raw := `{"type":"assistant","message":{"role":"assistant","model":"claude-opus-4-8","content":[{"type":"text","text":"old"}]},"uuid":"a1"}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","model":"claude-fable-5","content":[{"type":"text","text":"new"}]},"uuid":"a2"}` + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	withModel, err := readTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := transcriptModel(withModel); got != "claude-fable-5" {
		t.Fatalf("transcriptModel = %q, want claude-fable-5 (latest)", got)
	}
}

func TestPasteMarker(t *testing.T) {
	cases := map[string]string{
		"hello":                 "hello",
		"  spaced  ":            "spaced",
		"line one\nline two":    "",
		"":                      "",
		strings.Repeat("x", 50): "",
	}
	for in, want := range cases {
		if got := pasteMarker(in); got != want {
			t.Errorf("pasteMarker(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAwaitActiveTranscriptAcceptsRewrittenPrompt covers the paste-placeholder
// and slash-command-wrapper cases: the literal text never lands in the
// transcript, but a fresh typed user entry in a grown file must still be
// recognised as the prompt arriving.
func TestAwaitActiveTranscriptAcceptsRewrittenPrompt(t *testing.T) {
	dir := t.TempDir()
	baseline, err := snapshotSizes(dir)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(80 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "fresh.jsonl"),
			[]byte(`{"type":"user","message":{"role":"user","content":"[Pasted text #1 +40 lines]"},"uuid":"u1"}`+"\n"), 0o644)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The injected text was a big multi-line paste; its verbatim form never
	// appears, but the placeholder entry must still be locked onto.
	path, offset, err := awaitActiveTranscript(ctx, dir, baseline, "a very long\nmulti line prompt that got collapsed")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "fresh.jsonl" || offset != 0 {
		t.Fatalf("locked onto %s@%d, want fresh.jsonl@0", path, offset)
	}
}

// TestAwaitActiveTranscriptSlashCommandTimesOut covers a local slash command
// that writes nothing: it must fail fast with errPromptNotSeen rather than
// blocking for the full firstReplyTimeout.
func TestAwaitActiveTranscriptSlashCommandTimesOut(t *testing.T) {
	prev := commandReplyTimeout
	commandReplyTimeout = 200 * time.Millisecond
	defer func() { commandReplyTimeout = prev }()

	dir := t.TempDir()
	baseline, err := snapshotSizes(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, _, err = awaitActiveTranscript(ctx, dir, baseline, "/clear")
	if !errors.Is(err, errPromptNotSeen) {
		t.Fatalf("err = %v, want errPromptNotSeen", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("slash command waited %s, expected to give up near commandReplyTimeout", elapsed)
	}
}

// TestResetSessionRequiresWindow verifies ResetSession refuses (rather than
// injecting blindly) when no Claude Code window is running. ps/lsof report no
// interactive claude process in the test environment, so discovery is empty.
func TestResetSessionRequiresWindow(t *testing.T) {
	type resetter interface {
		ResetSession(ctx context.Context, directory string) (*adapter.Session, error)
	}
	r, ok := NewBackend().(resetter)
	if !ok {
		t.Fatal("claude-mirror backend must implement ResetSession")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The assertion only holds when no interactive Claude window is running. On
	// a developer machine with one open, ResetSession would legitimately find it
	// and not error, so skip rather than fail on ambient host state.
	if procs, err := interactiveClaudeProcesses(ctx); err == nil && len(procs) > 0 {
		t.Skip("interactive Claude Code window running; ResetSession would target it")
	}
	if _, err := r.ResetSession(ctx, "/no/such/dir"); err == nil {
		t.Fatal("ResetSession should fail when no Claude Code window is running")
	}
}

func TestFormatQuestions(t *testing.T) {
	text := formatQuestions(`{"questions":[{"question":"Which?","options":[{"label":"A"},{"label":"B"}]}]}`)
	for _, want := range []string{"Which?", "- A", "- B", "Answer in the Claude Code window"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatted questions missing %q in %q", want, text)
		}
	}
	if formatQuestions("not json") != "" {
		t.Error("invalid input should format to empty string")
	}
}
