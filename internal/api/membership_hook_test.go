package api

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/memberhook"
	"github.com/infrashift/chit/internal/memberhook/memberhooktest"
	"github.com/infrashift/chit/internal/model"
)

// The membership hook (P-CHIT-2) through the whole router: AuthExtract,
// on-behalf-of, OpenAPI validation and the handlers.

func newHookEnv(t *testing.T, policy func(*memberhook.Request) memberhooktest.Answer) (*bridgeEnv, *memberhooktest.Server) {
	t.Helper()
	e := newBridgeEnv(t)
	srv := memberhooktest.New(t, policy)
	e.a.MembershipHook = srv.Client(t)
	return e, srv
}

func allowEveryone(*memberhook.Request) memberhooktest.Answer { return memberhooktest.Allow }

// refusing returns a policy that refuses userID with a fixed reason.
func refusing(userID string) func(*memberhook.Request) memberhooktest.Answer {
	return memberhooktest.RefuseUser(userID, "okf_access", "This person cannot read an OKF namespace used here.")
}

// requireRefusalBody checks a 403 channel.membership_refused response.
func requireRefusalBody(t *testing.T, code int, body []byte) {
	t.Helper()
	if code != http.StatusForbidden {
		t.Fatalf("got %d, want 403: %s", code, body)
	}
	var got map[string]any
	decodeJSON(t, bytes.NewReader(body), &got)
	if got["id"] != app.ChannelErrMembershipRefused || got["reason"] != "okf_access" ||
		got["detail"] != "This person cannot read an OKF namespace used here." || got["status_code"] != float64(403) {
		t.Fatalf("refusal body %s", body)
	}
}

func TestMembershipHook_AddMemberRefusedIs403(t *testing.T) {
	e, srv := newHookEnv(t, refusing(extraUserID))
	e.seedPrivateChannel(false)
	w := e.do(t, http.MethodPost, "/api/v1/channels/"+conversationChannelID+"/members", "user:"+testKratosID,
		`{"user_id":"`+extraUserID+`"}`, nil)
	requireRefusalBody(t, w.Code, w.Body.Bytes())
	if _, err := e.ms.Channels.GetMember(t.Context(), conversationChannelID, extraUserID); err == nil {
		t.Fatal("a refused member was added")
	}
	calls := srv.Calls()
	want := memberhook.Request{
		ChannelID: conversationChannelID, UserID: extraUserID, KratosIdentityID: "kratos-002",
		ActorType: "human", AddedBy: testUserID,
	}
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("hook calls %+v, want [%+v]", calls, want)
	}
}

// HALO's Conversation flow: creating the private channel asks nobody (the
// person is its creator); adding halod to it on the person's behalf asks
// about halod, added by the person, via halod.
func TestMembershipHook_OnBehalfOfConversation(t *testing.T) {
	e, srv := newHookEnv(t, allowEveryone)
	e.ms.Teams.SeedMember(&model.TeamMember{TeamID: testTeamID, UserID: haloAgentID})

	w := e.do(t, http.MethodPost, "/api/v1/channels", halod,
		`{"team_id":"`+testTeamID+`","name":"conversation-1","display_name":"Conversation","type":"P"}`, obo(testUserID))
	if w.Code != http.StatusCreated {
		t.Fatalf("create channel: %d %s", w.Code, w.Body.String())
	}
	var ch model.Channel
	decodeJSON(t, w.Body, &ch)
	if n := len(srv.Calls()); n != 0 {
		t.Fatalf("hook called %d times for the creator", n)
	}

	w = e.do(t, http.MethodPost, "/api/v1/channels/"+ch.ID+"/members", halod, `{"user_id":"`+haloAgentID+`"}`, obo(testUserID))
	if w.Code != http.StatusCreated {
		t.Fatalf("add halod: %d %s", w.Code, w.Body.String())
	}
	want := memberhook.Request{
		ChannelID: ch.ID, UserID: haloAgentID, KratosIdentityID: "", ActorType: "agent",
		AddedBy: testUserID, ViaActorID: haloAgentID,
	}
	if calls := srv.Calls(); len(calls) != 1 || calls[0] != want {
		t.Fatalf("hook calls %+v, want [%+v]", calls, want)
	}

	// The person adds somebody the hook refuses, on behalf of: 403, with
	// the client named in the call.
	srv.SetPolicy(refusing(extraUserID))
	w = e.do(t, http.MethodPost, "/api/v1/channels/"+ch.ID+"/members", halod, `{"user_id":"`+extraUserID+`"}`, obo(testUserID))
	requireRefusalBody(t, w.Code, w.Body.Bytes())
	if calls := srv.Calls(); len(calls) != 2 || calls[1].ViaActorID != haloAgentID || calls[1].AddedBy != testUserID {
		t.Fatalf("hook calls %+v", calls)
	}
	// Only the allowed add is audited as channel_member_added.
	if recs := e.auditRecords(t, "channel_member_added"); len(recs) != 1 {
		t.Fatalf("%d channel_member_added records, want 1", len(recs))
	}
}

func TestMembershipHook_CreateOpenChannelRefusedIs403(t *testing.T) {
	e, _ := newHookEnv(t, refusing(extraUserID))
	w := e.do(t, http.MethodPost, "/api/v1/channels", "user:"+testKratosID,
		`{"team_id":"`+testTeamID+`","name":"open-one","display_name":"Open","type":"O"}`, nil)
	requireRefusalBody(t, w.Code, w.Body.Bytes())
	if _, err := e.ms.Channels.GetByName(t.Context(), testTeamID, "open-one"); err == nil {
		t.Fatal("the channel was created")
	}
}

func TestMembershipHook_DirectAndGroupRefusedIs403(t *testing.T) {
	e, _ := newHookEnv(t, refusing(extraUserID))
	w := e.do(t, http.MethodPost, "/api/v1/channels/direct", "user:"+testKratosID,
		`["`+testUserID+`","`+extraUserID+`"]`, nil)
	requireRefusalBody(t, w.Code, w.Body.Bytes())

	w = e.do(t, http.MethodPost, "/api/v1/channels/group", "user:"+testKratosID,
		`["`+testUserID+`","`+extraUserID+`","`+thirdUserID+`"]`, nil)
	requireRefusalBody(t, w.Code, w.Body.Bytes())

	if dms, _ := e.ms.Channels.GetDirectChannelsForUser(t.Context(), testUserID); len(dms) != 0 {
		t.Fatalf("channels %v after refusals", dms)
	}
}
