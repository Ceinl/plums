package claudemirror

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Ceinl/plums/internal/debuglog"
)

// injectPrompt types a prompt into the real Claude Code window that owns pid.
// It prefers tmux (precise pane targeting); otherwise it falls back to macOS
// System Events keystrokes, but only when that is provably unambiguous.
func injectPrompt(ctx context.Context, pid int, text string) error {
	parents, err := parentMap(ctx)
	if err != nil {
		return err
	}
	if pane, err := tmuxPaneForPID(ctx, pid, parents); err == nil {
		debuglog.Printf("claude-mirror: injecting into tmux pane %s for pid %d", pane, pid)
		return tmuxInject(ctx, pane, text)
	}
	debuglog.Printf("claude-mirror: no tmux pane for pid %d, trying Ghostty keystrokes", pid)
	return ghosttyInject(ctx, pid, parents, text)
}

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
	// Give the TUI a beat to ingest the paste before submitting.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}
	if out, err := exec.CommandContext(ctx, "tmux", "send-keys", "-t", pane, "Enter").CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys Enter: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ghosttyInject pastes the prompt via the clipboard and System Events
// (requires Accessibility permission). System Events cannot target a
// specific Ghostty window or tab, so this refuses to run unless the target
// pid actually descends from Ghostty and Ghostty has exactly one window —
// otherwise the keystrokes could land in the wrong session.
func ghosttyInject(ctx context.Context, pid int, parents map[int]int, text string) error {
	if !descendsFromGhostty(ctx, pid, parents) {
		return fmt.Errorf("session pid %d is not running under tmux or Ghostty; run Claude Code inside tmux for precise targeting", pid)
	}
	count, err := ghosttyWindowCount(ctx)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("Ghostty has %d windows and keystrokes cannot target a specific one; run Claude Code inside tmux instead", count)
	}
	clip := exec.CommandContext(ctx, "pbcopy")
	clip.Stdin = strings.NewReader(text)
	if err := clip.Run(); err != nil {
		return fmt.Errorf("copying prompt to clipboard: %w", err)
	}
	script := `tell application "Ghostty" to activate
delay 0.3
tell application "System Events" to keystroke "v" using command down
delay 0.3
tell application "System Events" to key code 36`
	if out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput(); err != nil {
		return fmt.Errorf("ghostty keystroke injection (is Accessibility permission granted?): %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// descendsFromGhostty reports whether any ancestor of pid is a ghostty
// process.
func descendsFromGhostty(ctx context.Context, pid int, parents map[int]int) bool {
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return false
	}
	ghostty := make(map[int]bool)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.Contains(strings.ToLower(fields[len(fields)-1]), "ghostty") {
			if p, err := strconv.Atoi(fields[0]); err == nil {
				ghostty[p] = true
			}
		}
	}
	for p := pid; p > 1; p = parents[p] {
		if ghostty[p] {
			return true
		}
	}
	return false
}

func ghosttyWindowCount(ctx context.Context) (int, error) {
	out, err := exec.CommandContext(ctx, "osascript", "-e",
		`tell application "System Events" to count windows of process "Ghostty"`).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("counting Ghostty windows (is Accessibility permission granted?): %w: %s", err, strings.TrimSpace(string(out)))
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("unexpected Ghostty window count output %q", strings.TrimSpace(string(out)))
	}
	return count, nil
}
