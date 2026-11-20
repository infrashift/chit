package main

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
)

// fakeLoader returns a fixed session or error from Load.
type fakeLoader struct {
	session *auth.StoredSession
	err     error
}

func (f fakeLoader) Load() (*auth.StoredSession, error) { return f.session, f.err }

// fakeChecker answers CheckSession with err and records the token it was asked about.
type fakeChecker struct {
	err     error
	checked *string
}

func (f fakeChecker) CheckSession(_ context.Context, token string) (*auth.Identity, error) {
	*f.checked = token
	if f.err != nil {
		return nil, f.err
	}
	return &auth.Identity{}, nil
}

func TestStoredToken(t *testing.T) {
	saved := fakeLoader{session: &auth.StoredSession{Token: "tok"}}
	tests := []struct {
		name        string
		load        fakeLoader
		check       error
		want        string
		wantChecked string
	}{
		{"valid", saved, nil, "tok", "tok"},
		{"rejected by Kratos", saved, &auth.KratosError{StatusCode: http.StatusUnauthorized}, "", "tok"},
		// An unreachable Kratos says nothing about the session. Discarding
		// it sent the user to sign in again whenever Kratos was down.
		{"Kratos unreachable", saved, errors.New("connection refused"), "tok", "tok"},
		{"nothing stored", fakeLoader{err: errors.New("no file")}, nil, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var checked string
			got := storedToken(context.Background(), tc.load, fakeChecker{err: tc.check, checked: &checked})
			if got != tc.want {
				t.Errorf("storedToken = %q, want %q", got, tc.want)
			}
			if checked != tc.wantChecked {
				t.Errorf("checked token = %q, want %q", checked, tc.wantChecked)
			}
		})
	}
}
