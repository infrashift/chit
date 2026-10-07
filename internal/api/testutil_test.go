package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store/storetest"
	"github.com/infrashift/chit/internal/websocket"
)

// ─── Pre-seeded test IDs ─────────────────────────────────────────

const (
	testUserID    = "019421a0-0000-7000-8000-000000000001"
	testKratosID  = "kratos-001"
	testTeamID    = "019421a0-0000-7000-8000-000000000010"
	testChannelID = "019421a0-0000-7000-8000-000000000020"
	testRootPost  = "019421a0-0000-7000-8000-000000000030"
	testReplyPost = "019421a0-0000-7000-8000-000000000031"
	testTagID     = "019421a0-0000-7000-8000-000000000040"
	extraUserID   = "019421a0-0000-7000-8000-000000000002"
	thirdUserID   = "019421a0-0000-7000-8000-000000000003"
	adminUserID   = "019421a0-0000-7000-8000-000000000004"
)

// ─── noopPubSub ──────────────────────────────────────────────────

type noopPubSub struct{}

func (noopPubSub) Publish(_ context.Context, _ string, _ []byte) error              { return nil }
func (noopPubSub) Subscribe(_ context.Context, _ string, _ func(data []byte)) error { return nil }
func (noopPubSub) Close() error                                                     { return nil }

// mockStore is the shared in-memory store; see package storetest.
type mockStore = storetest.Store

// ─── Setup helper ────────────────────────────────────────────────

func setupTestApp(t *testing.T) (a *app.App, ms *mockStore, cleanup func()) {
	t.Helper()

	ms = storetest.New()

	ms.Users.Seed(&model.User{
		ID:          testUserID,
		KratosID:    testKratosID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Roles:       "system_user",
		ActorType:   model.ActorTypeUser,
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.Users.Seed(&model.User{
		ID:          extraUserID,
		KratosID:    "kratos-002",
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       "system_user",
		ActorType:   model.ActorTypeUser,
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.Users.Seed(&model.User{
		ID:          thirdUserID,
		KratosID:    "kratos-003",
		Username:    "charlie",
		DisplayName: "Charlie",
		Email:       "charlie@example.com",
		Roles:       "system_user",
		ActorType:   model.ActorTypeUser,
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	// The only seeded system_admin. Roles are a space-separated string, and
	// IsSystemAdmin splits on it, so "system_user system_admin" is what a real
	// admin row looks like rather than the bare role on its own.
	ms.Users.Seed(&model.User{
		ID:          adminUserID,
		KratosID:    "kratos-004",
		Username:    "dana",
		DisplayName: "Dana Admin",
		Email:       "dana@example.com",
		Roles:       "system_user system_admin",
		ActorType:   model.ActorTypeUser,
		CreateAt:    1000,
		UpdateAt:    1000,
	})

	ms.Teams.Seed(&model.Team{
		ID:          testTeamID,
		Name:        "engineering",
		DisplayName: "Engineering",
		Type:        "O",
		CreatorID:   testUserID,
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	// testUser created the team, so holds team_admin, as CreateTeam grants.
	ms.Teams.SeedMember(&model.TeamMember{
		TeamID: testTeamID,
		UserID: testUserID,
		Roles:  "team_admin team_user",
	})
	ms.Teams.SeedMember(&model.TeamMember{
		TeamID: testTeamID,
		UserID: extraUserID,
	})
	ms.Teams.SeedMember(&model.TeamMember{
		TeamID: testTeamID,
		UserID: adminUserID,
	})

	ms.Channels.Seed(&model.Channel{
		ID:          testChannelID,
		TeamID:      testTeamID,
		CreatorID:   testUserID,
		Name:        "general",
		DisplayName: "General",
		Type:        "O",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.Channels.SeedMember(&model.ChannelMember{
		ChannelID: testChannelID,
		UserID:    testUserID,
		CreateAt:  1000,
	})

	ms.Posts.Seed(&model.Post{
		ID:        testRootPost,
		ChannelID: testChannelID,
		UserID:    testUserID,
		Content:   "Hello world",
		CreateAt:  2000,
		UpdateAt:  2000,
	})
	ms.Posts.Seed(&model.Post{
		ID:        testReplyPost,
		ChannelID: testChannelID,
		UserID:    extraUserID,
		RootID:    testRootPost,
		Content:   "Hi there",
		CreateAt:  3000,
		UpdateAt:  3000,
	})

	ms.Threads.Seed(&model.Thread{
		PostID:      testRootPost,
		ChannelID:   testChannelID,
		ReplyCount:  1,
		LastReplyAt: 3000,
	})
	ms.Threads.SeedMembership(&model.ThreadMembership{
		PostID:    testRootPost,
		UserID:    testUserID,
		Following: true,
	})

	ms.Tags.Seed(&model.Tag{ID: testTagID, Name: "important"})
	ms.Tags.SeedPostTag(testRootPost, testTagID)

	cfg := config.Defaults()
	cfg.TrustedProxyHeader = "X-User-Id"
	hub := websocket.NewHub(nil)
	a = app.New(ms, hub, noopPubSub{}, cfg)

	cleanup = func() {
		hub.Stop()
	}

	return a, ms, cleanup
}

// authedRequest wraps the given request with the test user in context.
func authedRequest(r *http.Request, user *model.User) *http.Request {
	return ContextSetUser(r, user)
}

// testUser returns the pre-seeded test user.
func testUser() *model.User {
	return &model.User{
		ID:          testUserID,
		KratosID:    testKratosID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Roles:       "system_user",
		ActorType:   model.ActorTypeUser,
		CreateAt:    1000,
		UpdateAt:    1000,
	}
}

// decodeJSON reads and unmarshals a response body.
func decodeJSON(t *testing.T, body io.Reader, v any) {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal body (%s): %v", string(data), err)
	}
}
