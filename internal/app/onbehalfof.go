package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/infrashift/chit/internal/model"
)

// On behalf of a person.
//
// An OAuth2 client in CHIT_ON_BEHALF_OF_CLIENTS may act as a person by naming
// them in the X-On-Behalf-Of header. HALO's halod uses it to post a Subject's
// message as that Subject. The request is then authorized as the person (the
// principal), with every membership check the person would face, and the
// client (the actor) is recorded beside them: in props.on_behalf_of on the
// post, and in the audit log.
//
// Only a machine actor authenticated by Hydra client_credentials can use it.
// A person's Kratos session never can, so a browser cannot post as somebody
// else.

// OnBehalfOfAllowed reports whether the machine actor may act on behalf of a
// person.
func (a *App) OnBehalfOfAllowed(actor *model.User) bool {
	return actor != nil && actor.OAuthClientID != "" &&
		slices.Contains(a.Config.OnBehalfOfClients, actor.OAuthClientID)
}

// KratosPrincipalPrefix marks an X-On-Behalf-Of value that names a person by
// their Kratos identity ID rather than their Chit user ID.
const KratosPrincipalPrefix = "kratos:"

// ResolvePrincipal returns the person that actor names in X-On-Behalf-Of:
// either their Chit user ID, or "kratos:" and their Kratos identity ID.
//
// It refuses (403) an actor that is not an allowlisted machine actor, and
// (400) a principal that is empty, not a user ID, unknown, deactivated, or not
// a person: an agent or bot acts for itself, never through another client.
//
// A Kratos identity is resolved as a browser request resolves it, by
// provisionUser: a person who has never opened Chit gets the user their first
// visit would have made. The value must be a canonical (lowercase,
// hyphenated) UUID; Kratos must know the identity, and it must be active.
// Creating the user writes a user_provisioned audit record naming the actor.
func (a *App) ResolvePrincipal(ctx context.Context, actor *model.User, principalID string) (*model.User, error) {
	const where = "App.ResolvePrincipal"
	if !a.OnBehalfOfAllowed(actor) {
		return nil, model.NewForbiddenError(where, "this client may not act on behalf of a user")
	}
	if principalID == "" {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names no user")
	}
	var principal *model.User
	var err error
	if kratosID, ok := strings.CutPrefix(principalID, KratosPrincipalPrefix); ok {
		principal, err = a.provisionPrincipal(ctx, actor, kratosID)
	} else {
		principal, err = a.lookupPrincipal(ctx, principalID)
	}
	if err != nil {
		return nil, err
	}
	if principal.DeleteAt != 0 {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names a deactivated user")
	}
	if principal.ActorType != model.ActorTypeUser {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of must name a person, not an agent or bot")
	}
	return principal, nil
}

// lookupPrincipal reads the existing user principalID names for
// ResolvePrincipal.
func (a *App) lookupPrincipal(ctx context.Context, principalID string) (*model.User, error) {
	const where = "App.ResolvePrincipal"
	if !model.IsValidID(principalID) {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of is not a user ID")
	}
	p, err := a.Store.User().Get(ctx, principalID)
	if err != nil {
		if isNotFound(err) {
			return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names an unknown user")
		}
		return nil, err
	}
	return p, nil
}

// provisionPrincipal resolves the Kratos identity kratosID for
// ResolvePrincipal, creating the user when needed.
func (a *App) provisionPrincipal(ctx context.Context, actor *model.User, kratosID string) (*model.User, error) {
	const where = "App.ResolvePrincipal"
	// Canonical only: the ID goes into the Kratos admin URL and is the user
	// cache key, so one identity must have exactly one spelling.
	if parsed, err := uuid.Parse(kratosID); err != nil || parsed.String() != kratosID {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of kratos: is not a canonical Kratos identity ID")
	}
	principal, created, err := a.provisionUser(ctx, kratosID)
	switch {
	case errors.Is(err, errUserDeactivated), errors.Is(err, ErrKratosIdentityInactive):
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names a deactivated user")
	case errors.Is(err, ErrKratosIdentityNotFound):
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names an unknown Kratos identity")
	case err != nil:
		return nil, err
	}
	if created {
		a.AuditOnBehalfOf(actor, "user_provisioned", principal.ID, "kratos_id", kratosID)
	}
	return principal, nil
}

// AuditOnBehalfOf writes an on_behalf_of audit record for an act that actor
// did on behalf of principalID. A request made as oneself (actor nil) writes
// nothing.
func (a *App) AuditOnBehalfOf(actor *model.User, action, principalID string, attrs ...any) {
	if actor == nil || a.AuditLogger == nil {
		return
	}
	a.AuditLogger.LogOnBehalfOf(model.NewID(), action, actor.ID, actor.OAuthClientID, principalID, attrs...)
}

// CheckOnBehalfOfClients fails unless every allowlisted client is a declared
// machine actor. A typo in the allowlist would otherwise authorize nothing
// and say nothing; a client that is not a machine actor cannot authenticate.
func CheckOnBehalfOfClients(allowlist []string, actors []MachineActor) error {
	for _, clientID := range allowlist {
		if !slices.ContainsFunc(actors, func(m MachineActor) bool { return m.ClientID == clientID }) {
			return fmt.Errorf("CHIT_ON_BEHALF_OF_CLIENTS names %q, which is not in CHIT_MACHINE_ACTORS", clientID)
		}
	}
	return nil
}
