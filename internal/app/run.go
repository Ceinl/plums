package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ceinl/plums/internal/core"
	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/debuglog"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui"
)

// RunConfig holds timing and behaviour overrides for the event loop.
type RunConfig struct {
	OpencodeServerURL    string
	BackendProvider      string
	ClipboardCommand     string
	SpinnerInterval      time.Duration
	HealthTimeout        time.Duration
	QuestionReplyTimeout time.Duration
	RecentModelTimeout   time.Duration
	ListTimeout          time.Duration
	WorkingDirectory     string
}

// ServerProcess abstracts the lifecycle of a managed backend server.
type ServerProcess interface {
	Stop()
	Done() <-chan struct{}
}

// Deps bundles all wired dependencies needed by the event loop.
type Deps struct {
	Terminal      *ui.Terminal
	Keyboard      <-chan keyboard.Event
	Backend       adapter.Backend
	Startup       func(ctx context.Context, backend adapter.Backend, out chan<- StartupResult)
	Backends      []BackendRuntime
	RenderConfig  *RenderConfig
	CommandConfig *CommandConfig
	Registry      *core.AgentRegistry
}

// BackendRuntime describes one selectable application backend.
type BackendRuntime struct {
	ID      string
	Name    string
	Backend adapter.Backend
	Startup func(ctx context.Context, backend adapter.Backend, out chan<- StartupResult)
}

// StartupResult is delivered on the startup channel once the backend is ready.
type StartupResult struct {
	BackendID string
	Session   *adapter.Session
	Server    ServerProcess
	Err       error
}

// Run executes the main event loop. It returns the server process (if any)
// so the caller can perform final cleanup.
func Run(ctx context.Context, deps Deps, cfg RunConfig) (ServerProcess, error) {
	state := NewState(deps.Terminal.W, deps.Terminal.H)
	state.SetAvailableLayouts(deps.RenderConfig.AvailableLayoutTypes())
	state.SetCommandConfig(deps.CommandConfig)
	runtimes := normalizeBackendRuntimes(deps)
	runtime := selectBackendRuntime(runtimes, cfg.BackendProvider)
	backend := runtime.Backend
	state.SetBackendProvider(runtime.ID)
	state.SetAvailableBackends(backendItemsFromRuntimes(runtimes, runtime.ID))
	if skills, err := DiscoverSkills(""); err == nil {
		state.SetAvailableSkills(skills)
	} else {
		debuglog.Printf("skills: discovery failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	startupCh := make(chan StartupResult, 1)
	startBackend := func(rt BackendRuntime) {
		state.SetBackendProvider(rt.ID)
		state.SetServerStarting(true)
		state.SetServerReady(false)
		go func() {
			ch := make(chan StartupResult, 1)
			rt.Startup(ctx, rt.Backend, ch)
			result := <-ch
			result.BackendID = rt.ID
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

	var aiStream <-chan adapter.StreamEvent
	var cancelStream context.CancelFunc
	var pendingQuestion *adapter.QuestionRequest
	var serverProc ServerProcess
	emittedTools := make(map[string]bool)

	for {
		select {
		case ev, ok := <-deps.Keyboard:
			if !ok {
				if cancelStream != nil {
					cancelStream()
				}
				return serverProc, nil
			}
			handled, quit := HandleKey(state, ev, cfg.ClipboardCommand)
			if quit {
				if cancelStream != nil {
					cancelStream()
				}
				return serverProc, nil
			}
			if action := state.ConsumePendingAction(); action != PaletteActionNone {
				if action == PaletteActionAnswerQuestion {
					if pendingQuestion != nil {
						if answer, ok := state.SelectedQuestionAnswer(); ok {
							if replyQuestion(ctx, state, backend, pendingQuestion.ID, [][]string{{answer}}, cfg.QuestionReplyTimeout) {
								pendingQuestion = nil
							}
						}
					}
				} else if action == PaletteActionBackendList {
					state.SetBackendItems(backendItemsFromRuntimes(runtimes, runtime.ID))
				} else if action == PaletteActionSelectBackend {
					backendID := state.SelectedBackendID()
					if selected, ok := backendRuntimeByID(runtimes, backendID); ok && selected.ID != runtime.ID {
						if cancelStream != nil {
							cancelStream()
							cancelStream = nil
						}
						if serverProc != nil {
							serverProc.Stop()
							serverProc = nil
						}
						runtime = selected
						backend = selected.Backend
						state.ResetBackendSession()
						state.AddMessage("system", "switching backend provider to "+selected.ID)
						startBackend(selected)
					}
				} else {
					handlePaletteAction(ctx, state, backend, action, cfg)
				}
			}
			if handled {
				if ev.Type == keyboard.KeyEnter && ev.Shift {
					input := state.ConsumeSubmittedInput()
					if input != "" {
						if pendingQuestion != nil {
							answers := parseQuestionAnswers(input, pendingQuestion)
							if replyQuestion(ctx, state, backend, pendingQuestion.ID, answers, cfg.QuestionReplyTimeout) {
								pendingQuestion = nil
							}
							Render(state, deps.RenderConfig)
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
							emittedTools = make(map[string]bool)
							agent := deps.Registry.ResolveAgent(state.Mode)
							aiStream = backend.SendMessageEvents(sctx, state.SessionID, input, state.ModelProvider, state.ModelID, agent)
						} else {
							state.AddMessage("system", "no active session for backend provider "+runtime.ID)
						}
					}
				}
				Render(state, deps.RenderConfig)
			}
		case event, ok := <-aiStream:
			if ok {
				if event.Question != nil {
					pendingQuestion = event.Question
					state.SetStreaming(false)
					state.SetQuestionItems(questionTitle(event.Question), questionOptionItems(event.Question))
				} else if text := displayTextForStreamEvent(event, emittedTools); text != "" {
					state.AppendAiOutput(text)
				}
				Render(state, deps.RenderConfig)
			} else {
				state.FinalizeAiOutput()
				refreshSessionModel(ctx, state, backend, cfg)
				aiStream = nil
				cancelStream = nil
				Render(state, deps.RenderConfig)
			}
		case <-spinTicker.C:
			if state.IsStreaming() || state.ServerStarting {
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
				applySession(state, result.Session)
				applyRecentModel(ctx, state, backend, cfg)
			}
			Render(state, deps.RenderConfig)
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
