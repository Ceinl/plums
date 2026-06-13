package claudemirror

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	// pasteSettleTimeout bounds how long we wait for a pasted prompt to show up
	// in the pane before pressing Enter; pasteSettlePoll is the check cadence.
	pasteSettleTimeout = 2 * time.Second
	pasteSettlePoll    = 40 * time.Millisecond
)

// tmuxPaneForPID finds the tmux pane whose process tree contains pid.
func tmuxPaneForPID(ctx context.Context, pid int, parents map[int]int) (string, error) {
	out, err := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_id} #{pane_pid}").Output()
	if err != nil {
		return "", fmt.Errorf("tmux not available: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		panePID, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		if hasAncestor(pid, panePID, parents) {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no tmux pane contains pid %d", pid)
}

func parentMap(ctx context.Context) (map[int]int, error) {
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil, fmt.Errorf("listing process parents: %w", err)
	}
	parents := make(map[int]int)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil {
			parents[pid] = ppid
		}
	}
	return parents, nil
}

func hasAncestor(pid, ancestor int, parents map[int]int) bool {
	for p := pid; p > 1; p = parents[p] {
		if p == ancestor {
			return true
		}
	}
	return false
}

// tmuxInject pastes the prompt into a pane (bracketed paste keeps newlines
// from submitting early) and then presses Enter.
func tmuxInject(ctx context.Context, pane, text string) error {
	if out, err := exec.CommandContext(ctx, "tmux", "set-buffer", "-b", "plums-mirror", "--", text).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux set-buffer: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.CommandContext(ctx, "tmux", "paste-buffer", "-p", "-d", "-b", "plums-mirror", "-t", pane).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux paste-buffer: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// Wait until the pane actually shows the pasted prompt before pressing
	// Enter: a fixed sleep races on a loaded machine and can submit a half-
	// ingested line. Claude Code collapses big pastes to a placeholder, so we
	// only require a short, single-line tail to be visible.
	if err := waitForPaste(ctx, pane, text); err != nil {
		return err
	}
	if out, err := exec.CommandContext(ctx, "tmux", "send-keys", "-t", pane, "Enter").CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys Enter: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// tmuxEscape presses Escape in a pane. It is used to decline a pending
// AskUserQuestion dialog so Claude Code flushes the buffered question turn to
// the transcript, where the mirror can read and surface it.
func tmuxEscape(ctx context.Context, pane string) error {
	if out, err := exec.CommandContext(ctx, "tmux", "send-keys", "-t", pane, "Escape").CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys Escape: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// waitForPaste blocks until the pane's visible content contains a marker
// proving the paste landed, or until pasteSettleTimeout elapses (after which it
// presses on rather than dropping the prompt — capture-pane can legitimately
// miss collapsed pastes).
func waitForPaste(ctx context.Context, pane, text string) error {
	marker := pasteMarker(text)
	deadline := time.Now().Add(pasteSettleTimeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pasteSettlePoll):
		}
		out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-p", "-t", pane).Output()
		if err == nil && (marker == "" || strings.Contains(string(out), marker)) {
			return nil
		}
		if time.Now().After(deadline) {
			return nil
		}
	}
}

// pasteMarker returns a short single-line fragment of text expected to appear
// verbatim in the pane. Multi-line or long prompts are collapsed by Claude Code
// into a placeholder, so they have no reliable marker and return "".
func pasteMarker(text string) string {
	t := strings.TrimSpace(text)
	if t == "" || strings.Contains(t, "\n") {
		return ""
	}
	const maxMarker = 40
	if len([]rune(t)) > maxMarker {
		return ""
	}
	return t
}
