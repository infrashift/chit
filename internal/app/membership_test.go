package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/memberhook"
	"github.com/infrashift/chit/internal/memberhook/memberhooktest"
	"github.com/infrashift/chit/internal/model"
)

// hookFixture is a fixture with the membership hook on: an httptest hook
// that answers with policy, and an audit log in a file.
type hookFixture struct {
	*fixture
	hook  *memberhooktest.Server
	audit string
}

func newHookFixture(t *testing.T, policy func(*memberhook.Request) memberhooktest.Answer) *hookFixture {
	t.Helper()
	f := newFixture(t)
	srv := memberhooktest.New(t, policy)
	f.app.MembershipHook = srv.Client(t)
	audit := filepath.Join(t.TempDir(), "audit.log")
	al, err := command.NewAuditLogger(audit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := al.Close(); err != nil {
			t.Errorf("close audit logger: %v", err)
		}
	})
	f.app.AuditLogger = al
	return &hookFixture{fixture: f, hook: srv, audit: audit}
}

func allowAll(*memberhook.Request) memberhooktest.Answer { return memberhooktest.Allow() }

// refuse returns a policy that refuses the named user of f.
func (f *hookFixture) refuse(name string) {
	f.hook.SetPolicy(memberhooktest.RefuseUser(f.user(name).ID, "tool_access", name+" cannot use a Tool in this channel"))
}

// calledFor returns the names of the users the hook was asked about, sorted.
func (f *hookFixture) calledFor() []string {
	byID := map[string]string{}
	for name, u := range f.users {
		byID[u.ID] = name
	}
	var names []string
	for _, id := range f.hook.CalledFor() {
		names = append(names, byID[id])
	}
	sort.Strings(names)
	return names
}

// auditRecords returns the membership_check records of the audit log.
func (f *hookFixture) auditRecords() []map[string]any {
	f.t.Helper()
	file, err := os.Open(f.audit)
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			f.t.Errorf("close audit log: %v", err)
		}
	}()
	var out []map[string]any
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		var rec map[string]any
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			f.t.Fatalf("audit line %q: %v", sc.Text(), err)
		}
		if rec["msg"] == "membership_check" {
			out = append(out, rec)
		}
	}
	return out
}

func (f *hookFixture) isMember(channelID, name string) bool {
	_, err := f.store.Channels.GetMember(f.t.Context(), channelID, f.user(name).ID)
	return err == nil
}

// requireRefusal fails unless err is the hook's 403 refusal with reason. It
// returns the refusal's detail.
func requireRefusal(t *testing.T, err error, reason string) string {
	t.Helper()
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != http.StatusForbidden ||
		appErr.ID != ChannelErrMembershipRefused || appErr.Reason != reason {
		t.Fatalf("err = %#v, want 403 %s with reason %q", err, ChannelErrMembershipRefused, reason)
	}
	return appErr.Detail
}

// ─── POST /channels/{id}/members (AddChannelMember) ─────────────────────

func TestMembershipHook_AddMember_AllowedAndAudited(t *testing.T) {
	f := newHookFixture(t, allowAll)
	private := f.privateChannel("secret")
	f.join(private, "alice")
	f.joinTeam("bob")

	if _, err := f.app.AddChannelMember(t.Context(), private.ID, f.user("bob").ID, f.user("alice").ID); err != nil {
		t.Fatal(err)
	}
	if !f.isMember(private.ID, "bob") {
		t.Fatal("bob was not added")
	}
	want := memberhook.Request{
		ChannelID: private.ID, UserID: f.user("bob").ID, KratosIdentityID: f.user("bob").KratosID,
		ActorType: memberhook.ActorHuman, AddedBy: f.user("alice").ID,
	}
	if calls := f.hook.Calls(); len(calls) != 1 || calls[0] != want {
		t.Fatalf("hook calls %+v, want [%+v]", calls, want)
	}
	recs := f.auditRecords()
	if len(recs) != 1 || recs[0]["allowed"] != true || recs[0]["channel_id"] != private.ID ||
		recs[0]["user_id"] != f.user("bob").ID || recs[0]["added_by"] != f.user("alice").ID ||
		recs[0]["member_actor_type"] != "human" {
		t.Fatalf("audit = %v", recs)
	}
}

func TestMembershipHook_AddMember_RefusedAddsNothing(t *testing.T) {
	f := newHookFixture(t, allowAll)
	private := f.privateChannel("secret")
	f.join(private, "alice")
	f.joinTeam("bob")
	f.refuse("bob")

	_, err := f.app.AddChannelMember(t.Context(), private.ID, f.user("bob").ID, f.user("alice").ID)
	if detail := requireRefusal(t, err, "tool_access"); detail != "bob cannot use a Tool in this channel" {
		t.Fatalf("detail %q", detail)
	}
	if f.isMember(private.ID, "bob") {
		t.Fatal("a refused member was added")
	}
	f.app.WaitBackground()
	if f.keto.writes != 0 {
		t.Fatalf("keto writes = %d for a refused add", f.keto.writes)
	}
	recs := f.auditRecords()
	if len(recs) != 1 || recs[0]["allowed"] != false || recs[0]["reason"] != "tool_access" {
		t.Fatalf("audit = %v", recs)
	}
}

// Joining an open channel is an add like any other.
func TestMembershipHook_JoinOpenChannel(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("bob")
	f.refuse("bob")
	_, err := f.app.AddChannelMember(t.Context(), f.channel.ID, f.user("bob").ID, f.user("bob").ID)
	requireRefusal(t, err, "tool_access")
	if f.isMember(f.channel.ID, "bob") {
		t.Fatal("bob joined although refused")
	}

	f.hook.SetPolicy(allowAll)
	if _, err := f.app.AddChannelMember(t.Context(), f.channel.ID, f.user("bob").ID, f.user("bob").ID); err != nil {
		t.Fatal(err)
	}
	calls := f.hook.Calls()
	if len(calls) != 2 || calls[1].AddedBy != f.user("bob").ID || calls[1].UserID != f.user("bob").ID {
		t.Fatalf("hook calls %+v", calls)
	}
}

// Re-adding a member adds nobody, so nobody is asked about.
func TestMembershipHook_ReAddIsNotAsked(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.join(f.channel, "alice", "bob")
	if _, err := f.app.AddChannelMember(t.Context(), f.channel.ID, f.user("bob").ID, f.user("alice").ID); err != nil {
		t.Fatal(err)
	}
	if n := len(f.hook.Calls()); n != 0 {
		t.Fatalf("hook called %d times for an existing member", n)
	}
}

// An add that fails its own checks never reaches the hook.
func TestMembershipHook_UnauthorizedAddIsNotAsked(t *testing.T) {
	f := newHookFixture(t, allowAll)
	private := f.privateChannel("secret")
	f.joinTeam("mallory", "bob")
	if _, err := f.app.AddChannelMember(t.Context(), private.ID, f.user("bob").ID, f.user("mallory").ID); err == nil {
		t.Fatal("a non-member added somebody to a private channel")
	}
	if n := len(f.hook.Calls()); n != 0 {
		t.Fatalf("hook called %d times", n)
	}
}

// On behalf of a person: added_by is the person, via_actor_id the client.
// A machine actor is an agent with no Kratos identity.
func TestMembershipHook_OnBehalfOfAddsAMachineActor(t *testing.T) {
	f := newHookFixture(t, allowAll)
	private := f.privateChannel("halo-conv")
	f.join(private, "erin")
	halod := &model.User{
		ID: model.NewID(), Username: "halod", DisplayName: "halod", Roles: "system_user",
		ActorType: model.ActorTypeAgent, OAuthClientID: "halod", CreateAt: 1, UpdateAt: 1,
	}
	f.store.Users.Seed(halod)
	f.store.Teams.SeedMember(&model.TeamMember{TeamID: f.team.ID, UserID: halod.ID, Roles: model.TeamRoleUser})

	ctx := WithViaActor(t.Context(), halod.ID)
	if _, err := f.app.AddChannelMember(ctx, private.ID, halod.ID, f.user("erin").ID); err != nil {
		t.Fatal(err)
	}
	want := memberhook.Request{
		ChannelID: private.ID, UserID: halod.ID, KratosIdentityID: "", ActorType: memberhook.ActorAgent,
		AddedBy: f.user("erin").ID, ViaActorID: halod.ID,
	}
	if calls := f.hook.Calls(); len(calls) != 1 || calls[0] != want {
		t.Fatalf("hook calls %+v, want [%+v]", calls, want)
	}
	if recs := f.auditRecords(); len(recs) != 1 || recs[0]["via_actor_id"] != halod.ID {
		t.Fatalf("audit = %v", recs)
	}
}

func TestHookActorType(t *testing.T) {
	for in, want := range map[string]string{
		model.ActorTypeUser: "human", model.ActorTypeAgent: "agent", model.ActorTypeBot: "bot", "": "human",
	} {
		if got := hookActorType(in); got != want {
			t.Errorf("hookActorType(%q) = %q, want %q", in, got, want)
		}
	}
}

// ─── fail closed ────────────────────────────────────────────────────────

func TestMembershipHook_FailsClosed(t *testing.T) {
	cases := map[string]memberhooktest.Answer{
		"500":          {Status: http.StatusInternalServerError, Body: `{"allow":true}`},
		"invalid JSON": {Status: http.StatusOK, Body: `{"allow":`},
		"no allow":     {Status: http.StatusOK, Body: `{}`},
	}
	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			f := newHookFixture(t, func(*memberhook.Request) memberhooktest.Answer { return answer })
			f.joinTeam("bob")
			_, err := f.app.AddChannelMember(t.Context(), f.channel.ID, f.user("bob").ID, f.user("bob").ID)
			requireRefusal(t, err, ReasonHookUnavailable)
			if f.isMember(f.channel.ID, "bob") {
				t.Fatal("added although the hook failed")
			}
			if recs := f.auditRecords(); len(recs) != 1 || recs[0]["allowed"] != false || recs[0]["reason"] != ReasonHookUnavailable {
				t.Fatalf("audit = %v", recs)
			}
		})
	}
}

func TestMembershipHook_TimeoutFailsClosed(t *testing.T) {
	release := make(chan struct{})
	f := newHookFixture(t, func(*memberhook.Request) memberhooktest.Answer {
		<-release
		return memberhooktest.Allow()
	})
	t.Cleanup(func() { close(release) })
	hook, err := memberhook.New(&memberhook.Config{
		URL: f.hook.Hook.URL, Timeout: 200 * time.Millisecond, TokenURL: f.hook.Token.URL,
		ClientID: memberhooktest.ClientID, ClientSecret: memberhooktest.ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.app.MembershipHook = hook
	f.joinTeam("bob")
	_, err = f.app.AddChannelMember(t.Context(), f.channel.ID, f.user("bob").ID, f.user("bob").ID)
	requireRefusal(t, err, ReasonHookUnavailable)
	if f.isMember(f.channel.ID, "bob") {
		t.Fatal("added although the hook timed out")
	}
}

// ─── channel creation ───────────────────────────────────────────────────

// The creator of a new channel is the one member added without asking: a
// private channel asks nobody.
func TestMembershipHook_CreatePrivateChannelAsksNobody(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("creator", "alice")
	created, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "priv-chan", DisplayName: "P", Type: model.ChannelPrivate, CreatorID: f.user("creator").ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(f.hook.Calls()); n != 0 {
		t.Fatalf("hook called %d times for the creator", n)
	}
	if got := channelMemberNames(t, f.fixture, created.ID); !slices.Equal(got, []string{"creator"}) {
		t.Fatalf("members %v", got)
	}
}

// An open channel adds its team: each member but the creator is asked
// about, once, under the new channel's ID.
func TestMembershipHook_CreateOpenChannelAsksForEachTeamMember(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("creator", "alice", "bob")
	created, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "open-chan", DisplayName: "O", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.calledFor(); !slices.Equal(got, []string{"alice", "bob"}) {
		t.Fatalf("hook asked about %v, want alice and bob", got)
	}
	for _, c := range f.hook.Calls() {
		if c.ChannelID != created.ID || c.AddedBy != f.user("creator").ID {
			t.Fatalf("call %+v: want channel %s, added by the creator", c, created.ID)
		}
	}
	if got := channelMemberNames(t, f.fixture, created.ID); !slices.Equal(got, []string{"alice", "bob", "creator"}) {
		t.Fatalf("members %v", got)
	}
}

// One refusal refuses the channel: nothing is created, nobody added.
func TestMembershipHook_CreateOpenChannelRefusedCreatesNothing(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("creator", "alice", "bob")
	f.refuse("bob")
	_, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "open-chan", DisplayName: "O", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	requireRefusal(t, err, "tool_access")
	if _, err := f.store.Channels.GetByName(t.Context(), f.team.ID, "open-chan"); err == nil {
		t.Fatal("the channel was created")
	}
	if slices.Contains(f.calledFor(), "creator") {
		t.Fatal("the creator was asked about")
	}
}

// Only the members the hook allowed are added: one who joins the team while
// the hook is asked is not.
func TestMembershipHook_CreateOpenChannelAddsOnlyCheckedMembers(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("creator", "alice")
	late := f.user("late")
	f.hook.SetPolicy(func(*memberhook.Request) memberhooktest.Answer {
		f.store.Teams.SeedMember(&model.TeamMember{TeamID: f.team.ID, UserID: late.ID, Roles: model.TeamRoleUser})
		return memberhooktest.Allow()
	})
	created, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "open-chan", DisplayName: "O", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := channelMemberNames(t, f.fixture, created.ID); !slices.Equal(got, []string{"alice", "creator"}) {
		t.Fatalf("members %v; the late team member was never asked about", got)
	}
}

// A member the hook allowed but who left the team before the insert is not
// added: the add is still limited to the team.
func TestMembershipHook_CreateOpenChannelSkipsAMemberWhoLeft(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("creator", "alice", "leaver")
	leaver := f.user("leaver")
	f.hook.SetPolicy(func(*memberhook.Request) memberhooktest.Answer {
		// The hook server's goroutine: t.Errorf, not t.Fatal.
		if err := f.store.Teams.RemoveMember(context.Background(), f.team.ID, leaver.ID); err != nil {
			t.Errorf("remove the leaver from the team: %v", err)
		}
		return memberhooktest.Allow()
	})
	created, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "open-chan", DisplayName: "O", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := channelMemberNames(t, f.fixture, created.ID); !slices.Equal(got, []string{"alice", "creator"}) {
		t.Fatalf("members %v; the member who left the team was added", got)
	}
}

// A large team: one call for each member, and one refusal adds nobody.
func TestMembershipHook_BatchIsWholeOrNothing(t *testing.T) {
	f := newHookFixture(t, allowAll)
	names := []string{"creator"}
	for i := range 30 {
		names = append(names, fmt.Sprintf("m%02d", i))
	}
	f.joinTeam(names...)
	f.refuse("m17")
	_, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "big", DisplayName: "Big", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	requireRefusal(t, err, "tool_access")
	if _, lookupErr := f.store.Channels.GetByName(t.Context(), f.team.ID, "big"); lookupErr == nil {
		t.Fatal("the channel was created")
	}

	// A new hook: requests the first batch abandoned may still reach the
	// old one after the refusal.
	fresh := memberhooktest.New(t, allowAll)
	f.app.MembershipHook = fresh.Client(t)
	created, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "big", DisplayName: "Big", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	asked := fresh.CalledFor()
	n := len(asked)
	slices.Sort(asked)
	if n != 30 || len(slices.Compact(asked)) != 30 {
		t.Fatalf("asked %d times about %d users, want once about each of 30", n, len(slices.Compact(asked)))
	}
	if got := channelMemberNames(t, f.fixture, created.ID); len(got) != 31 {
		t.Fatalf("%d members, want 31", len(got))
	}
}

// When one member of a batch is refused, the checks still in flight are
// abandoned: they are not answers, so they are not audited.
func TestMembershipHook_AbandonedChecksAreNotAudited(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.joinTeam("creator", "bob", "carol", "dave")
	carol := f.user("carol").ID
	refused := make(chan struct{})
	f.hook.SetPolicy(func(r *memberhook.Request) memberhooktest.Answer {
		if r.UserID == carol {
			defer close(refused)
			return memberhooktest.Refuse("tool_access", "")
		}
		// Bob and Dave answer only after Carol's refusal, by which time
		// Chit has given up on them.
		select {
		case <-refused:
		case <-time.After(5 * time.Second):
		}
		time.Sleep(200 * time.Millisecond)
		return memberhooktest.Allow()
	})
	_, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "open-chan", DisplayName: "O", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	requireRefusal(t, err, "tool_access")
	recs := f.auditRecords()
	if len(recs) != 1 || recs[0]["user_id"] != carol || recs[0]["allowed"] != false {
		t.Fatalf("audit = %v, want only Carol's refusal", recs)
	}
}

// ─── direct and group channels ──────────────────────────────────────────

func TestMembershipHook_DirectChannel(t *testing.T) {
	f := newHookFixture(t, allowAll)
	alice, bob := f.user("alice"), f.user("bob")
	f.refuse("bob")
	if _, err := f.app.CreateDirectChannel(t.Context(), alice.ID, alice.ID, bob.ID); err == nil {
		t.Fatal("a refused DM was created")
	} else {
		requireRefusal(t, err, "tool_access")
	}
	if _, err := f.store.Channels.GetDirectChannelsForUser(t.Context(), alice.ID); err != nil {
		t.Fatal(err)
	}
	dms, err := f.store.Channels.GetDirectChannelsForUser(t.Context(), alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(dms) != 0 {
		t.Fatalf("DMs %v after a refusal", dms)
	}

	f.hook.SetPolicy(allowAll)
	dm, err := f.app.CreateDirectChannel(t.Context(), alice.ID, alice.ID, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	calls := f.hook.Calls()
	last := calls[len(calls)-1]
	if last.UserID != bob.ID || last.AddedBy != alice.ID || last.ChannelID != dm.ID {
		t.Fatalf("call %+v; want bob, added by alice, in %s", last, dm.ID)
	}
	if slices.Contains(f.hook.CalledFor(), alice.ID) {
		t.Fatal("the creator of the DM was asked about")
	}

	// The DM exists now: opening it again adds nobody.
	n := len(f.hook.Calls())
	if _, err := f.app.CreateDirectChannel(t.Context(), bob.ID, alice.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.hook.Calls()) != n {
		t.Fatal("the hook was asked about an existing DM")
	}
}

func TestMembershipHook_GroupChannelIsWholeOrNothing(t *testing.T) {
	f := newHookFixture(t, allowAll)
	ids := []string{f.user("alice").ID, f.user("bob").ID, f.user("carol").ID}
	f.refuse("carol")
	_, err := f.app.CreateGroupChannel(t.Context(), f.user("alice").ID, ids)
	requireRefusal(t, err, "tool_access")
	gms, err := f.store.Channels.GetDirectChannelsForUser(t.Context(), f.user("alice").ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gms) != 0 {
		t.Fatalf("group channels %v after a refusal", gms)
	}

	f.hook.SetPolicy(allowAll)
	before := len(f.hook.Calls())
	gm, err := f.app.CreateGroupChannel(t.Context(), f.user("alice").ID, ids)
	if err != nil {
		t.Fatal(err)
	}
	asked := f.hook.CalledFor()[before:]
	slices.Sort(asked)
	want := []string{f.user("bob").ID, f.user("carol").ID}
	slices.Sort(want)
	if !slices.Equal(asked, want) {
		t.Fatalf("asked about %v, want bob and carol", asked)
	}
	if got := channelMemberNames(t, f.fixture, gm.ID); !slices.Equal(got, []string{"alice", "bob", "carol"}) {
		t.Fatalf("members %v", got)
	}
}

// ─── /invite ────────────────────────────────────────────────────────────

func TestMembershipHook_InviteCommand(t *testing.T) {
	f := newHookFixture(t, allowAll)
	f.join(f.channel, "alice")
	f.joinTeam("bob")
	f.refuse("bob")
	res, err := f.app.HandleInvite(t.Context(), f.user("alice").ID, f.channel.ID, "@bob")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.ResponseText, "tool_access") || !strings.Contains(res.ResponseText, "bob cannot use a Tool") ||
		res.Triggered {
		t.Fatalf("response %+v", res)
	}
	if f.isMember(f.channel.ID, "bob") {
		t.Fatal("/invite added a refused member")
	}

	f.hook.SetPolicy(allowAll)
	if res, err = f.app.HandleInvite(t.Context(), f.user("alice").ID, f.channel.ID, "bob"); err != nil || !res.Triggered {
		t.Fatalf("HandleInvite = %+v, %v", res, err)
	}
	if !f.isMember(f.channel.ID, "bob") || len(f.hook.Calls()) != 2 {
		t.Fatalf("bob member %v, calls %d", f.isMember(f.channel.ID, "bob"), len(f.hook.Calls()))
	}
}

// ─── hook off ───────────────────────────────────────────────────────────

// With no hook every path adds as before, and no hook is asked.
func TestMembershipHook_OffAddsAsBefore(t *testing.T) {
	f := newFixture(t)
	srv := memberhooktest.New(t, func(*memberhook.Request) memberhooktest.Answer {
		return memberhooktest.Refuse("never", "")
	})
	f.joinTeam("creator", "alice", "bob")
	open, err := f.app.CreateChannel(t.Context(), &model.Channel{
		TeamID: f.team.ID, Name: "open-chan", DisplayName: "O", Type: model.ChannelOpen, CreatorID: f.user("creator").ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := channelMemberNames(t, f, open.ID); !slices.Equal(got, []string{"alice", "bob", "creator"}) {
		t.Fatalf("members %v", got)
	}
	if _, err := f.app.AddChannelMember(t.Context(), f.channel.ID, f.user("bob").ID, f.user("bob").ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.CreateDirectChannel(t.Context(), f.user("alice").ID, f.user("alice").ID, f.user("bob").ID); err != nil {
		t.Fatal(err)
	}
	if n := len(srv.Calls()); n != 0 {
		t.Fatalf("hook called %d times while off", n)
	}
}
