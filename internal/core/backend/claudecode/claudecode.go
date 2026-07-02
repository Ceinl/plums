package claudecode

import (
	"context"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/core/backend/streambridge"
	provider "github.com/Ceinl/plums/internal/core/provider/claudecode"
	"github.com/Ceinl/plums/internal/debuglog"
)

const streamSource = "claude.stream-json"

func Registration() capabilities.BackendRegistration {
	return capabilities.BackendRegistration{
		Name:    "claude",
		Label:   "Claude Code",
		Backend: backend{client: provider.NewClient()},
		Startup: startup(),
	}
}

type backend struct {
	client *provider.Client
}

func (b backend) Health(ctx context.Context) error {
	return b.client.Health(ctx)
}

func (b backend) CreateSession(ctx context.Context, dir string) (*capabilities.Session, error) {
	session, err := b.client.CreateSession(ctx, dir)
	return streambridge.Ptr(session, toSession), err
}

func (b backend) ListSessions(ctx context.Context) ([]capabilities.Session, error) {
	sessions, err := b.client.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	return streambridge.Map(sessions, toSession), nil
}

func (b backend) GetSession(ctx context.Context, id string) (*capabilities.Session, error) {
	session, err := b.client.GetSession(ctx, id)
	return streambridge.Ptr(session, toSession), err
}

func (b backend) ListMessages(ctx context.Context, id string) ([]capabilities.MessageResponse, error) {
	messages, err := b.client.ListMessages(ctx, id)
	if err != nil {
		return nil, err
	}
	return streambridge.Map(messages, toMessageResponse), nil
}

func (b backend) ListProviders(ctx context.Context) ([]capabilities.Provider, []string, error) {
	providers, connected, err := b.client.ListProviders(ctx)
	if err != nil {
		return nil, nil, err
	}
	return streambridge.Map(providers, toProvider), append([]string(nil), connected...), nil
}

func (b backend) SendMessageEvents(ctx context.Context, id, text, providerID, modelID, agent string) <-chan capabilities.StreamEvent {
	in := b.client.SendMessageEvents(ctx, id, text, providerID, modelID, agent)
	return streambridge.Forward(ctx, in, toStreamEvent)
}

func (b backend) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	return b.client.ReplyQuestion(ctx, requestID, answers)
}

func startup() func(context.Context, capabilities.Backend) (*capabilities.StartupResult, error) {
	return func(startCtx context.Context, b capabilities.Backend) (*capabilities.StartupResult, error) {
		if err := b.Health(startCtx); err != nil {
			debuglog.Printf("startup: claude health check failed: %v", err)
			return nil, err
		}
		debuglog.Printf("startup: claude ready (session deferred)")
		return &capabilities.StartupResult{}, nil
	}
}

func toSession(session provider.Session) capabilities.Session {
	return capabilities.Session{
		ID:        session.ID,
		Title:     session.Title,
		Directory: session.Directory,
		Model:     toModelRefPtr(session.Model),
		Time: capabilities.SessionTime{
			Created: session.Time.Created,
			Updated: session.Time.Updated,
		},
	}
}

func toModelRefPtr(model *provider.ModelRef) *capabilities.ModelRef {
	if model == nil {
		return nil
	}
	return &capabilities.ModelRef{
		ID:         model.ID,
		ProviderID: model.ProviderID,
		Variant:    model.Variant,
	}
}

func toMessageResponse(message provider.MessageResponse) capabilities.MessageResponse {
	out := capabilities.MessageResponse{
		Info: capabilities.MessageInfo{
			ID:   message.Info.ID,
			Role: message.Info.Role,
		},
		Parts: make([]capabilities.Part, 0, len(message.Parts)),
	}
	for _, part := range message.Parts {
		out.Parts = append(out.Parts, capabilities.Part{
			Type: part.Type,
			Text: part.Text,
			Tool: toToolCall(part.Tool),
		})
	}
	return out
}

func toProvider(provider provider.Provider) capabilities.Provider {
	models := make(map[string]capabilities.Model, len(provider.Models))
	for id, model := range provider.Models {
		models[id] = capabilities.Model{
			ID:         model.ID,
			ProviderID: model.ProviderID,
			Name:       model.Name,
			Status:     model.Status,
		}
	}
	return capabilities.Provider{
		ID:     provider.ID,
		Name:   provider.Name,
		Models: models,
	}
}

func toStreamEvent(event provider.StreamEvent) capabilities.StreamEvent {
	return capabilities.StreamEvent{
		Source: streamSource,
		Text:   event.Text,
		Tool:   toToolCall(event.Tool),
	}
}

func toToolCall(tool *provider.ToolEvent) *capabilities.ToolCall {
	if tool == nil {
		return nil
	}
	return &capabilities.ToolCall{
		ID:     tool.ID,
		Name:   tool.Name,
		Input:  tool.Input,
		Output: tool.Output,
		Error:  tool.Error,
	}
}
