package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// privateChannelID is a private channel on testTeamID whose only member is
// testUser. alice is on the team but not in the channel.
const privateChannelID = "019421a0-0000-7000-8000-000000000021"

func seedPrivateChannel(ms *mockStore) {
	ms.channel.seed(&model.Channel{
		ID: privateChannelID, TeamID: testTeamID, CreatorID: testUserID,
		Name: "secret", DisplayName: "Secret", Header: "the plan",
		Type: model.ChannelPrivate, CreateAt: 1000, UpdateAt: 1000,
	})
	ms.channel.seedMember(&model.ChannelMember{ChannelID: privateChannelID, UserID: testUserID})
}

// serveChannelRoute runs handler for a /channels/{id} route as user, with an
// optional JSON body.
func serveChannelRoute(t *testing.T, handler http.HandlerFunc, method, channelID, body string, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), method, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", channelID)
	r = authedRequest(r, user)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

// getChannel had no check at all, so any caller could read a private
// channel's header and purpose, or a DM's name, which is its two members' IDs.
func TestGetChannel_Access(t *testing.T) {
	cases := []struct {
		name      string
		channelID string
		user      *model.User
		want      int
	}{
		{"private channel, member", privateChannelID, testUser(), http.StatusOK},
		{"private channel, team member outside it", privateChannelID, aliceUser(), http.StatusForbidden},
		{"private channel, stranger", privateChannelID, charlieUser(), http.StatusForbidden},
		{"open channel, team member outside it", testChannelID, aliceUser(), http.StatusOK},
		{"open channel, not on the team", testChannelID, charlieUser(), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ms, cleanup := setupTestApp(t)
			defer cleanup()
			seedPrivateChannel(ms)

			w := serveChannelRoute(t, getChannel(a), http.MethodGet, tc.channelID, "", tc.user)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// POST /channels accepted type D with no team, so a caller could create the
// channel named "<alice>__<bob>" with only themselves in it, add the pair, and
// then receive every DM the two later "started", because CreateDirectChannel
// returns an existing channel by that name.
func TestCreateChannel_RejectsDirectAndTeamless(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"direct", `{"type":"D","name":"` + testUserID + `__` + extraUserID + `","display_name":"x"}`},
		{"group", `{"type":"G","name":"a__b__c","display_name":"x"}`},
		{"no team", `{"type":"O","name":"lobby","display_name":"Lobby"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, cleanup := setupTestApp(t)
			defer cleanup()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels", strings.NewReader(tc.body))
			r = authedRequest(r, charlieUser())
			w := httptest.NewRecorder()
			createChannel(a).ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateChannel_IgnoresServerOwnedFields(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	const forgedID = "019421a0-0000-7000-8000-0000000000ee"
	body := `{"id":"` + forgedID + `","team_id":"` + testTeamID + `","type":"O","name":"lobby","display_name":"Lobby","delete_at":5}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels", strings.NewReader(body))
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()
	createChannel(a).ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var ch model.Channel
	decodeJSON(t, w.Body, &ch)
	if ch.ID == forgedID || ch.DeleteAt != 0 {
		t.Fatalf("client-supplied id/delete_at were honored: %+v", ch)
	}
}

func TestUpdateChannel_Patch(t *testing.T) {
	t.Run("rename is rejected", func(t *testing.T) {
		a, _, cleanup := setupTestApp(t)
		defer cleanup()
		w := serveChannelRoute(t, updateChannel(a), http.MethodPut, testChannelID, `{"name":"renamed"}`, testUser())
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
		}
	})
	t.Run("an empty header clears it", func(t *testing.T) {
		a, ms, cleanup := setupTestApp(t)
		defer cleanup()
		seedPrivateChannel(ms)
		w := serveChannelRoute(t, updateChannel(a), http.MethodPut, privateChannelID, `{"header":""}`, testUser())
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
		}
		if ch, _ := ms.channel.Get(t.Context(), privateChannelID); ch.Header != "" {
			t.Fatalf("header = %q, want cleared", ch.Header)
		}
	})
	t.Run("an invalid display name is rejected", func(t *testing.T) {
		a, _, cleanup := setupTestApp(t)
		defer cleanup()
		w := serveChannelRoute(t, updateChannel(a), http.MethodPut, testChannelID, `{"display_name":""}`, testUser())
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
		}
	})
}

func TestAddChannelMember_Rules(t *testing.T) {
	t.Run("a DM cannot gain a third member", func(t *testing.T) {
		a, _, cleanup := setupTestApp(t)
		defer cleanup()
		dm, err := a.CreateDirectChannel(t.Context(), testUserID, testUserID, extraUserID)
		if err != nil {
			t.Fatalf("CreateDirectChannel: %v", err)
		}
		w := serveChannelRoute(t, addChannelMember(a), http.MethodPost, dm.ID, `{"user_id":"`+thirdUserID+`"}`, testUser())
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
		}
	})
	t.Run("the user added must be on the team", func(t *testing.T) {
		a, _, cleanup := setupTestApp(t)
		defer cleanup()
		w := serveChannelRoute(t, addChannelMember(a), http.MethodPost, testChannelID, `{"user_id":"`+thirdUserID+`"}`, testUser())
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
		}
	})
}

func TestCreateDirectChannel_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"self", `["` + testUserID + `","` + testUserID + `"]`},
		{"unknown user", `["` + testUserID + `","019421a0-0000-7000-8000-0000000000ff"]`},
		{"not a uuid", `["` + testUserID + `","bob"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, cleanup := setupTestApp(t)
			defer cleanup()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tc.body))
			r = authedRequest(r, testUser())
			w := httptest.NewRecorder()
			createDirectChannel(a).ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// A repeated ID passed the 3-8 size check and then failed the member insert
// on its primary key, as a 500.
func TestCreateGroupChannel_DuplicateIDs(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	body := `["` + testUserID + `","` + testUserID + `","` + extraUserID + `"]`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()
	createGroupChannel(a).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for two distinct members, got %d; body: %s", w.Code, w.Body.String())
	}
}
