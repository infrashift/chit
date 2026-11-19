package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

// Channel membership in channel_members is the authoritative access check.
// Keto tuples are dual-written best-effort and are only authoritative for
// slash-command permissions.

// requireChannelMember returns a 403 error unless the user is a member of the channel.
func (a *App) requireChannelMember(ctx context.Context, channelID, userID string) error {
	assert.That(a.Store != nil, "App.requireChannelMember: nil store")
	assert.That(userID != "", "App.requireChannelMember: empty user id")
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
	assert.That(a.Store != nil, "App.isChannelMember: nil store")
	assert.That(userID != "", "App.isChannelMember: empty user id")
	_, err := a.Store.Channel().GetMember(ctx, channelID, userID)
	return err == nil
}

// requireTeamMember returns a 403 error unless the user is a member of the team.
func (a *App) requireTeamMember(ctx context.Context, teamID, userID string) error {
	assert.That(a.Store != nil, "App.requireTeamMember: nil store")
	assert.That(userID != "", "App.requireTeamMember: empty user id")
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

// requireTeamAdmin returns a 403 error unless actor is a team_admin of the
// team or a system_admin. Team admin is the role CreateTeam grants the
// creator; until this check existed nothing read it, and anyone could rename or
// delete any team.
func (a *App) requireTeamAdmin(ctx context.Context, teamID string, actor *model.User, op string) error {
	assert.That(actor != nil, "App.requireTeamAdmin: nil actor")
	assert.That(op != "", "App.requireTeamAdmin: empty op")
	assert.That(a.Store != nil, "App.requireTeamAdmin: nil store")
	if actor.IsSystemAdmin() {
		return nil
	}
	member, err := a.Store.Team().GetMember(ctx, teamID, actor.ID)
	if err != nil && !isNotFound(err) {
		return err
	}
	assert.That(err != nil || member != nil, "App.requireTeamAdmin: nil member without an error")
	if err == nil && member.IsTeamAdmin() {
		return nil
	}
	return model.NewForbiddenError(op, "requires team_admin")
}

// requireTeamVisible returns a 403 error unless actor may see the team: open
// teams are visible to everyone, invite-only teams to their members and to
// system admins.
func (a *App) requireTeamVisible(ctx context.Context, team *model.Team, actor *model.User) error {
	assert.That(team != nil, "App.requireTeamVisible: nil team")
	assert.That(actor != nil, "App.requireTeamVisible: nil actor")
	if team.Type == model.TeamOpen || actor.IsSystemAdmin() {
		return nil
	}
	return a.requireTeamMember(ctx, team.ID, actor.ID)
}

// isNotFound reports whether err is an AppError carrying 404.
func isNotFound(err error) bool {
	var appErr *model.AppError
	return errors.As(err, &appErr) && appErr.StatusCode == http.StatusNotFound
}

// requireSystemAdmin returns a 403 error unless the actor holds system_admin.
//
// A failed user lookup is a DENIAL, not an error, matching requirePostOwner
// above: an actor who cannot be resolved is not an admin, and surfacing that as
// a 500 would turn a permission decision into something an operator reads as an
// outage.
func (a *App) requireSystemAdmin(ctx context.Context, actorID, op string) error {
	assert.That(op != "", "App.requireSystemAdmin: empty op")
	assert.That(a.Store != nil, "App.requireSystemAdmin: nil store")
	user, err := a.Store.User().Get(ctx, actorID)
	if err != nil || !user.IsSystemAdmin() {
		return model.NewForbiddenError(op, "requires system_admin")
	}
	return nil
}

// requirePostOwner returns a 403 error unless userID authored the post or is a
// system admin.
func (a *App) requirePostOwner(ctx context.Context, post *model.Post, userID string) error {
	assert.That(post != nil, "App.requirePostOwner: nil post")
	assert.That(userID != "", "App.requirePostOwner: empty user id")
	assert.That(a.Store != nil, "App.requirePostOwner: nil store")
	if post.UserID == userID {
		return nil
	}
	if user, err := a.Store.User().Get(ctx, userID); err == nil && user.IsSystemAdmin() {
		return nil
	}
	return model.NewForbiddenError("App.requirePostOwner", "not the author of this post")
}
