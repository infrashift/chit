package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/infrashift/chit/internal/config"
)

// TWO SCHEMAS, ONE READER.
//
// chit's own deploy/kratos/identity.schema.json names the human-readable field
// `display_name`. The shared cluster schema chit authenticates against in the
// service mesh — terraform/live/ory-identity/config/kratos/identity.schema.json.tftpl
// — names it `name`, and additionally carries `role` and `kind`.
//
// This is the failure mode worth a test: neither spelling is an ERROR. An
// identity carrying only `name` decoded cleanly into a struct that only knew
// `display_name`, authenticated perfectly, and provisioned a user with an EMPTY
// display name. Nothing logs it; the first symptom is a blank byline on a post,
// a long way from the cause.
func TestFetchKratosIdentity_BothSchemaSpellings(t *testing.T) {
	cases := []struct {
		name    string
		traits  string
		want    string
		comment string
	}{
		{
			name:    "chit's own schema uses display_name",
			traits:  `{"username":"alice","display_name":"Alice A","email":"alice@test.com"}`,
			want:    "Alice A",
			comment: "the standalone deployment",
		},
		{
			name:    "the cluster schema uses name",
			traits:  `{"username":"bob","name":"Bob B","email":"bob@test.com","role":"developer","kind":"human"}`,
			want:    "Bob B",
			comment: "ory-identity in the mesh",
		},
		{
			name:    "display_name wins when both are present",
			traits:  `{"username":"carol","display_name":"Carol D","name":"Carol N","email":"c@test.com"}`,
			want:    "Carol D",
			comment: "no build flag needed to serve both",
		},
		{
			name:    "username is the fallback, never an empty byline",
			traits:  `{"username":"dave","email":"dave@test.com"}`,
			want:    "dave",
			comment: "an agent identity may carry neither",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"traits":` + tc.traits + `}`))
				}))
			defer srv.Close()

			a := New(nil, nil, nil, &config.Config{
				KratosAdminURL: srv.URL,
				CacheSize:      10,
			})

			_, displayName, _, err := a.FetchKratosIdentity(t.Context(), "kratos-id")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if displayName != tc.want {
				t.Fatalf("%s: display name = %q, want %q", tc.comment, displayName, tc.want)
			}
		})
	}
}

// The role, kind and pubkey traits the cluster schema adds must not disturb
// the fields chit does read.
func TestFetchKratosIdentity_ClusterTraitsDoNotBreakDecoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"traits":{"username":"erin","name":"Erin E",` +
				`"email":"erin@test.com","role":"auditor","kind":"agent",` +
				`"pubkey":"0123456789abcdef"}}`))
		}))
	defer srv.Close()

	a := New(nil, nil, nil, &config.Config{KratosAdminURL: srv.URL, CacheSize: 10})

	username, displayName, email, err := a.FetchKratosIdentity(t.Context(), "kratos-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "erin" {
		t.Errorf("username = %q, want erin", username)
	}
	if displayName != "Erin E" {
		t.Errorf("displayName = %q, want Erin E", displayName)
	}
	if email != "erin@test.com" {
		t.Errorf("email = %q, want erin@test.com", email)
	}
}
