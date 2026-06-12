package claudemirror

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// instance is a running interactive Claude Code process paired with the
// transcript file of the session it is (most likely) driving.
type instance struct {
	pid        int
	started    time.Time
	cwd        string
	sessionID  string
	transcript string
}

type claudeProc struct {
	pid     int
	started time.Time
	cwd     string
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

// livePrefix marks placeholder session ids for windows that have not written
// a transcript yet (Claude Code creates the file on the first message).
const livePrefix = "live-"

// discoverInstances finds interactive `claude` processes and pairs each with
// a transcript in its project directory. With one process per cwd the most
// recently updated transcript wins; with several, processes and transcripts
// are matched in session-start order (process start time vs transcript file
// birth time), since Claude Code keeps no open fd on its transcript to pair
// against directly. Processes without any transcript yet get a placeholder
// session id so fresh windows are still attachable.
func discoverInstances(ctx context.Context) ([]instance, error) {
	procs, err := interactiveClaudeProcesses(ctx)
	if err != nil {
		return nil, err
	}
	root, err := projectsDir()
	if err != nil {
		return nil, err
	}
	byDir := make(map[string][]claudeProc)
	for _, proc := range procs {
		cwd, err := processCwd(ctx, proc.pid)
		if err != nil || cwd == "" {
			continue
		}
		proc.cwd = cwd
		dir := filepath.Join(root, encodeProjectDir(cwd))
		byDir[dir] = append(byDir[dir], proc)
	}
	var instances []instance
	for dir, group := range byDir {
		transcripts, _ := listTranscripts(dir)
		instances = append(instances, pairGroup(group, transcripts)...)
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].pid < instances[j].pid })
	return instances, nil
}

// pairGroup matches processes sharing one cwd to that cwd's transcripts. The
// N most recently active transcripts are assumed to belong to the N live
// processes, matched oldest session to oldest process. Processes left over
// (more windows than transcripts — i.e. fresh windows that haven't written
// anything yet) get placeholder sessions.
func pairGroup(procs []claudeProc, transcripts []transcriptFile) []instance {
	sort.Slice(transcripts, func(i, j int) bool { return transcripts[i].modified.After(transcripts[j].modified) })
	if len(transcripts) > len(procs) {
		transcripts = transcripts[:len(procs)]
	}
	sort.Slice(transcripts, func(i, j int) bool { return transcripts[i].born.Before(transcripts[j].born) })
	sort.Slice(procs, func(i, j int) bool { return procs[i].started.Before(procs[j].started) })
	instances := make([]instance, 0, len(procs))
	for i, t := range transcripts {
		instances = append(instances, instance{
			pid:        procs[i].pid,
			started:    procs[i].started,
			cwd:        procs[i].cwd,
			sessionID:  t.sessionID,
			transcript: t.path,
		})
	}
	for _, proc := range procs[len(transcripts):] {
		instances = append(instances, instance{
			pid:       proc.pid,
			started:   proc.started,
			cwd:       proc.cwd,
			sessionID: fmt.Sprintf("%s%d", livePrefix, proc.pid),
		})
	}
	return instances
}

type transcriptFile struct {
	path      string
	sessionID string
	modified  time.Time
	born      time.Time
}

func listTranscripts(dir string) ([]transcriptFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []transcriptFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		file := transcriptFile{
			path:      filepath.Join(dir, entry.Name()),
			sessionID: strings.TrimSuffix(entry.Name(), ".jsonl"),
			modified:  info.ModTime(),
			born:      info.ModTime(),
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			file.born = time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec)
		}
		files = append(files, file)
	}
	return files, nil
}

// interactiveClaudeProcesses lists `claude` processes that look like the
// interactive TUI: not headless -p/--print runs and not stream-json driven
// instances spawned by other apps (e.g. the Claude desktop app).
func interactiveClaudeProcesses(ctx context.Context) ([]claudeProc, error) {
	out, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,lstart=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	var procs []claudeProc
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// pid + 5 lstart fields ("Fri Jun 13 00:09:03 2026") + command.
		if len(fields) < 7 {
			continue
		}
		command := strings.Join(fields[6:], " ")
		if !isInteractiveClaudeCommand(command) {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		started, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(fields[1:6], " "), time.Local)
		if err != nil {
			started = time.Time{}
		}
		procs = append(procs, claudeProc{pid: pid, started: started})
	}
	return procs, nil
}

// headlessFlags mark claude invocations driven by another program rather
// than a human at a TUI.
var headlessFlags = map[string]bool{
	"-p":                       true,
	"--print":                  true,
	"--version":                true,
	"--help":                   true,
	"--input-format":           true,
	"--output-format":          true,
	"--permission-prompt-tool": true,
	"--replay-user-messages":   true,
}

func isInteractiveClaudeCommand(command string) bool {
	args := strings.Fields(command)
	if len(args) == 0 {
		return false
	}
	if !isClaudeArgv(args) {
		return false
	}
	for _, arg := range args[1:] {
		if headlessFlags[arg] || headlessFlags[strings.SplitN(arg, "=", 2)[0]] {
			return false
		}
	}
	return true
}

// isClaudeArgv recognises both direct `claude` binaries and interpreter
// launches like `node .../claude-code/cli.js` or shim wrappers.
func isClaudeArgv(args []string) bool {
	base := filepath.Base(args[0])
	if base == "claude" {
		return true
	}
	switch base {
	case "node", "bun", "deno":
		for _, arg := range args[1:] {
			if strings.HasPrefix(arg, "-") {
				continue
			}
			return filepath.Base(arg) == "claude" ||
				strings.Contains(strings.ToLower(arg), "claude-code") ||
				strings.Contains(strings.ToLower(arg), "@anthropic-ai/claude")
		}
	}
	return false
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
