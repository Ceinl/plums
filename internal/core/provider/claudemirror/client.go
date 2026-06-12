package claudemirror

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	mu        sync.Mutex
	instances map[string]instance // session id -> attached window
	bindings  map[string]int      // transcript path -> verified owning pid
}

func NewBackend() adapter.Backend {
	return &Client{
		instances: make(map[string]instance),
		bindings:  make(map[string]int),
	}
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
// anything: the instance whose cwd matches directory, or the only live one.
// With several candidates and no directory match it refuses rather than
// guessing — injecting into the wrong window must never happen.
func (c *Client) CreateSession(ctx context.Context, directory string) (*adapter.Session, error) {
	instances, err := discoverInstances(ctx)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("no running interactive Claude Code instance found; open one in a terminal first")
	}
	var chosen *instance
	for i := range instances {
		if instances[i].cwd == directory {
			chosen = &instances[i]
			break
		}
	}
	if chosen == nil && len(instances) == 1 {
		chosen = &instances[0]
	}
	if chosen == nil {
		var dirs []string
		for _, inst := range instances {
			dirs = append(dirs, inst.cwd)
		}
		return nil, fmt.Errorf("no Claude Code window running in %s and %d candidates elsewhere (%s); pick one from the session list instead",
			directory, len(instances), strings.Join(dirs, ", "))
	}
	c.remember(*chosen)
	session := c.sessionFromInstance(*chosen)
	return &session, nil
}

func (c *Client) ListSessions(ctx context.Context) ([]adapter.Session, error) {
	instances, err := discoverInstances(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]adapter.Session, 0, len(instances))
	for _, inst := range instances {
		c.remember(inst)
		sessions = append(sessions, c.sessionFromInstance(inst))
	}
	return sessions, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*adapter.Session, error) {
	instances, err := discoverInstances(ctx)
	if err == nil {
		for _, inst := range instances {
			if inst.sessionID == sessionID {
				c.remember(inst)
				session := c.sessionFromInstance(inst)
				return &session, nil
			}
		}
	}
	if inst, ok := c.instance(sessionID); ok {
		session := c.sessionFromInstance(inst)
		return &session, nil
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
	path, err := c.transcriptFor(sessionID)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // fresh window, nothing written yet
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
// transcript until the assistant finishes its turn. The session's paired
// transcript is only a guess (mtime heuristic), so after injecting it locks
// onto whichever transcript in the project directory actually receives the
// prompt — a fresh window may write to a file that didn't exist yet.
func (c *Client) runMirroredTurn(ctx context.Context, out chan<- adapter.StreamEvent, sessionID, text string) error {
	inst, err := c.resolveInstance(ctx, sessionID)
	if err != nil {
		return err
	}
	if inst.transcript != "" {
		if entries, err := readTranscript(inst.transcript); err == nil && hasPendingQuestion(entries) {
			return fmt.Errorf("the Claude Code window is waiting for you to answer a question; answer it there before sending more prompts")
		}
	}
	pane, pid, err := c.resolvePane(ctx, inst)
	if err != nil {
		return err
	}
	dir, err := projectDirFor(inst)
	if err != nil {
		return err
	}
	baseline, err := snapshotSizes(dir)
	if err != nil {
		return err
	}
	debuglog.Printf("claude-mirror: injecting into tmux pane %s (pid %d) for session %s", pane, pid, sessionID)
	if err := tmuxInject(ctx, pane, text); err != nil {
		return err
	}
	activePath, offset, err := awaitActiveTranscript(ctx, dir, baseline, text)
	if err != nil {
		return err
	}
	if activePath != inst.transcript || pid != inst.pid {
		debuglog.Printf("claude-mirror: session %s verified as pid %d writing %s", sessionID, pid, activePath)
		inst.transcript = activePath
		inst.pid = pid
		c.remember(inst)
	}
	// The prompt provably landed in this transcript via this pid: a verified
	// binding that outlives the discovery heuristics.
	c.bind(activePath, pid)
	debuglog.Printf("claude-mirror: tailing %s from %d", activePath, offset)
	return tailTurn(ctx, out, activePath, offset)
}

// resolvePane finds the tmux pane to type into. Discovery's pid pairing is a
// heuristic, so it prefers a binding verified by an earlier turn; failing
// that, the discovered pid's pane; failing that, the only tmux-hosted Claude
// Code sibling in the same directory (the discovered pid then likely belongs
// to a non-tmux process such as another agent working in the same repo).
func (c *Client) resolvePane(ctx context.Context, inst instance) (string, int, error) {
	parents, err := parentMap(ctx)
	if err != nil {
		return "", 0, err
	}
	if inst.transcript != "" {
		if pid, ok := c.binding(inst.transcript); ok && processAlive(pid) {
			if pane, err := tmuxPaneForPID(ctx, pid, parents); err == nil {
				return pane, pid, nil
			}
		}
	}
	if pane, err := tmuxPaneForPID(ctx, inst.pid, parents); err == nil {
		return pane, inst.pid, nil
	}
	instances, err := discoverInstances(ctx)
	if err != nil {
		return "", 0, err
	}
	var panes []string
	var pids []int
	for _, sibling := range instances {
		if sibling.cwd != inst.cwd || sibling.pid == inst.pid {
			continue
		}
		if pane, err := tmuxPaneForPID(ctx, sibling.pid, parents); err == nil {
			panes = append(panes, pane)
			pids = append(pids, sibling.pid)
		}
	}
	switch len(panes) {
	case 1:
		debuglog.Printf("claude-mirror: pid %d has no tmux pane; using sibling pid %d in %s", inst.pid, pids[0], inst.cwd)
		return panes[0], pids[0], nil
	case 0:
		return "", 0, fmt.Errorf("no Claude Code window in %s runs inside tmux; claude-mirror can only type into tmux panes", inst.cwd)
	default:
		return "", 0, fmt.Errorf("%d tmux Claude Code windows in %s; cannot tell which one this session belongs to — close the extras or attach via the session list", len(panes), inst.cwd)
	}
}

func (c *Client) bind(transcript string, pid int) {
	c.mu.Lock()
	c.bindings[transcript] = pid
	c.mu.Unlock()
}

func (c *Client) binding(transcript string) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pid, ok := c.bindings[transcript]
	return pid, ok
}

// projectDirFor returns the transcript directory to watch for an instance,
// derived from its cwd when no transcript exists yet.
func projectDirFor(inst instance) (string, error) {
	if inst.transcript != "" {
		return filepath.Dir(inst.transcript), nil
	}
	root, err := projectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, encodeProjectDir(inst.cwd)), nil
}

// snapshotSizes records the current size of every transcript in a project
// directory so growth after injection can be detected.
func snapshotSizes(dir string) (map[string]int64, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sizes := make(map[string]int64, len(matches))
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil {
			sizes[match] = info.Size()
		}
	}
	return sizes, nil
}

// awaitActiveTranscript polls the project directory until some transcript
// (possibly a brand-new file) receives a user entry containing the injected
// prompt, returning that file and the offset it grew from.
func awaitActiveTranscript(ctx context.Context, dir string, baseline map[string]int64, text string) (string, int64, error) {
	needle := strings.TrimSpace(text)
	deadline := time.Now().Add(firstReplyTimeout)
	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-time.After(tailPollInterval):
		}
		matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		if err != nil {
			return "", 0, err
		}
		for _, match := range matches {
			start := baseline[match] // 0 for files created after injection
			info, err := os.Stat(match)
			if err != nil || info.Size() <= start {
				continue
			}
			entries, _, err := readNewEntries(match, start)
			if err != nil {
				continue
			}
			if transcriptHasPrompt(entries, needle) {
				return match, start, nil
			}
			// Growth without our prompt is another session's activity.
			baseline[match] = info.Size()
		}
		if time.Now().After(deadline) {
			return "", 0, fmt.Errorf("prompt was sent but never appeared in any transcript within %s — check the Claude Code window", firstReplyTimeout)
		}
	}
}

func transcriptHasPrompt(entries []transcriptEntry, needle string) bool {
	for _, entry := range entries {
		if !conversationEntry(entry) || entry.Message.Role != "user" {
			continue
		}
		for _, block := range contentBlocks(entry.Message.Content) {
			if block.Type == "text" && strings.Contains(block.Text, needle) {
				return true
			}
		}
	}
	return false
}

// resolveInstance returns the attached window for a session, refreshing
// discovery if the cached process is gone.
func (c *Client) resolveInstance(ctx context.Context, sessionID string) (instance, error) {
	if inst, ok := c.instance(sessionID); ok && processAlive(inst.pid) {
		return inst, nil
	}
	instances, err := discoverInstances(ctx)
	if err != nil {
		return instance{}, err
	}
	for _, inst := range instances {
		if inst.sessionID == sessionID {
			c.remember(inst)
			return inst, nil
		}
	}
	return instance{}, fmt.Errorf("no running Claude Code window found for session %s", sessionID)
}

// tailTurn streams new transcript entries until the assistant ends its turn,
// the transcript goes idle (e.g. a permission prompt is blocking the real
// window), or the context is cancelled.
func tailTurn(ctx context.Context, out chan<- adapter.StreamEvent, path string, offset int64) error {
	deadline := time.Now().Add(firstReplyTimeout)
	sawActivity := false
	lastActivity := time.Now()
	// Interactive Claude Code writes one transcript line per content block,
	// each repeating the message's final stop_reason — so an end_turn line is
	// not the end of the writes. End only once an end_turn was seen and a
	// poll produces nothing further.
	turnEnded := false
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
		progressed := false
		for _, entry := range entries {
			if !conversationEntry(entry) {
				continue
			}
			progressed = true
			sawActivity = true
			lastActivity = time.Now()
			if entry.Message.Role == "assistant" || entry.Type == "user" {
				emitEntry(ctx, out, entry)
			}
			if entry.Type == "assistant" && entry.Message.StopReason != "" {
				turnEnded = entry.Message.StopReason != "tool_use"
			}
		}
		if turnEnded && !progressed {
			return nil
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

func (c *Client) remember(inst instance) {
	c.mu.Lock()
	c.instances[inst.sessionID] = inst
	c.mu.Unlock()
}

func (c *Client) instance(sessionID string) (instance, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	inst, ok := c.instances[sessionID]
	return inst, ok
}

// transcriptFor resolves a session id to its transcript path; empty for a
// fresh window that has not written one yet.
func (c *Client) transcriptFor(sessionID string) (string, error) {
	if inst, ok := c.instance(sessionID); ok {
		return inst.transcript, nil
	}
	if strings.HasPrefix(sessionID, livePrefix) {
		return "", nil
	}
	return findTranscript(sessionID)
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(nil) == nil
}

func (c *Client) sessionFromInstance(inst instance) adapter.Session {
	if inst.transcript == "" {
		now := inst.started.UnixMilli()
		return adapter.Session{
			ID:        inst.sessionID,
			Title:     "Claude window (new)",
			Directory: inst.cwd,
			Model:     &adapter.ModelRef{ID: defaultModelID, ProviderID: providerID},
			Time:      adapter.SessionTime{Created: now, Updated: now},
		}
	}
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
