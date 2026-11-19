package server

import (
	"context"
	"fmt"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/command"
)

// helpCommand is /help: it lists every registered command.
type helpCommand struct {
	reg *command.Registry
}

// Execute lists the registry's commands.
func (h helpCommand) Execute(_ context.Context, _, _, _ string) (*command.CommandResult, error) {
	var text string
	for _, c := range h.reg.All() {
		text += fmt.Sprintf("- `/%s` — %s\n", c.Slug, c.Description)
	}
	if text == "" {
		text = "No commands available."
	}
	return &command.CommandResult{ResponseText: text}, nil
}

// inviteCommand is /invite.
type inviteCommand struct {
	app *app.App
}

// Execute runs App.HandleInvite.
func (h inviteCommand) Execute(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	return h.app.HandleInvite(ctx, actorID, channelID, args)
}

// kickCommand is /kick.
type kickCommand struct {
	app *app.App
}

// Execute runs App.HandleKick.
func (h kickCommand) Execute(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	return h.app.HandleKick(ctx, actorID, channelID, args)
}

// topicCommand is /topic.
type topicCommand struct {
	app *app.App
}

// Execute runs App.HandleTopic.
func (h topicCommand) Execute(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	return h.app.HandleTopic(ctx, actorID, channelID, args)
}
