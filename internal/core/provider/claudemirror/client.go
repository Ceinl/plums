package claudemirror

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/debuglog"
)

const (
	providerID     = "claude-mirror"
	defaultModelID = "live"

	tailPollInterval  = 300 * time.Millisecond
	firstReplyTimeout = 60 * time.Second
	idleTimeout       = 10 * time.Minute
)

// Client implements adapter.Backend by attaching to already-running
// interactive Claude Code instances: prompts are typed into the real
// window's tmux pane and output is mirrored by tailing the session's
// JSONL transcript under ~/.claude/projects. The real window stays the
// driver; this backend never spawns headless `claude -p` turns.
type Client struct {
	mu   sync.Mutex
	pids map[string]int // session id -> owning interactive claude pid
}

func NewBackend() adapter.Backend {
	return &Client{pids: make(map[string]int)}
}

func (c *Client) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := projectsDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("claude transcripts directory not found (has Claude Code run before?): %w", err)
	}
	instances, err := discoverInstances(ctx)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return fmt.Errorf("no running interactive Claude Code window found; open one in a terminal first")
	}
	return nil
}

// CreateSession attaches to a running Claude Code window rather than creating
// anything: it prefers an instance whose cwd matches directory, then any
// live instance.
func (c *Client) CreateSession(ctx context.Context, directory string) (*adapter.Session, error) {
	instances, err := discoverInstances(ctx)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("no running interactive Claude Code instance found; open one in a terminal first")
	}
	chosen := instances[0]
	for _, inst := range instances {
		if inst.cwd == directory {
			chosen = inst
			break
		}
	}
	c.rememberPID(chosen.sessionID, chosen.pid)
	session := c.sessionFromInstance(chosen)
	return &session, nil
}

func (c *Client) ListSessions(ctx context.Context) ([]adapter.Session, error) {
	instances, err := discoverInstances(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]adapter.Session, 0, len(instances))
	for _, inst := range instances {
		c.rememberPID(inst.sessionID, inst.pid)
		sessions = append(sessions, c.sessionFromInstance(inst))
	}
	return sessions, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*adapter.Session, error) {
	instances, err := discoverInstances(ctx)
	if err == nil {
		for _, inst := range instances {
			if inst.sessionID == sessionID {
				c.rememberPID(inst.sessionID, inst.pid)
				session := c.sessionFromInstance(inst)
				return &session, nil
			}
		}
	}
	// The window may have closed; the transcript still mirrors history.
	path, err := findTranscript(sessionID)
	if err != nil {
		return nil, fmt.Errorf("claude-mirror session %q not found: %w", sessionID, err)
	}
	session := c.sessionFromTranscript(sessionID, path, "")
	return &session, nil
}

func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]adapter.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := findTranscript(sessionID)
	if err != nil {
		return nil, err
	}
	entries, err := readTranscript(path)
	if err != nil {
		return nil, err
	}
	var messages []adapter.MessageResponse
	for _, entry := range entries {
		if !conversationEntry(entry) {
			continue
		}
		parts := entryParts(entry)
		if len(parts) == 0 {
			continue
		}
		messages = append(messages, adapter.MessageResponse{
			Info:  adapter.MessageInfo{ID: entry.UUID, Role: entry.Message.Role},
			Parts: parts,
		})
	}
	return messages, nil
}

func (c *Client) ListProviders(ctx context.Context) ([]adapter.Provider, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	provider := adapter.Provider{
		ID:   providerID,
		Name: "Claude Mirror",
		Models: map[string]adapter.Model{
			// The real window decides the model; this is a placeholder.
			defaultModelID: {ID: defaultModelID, ProviderID: providerID, Name: "Live window"},
		},
	}
	return []adapter.Provider{provider}, []string{providerID}, nil
}

func (c *Client) SendMessageEvents(ctx context.Context, sessionID, text, _ string, _, _ string) <-chan adapter.StreamEvent {
	out := make(chan adapter.StreamEvent)
	go func() {
		defer close(out)
		if strings.TrimSpace(text) == "" {
			return
		}
		if err := c.runMirroredTurn(ctx, out, sessionID, text); err != nil {
			emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf("\nClaude mirror error: %v\n", err)})
		}
	}()
	return out
}

func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	return fmt.Errorf("claude-mirror: answer the prompt in the real Claude Code window")
}

func (c *Client) BaseURL() string {
	return "claude-mirror://local"
}

// runMirroredTurn injects the prompt into the live window and mirrors the
// transcript until the assistant finishes its turn.
func (c *Client) runMirroredTurn(ctx context.Context, out chan<- adapter.StreamEvent, sessionID, text string) error {
	path, err := findTranscript(sessionID)
	if err != nil {
		return err
	}
	pid, err := c.resolvePID(ctx, sessionID)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	offset := info.Size()
	if entries, err := readTranscript(path); err == nil && hasPendingQuestion(entries) {
		return fmt.Errorf("the Claude Code window is waiting for you to answer a question; answer it there before sending more prompts")
	}
	if err := injectPrompt(ctx, pid, text); err != nil {
		return err
	}
	debuglog.Printf("claude-mirror: prompt injected into pid %d, tailing %s from %d", pid, path, offset)
	return tailTurn(ctx, out, path, offset)
}

// resolvePID returns the interactive claude pid driving a session, refreshing
// discovery if the cached pid is gone.
func (c *Client) resolvePID(ctx context.Context, sessionID string) (int, error) {
	c.mu.Lock()
	pid, ok := c.pids[sessionID]
	c.mu.Unlock()
	if ok && processAlive(pid) {
		return pid, nil
	}
	instances, err := discoverInstances(ctx)
	if err != nil {
		return 0, err
	}
	for _, inst := range instances {
		if inst.sessionID == sessionID {
			c.rememberPID(sessionID, inst.pid)
			return inst.pid, nil
		}
	}
	return 0, fmt.Errorf("no running Claude Code window found for session %s", sessionID)
}

// tailTurn streams new transcript entries until the assistant ends its turn,
// the transcript goes idle (e.g. a permission prompt is blocking the real
// window), or the context is cancelled.
func tailTurn(ctx context.Context, out chan<- adapter.StreamEvent, path string, offset int64) error {
	deadline := time.Now().Add(firstReplyTimeout)
	sawActivity := false
	lastActivity := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(tailPollInterval):
		}
		entries, newOffset, err := readNewEntries(path, offset)
		if err != nil {
			return err
		}
		offset = newOffset
		for _, entry := range entries {
			if !conversationEntry(entry) {
				continue
			}
			sawActivity = true
			lastActivity = time.Now()
			if entry.Message.Role == "assistant" || entry.Type == "user" {
				emitEntry(ctx, out, entry)
			}
			if entry.Type == "assistant" && entry.Message.StopReason != "" && entry.Message.StopReason != "tool_use" {
				return nil
			}
		}
		if !sawActivity && time.Now().After(deadline) {
			return fmt.Errorf("no transcript activity within %s — the prompt may not have reached the window", firstReplyTimeout)
		}
		if sawActivity && time.Since(lastActivity) > idleTimeout {
			emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf(
				"\n[claude-mirror] no transcript activity for %s — the real window may be waiting for input (permission prompt or question); check it there\n",
				idleTimeout)})
			return nil
		}
	}
}

// emitEntry converts one transcript entry into stream events. The echoed user
// prompt itself produces no parts worth mirroring (tool results do).
func emitEntry(ctx context.Context, out chan<- adapter.StreamEvent, entry transcriptEntry) {
	for _, part := range entryParts(entry) {
		switch {
		case part.Tool != nil:
			emit(ctx, out, adapter.StreamEvent{Tool: part.Tool})
			// Questions block the real window; mirror them readably so the
			// user knows to answer there.
			if part.Tool.Name == "AskUserQuestion" {
				if text := formatQuestions(part.Tool.Input); text != "" {
					emit(ctx, out, adapter.StreamEvent{Text: text})
				}
			}
		case entry.Message.Role == "assistant":
			if text := adapter.DisplayTextForPart(part); text != "" {
				emit(ctx, out, adapter.StreamEvent{Text: text})
			}
		}
	}
}

// readNewEntries parses complete JSONL lines appended past offset, returning
// the offset just after the last complete line.
func readNewEntries(path string, offset int64) ([]transcriptEntry, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	var entries []transcriptEntry
	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// Partial trailing line: leave it for the next poll.
			return entries, offset, nil
		}
		offset += int64(len(line))
		var entry transcriptEntry
		if jsonErr := json.Unmarshal(line, &entry); jsonErr == nil {
			entries = append(entries, entry)
		}
	}
}

func (c *Client) rememberPID(sessionID string, pid int) {
	c.mu.Lock()
	c.pids[sessionID] = pid
	c.mu.Unlock()
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(nil) == nil
}

func (c *Client) sessionFromInstance(inst instance) adapter.Session {
	return c.sessionFromTranscript(inst.sessionID, inst.transcript, inst.cwd)
}

func (c *Client) sessionFromTranscript(sessionID, path, cwd string) adapter.Session {
	title := "Claude window"
	created := int64(0)
	updated := int64(0)
	if info, err := os.Stat(path); err == nil {
		updated = info.ModTime().UnixMilli()
	}
	if entries, err := readTranscript(path); err == nil && len(entries) > 0 {
		title = transcriptTitle(entries, title)
		for _, entry := range entries {
			if entry.Cwd != "" && cwd == "" {
				cwd = entry.Cwd
			}
			if entry.Timestamp != "" {
				if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
					created = t.UnixMilli()
					break
				}
			}
		}
	}
	return adapter.Session{
		ID:        sessionID,
		Title:     title,
		Directory: cwd,
		Model:     &adapter.ModelRef{ID: defaultModelID, ProviderID: providerID},
		Time:      adapter.SessionTime{Created: created, Updated: updated},
	}
}

func emit(ctx context.Context, out chan<- adapter.StreamEvent, event adapter.StreamEvent) {
	select {
	case <-ctx.Done():
	case out <- event:
	}
}
