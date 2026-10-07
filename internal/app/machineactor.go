package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/infrashift/chit/internal/model"
)

// MachineActor is a declared non-human caller: a process that authenticates
// with an OAuth2 client_credentials grant and holds no Kratos identity.
//
// A HUMAN NEEDS NO DECLARATION and a machine does. ProvisionUser creates a user
// row from a Kratos identity on that person's first authenticated request,
// because the identity provider has already vouched for them. There is no
// equivalent for a machine: ResolveOAuthClient refuses a client id it does not
// recognise rather than inventing a user for it, so an OAuth2 client that is
// registered with Hydra and unknown here is authenticated and nobody. That is
// deliberate — auto-provisioning would mean any client the IdP will mint a
// token for becomes a chit user — and it is why this list exists.
type MachineActor struct {
	ClientID    string `json:"client_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	// Space-separated, matching users.roles. "system_admin" is a real grant:
	// it is what lets an actor create users, which the forge notifier must do
	// to address a person who has never signed in.
	Roles string `json:"roles"`
	// agent or bot, and required. A machine is not a "user": left empty,
	// PreSave would default it to one, so the declaration must say which.
	ActorType string `json:"actor_type"`
}

// normalize applies defaults as PreSave applies them to a row, so the
// declaration and the row compare like for like. Comparing the raw
// declaration made an omitted roles field read as "" against the row's
// defaulted "system_user", and the second boot stripped every role.
func (m *MachineActor) normalize() {
	if m.Roles == "" {
		m.Roles = "system_user"
	}
}

// ParseMachineActors reads the declared actors from their JSON encoding.
// An empty or absent value is not an error: a deployment with no machine
// actors is legitimate, and the humans still arrive through Kratos.
func ParseMachineActors(raw string) ([]MachineActor, error) {
	if raw == "" {
		return nil, nil
	}
	var actors []MachineActor
	if err := json.Unmarshal([]byte(raw), &actors); err != nil {
		return nil, fmt.Errorf("parse machine actors: %w", err)
	}
	for i := range actors {
		if err := actors[i].validate(); err != nil {
			return nil, fmt.Errorf("machine actor %d: %w", i, err)
		}
	}
	return actors, nil
}

// validate checks what the declaration itself must supply; the rest is left
// to the user model's validation.
func (m *MachineActor) validate() error {
	if m.ClientID == "" {
		return fmt.Errorf("no client_id")
	}
	if m.Username == "" {
		return fmt.Errorf("%q has no username", m.ClientID)
	}
	if m.ActorType != model.ActorTypeAgent && m.ActorType != model.ActorTypeBot {
		return fmt.Errorf("%q: actor_type must be %q or %q, got %q",
			m.ClientID, model.ActorTypeAgent, model.ActorTypeBot, m.ActorType)
	}
	return nil
}

// EnsureMachineActors makes the declared actors exist, and is safe to run on
// every boot.
//
// IT RECONCILES RATHER THAN CREATES. An actor whose roles change in the
// declaration has them changed on the row, because the declaration is the
// source of truth and a role that can only ever be ADDED is a revocation that
// silently does nothing. It does not delete: removing an actor from the list
// leaves the row, since deleting a user cascades into messages and membership
// and is not a thing to do as a side effect of an edit to a config value.
func (a *App) EnsureMachineActors(ctx context.Context, actors []MachineActor) error {
	for i := range actors {
		actor := actors[i] // a copy: normalizing must not edit the caller's slice
		if err := actor.validate(); err != nil {
			return fmt.Errorf("machine actor: %w", err)
		}
		actor.normalize()
		existing, err := a.Store.User().GetByOAuthClientID(ctx, actor.ClientID)
		if err == nil {
			if existing.Roles == actor.Roles && existing.DisplayName == actor.DisplayName &&
				existing.Username == actor.Username && existing.ActorType == actor.ActorType {
				continue
			}
			existing.Roles = actor.Roles
			existing.DisplayName = actor.DisplayName
			existing.Username = actor.Username
			existing.ActorType = actor.ActorType
			existing.PreUpdate()
			// Validated as creation is: a declared empty display_name was
			// refused on create and silently written on update.
			if verr := existing.IsValid(); verr != nil {
				return fmt.Errorf("machine actor %q is invalid: %w", actor.ClientID, verr)
			}
			if _, err := a.Store.User().Update(ctx, existing); err != nil {
				return fmt.Errorf("update machine actor %q: %w", actor.ClientID, err)
			}
			// The cache is keyed by client id and holds the OLD roles, so a
			// reconcile that skipped this would take effect only after the
			// entry expired — which reads as the change not having applied.
			a.invalidateUserCache(existing)
			slog.Info("machine actor reconciled", "client_id", actor.ClientID, "roles", actor.Roles)
			continue
		}

		user := &model.User{
			Username:      actor.Username,
			DisplayName:   actor.DisplayName,
			Roles:         actor.Roles,
			ActorType:     actor.ActorType,
			OAuthClientID: actor.ClientID,
		}
		user.PreSave()
		if verr := user.IsValid(); verr != nil {
			return fmt.Errorf("machine actor %q is invalid: %w", actor.ClientID, verr)
		}
		if _, err := a.Store.User().Save(ctx, user); err != nil {
			// Another instance may have created it. Re-read before failing:
			// the row existing is the outcome this asked for.
			if _, err2 := a.Store.User().GetByOAuthClientID(ctx, actor.ClientID); err2 == nil {
				continue
			}
			return fmt.Errorf("create machine actor %q: %w", actor.ClientID, err)
		}
		slog.Info("machine actor created", "client_id", actor.ClientID,
			"username", actor.Username, "roles", actor.Roles, "actor_type", actor.ActorType)
	}
	return nil
}
