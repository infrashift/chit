package app

import (
	"context"
	"fmt"
	"slices"

	"github.com/infrashift/chit/internal/model"
)

// On-behalf-of posting.
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

// ResolvePrincipal returns the person that actor names in X-On-Behalf-Of.
//
// It refuses (403) an actor that is not an allowlisted machine actor, and
// (400) a principal that is empty, not a user ID, unknown, deactivated, or not
// a person: an agent or bot acts for itself, never through another client.
func (a *App) ResolvePrincipal(ctx context.Context, actor *model.User, principalID string) (*model.User, error) {
	const where = "App.ResolvePrincipal"
	if !a.OnBehalfOfAllowed(actor) {
		return nil, model.NewForbiddenError(where, "this client may not act on behalf of a user")
	}
	if principalID == "" {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names no user")
	}
	if !model.IsValidID(principalID) {
		return nil, model.NewBadRequestError(where, "X-On-Behalf-Of is not a user ID")
	}
	principal, err := a.Store.User().Get(ctx, principalID)
	if err != nil {
		if isNotFound(err) {
			return nil, model.NewBadRequestError(where, "X-On-Behalf-Of names an unknown user")
		}
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
