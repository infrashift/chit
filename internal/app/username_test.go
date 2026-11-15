package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

const derivedKratosID = "6f1f2a64-2b8e-4a53-9c1e-0d6b2a1f4c11"

// validUsername reports whether name passes User.IsValid as a person's name.
func validUsername(name string) bool {
	u := &model.User{KratosID: derivedKratosID, Username: name, DisplayName: "x", Email: "x@example.com"}
	u.PreSave()
	return u.IsValid() == nil && u.Username == name
}

func TestUsernameCandidates(t *testing.T) {
	a30 := strings.Repeat("a", 30)
	long := a30 + "." + strings.Repeat("b", 60)
	cases := []struct {
		name, trait, email string
		want               []string
	}{
		{"an explicit trait wins", "Alice", "alice@example.com", []string{"alice"}},
		{"email only", "", "Sam.Smith@example.com",
			[]string{"sam.smith-6f1f2a64", "sam.smith-6f1f2a642b8e", "sam.smith-6f1f2a642b8e4a53", "sam.smith-6f1f2a642b8e4a539c1e0d6b2a1f4c11"}},
		{"no email", "", "",
			[]string{"user-6f1f2a642b8e", "user-6f1f2a642b8e4a53", "user-6f1f2a642b8e4a539c1e0d6b2a1f4c11"}},
		{"invalid characters", "", "-Jöhn+Tag!!o'Brien.@example.com",
			[]string{"j-hn-tag-o-brien-6f1f2a64", "j-hn-tag-o-brien-6f1f2a642b8e", "j-hn-tag-o-brien-6f1f2a642b8e4a53", "j-hn-tag-o-brien-6f1f2a642b8e4a539c1e0d6b2a1f4c11"}},
		{"only invalid characters", "", "+++@example.com",
			[]string{"user-6f1f2a642b8e", "user-6f1f2a642b8e4a53", "user-6f1f2a642b8e4a539c1e0d6b2a1f4c11"}},
		{"maximum length", "", long + "@example.com", []string{
			long[:55] + "-6f1f2a64",
			long[:51] + "-6f1f2a642b8e",
			long[:47] + "-6f1f2a642b8e4a53",
			// Cut just after the ".", which may not end a name.
			a30 + "-6f1f2a642b8e4a539c1e0d6b2a1f4c11",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := usernameCandidates(tc.trait, tc.email, derivedKratosID)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
			for _, name := range got {
				if !validUsername(name) || len(name) > maxUsernameLen {
					t.Errorf("%q (%d characters) does not pass User.IsValid", name, len(name))
				}
			}
		})
	}
}

func TestProvisionUser_DerivedUsername(t *testing.T) {
	t.Run("an email-only identity", func(t *testing.T) {
		f := newFixture(t)
		fakeKratos(t, f, map[string]string{"email": "Sam.Smith@example.com"}, http.StatusOK)
		u, err := f.app.ProvisionUser(t.Context(), derivedKratosID)
		if err != nil {
			t.Fatalf("ProvisionUser: %v", err)
		}
		if u.Username != "sam.smith-6f1f2a64" || u.DisplayName != "sam.smith-6f1f2a64" {
			t.Fatalf("username %q, display name %q", u.Username, u.DisplayName)
		}
	})

	t.Run("a taken name moves to the next candidate", func(t *testing.T) {
		f := newFixture(t)
		fakeKratos(t, f, map[string]string{"email": "sam.smith@example.com", "name": "Sam"}, http.StatusOK)
		f.store.Users.Seed(&model.User{ID: model.NewID(), KratosID: model.NewID(), Username: "sam.smith-6f1f2a64",
			DisplayName: "Other", Email: "other@example.com", Roles: "system_user", ActorType: model.ActorTypeUser, CreateAt: 1, UpdateAt: 1})
		u, err := f.app.ProvisionUser(t.Context(), derivedKratosID)
		if err != nil {
			t.Fatalf("ProvisionUser: %v", err)
		}
		if u.Username != "sam.smith-6f1f2a642b8e" || u.DisplayName != "Sam" {
			t.Fatalf("username %q, display name %q", u.Username, u.DisplayName)
		}
	})

	// User.IsValid requires a person's email, so an identity without one is
	// still refused, but for its email, not for its username.
	t.Run("an identity without email", func(t *testing.T) {
		f := newFixture(t)
		fakeKratos(t, f, map[string]string{"name": "No Mail"}, http.StatusOK)
		_, err := f.app.ProvisionUser(t.Context(), derivedKratosID)
		var appErr *model.AppError
		if !errors.As(err, &appErr) || appErr.Message != "invalid email" {
			t.Fatalf("got %v, want the invalid email refusal", err)
		}
	})
}
