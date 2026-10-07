//go:build integration

package sqlstore

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func newTestUser(username string) *model.User {
	return &model.User{
		KratosID:    model.NewID(),
		Username:    username,
		DisplayName: "Test " + username,
		Email:       username + "@test.local",
	}
}

func TestUserStoreIntegration_SaveAndGet(t *testing.T) {
	ss := testStore(t)
	store := ss.User()

	user := newTestUser("alice")
	saved, err := store.Save(t.Context(), user)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("Save: expected non-empty ID")
	}

	got, err := store.Get(t.Context(), saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != saved.ID {
		t.Errorf("ID: got %q, want %q", got.ID, saved.ID)
	}
	if got.Username != saved.Username {
		t.Errorf("Username: got %q, want %q", got.Username, saved.Username)
	}
	if got.DisplayName != saved.DisplayName {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, saved.DisplayName)
	}
	if got.Email != saved.Email {
		t.Errorf("Email: got %q, want %q", got.Email, saved.Email)
	}
	if got.KratosID != saved.KratosID {
		t.Errorf("KratosID: got %q, want %q", got.KratosID, saved.KratosID)
	}
	if got.Roles != "system_user" {
		t.Errorf("Roles: got %q, want %q", got.Roles, "system_user")
	}
}

func TestUserStoreIntegration_GetByKratosID(t *testing.T) {
	ss := testStore(t)
	store := ss.User()

	user := newTestUser("bob")
	saved, err := store.Save(t.Context(), user)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.GetByKratosID(t.Context(), saved.KratosID)
	if err != nil {
		t.Fatalf("GetByKratosID: %v", err)
	}

	if got.ID != saved.ID {
		t.Errorf("ID: got %q, want %q", got.ID, saved.ID)
	}
	if got.KratosID != saved.KratosID {
		t.Errorf("KratosID: got %q, want %q", got.KratosID, saved.KratosID)
	}
}

func TestUserStoreIntegration_Update(t *testing.T) {
	ss := testStore(t)
	store := ss.User()

	user := newTestUser("carol")
	saved, err := store.Save(t.Context(), user)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	saved.DisplayName = "Carol Updated"
	saved.Email = "carol.updated@test.local"
	updated, err := store.Update(t.Context(), saved)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Get(t.Context(), updated.ID)
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}

	if got.DisplayName != "Carol Updated" {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, "Carol Updated")
	}
	if got.Email != "carol.updated@test.local" {
		t.Errorf("Email: got %q, want %q", got.Email, "carol.updated@test.local")
	}
	if got.UpdateAt <= saved.CreateAt {
		t.Error("UpdateAt should be greater than CreateAt after update")
	}
}

func TestUserStoreIntegration_Search(t *testing.T) {
	ss := testStore(t)
	store := ss.User()

	users := []string{"dave", "diana", "eve"}
	for _, name := range users {
		if _, err := store.Save(t.Context(), newTestUser(name)); err != nil {
			t.Fatalf("Save %s: %v", name, err)
		}
	}

	// Search for "d" should match dave and diana
	results, err := store.Search(t.Context(), "d", 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search: got %d results, want 2", len(results))
	}

	// Results ordered by username: dave, diana
	if results[0].Username != "dave" {
		t.Errorf("Search[0].Username: got %q, want %q", results[0].Username, "dave")
	}
	if results[1].Username != "diana" {
		t.Errorf("Search[1].Username: got %q, want %q", results[1].Username, "diana")
	}

	// Search for "eve" should match exactly one
	results, err = store.Search(t.Context(), "eve", 0, 10)
	if err != nil {
		t.Fatalf("Search eve: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search eve: got %d results, want 1", len(results))
	}
}

func TestUserStoreIntegration_ActorTypeRoundTrip(t *testing.T) {
	ss := testStore(t)
	store := ss.User()

	agent := newTestUser("agent-bot")
	agent.ActorType = model.ActorTypeAgent
	saved, err := store.Save(t.Context(), agent)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(t.Context(), saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ActorType != model.ActorTypeAgent {
		t.Errorf("ActorType: got %q, want %q", got.ActorType, model.ActorTypeAgent)
	}

	// Default actor type is user.
	human, err := store.Save(t.Context(), newTestUser("plain-human"))
	if err != nil {
		t.Fatalf("Save human: %v", err)
	}
	got, err = store.Get(t.Context(), human.ID)
	if err != nil {
		t.Fatalf("Get human: %v", err)
	}
	if got.ActorType != model.ActorTypeUser {
		t.Errorf("default ActorType: got %q, want %q", got.ActorType, model.ActorTypeUser)
	}
}

// TWO machine actors, because one proves nothing about the constraint that
// matters. kratos_id and email are UNIQUE and were NOT NULL until 000006; a
// store that wrote ” instead of NULL would save the first machine and fail
// the second with a unique violation naming a column the caller never set.
//
// It also pins the CAST. NULLIF($2, ”) makes Postgres infer text, and
// assigning text to a uuid column fails with "column kratos_id is of type uuid
// but expression is of type text" — which the mock-backed unit tests cannot
// see, and which crash-looped chitd on first deploy.
func TestUserStoreIntegration_MachineActorsHaveNoIdentity(t *testing.T) {
	ss := testStore(t)
	store := ss.User()

	machine := func(username, clientID string) *model.User {
		u := &model.User{
			Username:      username,
			DisplayName:   "Machine " + username,
			ActorType:     model.ActorTypeBot,
			Roles:         "system_user",
			OAuthClientID: clientID,
		}
		u.PreSave()
		return u
	}

	for _, m := range []*model.User{
		machine("notifier", "chit-notifier"),
		machine("tui", "chit-tui"),
	} {
		if _, err := store.Save(t.Context(), m); err != nil {
			t.Fatalf("save %s: %v", m.Username, err)
		}
	}

	got, err := store.GetByOAuthClientID(t.Context(), "chit-notifier")
	if err != nil {
		t.Fatalf("GetByOAuthClientID: %v", err)
	}
	if got.KratosID != "" || got.Email != "" {
		t.Fatalf("kratos_id=%q email=%q — a machine has neither, and NULL must read back as empty", got.KratosID, got.Email)
	}
	if got.OAuthClientID != "chit-notifier" || got.ActorType != model.ActorTypeBot {
		t.Fatalf("round trip lost the machine's identity: %+v", got)
	}

	// And a person still round-trips with both columns populated.
	p := newTestUser("alice")
	if _, err = store.Save(t.Context(), p); err != nil {
		t.Fatalf("save person: %v", err)
	}
	back, err := store.GetByKratosID(t.Context(), p.KratosID)
	if err != nil {
		t.Fatalf("GetByKratosID: %v", err)
	}
	if back.KratosID != p.KratosID || back.Email != p.Email {
		t.Fatalf("person round trip: got kratos_id=%q email=%q", back.KratosID, back.Email)
	}
}
