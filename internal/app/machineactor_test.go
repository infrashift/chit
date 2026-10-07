package app

import (
	"context"
	"testing"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/store/storetest"
)

// countingUsers counts writes, because every property worth asserting here is
// about what the SECOND boot does to the row.
type countingUsers struct {
	*storetest.UserStore
	saves, updates int
}

func (c *countingUsers) Save(ctx context.Context, u *model.User) (*model.User, error) {
	c.saves++
	return c.UserStore.Save(ctx, u)
}

func (c *countingUsers) Update(ctx context.Context, u *model.User) (*model.User, error) {
	c.updates++
	return c.UserStore.Update(ctx, u)
}

type maStore struct {
	*storetest.Store
	users *countingUsers
}

func (m *maStore) User() store.UserStore { return m.users }

func newMAApp() (*App, *countingUsers) {
	ms := storetest.New()
	us := &countingUsers{UserStore: ms.Users}
	return &App{Store: &maStore{Store: ms, users: us}}, us
}

const notifier = "chit-notifier"

func TestEnsureMachineActors_CreatesTheRowAResolveNeeds(t *testing.T) {
	a, us := newMAApp()
	actors := []MachineActor{{
		ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier",
		Roles: "system_user system_admin", ActorType: model.ActorTypeBot,
	}}

	if err := a.EnsureMachineActors(context.Background(), actors); err != nil {
		t.Fatalf("EnsureMachineActors: %v", err)
	}

	got, err := a.Store.User().GetByOAuthClientID(context.Background(), notifier)
	if err != nil {
		t.Fatalf("the client is still unbound: %v", err)
	}
	if got.ActorType != model.ActorTypeBot {
		t.Fatalf("actor_type = %q, want bot", got.ActorType)
	}
	if !got.IsSystemAdmin() {
		t.Fatal("declared system_admin did not reach the row")
	}
	if us.saves != 1 {
		t.Fatalf("saves = %d, want 1", us.saves)
	}
}

func TestEnsureMachineActors_IsIdempotent(t *testing.T) {
	a, us := newMAApp()
	actors := []MachineActor{{
		ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier",
		Roles: "system_user", ActorType: model.ActorTypeBot,
	}}
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := a.EnsureMachineActors(ctx, actors); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if us.saves != 1 || us.updates != 0 {
		t.Fatalf("saves=%d updates=%d, want 1 and 0 — boot must not churn the row", us.saves, us.updates)
	}
}

// THE ONE THAT MATTERS. A version of this that only ever CREATED would pass
// every test above and silently make role removal impossible: the declaration
// would say one thing and the row another, forever.
func TestEnsureMachineActors_ReconcilesRolesBothWays(t *testing.T) {
	a, us := newMAApp()
	ctx := context.Background()
	base := MachineActor{
		ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier",
		ActorType: model.ActorTypeBot,
	}

	granted := base
	granted.Roles = "system_user system_admin"
	if err := a.EnsureMachineActors(ctx, []MachineActor{granted}); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Store.User().GetByOAuthClientID(ctx, notifier)
	if !got.IsSystemAdmin() {
		t.Fatal("grant did not apply")
	}

	revoked := base
	revoked.Roles = "system_user"
	if err := a.EnsureMachineActors(ctx, []MachineActor{revoked}); err != nil {
		t.Fatal(err)
	}
	got, _ = a.Store.User().GetByOAuthClientID(ctx, notifier)
	if got.IsSystemAdmin() {
		t.Fatal("REVOKING system_admin in the declaration did not revoke it on the row")
	}
	if us.updates != 1 {
		t.Fatalf("updates = %d, want 1", us.updates)
	}
}

func TestEnsureMachineActors_RejectsAnInvalidActorType(t *testing.T) {
	a, us := newMAApp()
	// Everything else is valid, so actor_type is the only reason to refuse.
	// Without a display name this passed on display_name validation and would
	// have kept passing with the actor_type check deleted.
	err := a.EnsureMachineActors(context.Background(), []MachineActor{{
		ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier", ActorType: "robot",
	}})
	if err == nil {
		t.Fatal("expected an invalid actor_type to be refused")
	}
	if us.saves != 0 {
		t.Fatalf("saves = %d, want 0 — an invalid actor must not reach the store", us.saves)
	}
}

func TestParseMachineActors(t *testing.T) {
	if got, err := ParseMachineActors(""); err != nil || got != nil {
		t.Fatalf("empty declaration must be legal: %v %v", got, err)
	}
	if _, err := ParseMachineActors(`[{"username":"x"}]`); err == nil {
		t.Fatal("an actor with no client_id must be refused")
	}
	if _, err := ParseMachineActors(`[{"client_id":"x"}]`); err == nil {
		t.Fatal("an actor with no username must be refused")
	}
	if _, err := ParseMachineActors(`[{"client_id":"x","username":"y"}]`); err == nil {
		t.Fatal("an actor with no actor_type must be refused: PreSave would make it a user")
	}
	got, err := ParseMachineActors(`[{"client_id":"a","username":"b","roles":"system_user","actor_type":"bot"}]`)
	if err != nil || len(got) != 1 || got[0].ClientID != "a" {
		t.Fatalf("well-formed declaration did not parse: %v %v", got, err)
	}
}

// Omitting roles defaulted the row to system_user on create, then compared
// the raw "" against it on the next boot and wrote "" back: every second boot
// stripped the actor's roles.
func TestEnsureMachineActors_OmittedRolesAreStable(t *testing.T) {
	a, us := newMAApp()
	actors := []MachineActor{{ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier", ActorType: model.ActorTypeBot}}

	for i := range 2 {
		if err := a.EnsureMachineActors(context.Background(), actors); err != nil {
			t.Fatalf("boot %d: %v", i, err)
		}
	}
	got, _ := a.Store.User().GetByOAuthClientID(context.Background(), notifier)
	if got.Roles != "system_user" {
		t.Fatalf("roles = %q after a second boot, want system_user", got.Roles)
	}
	if us.updates != 0 {
		t.Fatalf("updates = %d, want 0: nothing changed", us.updates)
	}
}

func TestEnsureMachineActors_ReconcilesEveryDeclaredField(t *testing.T) {
	a, _ := newMAApp()
	base := MachineActor{ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier", ActorType: model.ActorTypeBot}
	if err := a.EnsureMachineActors(context.Background(), []MachineActor{base}); err != nil {
		t.Fatal(err)
	}

	changed := base
	changed.Username, changed.DisplayName, changed.ActorType = "forge", "Forge", model.ActorTypeAgent
	if err := a.EnsureMachineActors(context.Background(), []MachineActor{changed}); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Store.User().GetByOAuthClientID(context.Background(), notifier)
	if got.Username != "forge" || got.DisplayName != "Forge" || got.ActorType != model.ActorTypeAgent {
		t.Fatalf("row = %+v, want every declared field applied", got)
	}

	blank := changed
	blank.DisplayName = ""
	if err := a.EnsureMachineActors(context.Background(), []MachineActor{blank}); err == nil {
		t.Fatal("an update to an empty display_name was accepted; create refuses it")
	}
}

func TestEnsureMachineActors_RefusesAnActorWithoutAMachineType(t *testing.T) {
	for _, typ := range []string{"", model.ActorTypeUser} {
		a, us := newMAApp()
		err := a.EnsureMachineActors(context.Background(), []MachineActor{{
			ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier", ActorType: typ,
		}})
		if err == nil || us.saves != 0 {
			t.Fatalf("actor_type %q: err=%v saves=%d, want a refusal before any write", typ, err, us.saves)
		}
	}
}

// A declared username already held by a person cannot be created; boot must
// fail saying so rather than retry forever.
func TestEnsureMachineActors_UsernameClash(t *testing.T) {
	a, _ := newMAApp()
	person := &model.User{KratosID: model.NewID(), Username: "notifier", DisplayName: "A Person", Email: "p@example.com"}
	if _, err := a.Store.User().Save(context.Background(), person); err != nil {
		t.Fatal(err)
	}
	err := a.EnsureMachineActors(context.Background(), []MachineActor{{
		ClientID: notifier, Username: "notifier", DisplayName: "Forge Notifier", ActorType: model.ActorTypeBot,
	}})
	if err == nil {
		t.Fatal("a machine actor was created over a person's username")
	}
}
