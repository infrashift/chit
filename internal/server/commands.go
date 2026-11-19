package server

import (
	"context"
	"fmt"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/command"
)

// helpCommand is /help: it lists every registered command.
type helpCommand struct {
	reg *command.Registry
}

// Execute lists the registry's commands.
func (h helpCommand) Execute(_ context.Context, _, _, _ string) (*command.CommandResult, error) {
	assert.That(h.reg != nil, "helpCommand.Execute: nil registry")
	var text string
	for _, c := range h.reg.All() {
		text += fmt.Sprintf("- `/%s` — %s\n", c.Slug, c.Description)
	}
	if text == "" {
		text = "No commands available."
	}
	assert.That(text != "", "helpCommand.Execute: empty response")
	return &command.CommandResult{ResponseText: text}, nil
}

// inviteCommand is /invite.
type inviteCommand struct {
	app *app.App
}

// Execute runs App.HandleInvite.
func (h inviteCommand) Execute(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	assert.That(h.app != nil, "inviteCommand.Execute: nil app")
	assert.That(actorID != "", "inviteCommand.Execute: empty actor id")
	return h.app.HandleInvite(ctx, actorID, channelID, args)
}

// kickCommand is /kick.
type kickCommand struct {
	app *app.App
}

// Execute runs App.HandleKick.
func (h kickCommand) Execute(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	assert.That(h.app != nil, "kickCommand.Execute: nil app")
	assert.That(actorID != "", "kickCommand.Execute: empty actor id")
	return h.app.HandleKick(ctx, actorID, channelID, args)
}

// topicCommand is /topic.
type topicCommand struct {
	app *app.App
}

// Execute runs App.HandleTopic.
func (h topicCommand) Execute(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	assert.That(h.app != nil, "topicCommand.Execute: nil app")
	assert.That(actorID != "", "topicCommand.Execute: empty actor id")
	return h.app.HandleTopic(ctx, actorID, channelID, args)
}
