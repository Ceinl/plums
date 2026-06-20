package app

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Ceinl/plums/capabilities"
	cfgpkg "github.com/Ceinl/plums/config"
	"github.com/Ceinl/plums/internal/core"
	"github.com/Ceinl/plums/internal/debuglog"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui"
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

const doubleEscapeStopWindow = 750 * time.Millisecond

type doubleEscapeStopper struct {
	lastEscape time.Time
}

func (s *doubleEscapeStopper) Reset() {
	s.lastEscape = time.Time{}
}

func (s *doubleEscapeStopper) ShouldStop(ev keyboard.Event, streaming bool, now time.Time) bool {
	if !streaming {
		s.Reset()
		return false
	}
	if ev.Type != keyboard.KeyEscape {
		s.Reset()
		return false
	}
	if ev.Alt {
		s.Reset()
		return true
	}
	if !s.lastEscape.IsZero() && now.Sub(s.lastEscape) <= doubleEscapeStopWindow {
		s.Reset()
		return true
	}
	s.lastEscape = now
	return false
}

// RunConfig holds timing and behaviour overrides for the event loop.
type RunConfig struct {
	BackendProvider      string
	ClipboardCommand     string
	SpinnerInterval      time.Duration
	HealthTimeout        time.Duration
	QuestionReplyTimeout time.Duration
	RecentModelTimeout   time.Duration
	ListTimeout          time.Duration
	WorkingDirectory     string
	// DynamicPrefs is the resolved Opts; it tells the runtime which fields were
	// declared cfg.Dynamic and so should be persisted to state.toml on change.
	DynamicPrefs   cfgpkg.Opts
	DefaultLayout  string
	Theme          capabilities.Theme
	HideThinking   bool
	SplitLeftWidth int
	// ClearHistory hides backend sessions that were not created in this run.
	ClearHistory bool
}

// Deps bundles all wired dependencies needed by the event loop.
type Deps struct {
	Terminal     *ui.Terminal
	Keyboard     <-chan keyboard.Event
	Backend      capabilities.Backend
	Startup      func(ctx context.Context, backend capabilities.Backend) (*capabilities.StartupResult, error)
	Backends     []BackendRuntime
	RenderConfig *RenderConfig
	Layouts      []LayoutType
	Commands     []capabilities.Command
	Keybinds     []capabilities.Keybind
	Components   map[string]ComponentFactory
	Hooks        Hooks
	Registry     *core.AgentRegistry
	Skills       capabilities.SkillProvider
	GitDiff      capabilities.GitDiffProvider
	Question     capabilities.QuestionProvider
	// CompletionSources are the completion sources plugins contributed at Init via
	// Host.Services().Completion(). Core prepends its built-in @file/slash sources
	// when building the runtime completion registry.
	CompletionSources []capabilities.CompletionSource
}

// BackendRuntime describes one selectable application backend.
type BackendRuntime struct {
	ID      string
	Name    string
	Backend capabilities.Backend
	Startup func(ctx context.Context, backend capabilities.Backend) (*capabilities.StartupResult, error)
}

// StartupResult is delivered on the startup channel once the backend is ready.
type StartupResult struct {
	BackendID string
	Session   *capabilities.Session
	Server    capabilities.ServerProcess
	Err       error
}

// Run executes the main event loop. It returns the server process (if any)
// so the caller can perform final cleanup.
func Run(ctx context.Context, deps Deps, cfg RunConfig) (capabilities.ServerProcess, error) {
	defer runShutdownHooks(deps.Hooks.OnShutdown)

	state := NewState(deps.Terminal.W, deps.Terminal.H)
	if len(deps.Layouts) > 0 {
		state.SetAvailableLayouts(deps.Layouts)
	} else {
		state.SetAvailableLayouts(deps.RenderConfig.AvailableLayoutTypes())
	}
	state.SetCommands(deps.Commands)
	state.SetComponentFactories(deps.Components)
	// Apply remembered config values (always override NewState's hardcoded default).
	state.Layout = LayoutTypeFromString(cfg.DefaultLayout)
	state.SetTheme(cfg.Theme)
	if cfg.HideThinking {
		state.ThinkingMode = components.ThinkingVisibilityHidden
	} else {
		state.ThinkingMode = components.ThinkingVisibilityFull
	}
	if cfg.SplitLeftWidth > 0 && cfg.SplitLeftWidth <= 100 {
		state.OutputPercent = 100 - cfg.SplitLeftWidth
	}
	runtimes := normalizeBackendRuntimes(deps)
	runtime := selectBackendRuntime(runtimes, cfg.BackendProvider)
	backend := runtime.Backend
	state.SetBackendProvider(runtime.ID)
	state.SetAvailableBackends(backendItemsFromRuntimes(runtimes, runtime.ID))
	if deps.Skills != nil {
		if skills, err := deps.Skills.Skills(ctx, cfg.WorkingDirectory); err == nil {
			state.SetAvailableSkills(skills)
		} else {
			debuglog.Printf("skills: discovery failed: %v", err)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	startupCh := make(chan StartupResult, 1)
	startBackend := func(rt BackendRuntime) {
		state.SetBackendProvider(rt.ID)
		state.SetServerStarting(true)
		state.SetServerReady(false)
		go func() {
			result := StartupResult{BackendID: rt.ID}
			if rt.Startup == nil {
				result.Err = nil
				startupCh <- result
				return
			}
			started, err := rt.Startup(ctx, rt.Backend)
			if err != nil {
				result.Err = err
				startupCh <- result
				return
			}
			if started != nil {
				result.Session = started.Session
				result.Server = started.Server
			}
			startupCh <- result
		}()
	}
	startBackend(runtime)

	Render(state, deps.RenderConfig)

	spinInterval := cfg.SpinnerInterval
	if spinInterval <= 0 {
		spinInterval = 80 * time.Millisecond
	}
	spinTicker := time.NewTicker(spinInterval)
	defer spinTicker.Stop()

	var aiStream <-chan capabilities.StreamEvent
	var cancelStream context.CancelFunc
	var pendingQuestion *capabilities.QuestionRequest
	var serverProc capabilities.ServerProcess
	emittedTools := make(map[string]bool)
	// pendingStreamRender marks streamed output that has been appended but not yet
	// drawn; the spinner tick flushes it so render rate is decoupled from token rate.
	var pendingStreamRender bool
	var escStopper doubleEscapeStopper
	mutations := make(chan stateMutation, 64)
	actionQueue := make(chan runtimeAction, 64)
	promptRequests := make(chan string, 64)
	completion := buildCompletionRegistry(state, deps.CompletionSources)
	state.completion = completion
	actionFns := runtimeActions{}
	newHookCtx := func() *runtimeCtx {
		rt := newRuntimeCtx(state, cfg, mutations, promptRequests)
		rt.completion = completion
		rt.actions = actionQueue
		rt.actionFns = actionFns
		return rt
	}
	expandSubmittedInput := func(input string) string {
		if deps.Skills == nil {
			return input
		}
		return deps.Skills.Expand(input, state.SkillItems)
	}
	parseQuestionAnswers := func(input string, req *capabilities.QuestionRequest) [][]string {
		if deps.Question == nil {
			return [][]string{{strings.TrimSpace(input)}}
		}
		return deps.Question.ParseAnswers(input, req)
	}
	questionTitle := func(req *capabilities.QuestionRequest) string {
		if deps.Question == nil {
			return "Question"
		}
		return deps.Question.Title(req)
	}
	questionOptions := func(req *capabilities.QuestionRequest) []capabilities.QuestionOption {
		if deps.Question == nil {
			return nil
		}
		return deps.Question.Options(req)
	}
	refreshGitDiffIfNeeded := func() {
		if !state.ConsumeGitDiffDirty() {
			return
		}
		if deps.GitDiff == nil {
			state.SetGitDiff("git diff unavailable: no git-diff plugin configured")
			return
		}
		diffCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		diff, err := deps.GitDiff.GitDiff(diffCtx, cfg.WorkingDirectory)
		if err != nil {
			text := strings.TrimSpace(diff)
			if text != "" {
				text += "\n"
			}
			if diffCtx.Err() == context.DeadlineExceeded {
				text += "git diff timed out"
			} else {
				text += err.Error()
			}
			state.SetGitDiff(text)
			return
		}
		state.SetGitDiff(diff)
	}
	emitMessageHook := func(role, text string) {
		if text == "" {
			return
		}
		runMessageHooks(ctx, deps.Hooks.OnMessage, newHookCtx(), capabilities.Message{Role: role, Content: text})
	}
	emitSessionStartHook := func() {
		session := newHookCtx().Session()
		if session.ID == "" {
			return
		}
		runSessionStartHooks(ctx, deps.Hooks.OnSessionStart, newHookCtx(), session)
	}
	emitToolCallHook := func(tool capabilities.ToolCall) {
		runToolCallHooks(ctx, deps.Hooks.OnToolCall, newHookCtx(), tool)
	}

	// Dynamic preferences: snapshot the runtime-remembered fields and, on change,
	// persist only the ones the config declared cfg.Dynamic to state.toml.
	prefWriter := NewDynamicPrefWriter(cfg.DynamicPrefs)
	snapshotPrefs := func() map[string]string {
		return map[string]string{
			prefDefaultLayout: layoutLabel(state.Layout),
			prefHideThinking:  strconv.FormatBool(state.ThinkingMode == components.ThinkingVisibilityHidden),
			prefSplitLeft:     strconv.Itoa(state.SplitLeftPercent()),
			prefOutputPercent: strconv.Itoa(state.SplitOutputPercent()),
			prefBackend:       cfg.BackendProvider,
			prefModel:         state.ModelID,
			prefMode:          state.Mode,
		}
	}
	lastSaved := snapshotPrefs()
	saveConfigIfChanged := func() {
		current := snapshotPrefs()
		updates := map[string]string{}
		for key, value := range current {
			if lastSaved[key] != value {
				updates[key] = value
			}
		}
		if len(updates) == 0 {
			return
		}
		if err := prefWriter.Persist(updates); err != nil {
			debuglog.Printf("prefs: save failed: %v", err)
			return
		}
		lastSaved = current
	}

	resolveServerProc := func() capabilities.ServerProcess {
		if serverProc != nil {
			return serverProc
		}
		if sp, ok := backend.(capabilities.BackendServerProcess); ok {
			if proc := sp.ServerProcess(); proc != nil {
				return proc
			}
		}
		return nil
	}
	stopActiveStream := func() {
		if aborter, ok := backend.(capabilities.BackendSessionAborter); ok && state.SessionID != "" {
			sessionID := state.SessionID
			timeout := cfg.ListTimeout
			go func() {
				abortCtx := ctx
				var cancel context.CancelFunc
				if timeout > 0 {
					abortCtx, cancel = context.WithTimeout(ctx, timeout)
				} else {
					abortCtx, cancel = context.WithCancel(ctx)
				}
				defer cancel()
				if err := aborter.AbortSession(abortCtx, sessionID); err != nil && abortCtx.Err() == nil {
					debuglog.Printf("stream: abort failed: %v", err)
				}
			}()
		}
		if cancelStream != nil {
			cancelStream()
			cancelStream = nil
		}
		aiStream = nil
		if output := state.FinalizeAiOutput(); output != "" {
			emitMessageHook("ai", output)
		}
		escStopper.Reset()
	}
	resolveAgent := func() string {
		if deps.Registry == nil {
			return state.Mode
		}
		return deps.Registry.ResolveAgent(state.Mode)
	}
	submitConsumedPrompt := func(input string) {
		if input == "" {
			return
		}
		if pendingQuestion != nil {
			answers := parseQuestionAnswers(input, pendingQuestion)
			if replyQuestion(ctx, state, backend, pendingQuestion.ID, answers, cfg.QuestionReplyTimeout) {
				pendingQuestion = nil
			}
			return
		}
		hadSession := state.SessionID != ""
		if state.SessionID == "" {
			if err := ensureSession(ctx, state, backend, cfg); err != nil {
				state.AddMessage("system", err.Error())
				return
			}
		}
		if state.SessionID == "" {
			state.AddMessage("system", "no active session for backend provider "+runtime.ID)
			return
		}
		if !hadSession {
			emitSessionStartHook()
		}
		if cancelStream != nil {
			cancelStream()
		}
		var sctx context.Context
		sctx, cancelStream = context.WithCancel(ctx)
		state.SetStreaming(true)
		state.ClearAiOutput()
		escStopper.Reset()
		emittedTools = make(map[string]bool)
		aiStream = backend.SendMessageEvents(sctx, state.SessionID, input, state.ModelProvider, state.ModelID, resolveAgent())
	}
	submitPrompt := func(input string) {
		if state.SubmitPrompt(input) == "" {
			return
		}
		raw := state.ConsumeSubmittedMessage()
		expanded := expandSubmittedInput(state.ConsumeSubmittedInput())
		emitMessageHook("user", raw)
		submitConsumedPrompt(expanded)
	}
	runCommand := func(command capabilities.Command) {
		if command.Do == nil {
			return
		}
		commandCtx := newHookCtx()
		go func() {
			if err := command.Do(ctx, commandCtx); err != nil {
				commandCtx.Chat("system", err.Error())
			}
		}()
	}

	drainPendingListPick := func() {
		if item, onPick, ok := state.ConsumePendingListPick(); ok && onPick != nil {
			go onPick(item)
		}
	}
	actionFns = runtimeActions{
		openCommandPalette: func() { state.OpenPalette() },
		changeModel: func() {
			openModelList(ctx, state, backend, cfg)
		},
		setModel: func(providerID, modelID string) {
			if providerID == "" || modelID == "" {
				return
			}
			state.SetModel(providerID, modelID)
		},
		switchBackend: func() {
			state.SetBackendItems(backendItemsFromRuntimes(runtimes, runtime.ID))
		},
		selectBackend: func(backendID string) {
			if selected, ok := backendRuntimeByID(runtimes, backendID); ok && selected.ID != runtime.ID {
				if cancelStream != nil {
					cancelStream()
					cancelStream = nil
				}
				if serverProc != nil {
					serverProc.Stop()
					serverProc = nil
				} else if sp, ok := backend.(capabilities.BackendServerProcess); ok {
					if proc := sp.ServerProcess(); proc != nil {
						proc.Stop()
					}
				}
				runtime = selected
				backend = selected.Backend
				state.ResetBackendSession()
				state.SessionItems = nil
				startBackend(selected)
			}
		},
		newSession: func() {
			createNewSession(ctx, state, backend, cfg)
		},
		openSessions: func() {
			openSessionList(ctx, state, backend, cfg)
		},
		openSession: func(sessionID string) {
			openSessionByID(ctx, state, backend, cfg, sessionID)
		},
		openSkills: func() {
			openSkillsList(ctx, state, cfg, deps.Skills)
		},
		answerQuestion: func(answer string) {
			if pendingQuestion == nil {
				return
			}
			if replyQuestion(ctx, state, backend, pendingQuestion.ID, [][]string{{answer}}, cfg.QuestionReplyTimeout) {
				pendingQuestion = nil
			}
		},
		switchLayout: func() {
			state.SetLayoutItems()
		},
	}

	for {
		select {
		case ev, ok := <-deps.Keyboard:
			if !ok {
				if cancelStream != nil {
					cancelStream()
				}
				return resolveServerProc(), nil
			}
			handled := HandleConfiguredKeybind(state, ev, deps.Keybinds)
			quit := false
			if !handled {
				handled = HandlePublicComponentEvent(state, ev, newHookCtx())
			}
			if !handled {
				handled, quit = HandleKey(state, ev, cfg.ClipboardCommand)
			}
			if quit {
				if cancelStream != nil {
					cancelStream()
				}
				return resolveServerProc(), nil
			}
			if escStopper.ShouldStop(ev, state.IsStreaming(), time.Now()) {
				stopActiveStream()
				handled = true
			}
			drainPendingListPick()
			if command, ok := state.ConsumePendingCommand(); ok {
				runCommand(command)
			}
			if handled {
				if ev.Type == keyboard.KeyEnter {
					// In split layout Shift+Enter submits; in chat Enter submits.
					isSubmit := (!ev.Shift && state.EffectiveLayout() != LayoutSplit) || (ev.Shift && state.EffectiveLayout() == LayoutSplit)
					if isSubmit {
						raw := state.ConsumeSubmittedMessage()
						expanded := expandSubmittedInput(state.ConsumeSubmittedInput())
						emitMessageHook("user", raw)
						submitConsumedPrompt(expanded)
					}
				}
				refreshGitDiffIfNeeded()
				Render(state, deps.RenderConfig)
				saveConfigIfChanged()
			}
		case input := <-promptRequests:
			submitPrompt(input)
			Render(state, deps.RenderConfig)
			saveConfigIfChanged()
		case action := <-actionQueue:
			if action != nil {
				action()
				drainPendingListPick()
				refreshGitDiffIfNeeded()
				Render(state, deps.RenderConfig)
				saveConfigIfChanged()
			}
		case event, ok := <-aiStream:
			if ok {
				if event.Tool != nil {
					emitToolCallHook(*event.Tool)
				}
				if event.Question != nil {
					pendingQuestion = event.Question
					state.SetStreaming(false)
					state.SetQuestionItems(questionTitle(event.Question), questionOptions(event.Question))
					// Streaming just stopped, so the spinner tick won't redraw;
					// render the question prompt now.
					Render(state, deps.RenderConfig)
				} else if text := displayTextForStreamEvent(event, emittedTools); text != "" {
					// Don't render per token: high-rate streams (opencode emits a
					// delta per token) would re-render faster than a frame can be
					// built, stalling the loop. The spinner tick coalesces these
					// into ~12 fps while streaming.
					state.AppendAiOutput(text)
					pendingStreamRender = true
				}
			} else {
				pendingStreamRender = false
				if output := state.FinalizeAiOutput(); output != "" {
					emitMessageHook("ai", output)
				}
				refreshSessionModel(ctx, state, backend, cfg)
				aiStream = nil
				cancelStream = nil
				escStopper.Reset()
				Render(state, deps.RenderConfig)
			}
		case <-spinTicker.C:
			if state.IsStreaming() || state.ServerStarting || pendingStreamRender {
				pendingStreamRender = false
				Render(state, deps.RenderConfig)
			}
		case result := <-startupCh:
			if result.BackendID != runtime.ID {
				break
			}
			state.SetServerStarting(false)
			if result.Err != nil {
				state.AddMessage("system", result.Err.Error())
			} else {
				serverProc = result.Server
				state.SetServerReady(true)
				beforeSession := state.SessionID
				applySession(state, result.Session)
				if beforeSession == "" && state.SessionID != "" {
					emitSessionStartHook()
				}
				applyRecentModel(ctx, state, backend, cfg)
				if items, err := listSessionItems(ctx, state, backend, cfg); err == nil {
					state.SessionItems = items
				}
			}
			Render(state, deps.RenderConfig)
		case mutation := <-mutations:
			if mutation != nil {
				mutation(state)
				drainPendingListPick()
				refreshGitDiffIfNeeded()
				Render(state, deps.RenderConfig)
				saveConfigIfChanged()
			}
		case <-sigCh:
			if err := deps.Terminal.RefreshSize(); err == nil {
				state.Resize(deps.Terminal.W, deps.Terminal.H)
				Render(state, deps.RenderConfig)
			} else {
				debuglog.Printf("terminal: resize failed: %v", err)
			}
		}
	}
}

func normalizeBackendRuntimes(deps Deps) []BackendRuntime {
	if len(deps.Backends) > 0 {
		return deps.Backends
	}
	return []BackendRuntime{{ID: "opencode", Name: "Opencode", Backend: deps.Backend, Startup: deps.Startup}}
}

func selectBackendRuntime(runtimes []BackendRuntime, id string) BackendRuntime {
	if rt, ok := backendRuntimeByID(runtimes, id); ok {
		return rt
	}
	return runtimes[0]
}

func backendRuntimeByID(runtimes []BackendRuntime, id string) (BackendRuntime, bool) {
	for _, runtime := range runtimes {
		if runtime.ID == id {
			return runtime, true
		}
	}
	return BackendRuntime{}, false
}

func backendItemsFromRuntimes(runtimes []BackendRuntime, current string) []BackendListItem {
	items := make([]BackendListItem, 0, len(runtimes))
	for _, runtime := range runtimes {
		name := runtime.Name
		if name == "" {
			name = runtime.ID
		}
		items = append(items, BackendListItem{ID: runtime.ID, Name: name, Current: runtime.ID == current})
	}
	return items
}
