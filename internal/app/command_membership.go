package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/model"
)

// errCommandUsage is returned when no username was supplied. Command errors
// are echoed to the invoker verbatim, so they read as instructions.
//
//nolint:staticcheck // ST1005: shown to the user as-is, so it is prose, not a wrapped error.
var errCommandUsage = errors.New("Usage: specify a username, for example `bob`") // semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity

// HandleInvite implements /invite <user>, adding a user to the current
// channel. Authorization is left to AddChannelMember, which already enforces
// the team-vs-channel membership rules for each channel type, and asks the
// membership hook when it is on.
func (a *App) HandleInvite(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	assert.That(actorID != "", "App.HandleInvite: empty actor id")
	assert.That(channelID != "", "App.HandleInvite: empty channel id")
	target, err := a.resolveCommandUser(ctx, args)
	if err != nil {
		return &command.CommandResult{ResponseText: err.Error()}, nil
	}

	assert.That(target != nil, "App.HandleInvite: nil target without an error")
	if _, err := a.AddChannelMember(ctx, channelID, target.ID, actorID); err != nil {
		// A membership refusal reads as the policy's own words.
		if refusal, ok := IsMembershipRefusal(err); ok {
			text := fmt.Sprintf("Could not add %#q: the membership policy refused (%s).", target.Username, refusal.Reason)
			if refusal.Detail != "" {
				text += " " + refusal.Detail
			}
			return &command.CommandResult{ResponseText: text}, nil
		}
		return &command.CommandResult{
			ResponseText: fmt.Sprintf("Could not add %#q: %s", target.Username, err.Error()),
		}, nil
	}

	return &command.CommandResult{
		ResponseText: fmt.Sprintf("Added %#q to the channel.", target.Username),
		ChannelID:    channelID,
		Triggered:    true,
	}, nil
}

// HandleKick implements /kick <user>, removing a user from the current
// channel.
func (a *App) HandleKick(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	assert.That(actorID != "", "App.HandleKick: empty actor id")
	assert.That(channelID != "", "App.HandleKick: empty channel id")
	target, err := a.resolveCommandUser(ctx, args)
	if err != nil {
		return &command.CommandResult{ResponseText: err.Error()}, nil
	}

	assert.That(target != nil, "App.HandleKick: nil target without an error")
	if err := a.RemoveChannelMember(ctx, channelID, target.ID, actorID); err != nil {
		return &command.CommandResult{
			ResponseText: fmt.Sprintf("Could not remove %#q: %s", target.Username, err.Error()),
		}, nil
	}

	return &command.CommandResult{
		ResponseText: fmt.Sprintf("Removed %#q from the channel.", target.Username),
		ChannelID:    channelID,
		Triggered:    true,
	}, nil
}

// resolveCommandUser turns a command argument into a user. The returned error
// is written for the person who typed the command, since it is echoed back to
// them verbatim as the command response.
func (a *App) resolveCommandUser(ctx context.Context, args string) (*model.User, error) {
	assert.That(a.Store != nil, "App.resolveCommandUser: nil store")
	// Only the first word is used, so a trailing comment or a second name is
	// ignored rather than producing a confusing "user not found".
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return nil, errCommandUsage
	}

	name := strings.TrimPrefix(fields[0], "@")
	if name == "" {
		return nil, errCommandUsage
	}
	assert.That(!strings.ContainsAny(name, " \t\n"), "App.resolveCommandUser: name holds white space")

	user, err := a.Store.User().GetByUsername(ctx, name)
	if err != nil {
		//nolint:staticcheck // ST1005: echoed to the invoker as a sentence.
		return nil, fmt.Errorf("No user named `%s`.", name)
	}
	assert.That(user != nil, "App.resolveCommandUser: nil user without an error")
	return user, nil
}

// HandleTopic implements /topic <text>, setting the channel header. With no
// argument it reports the current one, which is the only way to read it from
// a client that does not render headers.
func (a *App) HandleTopic(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	assert.That(actorID != "", "App.HandleTopic: empty actor id")
	assert.That(a.Store != nil, "App.HandleTopic: nil store")
	channel, err := a.Store.Channel().Get(ctx, channelID)
	if err != nil || channel == nil {
		return &command.CommandResult{ResponseText: "Could not read this channel."}, nil
	}

	topic := strings.TrimSpace(args)
	if topic == "" {
		if channel.Header == "" {
			return &command.CommandResult{
				ResponseText: "No topic set. Use `/topic <text>` to set one.",
			}, nil
		}
		return &command.CommandResult{
			ResponseText: fmt.Sprintf("Current topic: %s", channel.Header),
		}, nil
	}

	assert.That(topic == strings.TrimSpace(topic), "App.HandleTopic: topic is not trimmed")
	if _, err := a.UpdateChannel(ctx, channelID, ChannelPatch{Header: &topic}, actorID); err != nil {
		return &command.CommandResult{
			ResponseText: fmt.Sprintf("Could not set the topic: %s", err.Error()),
		}, nil
	}

	return &command.CommandResult{
		ResponseText: fmt.Sprintf("Topic set to: %s", topic),
		ChannelID:    channelID,
		Triggered:    true,
	}, nil
}
