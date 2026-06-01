package app

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/Ceinl/plums/internal/core/adapter"
)

func listSessionItems(ctx context.Context, state *State, client adapter.Backend, cfg RunConfig) ([]SessionListItem, error) {
	listCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
	defer cancel()
	sessions, err := client.ListSessions(listCtx)
	if err != nil {
		return nil, err
	}
	items := make([]SessionListItem, len(sessions))
	for i, session := range sessions {
		items[i] = SessionListItem{
			ID:        session.ID,
			Title:     session.Title,
			Directory: session.Directory,
			Updated:   session.Time.Updated,
			Current:   session.ID == state.SessionID,
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Directory != items[j].Directory {
			return items[i].Directory < items[j].Directory
		}
		return items[i].Updated > items[j].Updated
	})
	return items, nil
}

func handlePaletteAction(ctx context.Context, state *State, client adapter.Backend, action PaletteAction, cfg RunConfig) {
	switch action {
	case PaletteActionOpenPalette:
		state.OpenPalette()
	case PaletteActionNewSession:
		wd := cfg.WorkingDirectory
		if wd == "" {
			var err error
			wd, err = os.Getwd()
			if err != nil {
				state.AddMessage("system", fmt.Sprintf("failed to get working directory: %v", err))
				return
			}
		}
		session, err := client.CreateSession(ctx, wd)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to create session: %v", err))
			return
		}
		applySession(state, session)
		if session.Model == nil && state.ModelID == "" {
			applyRecentModel(ctx, state, client, cfg)
		}
		state.ClearConversation()
		state.AddMessage("system", "started new session "+sessionDisplayName(session))
		if items, err := listSessionItems(ctx, state, client, cfg); err == nil {
			state.SessionItems = items
		}
	case PaletteActionSwitchMode:
		state.ToggleMode()
		state.AddMessage("system", "switched to "+state.Mode+" mode")
	case PaletteActionCycleThinkingVisibility:
		state.CycleThinkingVisibility()
		state.AddMessage("system", "thinking visibility: "+state.ThinkingVisibilityLabel())
	case PaletteActionLayoutsList:
		state.SetLayoutItems()
	case PaletteActionChatLayout:
		state.SetLayout(LayoutChat)
		state.AddMessage("system", "layout: "+state.LayoutLabel())
	case PaletteActionSelectLayout:
		layoutType, ok := state.SelectedLayout()
		if !ok {
			return
		}
		state.SetLayout(layoutType)
		state.AddMessage("system", "layout: "+state.LayoutLabel())
	case PaletteActionChangeModel:
		providersCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
		providers, connected, err := client.ListProviders(providersCtx)
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
		state.AddMessage("system", "switched model to "+providerID+"/"+modelID)
	case PaletteActionSessionsList:
		items, err := listSessionItems(ctx, state, client, cfg)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list sessions: %v", err))
			return
		}
		state.SetSessionItems(items)
	case PaletteActionSkillsList:
		skills, err := DiscoverSkills("")
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list skills: %v", err))
			return
		}
		state.SetSkillItems(skills)
	case PaletteActionSelectSession:
		sessionID := state.SelectedSessionID()
		if sessionID == "" {
			return
		}
		sessionCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
		session, err := client.GetSession(sessionCtx, sessionID)
		cancel()
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to get session: %v", err))
			return
		}
		applySession(state, session)
		messagesCtx, cancel := context.WithTimeout(ctx, cfg.ListTimeout)
		messages, err := client.ListMessages(messagesCtx, sessionID)
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

func modelItemsFromProviders(providers []adapter.Provider, connected []string, currentProvider, currentModel string) []ModelListItem {
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
