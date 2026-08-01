package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/model"
)

// errCommandUsage is returned when no username was supplied. Command errors
// are echoed to the invoker verbatim, so they read as instructions.
var errCommandUsage = errors.New("Usage: specify a username, for example `bob`")

// HandleInvite implements /invite <user>, adding a user to the current
// channel. Authorization is left to AddChannelMember, which already enforces
// the team-vs-channel membership rules for each channel type.
func (a *App) HandleInvite(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	target, err := a.resolveCommandUser(ctx, args)
	if err != nil {
		return &command.CommandResult{ResponseText: err.Error()}, nil
	}

	if _, err := a.AddChannelMember(ctx, channelID, target.ID, actorID); err != nil {
		return &command.CommandResult{
			ResponseText: fmt.Sprintf("Could not add `%s`: %s", target.Username, err.Error()),
		}, nil
	}

	return &command.CommandResult{
		ResponseText: fmt.Sprintf("Added `%s` to the channel.", target.Username),
		ChannelID:    channelID,
		Triggered:    true,
	}, nil
}

// HandleKick implements /kick <user>, removing a user from the current
// channel.
func (a *App) HandleKick(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
	target, err := a.resolveCommandUser(ctx, args)
	if err != nil {
		return &command.CommandResult{ResponseText: err.Error()}, nil
	}

	if err := a.RemoveChannelMember(ctx, channelID, target.ID, actorID); err != nil {
		return &command.CommandResult{
			ResponseText: fmt.Sprintf("Could not remove `%s`: %s", target.Username, err.Error()),
		}, nil
	}

	return &command.CommandResult{
		ResponseText: fmt.Sprintf("Removed `%s` from the channel.", target.Username),
		ChannelID:    channelID,
		Triggered:    true,
	}, nil
}

// resolveCommandUser turns a command argument into a user. The returned error
// is written for the person who typed the command, since it is echoed back to
// them verbatim as the command response.
func (a *App) resolveCommandUser(ctx context.Context, args string) (*model.User, error) {
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

	user, err := a.Store.User().GetByUsername(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("No user named `%s`.", name)
	}
	return user, nil
}
