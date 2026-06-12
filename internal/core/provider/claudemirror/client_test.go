package claudemirror

import (
	"os"
	"path/filepath"
	"testing"
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
		"claude -p --output-format json":     false,
		"claude --version":                   false,
		"vim notes.md":                       false,
	}
	for command, want := range cases {
		if got := isInteractiveClaudeCommand(command); got != want {
			t.Errorf("isInteractiveClaudeCommand(%q) = %t, want %t", command, got, want)
		}
	}
}
