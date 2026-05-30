package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/core"
	"github.com/Ceinl/plums/internal/debuglog"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui"
)

// RunConfig holds timing and behaviour overrides for the event loop.
type RunConfig struct {
	OpencodeServerURL    string
	ClipboardCommand     string
	SpinnerInterval        time.Duration
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
	RenderConfig  *RenderConfig
	CommandConfig *CommandConfig
	Registry      *core.AgentRegistry
}

// StartupResult is delivered on the startup channel once the backend is ready.
type StartupResult struct {
	Session *adapter.Session
	Server  ServerProcess
	Err     error
}

// Run executes the main event loop. It returns the server process (if any)
// so the caller can perform final cleanup.
func Run(ctx context.Context, deps Deps, cfg RunConfig) (ServerProcess, error) {
	state := NewState(deps.Terminal.W, deps.Terminal.H)
	state.SetAvailableLayouts(deps.RenderConfig.AvailableLayoutTypes())
	state.SetCommandConfig(deps.CommandConfig)
	if skills, err := DiscoverSkills(""); err == nil {
		state.SetAvailableSkills(skills)
	} else {
		debuglog.Printf("skills: discovery failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	startupCh := make(chan StartupResult, 1)
	state.SetServerStarting(true)
	go deps.Startup(ctx, deps.Backend, startupCh)

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
							if replyQuestion(ctx, state, deps.Backend, pendingQuestion.ID, [][]string{{answer}}, cfg.QuestionReplyTimeout) {
								pendingQuestion = nil
							}
						}
					}
				} else {
					handlePaletteAction(ctx, state, deps.Backend, action, cfg)
				}
			}
			if handled {
				if ev.Type == keyboard.KeyEnter && ev.Shift {
					input := state.ConsumeSubmittedInput()
					if input != "" {
						if pendingQuestion != nil {
							answers := parseQuestionAnswers(input, pendingQuestion)
							if replyQuestion(ctx, state, deps.Backend, pendingQuestion.ID, answers, cfg.QuestionReplyTimeout) {
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
							agent := deps.Registry.ResolveAgent(state.Mode)
							aiStream = deps.Backend.SendMessageEvents(sctx, state.SessionID, input, state.ModelProvider, state.ModelID, agent)
						} else {
							state.AddMessage("system", "no active session – is opencode serve running?")
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
				} else if event.Text != "" {
					state.AppendAiOutput(event.Text)
				}
				Render(state, deps.RenderConfig)
			} else {
				state.FinalizeAiOutput()
				refreshSessionModel(ctx, state, deps.Backend, cfg)
				aiStream = nil
				cancelStream = nil
				Render(state, deps.RenderConfig)
			}
		case <-spinTicker.C:
			if state.IsStreaming() || state.ServerStarting {
				Render(state, deps.RenderConfig)
			}
		case result := <-startupCh:
			state.SetServerStarting(false)
			if result.Err != nil {
				state.AddMessage("system", result.Err.Error())
			} else {
				serverProc = result.Server
				state.SetServerReady(true)
				applySession(state, result.Session)
				applyRecentModel(ctx, state, deps.Backend, cfg)
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
