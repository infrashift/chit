package command

import "context"

// CommandResult holds the output of a slash command execution.
type CommandResult struct {
	ResponseText string
	ChannelID    string
	Triggered    bool // true if the command should dispatch a webhook event
}

// Handler executes a slash command.
type Handler interface {
	Execute(ctx context.Context, actorID, channelID, args string) (*CommandResult, error)
}

// HandlerFunc is an adapter to allow ordinary functions to serve as Handlers.
type HandlerFunc func(ctx context.Context, actorID, channelID, args string) (*CommandResult, error)

// Execute calls f(ctx, actorID, channelID, args).
func (f HandlerFunc) Execute(ctx context.Context, actorID, channelID, args string) (*CommandResult, error) {
	return f(ctx, actorID, channelID, args)
}
