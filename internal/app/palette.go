package app

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/Ceinl/plums/capabilities"
)

func listSessionItems(ctx context.Context, state *State, client capabilities.Backend, cfg RunConfig) ([]SessionListItem, error) {
	sessionsBackend, err := backendSessions(client)
	if err != nil {
		return nil, err
	}
	listCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
	defer cancel()
	sessions, err := sessionsBackend.ListSessions(listCtx)
	if err != nil {
		return nil, err
	}
	items := make([]SessionListItem, 0, len(sessions))
	for _, session := range sessions {
		if cfg.ClearHistory && !state.IsRunSession(session.ID) {
			continue
		}
		items = append(items, SessionListItem{
			ID:        session.ID,
			Title:     session.Title,
			Directory: session.Directory,
			Updated:   session.Time.Updated,
			Current:   session.ID == state.SessionID,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Directory != items[j].Directory {
			return items[i].Directory < items[j].Directory
		}
		return items[i].Updated > items[j].Updated
	})
	return items, nil
}

func handlePaletteAction(ctx context.Context, state *State, client capabilities.Backend, action PaletteAction, cfg RunConfig, skills capabilities.SkillProvider) {
	switch action {
	case PaletteActionOpenPalette:
		state.OpenPalette()
	case PaletteActionNewSession:
		sessionsBackend, err := backendSessions(client)
		if err != nil {
			state.AddMessage("system", err.Error())
			return
		}
		wd := cfg.WorkingDirectory
		if wd == "" {
			wd, err = os.Getwd()
			if err != nil {
				state.AddMessage("system", fmt.Sprintf("failed to get working directory: %v", err))
				return
			}
		}
		// A backend that can't literally create a session (claude-mirror attaches
		// to a live window) defines "new session" via ResetSession instead.
		newSession := sessionsBackend.CreateSession
		if resetter, ok := client.(capabilities.BackendSessionResetter); ok {
			newSession = resetter.ResetSession
		}
		session, err := newSession(ctx, wd)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to create session: %v", err))
			return
		}
		state.MarkRunSession(session.ID)
		applySession(state, session)
		if session.Model == nil && state.ModelID == "" {
			applyRecentModel(ctx, state, client, cfg)
		}
		state.ClearConversation()
		if items, err := listSessionItems(ctx, state, client, cfg); err == nil {
			state.SessionItems = items
		}
	case PaletteActionSwitchMode:
		state.ToggleMode()
	case PaletteActionCycleThinkingVisibility:
		state.CycleThinkingVisibility()
	case PaletteActionCycleToolCallVisibility:
		state.CycleToolCallVisibility()
	case PaletteActionLayoutsList:
		state.SetLayoutItems()
	case PaletteActionSelectLayout:
		layoutType, ok := state.SelectedLayout()
		if !ok {
			return
		}
		state.SetLayout(layoutType)
	case PaletteActionChangeModel:
		modelsBackend, err := backendModels(client)
		if err != nil {
			state.AddMessage("system", err.Error())
			return
		}
		providersCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
		providers, connected, err := modelsBackend.ListProviders(providersCtx)
		cancel()
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list models: %v", err))
			return
		}
		state.SetModelItems(modelItemsFromProviders(providers, connected, state.ModelProvider, state.ModelID))
	case PaletteActionSelectModel:
		providerID, modelID := state.SelectedModel()
		if providerID == "" || modelID == "" {
			return
		}
		state.SetModel(providerID, modelID)
	case PaletteActionSessionsList:
		items, err := listSessionItems(ctx, state, client, cfg)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list sessions: %v", err))
			return
		}
		state.SetSessionItems(items)
	case PaletteActionSkillsList:
		if skills == nil {
			state.AddMessage("system", "no skills plugin configured")
			return
		}
		discovered, err := skills.Skills(ctx, cfg.WorkingDirectory)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list skills: %v", err))
			return
		}
		state.SetSkillItems(discovered)
	case PaletteActionSelectSession:
		sessionsBackend, err := backendSessions(client)
		if err != nil {
			state.AddMessage("system", err.Error())
			return
		}
		sessionID := state.SelectedSessionID()
		if sessionID == "" {
			return
		}
		sessionCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
		session, err := sessionsBackend.GetSession(sessionCtx, sessionID)
		cancel()
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to get session: %v", err))
			return
		}
		applySession(state, session)
		messagesCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
		messages, err := sessionsBackend.ListMessages(messagesCtx, sessionID)
		cancel()
		if err != nil {
			state.ClearConversation()
			state.AddMessage("system", fmt.Sprintf("attached session %s; failed to load messages: %v", sessionDisplayName(session), err))
			return
		}
		conversation := make([]Message, 0, len(messages))
		for _, message := range messages {
			content := ""
			emittedTools := make(map[string]bool)
			for _, part := range message.Parts {
				content += displayTextForPart(part, emittedTools)
			}
			if content != "" {
				role := message.Info.Role
				if role == "assistant" {
					role = "ai"
				}
				conversation = append(conversation, Message{Role: role, Content: content})
			}
		}
		state.SetConversation(conversation)
		if items, err := listSessionItems(ctx, state, client, cfg); err == nil {
			state.SessionItems = items
		}
	case PaletteActionSelectSkill:
		skill, ok := state.SelectedSkill()
		if !ok {
			return
		}
		state.InsertSkillMarker(skill)
	}
}

func modelItemsFromProviders(providers []capabilities.Provider, connected []string, currentProvider, currentModel string) []ModelListItem {
	connectedSet := make(map[string]bool, len(connected))
	for _, providerID := range connected {
		connectedSet[providerID] = true
	}
	onlyConnected := len(connectedSet) > 0

	items := []ModelListItem{}
	for _, provider := range providers {
		if onlyConnected && !connectedSet[provider.ID] {
			continue
		}
		modelIDs := make([]string, 0, len(provider.Models))
		for modelID := range provider.Models {
			modelIDs = append(modelIDs, modelID)
		}
		sort.Strings(modelIDs)
		for _, modelID := range modelIDs {
			model := provider.Models[modelID]
			id := model.ID
			if id == "" {
				id = modelID
			}
			providerID := model.ProviderID
			if providerID == "" {
				providerID = provider.ID
			}
			items = append(items, ModelListItem{
				ProviderID:   providerID,
				ProviderName: provider.Name,
				ModelID:      id,
				ModelName:    model.Name,
				Current:      providerID == currentProvider && id == currentModel,
			})
		}
	}
	return items
}
