package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// writeClipboard is the package-level clipboard writer. Tests may replace it.
var writeClipboard = func(text, command string) error {
	if command == "" {
		command = DefaultClipboardCommand()
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty clipboard command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// readClipboard is the package-level clipboard reader backing
// capabilities.Clipboard.Paste. Tests may replace it. The default reader uses the
// platform paste command; an error is returned when none is available.
var readClipboard = func() (string, error) {
	command := defaultClipboardPasteCommand()
	if command == "" {
		return "", fmt.Errorf("clipboard paste unsupported on %s", runtime.GOOS)
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// defaultClipboardPasteCommand returns the default clipboard paste command for
// the current platform, or "" when unknown.
func defaultClipboardPasteCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "pbpaste"
	case "linux":
		return "xclip -selection clipboard -o"
	default:
		return ""
	}
}

// DefaultClipboardCommand returns the default clipboard copy command for the
// current platform.
func DefaultClipboardCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy"
	case "windows":
		return "clip"
	case "linux":
		return "xclip -selection clipboard"
	default:
		return "pbcopy"
	}
}

// copyEditorSelection copies the editor selection to the clipboard and returns
// true if a selection existed.
func copyEditorSelection(state *State, clipboardCmd string) bool {
	if state == nil || state.Editor == nil || !state.Editor.HasSelection() {
		return false
	}
	if err := writeClipboard(state.Editor.SelectedText(), clipboardCmd); err != nil {
		state.AddMessage("system", "copy failed: "+err.Error())
	}
	return true
}
