package claudemirror

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
