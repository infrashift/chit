package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/store/storetest"
)

// fakeKratos serves one identity's traits, counting lookups.
func fakeKratos(t *testing.T, f *fixture, traits map[string]string, status int) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"traits": traits})
	}))
	t.Cleanup(srv.Close)
	f.app.Config.KratosAdminURL = srv.URL
	return &calls
}

func TestProvisionUser(t *testing.T) {
	t.Run("a new identity is created from its traits, then cached", func(t *testing.T) {
		f := newFixture(t)
		calls := fakeKratos(t, f, map[string]string{"username": "Newbie", "name": "New Person", "email": "n@example.com"}, http.StatusOK)
		kratosID := model.NewID()

		u, err := f.app.ProvisionUser(t.Context(), kratosID)
		if err != nil {
			t.Fatalf("ProvisionUser: %v", err)
		}
		// The shared cluster schema calls the display name "name".
		if u.Username != "newbie" || u.DisplayName != "New Person" || u.Email != "n@example.com" {
			t.Fatalf("provisioned %+v", u)
		}
		if _, err = f.app.ProvisionUser(t.Context(), kratosID); err != nil || calls.Load() != 1 {
			t.Fatalf("second call: err=%v, Kratos called %d times; want the cache to answer", err, calls.Load())
		}
	})

	t.Run("an existing row needs no Kratos lookup", func(t *testing.T) {
		f := newFixture(t)
		calls := fakeKratos(t, f, nil, http.StatusInternalServerError)
		existing := f.user("known")

		u, err := f.app.ProvisionUser(t.Context(), existing.KratosID)
		if err != nil || u.ID != existing.ID || calls.Load() != 0 {
			t.Fatalf("got %v, %v after %d Kratos calls", u, err, calls.Load())
		}
	})

	t.Run("a Kratos failure is an error, not a nameless user", func(t *testing.T) {
		f := newFixture(t)
		fakeKratos(t, f, nil, http.StatusNotFound)
		if _, err := f.app.ProvisionUser(t.Context(), model.NewID()); err == nil {
			t.Fatal("provisioned a user Kratos does not know")
		}
	})

	// Two first requests from the same person race to create their row. The
	// loser's Save fails on the unique kratos_id and must return the
	// winner's row rather than an error.
	t.Run("losing the creation race returns the winner's row", func(t *testing.T) {
		f := newFixture(t)
		fakeKratos(t, f, map[string]string{"username": "racer", "display_name": "Racer", "email": "r@example.com"}, http.StatusOK)
		kratosID := model.NewID()
		winner := &model.User{ID: model.NewID(), KratosID: kratosID, Username: "racer", DisplayName: "Racer",
			Email: "r@example.com", Roles: "system_user", ActorType: model.ActorTypeUser, CreateAt: 1, UpdateAt: 1}
		f.app.Store = &racingStore{Store: f.store, users: &racingUsers{UserStore: f.store.Users, winner: winner}}

		u, err := f.app.ProvisionUser(t.Context(), kratosID)
		if err != nil || u.ID != winner.ID {
			t.Fatalf("got %v, %v; want the winner's row", u, err)
		}
	})
}

// racingUsers simulates another request creating the user between this
// request's lookup and its Save.
type racingUsers struct {
	*storetest.UserStore
	winner *model.User
}

func (r *racingUsers) Save(ctx context.Context, u *model.User) (*model.User, error) {
	r.Seed(r.winner)
	return r.UserStore.Save(ctx, u)
}

type racingStore struct {
	*storetest.Store
	users *racingUsers
}

func (s *racingStore) User() store.UserStore { return s.users }
