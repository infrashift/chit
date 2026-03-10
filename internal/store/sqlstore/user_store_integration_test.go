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
	saved, err := store.Save(user)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("Save: expected non-empty ID")
	}

	got, err := store.Get(saved.ID)
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
	saved, err := store.Save(user)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.GetByKratosID(saved.KratosID)
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
	saved, err := store.Save(user)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	saved.DisplayName = "Carol Updated"
	saved.Email = "carol.updated@test.local"
	updated, err := store.Update(saved)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Get(updated.ID)
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
		if _, err := store.Save(newTestUser(name)); err != nil {
			t.Fatalf("Save %s: %v", name, err)
		}
	}

	// Search for "d" should match dave and diana
	results, err := store.Search("d", 0, 10)
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
	results, err = store.Search("eve", 0, 10)
	if err != nil {
		t.Fatalf("Search eve: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search eve: got %d results, want 1", len(results))
	}
}
