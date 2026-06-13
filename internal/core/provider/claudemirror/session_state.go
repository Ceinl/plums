package claudemirror

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// sessionState is the subset of ~/.claude/sessions/<pid>.json that tells us
// whether an interactive Claude Code window is parked waiting for the user.
//
// This file is load-bearing for question mirroring: Claude Code does NOT write
// a pending AskUserQuestion turn to the JSONL transcript until it is answered
// or declined, so while the dialog is open the transcript shows nothing past
// the user's prompt. The session file is the only signal that the window is
// blocked on input rather than still working, which lets us decline the
// question (Esc) to force the flush instead of waiting out the idle timeout.
type sessionState struct {
	SessionID  string `json:"sessionId"`
	Status     string `json:"status"`     // e.g. "busy", "waiting"
	WaitingFor string `json:"waitingFor"` // e.g. "permission prompt"
}

func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "sessions"), nil
}

// readSessionState reads the live state Claude Code maintains for an
// interactive window, keyed by its process id.
func readSessionState(pid int) (sessionState, error) {
	if pid <= 0 {
		return sessionState{}, os.ErrNotExist
	}
	dir, err := sessionsDir()
	if err != nil {
		return sessionState{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(pid)+".json"))
	if err != nil {
		return sessionState{}, err
	}
	var st sessionState
	if err := json.Unmarshal(data, &st); err != nil {
		return sessionState{}, err
	}
	return st, nil
}

// isWaitingForInput reports whether the window is parked waiting for the user
// (a question or permission prompt) rather than actively working a turn.
func (s sessionState) isWaitingForInput() bool {
	return s.Status == "waiting"
}
