package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// Tests for HALO prerequisite P-CHIT-8: X-On-Behalf-Of names a person by
// Kratos identity (kratos:<id>), and halod creates a person's private channel,
// adds itself, posts as the person and reads as itself. They run the whole
// router, so AuthExtract, OpenAPI validation and the handlers all take part.

const (
	newKratosID           = "6f1f2a64-2b8e-4a53-9c1e-0d6b2a1f4c11" // never seen by Chit
	inactiveKratosID      = "6f1f2a64-2b8e-4a53-9c1e-0d6b2a1f4c22"
	missingKratosID       = "6f1f2a64-2b8e-4a53-9c1e-0d6b2a1f4c33" // Kratos answers 404
	goneKratosID          = "6f1f2a64-2b8e-4a53-9c1e-0d6b2a1f4c44" // a deactivated Chit user
	goneUserID            = "019421a0-0000-7000-8000-0000000000ee"
	emailOnlyKratosID     = "9a8b7c6d-2b8e-4a53-9c1e-0d6b2a1f4c55" // traits: email only, as HALO's schema
	conversationChannelID = "019421a0-0000-7000-8000-000000000021"
)

// fakeKratos serves the Kratos admin identity API for newKratosID (active)
// and inactiveKratosID, emailOnlyKratosID (no username or name trait), and
// 404 for anything else. It counts the lookups.
func (e *bridgeEnv) fakeKratos(t *testing.T) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		state := "active"
		traits := map[string]string{"username": "Subject", "name": "Sam Subject", "email": "sam@example.com"}
		switch r.URL.Path {
		case "/admin/identities/" + newKratosID:
		case "/admin/identities/" + inactiveKratosID:
			state = "inactive"
		case "/admin/identities/" + emailOnlyKratosID:
			traits = map[string]string{"email": "Erin.Only+halo@example.com"}
		default:
			http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id": strings.TrimPrefix(r.URL.Path, "/admin/identities/"), "state": state, "traits": traits,
		}); err != nil {
			t.Errorf("encode identity: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	e.a.Config.KratosAdminURL = srv.URL
	return &calls
}

// auditRecords returns the audit log's records whose action is action.
func (e *bridgeEnv) auditRecords(t *testing.T, action string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(e.audit)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("audit line %q: %v", line, err)
		}
		if rec["action"] == action {
			out = append(out, rec)
		}
	}
	return out
}

func (e *bridgeEnv) userByKratosID(t *testing.T, kratosID string) *model.User {
	t.Helper()
	u, err := e.ms.Users.GetByKratosID(t.Context(), kratosID)
	if err != nil {
		return nil
	}
	return u
}

func kratosOBO(id string) map[string]string { return obo("kratos:" + id) }

const halod = "client:" + haloClientID

// A Kratos identity that Chit has never seen gets the user that the person's
// first browser request would have made, and the provisioning is audited.
func TestOnBehalfOfKratos_ProvisionsTheUserABrowserWould(t *testing.T) {
	e := newBridgeEnv(t)
	calls := e.fakeKratos(t)

	w := e.do(t, http.MethodGet, "/api/v1/users/me", halod, "", kratosOBO(newKratosID))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var person model.User
	decodeJSON(t, w.Body, &person)
	if person.KratosID != newKratosID || person.Username != "subject" || person.DisplayName != "Sam Subject" ||
		person.Email != "sam@example.com" || person.Roles != "system_user" || person.ActorType != model.ActorTypeUser {
		t.Fatalf("provisioned %+v", person)
	}

	// The person's own first browser request finds the same user.
	w = e.do(t, http.MethodGet, "/api/v1/users/me", "user:"+newKratosID, "", nil)
	var browser model.User
	decodeJSON(t, w.Body, &browser)
	if w.Code != http.StatusOK || browser.ID != person.ID {
		t.Fatalf("browser request: %d, user %q, want %q", w.Code, browser.ID, person.ID)
	}

	recs := e.auditRecords(t, "user_provisioned")
	if len(recs) != 1 {
		t.Fatalf("%d user_provisioned records, want 1", len(recs))
	}
	if r := recs[0]; r["msg"] != "on_behalf_of" || r["actor_id"] != haloAgentID || r["client_id"] != haloClientID ||
		r["principal_id"] != person.ID || r["kratos_id"] != newKratosID {
		t.Errorf("audit record = %v", r)
	}

	// Again: the same user, no second Kratos lookup, no second record.
	w = e.do(t, http.MethodGet, "/api/v1/users/me", halod, "", kratosOBO(newKratosID))
	var again model.User
	decodeJSON(t, w.Body, &again)
	if w.Code != http.StatusOK || again.ID != person.ID {
		t.Fatalf("second call: %d, user %q", w.Code, again.ID)
	}
	if calls.Load() != 1 || len(e.auditRecords(t, "user_provisioned")) != 1 {
		t.Fatalf("Kratos called %d times, %d records; want 1 and 1", calls.Load(), len(e.auditRecords(t, "user_provisioned")))
	}
}

// An identity with only an email (HALO's schema) gets a derived username on
// both paths: on behalf of, and the person's own browser request.
func TestKratosEmailOnlyIdentity_BothPaths(t *testing.T) {
	const want = "erin.only-halo-9a8b7c6d"
	for name, req := range map[string]struct{ who, obo string }{
		"on behalf of": {halod, "kratos:" + emailOnlyKratosID},
		"browser":      {"user:" + emailOnlyKratosID, ""},
	} {
		t.Run(name, func(t *testing.T) {
			e := newBridgeEnv(t)
			e.fakeKratos(t)
			var headers map[string]string
			if req.obo != "" {
				headers = obo(req.obo)
			}
			w := e.do(t, http.MethodGet, "/api/v1/users/me", req.who, "", headers)
			var u model.User
			decodeJSON(t, w.Body, &u)
			if w.Code != http.StatusOK || u.KratosID != emailOnlyKratosID || u.Username != want ||
				u.DisplayName != want || u.Email != "erin.only+halo@example.com" {
				t.Fatalf("got %d, %+v; want username and display name %q", w.Code, u, want)
			}
		})
	}
}

// A plain Chit user ID keeps working, and GET /users/me returns the person.
func TestOnBehalfOf_UsersMeReturnsThePrincipal(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/users/me", halod, "", obo(testUserID))
	var u model.User
	decodeJSON(t, w.Body, &u)
	if w.Code != http.StatusOK || u.ID != testUserID {
		t.Fatalf("got %d, user %q; want the principal %q", w.Code, u.ID, testUserID)
	}
}

// Without X-On-Behalf-Of a machine actor gets itself: this is how halod
// learns its own user ID.
func TestMachineActor_UsersMeReturnsItself(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/users/me", halod, "", nil)
	var u model.User
	decodeJSON(t, w.Body, &u)
	if w.Code != http.StatusOK || u.ID != haloAgentID || u.ActorType != model.ActorTypeAgent {
		t.Fatalf("got %d, %+v; want halod's own agent user", w.Code, u)
	}
}

// Every refusal leaves Chit without a new user. The ones decided before a
// lookup do not ask Kratos at all.
func TestOnBehalfOfKratos_Refusals(t *testing.T) {
	cases := []struct {
		name, method, path, who, principal string
		want                               int
		kratosCalls                        int32
	}{
		{"client not on the allowlist", http.MethodGet, "/api/v1/users/me", "client:" + otherClientID, "kratos:" + newKratosID, http.StatusForbidden, 0},
		{"a person's session", http.MethodGet, "/api/v1/users/me", "user:" + testKratosID, "kratos:" + newKratosID, http.StatusForbidden, 0},
		{"a route that does not take it", http.MethodGet, "/api/v1/channels/" + testChannelID + "/posts", halod, "kratos:" + newKratosID, http.StatusBadRequest, 0},
		{"empty kratos ID", http.MethodGet, "/api/v1/users/me", halod, "kratos:", http.StatusBadRequest, 0},
		{"not a UUID", http.MethodGet, "/api/v1/users/me", halod, "kratos:not-a-uuid", http.StatusBadRequest, 0},
		{"a path", http.MethodGet, "/api/v1/users/me", halod, "kratos:../../admin/identities", http.StatusBadRequest, 0},
		{"uppercase UUID", http.MethodGet, "/api/v1/users/me", halod, "kratos:" + strings.ToUpper(newKratosID), http.StatusBadRequest, 0},
		{"braced UUID", http.MethodGet, "/api/v1/users/me", halod, "kratos:{" + newKratosID + "}", http.StatusBadRequest, 0},
		{"wrong prefix case", http.MethodGet, "/api/v1/users/me", halod, "Kratos:" + newKratosID, http.StatusBadRequest, 0},
		{"unknown to Kratos", http.MethodGet, "/api/v1/users/me", halod, "kratos:" + missingKratosID, http.StatusBadRequest, 1},
		{"inactive in Kratos", http.MethodGet, "/api/v1/users/me", halod, "kratos:" + inactiveKratosID, http.StatusBadRequest, 1},
		{"a deactivated Chit user", http.MethodGet, "/api/v1/users/me", halod, "kratos:" + goneKratosID, http.StatusBadRequest, 0},
		{"post as a deactivated Chit user", http.MethodPost, "/api/v1/posts", halod, "kratos:" + goneKratosID, http.StatusBadRequest, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newBridgeEnv(t)
			calls := e.fakeKratos(t)
			e.ms.Users.Seed(&model.User{
				ID: goneUserID, KratosID: goneKratosID, Username: "gone", DisplayName: "Gone", Email: "gone@example.com",
				Roles: "system_user", ActorType: model.ActorTypeUser, CreateAt: 1000, UpdateAt: 1000, DeleteAt: 2000,
			})
			body := ""
			if tc.method == http.MethodPost {
				body = postBody("hi", "")
			}
			w := e.do(t, tc.method, tc.path, tc.who, body, obo(tc.principal))
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := calls.Load(); got != tc.kratosCalls {
				t.Errorf("Kratos called %d times, want %d", got, tc.kratosCalls)
			}
			for _, id := range []string{newKratosID, inactiveKratosID, missingKratosID, goneKratosID} {
				if u := e.userByKratosID(t, id); u != nil {
					t.Errorf("a refused request provisioned %+v", u)
				}
			}
			if recs := e.auditRecords(t, "user_provisioned"); len(recs) != 0 {
				t.Errorf("audit records for a refusal: %v", recs)
			}
		})
	}
}

// The flow HALO uses for a Conversation: provision the person, add them to
// halod's team, create a private channel as the person, add halod as the
// person, post as the person, read as halod.
func TestOnBehalfOf_ConversationFlow(t *testing.T) {
	e := newBridgeEnv(t)
	e.fakeKratos(t)
	e.ms.Teams.SeedMember(&model.TeamMember{TeamID: testTeamID, UserID: haloAgentID})

	// halod's own user ID.
	w := e.do(t, http.MethodGet, "/api/v1/users/me", halod, "", nil)
	var self model.User
	decodeJSON(t, w.Body, &self)

	// The person's user ID, created on the way.
	w = e.do(t, http.MethodGet, "/api/v1/users/me", halod, "", kratosOBO(newKratosID))
	var person model.User
	decodeJSON(t, w.Body, &person)
	if w.Code != http.StatusOK {
		t.Fatalf("provision: %d %s", w.Code, w.Body.String())
	}

	// halod, a member of the team, adds the person to it as itself.
	w = e.do(t, http.MethodPost, "/api/v1/teams/"+testTeamID+"/members", halod, `{"user_id":"`+person.ID+`"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("team add: %d %s", w.Code, w.Body.String())
	}

	w = e.do(t, http.MethodPost, "/api/v1/channels", halod,
		`{"team_id":"`+testTeamID+`","name":"conversation-1","display_name":"Conversation","type":"P"}`, kratosOBO(newKratosID))
	if w.Code != http.StatusCreated {
		t.Fatalf("create channel: %d %s", w.Code, w.Body.String())
	}
	var ch model.Channel
	decodeJSON(t, w.Body, &ch)
	if ch.CreatorID != person.ID || ch.Type != model.ChannelPrivate {
		t.Fatalf("channel %+v, want a private channel created by %s", ch, person.ID)
	}

	w = e.do(t, http.MethodPost, "/api/v1/channels/"+ch.ID+"/members", halod, `{"user_id":"`+self.ID+`"}`, kratosOBO(newKratosID))
	if w.Code != http.StatusCreated {
		t.Fatalf("add halod: %d %s", w.Code, w.Body.String())
	}

	w = e.do(t, http.MethodPost, "/api/v1/posts", halod, `{"channel_id":"`+ch.ID+`","content":"hello"}`, kratosOBO(newKratosID))
	if w.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", w.Code, w.Body.String())
	}
	var post model.Post
	decodeJSON(t, w.Body, &post)
	if post.UserID != person.ID {
		t.Fatalf("post author %q, want %q", post.UserID, person.ID)
	}

	// halod reads as itself, a member.
	w = e.do(t, http.MethodGet, "/api/v1/channels/"+ch.ID+"/posts", halod, "", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), post.ID) {
		t.Fatalf("read as halod: %d %s", w.Code, w.Body.String())
	}
	// Another machine actor, not a member, may not.
	w = e.do(t, http.MethodGet, "/api/v1/channels/"+ch.ID+"/posts", "client:"+otherClientID, "", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("read as a non-member machine actor: %d, want 403", w.Code)
	}

	for action, attrs := range map[string]map[string]any{
		"channel_created":      {"channel_id": ch.ID},
		"channel_member_added": {"channel_id": ch.ID, "user_id": self.ID},
	} {
		recs := e.auditRecords(t, action)
		if len(recs) != 1 {
			t.Fatalf("%d %s records, want 1", len(recs), action)
		}
		r := recs[0]
		if r["actor_id"] != haloAgentID || r["client_id"] != haloClientID || r["principal_id"] != person.ID {
			t.Errorf("%s record = %v", action, r)
		}
		for k, v := range attrs {
			if r[k] != v {
				t.Errorf("%s record %s = %v, want %v", action, k, r[k], v)
			}
		}
	}
}

// seedPrivateChannel makes a private channel whose only members are
// testUser and, when withHalod, halod's agent.
func (e *bridgeEnv) seedPrivateChannel(withHalod bool) {
	e.ms.Channels.Seed(&model.Channel{
		ID: conversationChannelID, TeamID: testTeamID, CreatorID: testUserID, Name: "private", DisplayName: "Private",
		Type: model.ChannelPrivate, CreateAt: 1000, UpdateAt: 1000,
	})
	e.ms.Channels.SeedMember(&model.ChannelMember{ChannelID: conversationChannelID, UserID: testUserID, CreateAt: 1000})
	if withHalod {
		e.ms.Channels.SeedMember(&model.ChannelMember{ChannelID: conversationChannelID, UserID: haloAgentID, CreateAt: 1000})
	}
	e.ms.Posts.Seed(&model.Post{ID: model.NewID(), ChannelID: conversationChannelID, UserID: testUserID, Content: "secret", CreateAt: 4000, UpdateAt: 4000})
}

// The new routes authorize the principal, not the client.
func TestOnBehalfOf_ChannelRoutesRefusals(t *testing.T) {
	t.Run("create: the principal is not on the team", func(t *testing.T) {
		e := newBridgeEnv(t)
		// halod is on the team; the principal (thirdUser) is not.
		e.ms.Teams.SeedMember(&model.TeamMember{TeamID: testTeamID, UserID: haloAgentID})
		w := e.do(t, http.MethodPost, "/api/v1/channels", halod,
			`{"team_id":"`+testTeamID+`","name":"nope","display_name":"Nope","type":"P"}`, obo(thirdUserID))
		if w.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403: %s", w.Code, w.Body.String())
		}
		if _, err := e.ms.Channels.GetByName(t.Context(), testTeamID, "nope"); err == nil {
			t.Fatal("a refused request created the channel")
		}
	})
	t.Run("create: a client not on the allowlist", func(t *testing.T) {
		e := newBridgeEnv(t)
		w := e.do(t, http.MethodPost, "/api/v1/channels", "client:"+otherClientID,
			`{"team_id":"`+testTeamID+`","name":"nope","display_name":"Nope","type":"P"}`, obo(testUserID))
		if w.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403: %s", w.Code, w.Body.String())
		}
	})
	t.Run("add member: the principal is not a member of the private channel", func(t *testing.T) {
		e := newBridgeEnv(t)
		e.seedPrivateChannel(true)
		// halod is a member; the principal (extraUser, on the team) is not.
		w := e.do(t, http.MethodPost, "/api/v1/channels/"+conversationChannelID+"/members", halod,
			`{"user_id":"`+extraUserID+`"}`, obo(extraUserID))
		if w.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403: %s", w.Code, w.Body.String())
		}
		if _, err := e.ms.Channels.GetMember(t.Context(), conversationChannelID, extraUserID); err == nil {
			t.Fatal("a refused request added the member")
		}
	})
	t.Run("add member: a client not on the allowlist", func(t *testing.T) {
		e := newBridgeEnv(t)
		e.seedPrivateChannel(false)
		e.ms.Teams.SeedMember(&model.TeamMember{TeamID: testTeamID, UserID: otherAgentID})
		w := e.do(t, http.MethodPost, "/api/v1/channels/"+conversationChannelID+"/members", "client:"+otherClientID,
			`{"user_id":"`+otherAgentID+`"}`, obo(testUserID))
		if w.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403: %s", w.Code, w.Body.String())
		}
		if _, err := e.ms.Channels.GetMember(t.Context(), conversationChannelID, otherAgentID); err == nil {
			t.Fatal("a refused request added the member")
		}
	})
}

// A machine actor reads a private channel's posts as itself when it is a
// member, and is refused when it is not.
func TestMachineActor_ReadsAPrivateChannelOnlyAsAMember(t *testing.T) {
	e := newBridgeEnv(t)
	e.seedPrivateChannel(true)
	path := "/api/v1/channels/" + conversationChannelID + "/posts"
	if w := e.do(t, http.MethodGet, path, halod, "", nil); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("member machine actor: %d %s", w.Code, w.Body.String())
	}
	if w := e.do(t, http.MethodGet, path, "client:"+otherClientID, "", nil); w.Code != http.StatusForbidden {
		t.Fatalf("non-member machine actor: %d, want 403", w.Code)
	}
}
