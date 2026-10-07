package main

import (
	"errors"
	"net/http"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
)

func TestStoredToken(t *testing.T) {
	saved := func() (*auth.StoredSession, error) { return &auth.StoredSession{Token: "tok"}, nil }
	tests := []struct {
		name  string
		load  func() (*auth.StoredSession, error)
		check error
		want  string
	}{
		{"valid", saved, nil, "tok"},
		{"rejected by Kratos", saved, &auth.KratosError{StatusCode: http.StatusUnauthorized}, ""},
		// An unreachable Kratos says nothing about the session. Discarding
		// it sent the user to sign in again whenever Kratos was down.
		{"Kratos unreachable", saved, errors.New("connection refused"), "tok"},
		{"nothing stored", func() (*auth.StoredSession, error) { return nil, errors.New("no file") }, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := storedToken(tc.load, func(string) error { return tc.check })
			if got != tc.want {
				t.Errorf("storedToken = %q, want %q", got, tc.want)
			}
		})
	}
}
