package claudemirror

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
