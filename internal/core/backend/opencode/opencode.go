package opencode

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/Ceinl/plums/capabilities"
	provider "github.com/Ceinl/plums/internal/core/provider/opencode"
	"github.com/Ceinl/plums/internal/debuglog"
)

const streamSource = "opencode.sdk.event"
const DefaultBaseURL = provider.DefaultBaseURL

type Options struct {
	WorkingDirectory string
	ServerURL        string
	HealthTimeout    time.Duration
}

func Registration(options Options) capabilities.BackendRegistration {
	client := provider.NewClientWithURL(serverURL(options.ServerURL))
	return capabilities.BackendRegistration{
		Name:    "opencode",
		Label:   "Opencode",
		Backend: backend{client: client},
		Startup: startup(options.WorkingDirectory, options.HealthTimeout, client),
	}
}

func DefaultPortForDir(dir string) int {
	if dir == "" {
		return 4096
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(dir))
	return 49152 + int(h.Sum32()%16384)
}

func DefaultBaseURLForDir(dir string) string {
	return fmt.Sprintf("http://127.0.0.1:%d", DefaultPortForDir(dir))
}

func serverURL(value string) string {
	if value == "" {
		return DefaultBaseURL
	}
	return value
}

type backend struct {
	client *provider.Client
}

func (b backend) Health(ctx context.Context) error {
	return b.client.Health(ctx)
}

func (b backend) CreateSession(ctx context.Context, dir string) (*capabilities.Session, error) {
	session, err := b.client.CreateSession(ctx, dir)
	return toSessionPtr(session), err
}

func (b backend) ListSessions(ctx context.Context) ([]capabilities.Session, error) {
	sessions, err := b.client.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]capabilities.Session, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, toSession(session))
	}
	return out, nil
}

func (b backend) GetSession(ctx context.Context, id string) (*capabilities.Session, error) {
	session, err := b.client.GetSession(ctx, id)
	return toSessionPtr(session), err
}

func (b backend) ListMessages(ctx context.Context, id string) ([]capabilities.MessageResponse, error) {
	messages, err := b.client.ListMessages(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]capabilities.MessageResponse, 0, len(messages))
	for _, message := range messages {
		out = append(out, toMessageResponse(message))
	}
	return out, nil
}

func (b backend) ListProviders(ctx context.Context) ([]capabilities.Provider, []string, error) {
	providers, connected, err := b.client.ListProviders(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]capabilities.Provider, 0, len(providers))
	for _, provider := range providers {
		out = append(out, toProvider(provider))
	}
	return out, append([]string(nil), connected...), nil
}

func (b backend) SendMessageEvents(ctx context.Context, id, text, providerID, modelID, agent string) <-chan capabilities.StreamEvent {
	in := b.client.SendMessageEvents(ctx, id, text, providerID, modelID, agent)
	out := make(chan capabilities.StreamEvent)
	go func() {
		defer close(out)
		for event := range in {
			out <- toStreamEvent(event)
		}
	}()
	return out
}

func (b backend) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	return b.client.ReplyQuestion(ctx, requestID, answers)
}

func (b backend) AbortSession(ctx context.Context, sessionID string) error {
	return b.client.AbortSession(ctx, sessionID)
}

func startup(wd string, healthTimeout time.Duration, client *provider.Client) func(context.Context, capabilities.Backend) (*capabilities.StartupResult, error) {
	return func(startCtx context.Context, b capabilities.Backend) (*capabilities.StartupResult, error) {
		var serverProc *provider.ServerProcess
		debuglog.Printf("startup: checking opencode health")
		if err := b.Health(startCtx); err != nil {
			debuglog.Printf("startup: health check failed: %v", err)
			proc, err := provider.StartServer(startCtx, client.BaseURL(), wd)
			if err != nil {
				debuglog.Printf("startup: start server failed: %v", err)
				return nil, fmt.Errorf("failed to start opencode server: %w", err)
			}
			serverProc = proc
			debuglog.Printf("startup: started opencode server process")
			if err := provider.WaitForHealthOrExit(startCtx, client, serverProc, healthTimeout); err != nil {
				debuglog.Printf("startup: wait for health failed: %v", err)
				serverProc.Stop()
				return nil, fmt.Errorf("failed to start opencode server: %w", err)
			}
		} else {
			debuglog.Printf("startup: existing opencode server is healthy")
		}

		debuglog.Printf("startup: opencode ready (session deferred)")
		result := &capabilities.StartupResult{}
		if serverProc != nil {
			result.Server = serverProc
		}
		return result, nil
	}
}

func toSessionPtr(session *provider.Session) *capabilities.Session {
	if session == nil {
		return nil
	}
	out := toSession(*session)
	return &out
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
		out.Parts = append(out.Parts, toPart(part))
	}
	return out
}

func toPart(part provider.Part) capabilities.Part {
	return capabilities.Part{
		Type: part.Type,
		Text: part.Text,
		Tool: toToolCall(part.Tool),
	}
}

func toProvider(provider provider.Provider) capabilities.Provider {
	models := make(map[string]capabilities.Model, len(provider.Models))
	for id, model := range provider.Models {
		models[id] = toModel(model)
	}
	return capabilities.Provider{
		ID:     provider.ID,
		Name:   provider.Name,
		Models: models,
	}
}

func toModel(model provider.Model) capabilities.Model {
	return capabilities.Model{
		ID:         model.ID,
		ProviderID: model.ProviderID,
		Name:       model.Name,
		Status:     model.Status,
	}
}

func toStreamEvent(event provider.StreamEvent) capabilities.StreamEvent {
	return capabilities.StreamEvent{
		Source:   streamSource,
		Text:     event.Text,
		Tool:     toToolCall(event.Tool),
		Question: toQuestionRequest(event.Question),
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

func toQuestionRequest(request *provider.QuestionRequest) *capabilities.QuestionRequest {
	if request == nil {
		return nil
	}
	out := &capabilities.QuestionRequest{
		ID:        request.ID,
		SessionID: request.SessionID,
		Questions: make([]capabilities.QuestionInfo, 0, len(request.Questions)),
	}
	for _, question := range request.Questions {
		out.Questions = append(out.Questions, toQuestionInfo(question))
	}
	return out
}

func toQuestionInfo(question provider.QuestionInfo) capabilities.QuestionInfo {
	options := make([]capabilities.QuestionOption, 0, len(question.Options))
	for _, option := range question.Options {
		options = append(options, capabilities.QuestionOption{
			Label:       option.Label,
			Description: option.Description,
		})
	}
	return capabilities.QuestionInfo{
		Question: question.Question,
		Header:   question.Header,
		Options:  options,
		Multiple: question.Multiple,
		Custom:   question.Custom,
	}
}
