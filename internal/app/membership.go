package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/memberhook"
	"github.com/infrashift/chit/internal/model"
)

// The membership policy hook (P-CHIT-2).
//
// When CHIT_MEMBERSHIP_HOOK_URL is set, Chit asks the hook (HALO's halod)
// before it adds anybody to a channel, on every path that adds one: POST
// /channels/{id}/members (a join and an on-behalf-of add included), the team
// members of a new open channel, the other members of a new direct or group
// channel, and /invite. The creator of a new channel is the one member added
// without a question. Any answer other than {"allow":true} refuses the add,
// with 403 channel.membership_refused, and nothing is added.

// MembershipChecker asks the policy whether a member may be added.
// *memberhook.Hook is the implementation.
type MembershipChecker interface {
	Check(ctx context.Context, req *memberhook.Request) (memberhook.Decision, error)
}

// ChannelErrMembershipRefused is the error ID of an add the hook refused.
const ChannelErrMembershipRefused = "channel.membership_refused"

// ReasonHookUnavailable is the reason of a refusal because the hook could
// not be asked or gave no valid answer: Chit fails closed.
const ReasonHookUnavailable = "hook_unavailable"

// membershipCheckWorkers bounds the hook requests in flight for one batch.
const membershipCheckWorkers = 8

func membershipRefused(userID, reason, detail string) *model.AppError {
	return &model.AppError{
		ID:         ChannelErrMembershipRefused,
		Where:      "App.checkMembership",
		Message:    fmt.Sprintf("the membership policy refused to add user %s to this channel", userID),
		StatusCode: http.StatusForbidden,
		Reason:     reason,
		Detail:     detail,
	}
}

type viaActorKey struct{}

// WithViaActor records in ctx the on-behalf-of client that acts for the
// request's user, so that the membership hook can name it.
func WithViaActor(ctx context.Context, actorID string) context.Context {
	assert.That(ctx != nil, "WithViaActor: nil context")
	assert.That(actorID != "", "WithViaActor: empty actor id")
	return context.WithValue(ctx, viaActorKey{}, actorID)
}

func viaActor(ctx context.Context) string {
	assert.That(ctx != nil, "viaActor: nil context")
	id, ok := ctx.Value(viaActorKey{}).(string)
	if !ok {
		return ""
	}
	return id
}

// hookActorType maps Chit's actor type to the hook's.
func hookActorType(t string) string {
	switch t {
	case model.ActorTypeAgent:
		return memberhook.ActorAgent
	case model.ActorTypeBot:
		return memberhook.ActorBot
	default:
		return memberhook.ActorHuman
	}
}

// checkMembership asks the hook whether addedBy may add member to channelID.
// It returns nil when the hook is off or allows it, and a 403
// channel.membership_refused otherwise. Each answer is audited.
func (a *App) checkMembership(ctx context.Context, channelID string, member *model.User, addedBy string) error {
	if a.MembershipHook == nil {
		return nil
	}
	assert.That(member != nil, "App.checkMembership: nil member")
	assert.That(channelID != "" && addedBy != "", "App.checkMembership: empty channel id or adder")
	req := &memberhook.Request{
		ChannelID:        channelID,
		UserID:           member.ID,
		KratosIdentityID: member.KratosID,
		ActorType:        hookActorType(member.ActorType),
		AddedBy:          addedBy,
		ViaActorID:       viaActor(ctx),
	}
	decision, err := a.MembershipHook.Check(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			// Abandoned, not answered: the request went away, or another
			// member of the same batch was refused first. Nothing to audit.
			return membershipRefused(member.ID, ReasonHookUnavailable, "the membership check did not finish")
		}
		slog.Warn("membership hook failed; refusing the add",
			"channel_id", channelID, "user_id", member.ID, "error", err)
		decision = memberhook.Decision{Reason: ReasonHookUnavailable, Detail: "the membership check could not be completed"}
	}
	a.auditMembership(req, decision)
	if !decision.Allow {
		return membershipRefused(member.ID, decision.Reason, decision.Detail)
	}
	return nil
}

func (a *App) auditMembership(req *memberhook.Request, d memberhook.Decision) {
	assert.That(req != nil, "App.auditMembership: nil request")
	if a.AuditLogger == nil {
		return
	}
	a.AuditLogger.LogMembershipCheck(model.NewID(), req.ChannelID, req.UserID, req.ActorType,
		req.AddedBy, req.ViaActorID, d.Allow, d.Reason)
}

// checkMemberIDs asks the hook about each of userIDs, and fails when any one
// is refused: a batch is added whole or not at all. The checks run
// concurrently, at most membershipCheckWorkers at a time, and the first
// refusal stops the rest. A user ID that names no live user is skipped; the
// caller adds only the IDs it checked.
func (a *App) checkMemberIDs(ctx context.Context, channelID string, userIDs []string, addedBy string) ([]string, error) {
	if a.MembershipHook == nil || len(userIDs) == 0 {
		return userIDs, nil
	}
	assert.That(a.Store != nil, "App.checkMemberIDs: nil store")
	assert.That(channelID != "" && addedBy != "", "App.checkMemberIDs: empty channel id or adder")
	users, err := a.Store.User().GetByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan *model.User)
	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
	)
	for range min(membershipCheckWorkers, len(users)) {
		wg.Go(func() {
			for u := range jobs {
				if err := a.checkMembership(ctx, channelID, u, addedBy); err != nil {
					once.Do(func() {
						first = err
						cancel()
					})
				}
			}
		})
	}
feed:
	for _, u := range users {
		select {
		case jobs <- u:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return nil, first
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	checked := make([]string, 0, len(users))
	for _, u := range users {
		checked = append(checked, u.ID)
	}
	assert.That(len(checked) == len(users), "App.checkMemberIDs: lost a checked id")
	return checked, nil
}

// otherMembers returns ids without actorID: the members of a new channel
// that its creator adds.
func otherMembers(ids []string, actorID string) []string {
	out := slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return id == actorID })
	assert.That(len(out) <= len(ids) && !slices.Contains(out, actorID), "otherMembers: the actor is still a member")
	return out
}

// IsMembershipRefusal reports whether err is a refusal of the membership
// hook.
func IsMembershipRefusal(err error) (*model.AppError, bool) {
	var appErr *model.AppError
	if errors.As(err, &appErr) && appErr.ID == ChannelErrMembershipRefused {
		return appErr, true
	}
	return nil, false
}
