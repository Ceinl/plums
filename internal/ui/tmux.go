package ui

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TmuxKeys manages tmux's server-wide extended-keys option for the lifetime of
// the app. plums asks the terminal to distinguish modified keys like
// Shift+Enter (CSI-u / modifyOtherKeys), but tmux only forwards those to the
// application when extended-keys is enabled; with the common default of "off"
// Shift+Enter arrives as a bare Enter, which inserts a newline instead of
// submitting in split layout. EnableTmuxExtendedKeys turns it on at startup and
// Restore puts the prior value back on exit.
type TmuxKeys struct {
	active   bool
	previous string
}

// EnableTmuxExtendedKeys enables tmux extended-keys when running inside tmux and
// the option is not already on. It returns a handle whose Restore method undoes
// the change; both are no-ops outside tmux or when tmux is unavailable.
func EnableTmuxExtendedKeys() *TmuxKeys {
	keys := &TmuxKeys{}
	if os.Getenv("TMUX") == "" {
		return keys
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	current := tmuxShowExtendedKeys(ctx)
	keys.previous = current
	// "on" forwards extended keys to apps that request them; "always" forwards
	// unconditionally. Either already satisfies us, so only act on "off"/unset.
	if current == "on" || current == "always" {
		return keys
	}
	if err := exec.CommandContext(ctx, "tmux", "set", "-s", "extended-keys", "on").Run(); err != nil {
		return keys
	}
	keys.active = true
	return keys
}

// Restore reverts the extended-keys option to its prior value if this handle
// changed it.
func (k *TmuxKeys) Restore() {
	if k == nil || !k.active {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if k.previous == "" {
		// The option was at its default; "off" is tmux's documented default.
		_ = exec.CommandContext(ctx, "tmux", "set", "-s", "extended-keys", "off").Run()
		return
	}
	_ = exec.CommandContext(ctx, "tmux", "set", "-s", "extended-keys", k.previous).Run()
}

func tmuxShowExtendedKeys(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "tmux", "show", "-s", "-v", "extended-keys").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
