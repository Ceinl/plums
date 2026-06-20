package opencode

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ceinl/plums/internal/debuglog"
)

// ServerProcess owns an opencode server process started by plums.
type ServerProcess struct {
	cmd      *exec.Cmd
	done     chan struct{}
	waitErr  error
	stderr   bytes.Buffer
	stopOnce sync.Once
}

// StartServer starts `opencode serve` for baseURL and returns once the process
// has been launched. Call WaitForHealth before using the client.
func StartServer(ctx context.Context, baseURL, directory string) (*ServerProcess, error) {
	args, err := serverCommandArgs(baseURL)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("opencode"); err != nil {
		return nil, fmt.Errorf("opencode executable not found: %w", err)
	}

	debuglog.Printf("server: exec opencode %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = directory
	proc := &ServerProcess{
		cmd:  cmd,
		done: make(chan struct{}),
	}
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = io.Discard
	cmd.Stderr = &proc.stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start opencode serve: %w", err)
	}
	debuglog.Printf("server: opencode serve pid=%d", cmd.Process.Pid)
	go func() {
		proc.waitErr = cmd.Wait()
		debuglog.Printf("server: opencode serve exited: %v", proc.waitErr)
		close(proc.done)
	}()

	return proc, nil
}

func serverCommandArgs(baseURL string) ([]string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid opencode server URL %q", baseURL)
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("cannot auto-start opencode for %s URL %q; start the server yourself or use a local http URL", u.Scheme, baseURL)
	}

	host := u.Hostname()
	if !isLocalHost(host) {
		return nil, fmt.Errorf("cannot auto-start opencode for non-local URL %q; start the server yourself or use a local URL", baseURL)
	}

	args := []string{"serve", "--hostname", host}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid opencode server URL port %q", port)
	}
	args = append(args, "--port", port)
	return args, nil
}

func isLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

// Stop terminates the managed opencode server process.
func (p *ServerProcess) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	p.stopOnce.Do(func() {
		debuglog.Printf("server: stopping opencode serve pid=%d", p.cmd.Process.Pid)
		_ = p.cmd.Process.Kill()
		select {
		case <-p.done:
		case <-time.After(time.Second):
		}
	})
}

func (p *ServerProcess) exitError(err error) error {
	if p == nil {
		return err
	}
	stderr := strings.TrimSpace(p.stderr.String())
	if err == nil {
		if stderr == "" {
			return fmt.Errorf("opencode serve exited")
		}
		return fmt.Errorf("opencode serve exited: %s", stderr)
	}
	if stderr == "" {
		return fmt.Errorf("opencode serve exited: %w", err)
	}
	return fmt.Errorf("opencode serve exited: %w: %s", err, stderr)
}

// healthClient is the minimal client surface the health poller needs.
type healthClient interface {
	Health(context.Context) error
}

// WaitForHealthOrExit polls health until ready, timeout, cancellation, or the
// managed opencode process exits before becoming ready.
func WaitForHealthOrExit(ctx context.Context, client healthClient, proc *ServerProcess, timeout time.Duration) error {
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
		case <-processDone(proc):
			return proc.exitError(proc.waitErr)
		case <-deadline.C:
			return fmt.Errorf("opencode server did not become ready within %s", timeout)
		case <-ticker.C:
		}
	}
}

// Done returns a channel that is closed when the managed process exits.
func (p *ServerProcess) Done() <-chan struct{} {
	if p == nil {
		return nil
	}
	return p.done
}

func processDone(proc *ServerProcess) <-chan struct{} {
	if proc == nil {
		return nil
	}
	return proc.done
}
