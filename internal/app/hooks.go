package app

import (
	"context"

	"github.com/Ceinl/plums/capabilities"
)

type Hooks struct {
	OnMessage      []capabilities.OnMessage
	OnSessionStart []capabilities.OnSessionStart
	OnToolCall     []capabilities.OnToolCall
	OnShutdown     []capabilities.OnShutdown
}

func runMessageHooks(ctx context.Context, hooks []capabilities.OnMessage, runtime *runtimeCtx, message capabilities.Message) {
	if runtime == nil || len(hooks) == 0 {
		return
	}
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		hook := hook
		go hook.OnMessage(runtime, message)
	}
}

func runSessionStartHooks(ctx context.Context, hooks []capabilities.OnSessionStart, runtime *runtimeCtx, session capabilities.Session) {
	if runtime == nil || len(hooks) == 0 {
		return
	}
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		hook := hook
		go hook.OnSessionStart(runtime, session)
	}
}

func runToolCallHooks(ctx context.Context, hooks []capabilities.OnToolCall, runtime *runtimeCtx, tool capabilities.ToolCall) {
	if runtime == nil || len(hooks) == 0 {
		return
	}
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		hook := hook
		go hook.OnToolCall(runtime, tool)
	}
}

func runShutdownHooks(hooks []capabilities.OnShutdown) {
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		hook.OnShutdown()
	}
}
