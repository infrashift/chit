package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/infrashift/chit/internal/model"
)

// Channel membership in channel_members is the authoritative access check.
// Keto tuples are dual-written best-effort and are only authoritative for
// slash-command permissions.

// requireChannelMember returns a 403 error unless the user is a member of the channel.
func (a *App) requireChannelMember(ctx context.Context, channelID, userID string) error {
	_, err := a.Store.Channel().GetMember(ctx, channelID, userID)
	if err == nil {
		return nil
	}
	var appErr *model.AppError
	if errors.As(err, &appErr) && appErr.StatusCode == http.StatusNotFound {
		return model.NewForbiddenError("App.requireChannelMember", "not a member of this channel")
	}
	return err
}

// isChannelMember reports membership, treating lookup failures as non-membership.
func (a *App) isChannelMember(ctx context.Context, channelID, userID string) bool {
	_, err := a.Store.Channel().GetMember(ctx, channelID, userID)
	return err == nil
}

// requireTeamMember returns a 403 error unless the user is a member of the team.
func (a *App) requireTeamMember(ctx context.Context, teamID, userID string) error {
	_, err := a.Store.Team().GetMember(ctx, teamID, userID)
	if err == nil {
		return nil
	}
	var appErr *model.AppError
	if errors.As(err, &appErr) && appErr.StatusCode == http.StatusNotFound {
		return model.NewForbiddenError("App.requireTeamMember", "not a member of this team")
	}
	return err
}

// requirePostOwner returns a 403 error unless userID authored the post or is a
// system admin.
func (a *App) requirePostOwner(ctx context.Context, post *model.Post, userID string) error {
	if post.UserID == userID {
		return nil
	}
	if user, err := a.Store.User().Get(ctx, userID); err == nil && user.IsSystemAdmin() {
		return nil
	}
	return model.NewForbiddenError("App.requirePostOwner", "not the author of this post")
}
