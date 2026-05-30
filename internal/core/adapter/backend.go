package adapter

import "context"

// Backend is the abstract interface for any AI backend provider.
type Backend interface {
	Health(ctx context.Context) error
	CreateSession(ctx context.Context, directory string) (*Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	ListMessages(ctx context.Context, sessionID string) ([]MessageResponse, error)
	ListProviders(ctx context.Context) ([]Provider, []string, error)
	SendMessageEvents(ctx context.Context, sessionID, text, providerID, modelID, agent string) <-chan StreamEvent
	ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error
	BaseURL() string
}
