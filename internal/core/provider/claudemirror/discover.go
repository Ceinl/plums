package claudemirror

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// instance is a running interactive Claude Code process paired with the
// transcript file of the session it is (most likely) driving.
type instance struct {
	pid        int
	cwd        string
	sessionID  string
	transcript string
}

// projectsDir returns ~/.claude/projects.
func projectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// encodeProjectDir mirrors Claude Code's transcript directory naming: every
// character outside [A-Za-z0-9] in the absolute cwd becomes '-'.
func encodeProjectDir(cwd string) string {
	var b strings.Builder
	for _, r := range cwd {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// discoverInstances finds interactive `claude` processes and resolves each to
// the most recently updated transcript in its project directory.
func discoverInstances(ctx context.Context) ([]instance, error) {
	pids, err := interactiveClaudePIDs(ctx)
	if err != nil {
		return nil, err
	}
	root, err := projectsDir()
	if err != nil {
		return nil, err
	}
	var instances []instance
	seen := make(map[string]bool)
	for _, pid := range pids {
		cwd, err := processCwd(ctx, pid)
		if err != nil || cwd == "" {
			continue
		}
		path, sessionID, err := latestTranscript(filepath.Join(root, encodeProjectDir(cwd)))
		if err != nil || seen[sessionID] {
			continue
		}
		seen[sessionID] = true
		instances = append(instances, instance{pid: pid, cwd: cwd, sessionID: sessionID, transcript: path})
	}
	return instances, nil
}

// interactiveClaudePIDs lists pids of `claude` processes that look like the
// interactive TUI (not headless -p/--print runs and not this app).
func interactiveClaudePIDs(ctx context.Context) ([]int, error) {
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.SplitN(line, " ", 2)
		if len(fields) != 2 {
			continue
		}
		command := fields[1]
		if !isInteractiveClaudeCommand(command) {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func isInteractiveClaudeCommand(command string) bool {
	args := strings.Fields(command)
	if len(args) == 0 {
		return false
	}
	base := filepath.Base(args[0])
	if base != "claude" {
		// Node-launched installs show up as `node .../claude` or similar.
		if base != "node" || len(args) < 2 || filepath.Base(args[1]) != "claude" {
			return false
		}
		args = args[1:]
	}
	for _, arg := range args[1:] {
		if arg == "-p" || arg == "--print" || arg == "--version" || arg == "--help" {
			return false
		}
	}
	return true
}

// processCwd returns the working directory of a pid via lsof.
func processCwd(ctx context.Context, pid int) (string, error) {
	out, err := exec.CommandContext(ctx, "lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return "", fmt.Errorf("lsof cwd for pid %d: %w", pid, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimSpace(line[1:]), nil
		}
	}
	return "", fmt.Errorf("no cwd in lsof output for pid %d", pid)
}

// latestTranscript picks the most recently modified .jsonl in a project
// transcript directory. When several instances share one cwd this can pick
// the wrong sibling; the newest one is the best available signal.
func latestTranscript(dir string) (path, sessionID string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}
	var bestTime int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if mod := info.ModTime().UnixMilli(); mod > bestTime {
			bestTime = mod
			path = filepath.Join(dir, entry.Name())
			sessionID = strings.TrimSuffix(entry.Name(), ".jsonl")
		}
	}
	if path == "" {
		return "", "", fmt.Errorf("no transcripts in %s", dir)
	}
	return path, sessionID, nil
}

// findTranscript locates the transcript file for a session id across all
// project directories.
func findTranscript(sessionID string) (string, error) {
	root, err := projectsDir()
	if err != nil {
		return "", err
	}
	matches, err := filepath.Glob(filepath.Join(root, "*", sessionID+".jsonl"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no transcript found for session %s", sessionID)
	}
	return matches[0], nil
}
