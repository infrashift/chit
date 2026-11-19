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
