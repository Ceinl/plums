package ai

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"plums/internal/debuglog"
)

// ServerProcess owns an opencode server process started by plums.
type ServerProcess struct {
	cmd  *exec.Cmd
	done chan error
}

// StartServer starts `opencode serve` and returns once the process has been
// launched. Call WaitForHealth before using the client.
func StartServer(ctx context.Context) (*ServerProcess, error) {
	if _, err := exec.LookPath("opencode"); err != nil {
		return nil, fmt.Errorf("opencode executable not found: %w", err)
	}

	debuglog.Printf("server: exec opencode serve")
	cmd := exec.CommandContext(ctx, "opencode", "serve")
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start opencode serve: %w", err)
	}
	debuglog.Printf("server: opencode serve pid=%d", cmd.Process.Pid)

	proc := &ServerProcess{
		cmd:  cmd,
		done: make(chan error, 1),
	}
	go func() {
		err := cmd.Wait()
		debuglog.Printf("server: opencode serve exited: %v", err)
		proc.done <- err
	}()

	return proc, nil
}

// Stop terminates the managed opencode server process.
func (p *ServerProcess) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	debuglog.Printf("server: stopping opencode serve pid=%d", p.cmd.Process.Pid)
	_ = p.cmd.Process.Kill()
	select {
	case <-p.done:
	case <-time.After(time.Second):
	}
}

// WaitForHealth polls the opencode health endpoint until it is ready or the
// timeout expires.
func WaitForHealth(ctx context.Context, client *Client, timeout time.Duration) error {
	debuglog.Printf("server: waiting for health timeout=%s", timeout)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := client.Health(ctx); err == nil {
			debuglog.Printf("server: health ready")
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("opencode server did not become ready within %s", timeout)
		case <-ticker.C:
		}
	}
}
