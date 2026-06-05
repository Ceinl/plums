package app

import (
	"context"
	"fmt"
	"os"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/debuglog"
)

func applySession(state *State, session *adapter.Session) {
	if session == nil {
		return
	}
	state.SetSessionID(session.ID)
	state.SetSessionTitle(session.Title)
	if session.Model != nil {
		state.SetModel(session.Model.ProviderID, session.Model.ID)
	}
}

func applyRecentModel(ctx context.Context, state *State, client adapter.Backend, cfg RunConfig) {
	if state.ModelID != "" {
		return
	}
	lookupCtx, cancel := context.WithTimeout(ctx, cfg.RecentModelTimeout)
	defer cancel()
	sessions, err := client.ListSessions(lookupCtx)
	if err != nil {
		debuglog.Printf("session: recent model lookup failed: %v", err)
		return
	}
	wd := cfg.WorkingDirectory
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			debuglog.Printf("session: recent model working directory failed: %v", err)
			return
		}
	}
	var latest *adapter.Session
	for i := range sessions {
		session := &sessions[i]
		if session.Model == nil || session.Directory != wd {
			continue
		}
		if latest == nil || session.Time.Updated > latest.Time.Updated {
			latest = session
		}
	}
	if latest != nil {
		state.SetModel(latest.Model.ProviderID, latest.Model.ID)
	}
}

func latestSessionForDirectory(sessions []adapter.Session, directory string) *adapter.Session {
	var latest *adapter.Session
	for i := range sessions {
		session := &sessions[i]
		if session.Directory != directory {
			continue
		}
		if latest == nil || session.Time.Updated > latest.Time.Updated {
			latest = session
		}
	}
	return latest
}

func sessionDisplayName(session *adapter.Session) string {
	if session == nil || session.Title == "" {
		if session == nil {
			return ""
		}
		return session.ID
	}
	return session.Title
}

func refreshSessionModel(ctx context.Context, state *State, client adapter.Backend, cfg RunConfig) {
	if state.SessionID == "" {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, cfg.RecentModelTimeout)
	defer cancel()
	session, err := client.GetSession(refreshCtx, state.SessionID)
	if err != nil {
		debuglog.Printf("session: refresh model failed: %v", err)
		return
	}
	applySession(state, session)
}

func ensureSession(ctx context.Context, state *State, client adapter.Backend, cfg RunConfig) error {
	if state.SessionID != "" {
		return nil
	}
	wd := cfg.WorkingDirectory
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}
	listCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
	sessions, err := client.ListSessions(listCtx)
	cancel()
	if err == nil {
		if session := latestSessionForDirectory(sessions, wd); session != nil {
			applySession(state, session)
			if session.Model == nil && state.ModelID == "" {
				applyRecentModel(ctx, state, client, cfg)
			}
			return nil
		}
	} else {
		debuglog.Printf("session: startup session lookup failed: %v", err)
	}

	sessionCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
	defer cancel()
	session, err := client.CreateSession(sessionCtx, wd)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	if session.Directory != "" && session.Directory != wd {
		return fmt.Errorf("opencode session directory is %q, expected %q; stop the existing opencode server or configure plums to use a different port", session.Directory, wd)
	}
	applySession(state, session)
	if session.Model == nil && state.ModelID == "" {
		applyRecentModel(ctx, state, client, cfg)
	}
	return nil
}
