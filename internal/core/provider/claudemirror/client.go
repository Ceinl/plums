package claudemirror

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// commandReplyTimeout caps how long a slash-command turn waits for the prompt
// to surface in the transcript. Locally handled commands (/clear, /model, ...)
// run inside the real window and never write a transcript entry, so the full
// firstReplyTimeout would leave the user on a spinner for a minute; commands
// that do trigger a model turn (e.g. /compact) start writing well within this.
// Variable so tests can shorten it.
var commandReplyTimeout = 10 * time.Second

// errPromptNotSeen is returned by awaitActiveTranscript when the injected
// prompt never appears in any transcript before the deadline.
var errPromptNotSeen = fmt.Errorf("prompt was sent but never appeared in any transcript")

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
	chosen, err := chooseInstance(ctx, directory)
	if err != nil {
		return nil, err
	}
	c.remember(*chosen)
	session := c.sessionFromInstance(*chosen)
	return &session, nil
}

// chooseInstance picks the window to attach to: the one whose cwd matches
// directory, or the only live one. With several candidates and no directory
// match it refuses rather than guessing — attaching to the wrong window must
// never happen.
func chooseInstance(ctx context.Context, directory string) (*instance, error) {
	instances, err := discoverInstances(ctx)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("no running interactive Claude Code instance found; open one in a terminal first")
	}
	for i := range instances {
		if instances[i].cwd == directory {
			return &instances[i], nil
		}
	}
	if len(instances) == 1 {
		return &instances[0], nil
	}
	var dirs []string
	for _, inst := range instances {
		dirs = append(dirs, inst.cwd)
	}
	return nil, fmt.Errorf("no Claude Code window running in %s and %d candidates elsewhere (%s); pick one from the session list instead",
		directory, len(instances), strings.Join(dirs, ", "))
}

// ResetSession implements "new session" for the mirror: it can't spawn a
// window, so it starts a fresh conversation in the attached one by injecting
// Claude Code's own /clear, then returns the window as a fresh live instance
// whose next prompt re-discovers the new transcript.
func (c *Client) ResetSession(ctx context.Context, directory string) (*adapter.Session, error) {
	chosen, err := chooseInstance(ctx, directory)
	if err != nil {
		return nil, err
	}
	pane, pid, err := c.resolvePane(ctx, *chosen)
	if err != nil {
		return nil, err
	}
	debuglog.Printf("claude-mirror: starting new conversation via /clear in pane %s (pid %d)", pane, pid)
	if err := tmuxInject(ctx, pane, "/clear"); err != nil {
		return nil, fmt.Errorf("failed to clear the Claude Code window: %w", err)
	}
	// /clear starts a new transcript; until the next prompt writes it, treat the
	// window as a fresh live instance with no transcript bound yet.
	fresh := instance{pid: pid, started: chosen.started, cwd: chosen.cwd, sessionID: fmt.Sprintf("%s%d", livePrefix, pid)}
	c.remember(fresh)
	session := c.sessionFromInstance(fresh)
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

// ReplyQuestion delivers the user's pick. To surface the question at all the
// mirror declined it in the real window (Claude Code never writes a pending
// AskUserQuestion to the transcript), so the dialog is already closed and the
// window is waiting for the user's next message — the answer is simply injected
// as that next prompt, which Claude reads as the response to the question.
func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	sessionID, toolUseID, ok := splitQuestionRequestID(requestID)
	if !ok {
		return fmt.Errorf("claude-mirror: invalid question request id %q", requestID)
	}
	answer := questionAnswerText(answers)
	if answer == "" {
		return fmt.Errorf("claude-mirror: empty question answer")
	}
	inst, err := c.resolveInstance(ctx, sessionID)
	if err != nil {
		return err
	}
	pane, pid, err := c.resolvePane(ctx, inst)
	if err != nil {
		return err
	}
	debuglog.Printf("claude-mirror: answering question %s in tmux pane %s (pid %d)", toolUseID, pane, pid)
	return tmuxInject(ctx, pane, answer)
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
		// A slash command (e.g. /clear, /model) handled inside the real window
		// produces no transcript output; it was still injected, so report it as
		// sent rather than failing the turn.
		if isSlashCommand(text) && errors.Is(err, errPromptNotSeen) {
			emit(ctx, out, adapter.StreamEvent{Text: fmt.Sprintf(
				"\n[claude-mirror] sent %q to the Claude Code window (no transcript output to mirror)\n", strings.TrimSpace(text))})
			return nil
		}
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
	return tailTurn(ctx, out, activePath, offset, pane, pid)
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
// (possibly a brand-new file) receives the injected prompt, returning that
// file and the offset it grew from.
//
// The prompt is matched verbatim when possible, but Claude Code rewrites the
// transcript for some inputs: large/multi-line pastes collapse to a
// "[Pasted text #1 +N lines]" placeholder and slash commands are wrapped in
// <command-name> scaffolding — neither contains the literal text. So a fresh
// user entry in a transcript that grew after injection is accepted as a
// fallback, with the verbatim match preferred when several files grew at once.
// Slash commands handled entirely inside the real window (/clear, /model)
// never write anything; those time out with errPromptNotSeen at the shorter
// commandReplyTimeout so the caller can report them as fire-and-forget.
func awaitActiveTranscript(ctx context.Context, dir string, baseline map[string]int64, text string) (string, int64, error) {
	needle := strings.TrimSpace(text)
	timeout := firstReplyTimeout
	if isSlashCommand(needle) {
		timeout = commandReplyTimeout
	}
	deadline := time.Now().Add(timeout)
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
		var fallback string
		var fallbackStart int64
		for _, match := range matches {
			start := baseline[match] // 0 for files created after injection
			info, err := os.Stat(match)
			if err != nil || info.Size() <= start {
				continue
			}
			entries, newOffset, err := readNewEntries(match, start)
			if err != nil {
				continue
			}
			if transcriptHasPrompt(entries, needle) {
				return match, start, nil
			}
			if fallback == "" && hasFreshUserEntry(entries) {
				fallback, fallbackStart = match, start
			}
			// Advance only past complete lines: stepping to info.Size() could
			// land mid-line and skip the prompt entry on the next read.
			baseline[match] = newOffset
		}
		if fallback != "" {
			return fallback, fallbackStart, nil
		}
		if time.Now().After(deadline) {
			return "", 0, fmt.Errorf("%w within %s — check the Claude Code window", errPromptNotSeen, timeout)
		}
	}
}

func transcriptHasPrompt(entries []transcriptEntry, needle string) bool {
	if needle == "" {
		return false
	}
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

// hasFreshUserEntry reports whether entries contain a typed user message that
// is not harness scaffolding — the signal that an injected prompt landed even
// when Claude Code rewrote its text (paste placeholder, slash-command wrapper).
func hasFreshUserEntry(entries []transcriptEntry) bool {
	for _, entry := range entries {
		if isTypedUserPrompt(entry) {
			return true
		}
	}
	return false
}

// isTypedUserPrompt reports whether entry is a user message the human typed —
// not a tool result (also user-role) and not harness scaffolding.
func isTypedUserPrompt(entry transcriptEntry) bool {
	if !conversationEntry(entry) || entry.Message.Role != "user" {
		return false
	}
	for _, block := range contentBlocks(entry.Message.Content) {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" && !isHarnessText(block.Text) {
			return true
		}
	}
	return false
}

func isSlashCommand(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "/")
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
// the transcript goes idle, or the context is cancelled.
//
// A pending AskUserQuestion is the awkward case: Claude Code buffers the whole
// question turn and writes nothing to the transcript until it is answered or
// declined, so the window can sit "waiting" while the tail sees only silence.
// When the session state reports the window parked on input, tailTurn presses
// Escape once to decline the question — that forces Claude Code to flush the
// AskUserQuestion (with its full options) to the transcript, where the next
// poll reads it and surfaces the native picker in Plums. The user's selection
// is then injected as the next prompt (see ReplyQuestion).
func tailTurn(ctx context.Context, out chan<- adapter.StreamEvent, path string, offset int64, pane string, pid int) error {
	deadline := time.Now().Add(firstReplyTimeout)
	sawActivity := false
	lastActivity := time.Now()
	// Interactive Claude Code writes one transcript line per content block,
	// each repeating the message's final stop_reason — so an end_turn line is
	// not the end of the writes. End only once an end_turn was seen and a
	// poll produces nothing further.
	turnEnded := false
	declined := false    // pressed Escape to flush a pending question this turn
	declinedID := ""     // tool_use id of the question we declined, once it flushes
	actedOnWait := false // handled a "waiting for input" state this turn
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
			// The question we forced flushes as an AskUserQuestion tool_use
			// immediately followed by a synthetic "user declined" tool_result;
			// remember its id so that rejection is suppressed below — the user
			// never declined, the mirror did, and showing the error misleads.
			if declined && declinedID == "" {
				if id := askQuestionID(entry); id != "" {
					declinedID = id
				}
			}
			if entry.Message.Role == "assistant" || entry.Type == "user" {
				emitEntry(ctx, out, entry, declinedID)
			}
			// A new typed prompt starts the turn we're mirroring. The previous
			// turn's trailing end_turn line can flush late and land in the same
			// poll batch as this prompt (awaitActiveTranscript hands us the
			// offset before it); without this reset that stale end_turn would
			// satisfy turnEnded and make the tail return before the assistant
			// even responds — losing, e.g., an AskUserQuestion that follows.
			if entry.Type == "user" && isTypedUserPrompt(entry) {
				turnEnded = false
			}
			if entry.Type == "assistant" && entry.Message.StopReason != "" {
				turnEnded = entry.Message.StopReason != "tool_use"
			}
		}
		if turnEnded && !progressed {
			return nil
		}
		// A window parked "waiting" with no transcript progress is blocked on
		// input we cannot see — a pending AskUserQuestion or a tool-permission
		// prompt, which the session file reports identically ("permission
		// prompt"). We can only tell them apart by the permission mode: under
		// bypassPermissions nothing prompts for permission, so waiting must be a
		// question — decline it (Esc) to flush it and surface the picker. In any
		// other mode the wait could be a real permission prompt; declining it
		// would reject the tool, so we leave it alone and just tell the user to
		// handle it in the real window.
		if !actedOnWait && !turnEnded && pane != "" {
			if st, err := readSessionState(pid); err == nil && st.isWaitingForInput() {
				actedOnWait = true
				bypass := false
				if entries, rerr := readTranscript(path); rerr == nil {
					bypass = transcriptPermissionMode(entries) == "bypassPermissions"
				}
				if bypass {
					debuglog.Printf("claude-mirror: pid %d waiting (%s) in bypass mode; pressing Esc to flush pending question", pid, st.WaitingFor)
					if err := tmuxEscape(ctx, pane); err != nil {
						debuglog.Printf("claude-mirror: failed to decline question: %v", err)
						actedOnWait = false // let a later poll retry
					} else {
						declined = true
						lastActivity = time.Now()
					}
				} else {
					debuglog.Printf("claude-mirror: pid %d waiting (%s) without bypass; leaving it for the real window", pid, st.WaitingFor)
					emit(ctx, out, adapter.StreamEvent{Text: "\n[claude-mirror] the Claude Code window is waiting for your input (a question or a permission prompt) — handle it there; the reply will mirror back here\n"})
					lastActivity = time.Now()
				}
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

// askQuestionID returns the tool_use id of an AskUserQuestion in an assistant
// entry, or "" if the entry contains none.
func askQuestionID(entry transcriptEntry) string {
	if entry.Type != "assistant" {
		return ""
	}
	for _, block := range contentBlocks(entry.Message.Content) {
		if block.Type == "tool_use" && block.Name == "AskUserQuestion" {
			return block.ID
		}
	}
	return ""
}

// emitEntry converts one transcript entry into stream events. The echoed user
// prompt itself produces no parts worth mirroring (tool results do).
//
// suppressResultID is the tool_use id of a question the mirror auto-declined to
// flush it; that question's synthetic rejection tool_result is dropped so Plums
// does not show a "declined" error the user never caused.
func emitEntry(ctx context.Context, out chan<- adapter.StreamEvent, entry transcriptEntry, suppressResultID string) {
	for _, part := range entryParts(entry) {
		switch {
		case part.Tool != nil:
			// A tool_result carries Output/Error and no Name; drop the one that
			// belongs to the question we declined on the user's behalf.
			if suppressResultID != "" && part.Tool.Name == "" && part.Tool.ID == suppressResultID {
				continue
			}
			emit(ctx, out, adapter.StreamEvent{Tool: part.Tool})
			// Questions block the real window; surface them in Plums when the
			// input matches Claude's question tool schema.
			if part.Tool.Name == "AskUserQuestion" {
				if req := parseQuestionRequest(entry.SessionID, part.Tool.ID, part.Tool.Input); req != nil {
					emit(ctx, out, adapter.StreamEvent{Question: req})
				} else if text := formatQuestions(part.Tool.Input); text != "" {
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

func questionAnswerText(answers [][]string) string {
	lines := make([]string, 0, len(answers))
	for _, answer := range answers {
		var parts []string
		for _, part := range answer {
			part = strings.TrimSpace(part)
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, strings.Join(parts, ", "))
		}
	}
	return strings.Join(lines, "\n")
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
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 probes existence without delivering anything. os.Process.Signal
	// type-asserts to syscall.Signal, so a nil signal would fail the assertion
	// and never report a live process — syscall.Signal(0) is the real check.
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// An existing process we don't own returns EPERM rather than ESRCH.
	return errors.Is(err, syscall.EPERM)
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
	modelID := defaultModelID
	created := int64(0)
	updated := int64(0)
	if info, err := os.Stat(path); err == nil {
		updated = info.ModTime().UnixMilli()
	}
	if entries, err := readTranscript(path); err == nil && len(entries) > 0 {
		title = transcriptTitle(entries, title)
		if model := transcriptModel(entries); model != "" {
			modelID = model
		}
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
		Model:     &adapter.ModelRef{ID: modelID, ProviderID: providerID},
		Time:      adapter.SessionTime{Created: created, Updated: updated},
	}
}

func emit(ctx context.Context, out chan<- adapter.StreamEvent, event adapter.StreamEvent) {
	select {
	case <-ctx.Done():
	case out <- event:
	}
}
