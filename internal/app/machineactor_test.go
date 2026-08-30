package app

import (
	"context"
	"testing"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
)

// A stateful user store, because every property worth asserting here is about
// what the SECOND call sees.
type maUserStore struct {
	byClient map[string]*model.User
	saves    int
	updates  int
}

func newMAUserStore() *maUserStore {
	return &maUserStore{byClient: map[string]*model.User{}}
}

func (s *maUserStore) Save(_ context.Context, u *model.User) (*model.User, error) {
	s.saves++
	copy := *u
	s.byClient[u.OAuthClientID] = &copy
	return &copy, nil
}
func (s *maUserStore) Update(_ context.Context, u *model.User) (*model.User, error) {
	s.updates++
	copy := *u
	s.byClient[u.OAuthClientID] = &copy
	return &copy, nil
}
func (s *maUserStore) GetByOAuthClientID(_ context.Context, id string) (*model.User, error) {
	if u, ok := s.byClient[id]; ok {
		return u, nil
	}
	return nil, errNotFound
}
func (s *maUserStore) Get(_ context.Context, _ string) (*model.User, error) { return nil, errNotFound }
func (s *maUserStore) GetByKratosID(_ context.Context, _ string) (*model.User, error) {
	return nil, errNotFound
}
func (s *maUserStore) GetByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, errNotFound
}
func (s *maUserStore) GetByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, errNotFound
}
func (s *maUserStore) Search(_ context.Context, _ string, _, _ int) ([]*model.User, error) {
	return nil, nil
}
func (s *maUserStore) GetByIDs(_ context.Context, _ []string) ([]*model.User, error) {
	return nil, nil
}

type maStore struct{ users *maUserStore }

func (m *maStore) User() store.UserStore { return m.users }
func (m *maStore) Team() store.TeamStore { return mentionMockTeamStore{} }
func (m *maStore) Channel() store.ChannelStore {
	return &mentionMockChannelStore{}
}
func (m *maStore) Post() store.PostStore     { return &searchMockPostStore{} }
func (m *maStore) Thread() store.ThreadStore { return &mentionMockThreadStore{} }
func (m *maStore) Tag() store.TagStore       { return searchMockTagStore{} }
func (m *maStore) Close()                    {}

func newMAApp() (*App, *maUserStore) {
	us := newMAUserStore()
	return &App{Store: &maStore{users: us}}, us
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
	err := a.EnsureMachineActors(context.Background(), []MachineActor{{
		ClientID: notifier, Username: "notifier", ActorType: "robot",
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
	got, err := ParseMachineActors(`[{"client_id":"a","username":"b","roles":"system_user","actor_type":"bot"}]`)
	if err != nil || len(got) != 1 || got[0].ClientID != "a" {
		t.Fatalf("well-formed declaration did not parse: %v %v", got, err)
	}
}
