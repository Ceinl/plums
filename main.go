package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"plums/internal/ai"
	"plums/internal/app"
	"plums/internal/debuglog"
	"plums/internal/keyboard"
	"plums/internal/ui"
)

const plumsConfigPath = ".agents/plums/config/config.toml"

type startupResult struct {
	session *ai.Session
	server  *ai.ServerProcess
	err     error
}

func main() {
	defer debuglog.Close()

	configGlobal := flag.Bool("config-global", false, "use global plums layout config")
	configGlobalShort := flag.Bool("cg", false, "use global plums layout config")
	configLocal := flag.Bool("config-local", false, "use local plums layout config")
	configLocalShort := flag.Bool("cl", false, "use local plums layout config")
	flag.Parse()

	configPath, err := resolveConfigPath(*configGlobal || *configGlobalShort, *configLocal || *configLocalShort)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	renderConfig, err := app.LoadRenderConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load layout config: %v\n", err)
		os.Exit(1)
	}
	commandConfigPath, err := resolveCommandsConfigPath(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	commandConfig, err := app.LoadCommandConfig(commandConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load command config: %v\n", err)
		os.Exit(1)
	}
	if configPath != "" {
		debuglog.Printf("config: using %s", configPath)
	} else {
		debuglog.Printf("config: using built-in layout config")
	}
	if commandConfigPath != "" {
		debuglog.Printf("config: using %s", commandConfigPath)
	} else {
		debuglog.Printf("config: using built-in command config")
	}

	t := ui.NewTerminal(int(os.Stdin.Fd()))
	if err := t.Enter(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize terminal: %v\n", err)
		os.Exit(1)
	}
	defer t.Exit()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys := keyboard.Listen(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	state := app.NewState(t.W, t.H)
	state.SetAvailableLayouts(renderConfig.AvailableLayoutTypes())
	state.SetCommandConfig(commandConfig)
	if skills, err := app.DiscoverSkills(""); err == nil {
		state.SetAvailableSkills(skills)
	} else {
		debuglog.Printf("skills: discovery failed: %v", err)
	}

	opencodeServerURL, err := loadOpencodeServerURL(plumsConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load opencode server URL: %v\n", err)
		os.Exit(1)
	}
	debuglog.Printf("config: opencode server URL %s", opencodeServerURL)
	client := ai.NewClientWithURL(opencodeServerURL)
	var serverProc *ai.ServerProcess
	defer func() { serverProc.Stop() }()

	startupCh := make(chan startupResult, 1)
	state.SetServerStarting(true)
	go startOpencode(ctx, client, startupCh)

	app.Render(state, renderConfig)

	// Spinner ticker: re-render at 80 ms so the Braille animation is smooth
	// even while the model is thinking (no tokens arriving yet).
	spinTicker := time.NewTicker(80 * time.Millisecond)
	defer spinTicker.Stop()

	var aiStream <-chan ai.StreamEvent
	var cancelStream context.CancelFunc
	var pendingQuestion *ai.QuestionRequest

	for {
		select {
		case ev, ok := <-keys:
			if !ok {
				if cancelStream != nil {
					cancelStream()
				}
				cancel()
				return
			}
			handled, quit := handleKey(state, ev)
			if quit {
				if cancelStream != nil {
					cancelStream()
				}
				cancel()
				return
			}
			if action := state.ConsumePendingAction(); action != app.PaletteActionNone {
				if action == app.PaletteActionAnswerQuestion {
					if pendingQuestion != nil {
						if answer, ok := state.SelectedQuestionAnswer(); ok {
							if replyQuestion(ctx, state, client, pendingQuestion.ID, [][]string{{answer}}) {
								pendingQuestion = nil
							}
						}
					}
				} else {
					handlePaletteAction(ctx, state, client, action)
				}
			}
			if handled {
				if ev.Type == keyboard.KeyEnter && ev.Shift {
					input := state.ConsumeSubmittedInput()
					if input != "" {
						if pendingQuestion != nil {
							answers := parseQuestionAnswers(input, pendingQuestion)
							if replyQuestion(ctx, state, client, pendingQuestion.ID, answers) {
								pendingQuestion = nil
							}
							app.Render(state, renderConfig)
							continue
						}
						if state.SessionID != "" {
							if cancelStream != nil {
								cancelStream()
							}
							var sctx context.Context
							sctx, cancelStream = context.WithCancel(ctx)
							state.SetStreaming(true)
							state.ClearAiOutput()
							aiStream = client.SendMessageEvents(sctx, state.SessionID, input, state.ModelProvider, state.ModelID, state.Mode)
						} else {
							state.AddMessage("system", "no active session – is opencode serve running?")
						}
					}
				}
				app.Render(state, renderConfig)
			}
		case event, ok := <-aiStream:
			if ok {
				if event.Question != nil {
					pendingQuestion = event.Question
					state.SetStreaming(false)
					state.SetQuestionItems(questionTitle(event.Question), questionOptionItems(event.Question))
				} else if event.Text != "" {
					state.AppendAiOutput(event.Text)
				}
				app.Render(state, renderConfig)
			} else {
				state.FinalizeAiOutput()
				refreshSessionModel(ctx, state, client)
				aiStream = nil
				cancelStream = nil
				app.Render(state, renderConfig)
			}
		case <-spinTicker.C:
			if state.IsStreaming() {
				app.Render(state, renderConfig)
			}
		case result := <-startupCh:
			state.SetServerStarting(false)
			if result.err != nil {
				state.AddMessage("system", result.err.Error())
			} else {
				serverProc = result.server
				state.SetServerReady(true)
				applySession(state, result.session)
				applyRecentModel(ctx, state, client)
			}
			app.Render(state, renderConfig)
		case <-sigCh:
			if err := t.RefreshSize(); err == nil {
				state.Resize(t.W, t.H)
				app.Render(state, renderConfig)
			} else {
				debuglog.Printf("terminal: resize failed: %v", err)
			}
		}
	}
}

func loadOpencodeServerURL(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ai.DefaultBaseURL, nil
		}
		return "", err
	}
	defer func() { _ = file.Close() }()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "opencode_server_url" || section != "opencode" {
			continue
		}
		url, err := parseTomlString(value)
		if err != nil {
			return "", fmt.Errorf("%s opencode.opencode_server_url: %w", path, err)
		}
		if url == "" {
			return "", fmt.Errorf("%s opencode.opencode_server_url is empty", path)
		}
		return url, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return ai.DefaultBaseURL, nil
}

func parseTomlString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if i := strings.Index(value, " #"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("expected quoted string")
	}
	return strings.TrimSpace(value[1 : len(value)-1]), nil
}

func resolveConfigPath(global, local bool) (string, error) {
	if global && local {
		return "", fmt.Errorf("use only one of --config-global/-cg or --config-local/-cl")
	}
	if local {
		return "./.agents/plums/config/layout.json", nil
	}
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home + "/.config/plums/config/layout.json", nil
	}
	return "", nil
}

func resolveCommandsConfigPath(layoutConfigPath string) (string, error) {
	if layoutConfigPath == "" {
		return "", nil
	}
	path := strings.TrimSuffix(layoutConfigPath, "layout.json") + "commands.json"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

func replyQuestion(ctx context.Context, state *app.State, client *ai.Client, requestID string, answers [][]string) bool {
	replyCtx, cancelReply := context.WithTimeout(ctx, 5*time.Second)
	err := client.ReplyQuestion(replyCtx, requestID, answers)
	cancelReply()
	if err != nil {
		debuglog.Printf("question: reply failed: %v", err)
		state.AddMessage("system", fmt.Sprintf("failed to answer question: %v", err))
		return false
	}
	state.SetStreaming(true)
	return true
}

func questionTitle(req *ai.QuestionRequest) string {
	if req == nil || len(req.Questions) == 0 {
		return "Question"
	}
	q := req.Questions[0]
	if q.Header != "" {
		return q.Header + ": " + q.Question
	}
	return q.Question
}

func questionOptionItems(req *ai.QuestionRequest) []app.QuestionOptionItem {
	if req == nil || len(req.Questions) == 0 {
		return nil
	}
	q := req.Questions[0]
	items := make([]app.QuestionOptionItem, len(q.Options))
	for i, option := range q.Options {
		items[i] = app.QuestionOptionItem{Label: option.Label, Description: option.Description}
	}
	return items
}

func parseQuestionAnswers(input string, req *ai.QuestionRequest) [][]string {
	if req == nil || len(req.Questions) == 0 {
		return [][]string{{strings.TrimSpace(input)}}
	}

	lines := splitNonEmptyLines(input)
	answers := make([][]string, len(req.Questions))
	for i, q := range req.Questions {
		answer := strings.TrimSpace(input)
		if len(req.Questions) > 1 && i < len(lines) {
			answer = lines[i]
		}
		if q.Multiple {
			answers[i] = splitCommaAnswers(answer)
		} else {
			answers[i] = []string{answer}
		}
	}
	return answers
}

func splitNonEmptyLines(input string) []string {
	var out []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitCommaAnswers(input string) []string {
	var out []string
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(input)}
	}
	return out
}

func startOpencode(ctx context.Context, client *ai.Client, out chan<- startupResult) {
	var serverProc *ai.ServerProcess
	debuglog.Printf("startup: checking opencode health")
	if err := client.Health(ctx); err != nil {
		debuglog.Printf("startup: health check failed: %v", err)
		proc, err := ai.StartServer(ctx, client.BaseURL())
		if err != nil {
			debuglog.Printf("startup: start server failed: %v", err)
			out <- startupResult{err: fmt.Errorf("failed to start opencode server: %w", err)}
			return
		}
		serverProc = proc
		debuglog.Printf("startup: started opencode server process")
		if err := ai.WaitForHealthOrExit(ctx, client, serverProc, 10*time.Second); err != nil {
			debuglog.Printf("startup: wait for health failed: %v", err)
			serverProc.Stop()
			out <- startupResult{err: fmt.Errorf("failed to start opencode server: %w", err)}
			return
		}
	} else {
		debuglog.Printf("startup: existing opencode server is healthy")
	}

	debuglog.Printf("startup: creating session")
	session, err := client.CreateSession(ctx)
	if err != nil {
		debuglog.Printf("startup: create session failed: %v", err)
		serverProc.Stop()
		out <- startupResult{err: fmt.Errorf("failed to create session: %w", err)}
		return
	}
	debuglog.Printf("startup: session ready: %s", session.ID)
	out <- startupResult{session: session, server: serverProc}
}

func applySession(state *app.State, session *ai.Session) {
	if session == nil {
		return
	}
	state.SetSessionID(session.ID)
	state.SetSessionTitle(session.Title)
	if session.Model != nil {
		state.SetModel(session.Model.ProviderID, session.Model.ID)
	}
}

func applyRecentModel(ctx context.Context, state *app.State, client *ai.Client) {
	if state.ModelID != "" {
		return
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	sessions, err := client.ListSessions(lookupCtx)
	if err != nil {
		debuglog.Printf("session: recent model lookup failed: %v", err)
		return
	}
	wd, err := os.Getwd()
	if err != nil {
		debuglog.Printf("session: recent model working directory failed: %v", err)
		return
	}
	var latest *ai.Session
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

func sessionDisplayName(session *ai.Session) string {
	if session == nil || session.Title == "" {
		if session == nil {
			return ""
		}
		return session.ID
	}
	return session.Title
}

func refreshSessionModel(ctx context.Context, state *app.State, client *ai.Client) {
	if state.SessionID == "" {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	session, err := client.GetSession(refreshCtx, state.SessionID)
	if err != nil {
		debuglog.Printf("session: refresh model failed: %v", err)
		return
	}
	applySession(state, session)
}

func handlePaletteAction(ctx context.Context, state *app.State, client *ai.Client, action app.PaletteAction) {
	switch action {
	case app.PaletteActionOpenPalette:
		state.OpenPalette()
	case app.PaletteActionNewSession:
		session, err := client.CreateSession(ctx)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to create session: %v", err))
			return
		}
		applySession(state, session)
		if session.Model == nil && state.ModelID == "" {
			applyRecentModel(ctx, state, client)
		}
		state.ClearConversation()
		state.AddMessage("system", "started new session "+sessionDisplayName(session))
	case app.PaletteActionSwitchMode:
		state.ToggleMode()
		state.AddMessage("system", "switched to "+state.Mode+" mode")
	case app.PaletteActionChangeModel:
		providersCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		providers, connected, err := client.ListProviders(providersCtx)
		cancel()
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list models: %v", err))
			return
		}
		state.SetModelItems(modelItemsFromProviders(providers, connected, state.ModelProvider, state.ModelID))
	case app.PaletteActionSelectModel:
		providerID, modelID := state.SelectedModel()
		if providerID == "" || modelID == "" {
			return
		}
		state.SetModel(providerID, modelID)
		state.AddMessage("system", "switched model to "+providerID+"/"+modelID)
	case app.PaletteActionSessionsList:
		listCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		sessions, err := client.ListSessions(listCtx)
		cancel()
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list sessions: %v", err))
			return
		}
		items := make([]app.SessionListItem, len(sessions))
		for i, session := range sessions {
			items[i] = app.SessionListItem{ID: session.ID, Title: session.Title, Current: session.ID == state.SessionID}
		}
		state.SetSessionItems(items)
	case app.PaletteActionSkillsList:
		skills, err := app.DiscoverSkills("")
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to list skills: %v", err))
			return
		}
		state.SetSkillItems(skills)
	case app.PaletteActionSelectSession:
		sessionID := state.SelectedSessionID()
		if sessionID == "" {
			return
		}
		sessionCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		session, err := client.GetSession(sessionCtx, sessionID)
		cancel()
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to get session: %v", err))
			return
		}
		applySession(state, session)
		messagesCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		messages, err := client.ListMessages(messagesCtx, sessionID)
		cancel()
		if err != nil {
			state.ClearConversation()
			state.AddMessage("system", fmt.Sprintf("attached session %s; failed to load messages: %v", sessionDisplayName(session), err))
			return
		}
		conversation := make([]app.Message, 0, len(messages))
		for _, message := range messages {
			content := ""
			for _, part := range message.Parts {
				if part.Type == "text" {
					content += part.Text
				}
			}
			if content != "" {
				role := message.Info.Role
				if role == "assistant" {
					role = "ai"
				}
				conversation = append(conversation, app.Message{Role: role, Content: content})
			}
		}
		state.SetConversation(conversation)
	case app.PaletteActionSelectSkill:
		skill, ok := state.SelectedSkill()
		if !ok {
			return
		}
		state.InsertSkillMarker(skill)
	}
}

func modelItemsFromProviders(providers []ai.Provider, connected []string, currentProvider, currentModel string) []app.ModelListItem {
	connectedSet := make(map[string]bool, len(connected))
	for _, providerID := range connected {
		connectedSet[providerID] = true
	}
	onlyConnected := len(connectedSet) > 0

	items := []app.ModelListItem{}
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
			items = append(items, app.ModelListItem{
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
