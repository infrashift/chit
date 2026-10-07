//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

func TestGetMe(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	u, err := client.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("Username = %q, want %q", u.Username, "alice")
	}
	if u.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", u.Email, "alice@example.com")
	}
}

func TestGetMyTeams(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	teams, err := client.GetMyTeams(ctx)
	if err != nil {
		t.Fatalf("GetMyTeams: %v", err)
	}
	if len(teams) < 1 {
		t.Fatal("expected at least 1 team")
	}

	var found bool
	for _, tm := range teams {
		if tm.Name == "uat-team" {
			found = true
			break
		}
	}
	if !found {
		t.Error("uat-team not found in teams")
	}
}

func TestGetMyChannels(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	teams, err := client.GetMyTeams(ctx)
	if err != nil {
		t.Fatalf("GetMyTeams: %v", err)
	}

	var teamID string
	for _, tm := range teams {
		if tm.Name == "uat-team" {
			teamID = tm.ID
			break
		}
	}
	if teamID == "" {
		t.Fatal("uat-team not found")
	}

	channels, err := client.GetMyChannels(ctx, teamID)
	if err != nil {
		t.Fatalf("GetMyChannels: %v", err)
	}

	var found bool
	for _, ch := range channels {
		if ch.Name == "town-square" {
			found = true
			break
		}
	}
	if !found {
		t.Error("town-square not found in channels")
	}
}

func TestCreateAndGetPost(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	channelID := mustGetTownSquareID(t, client)

	created, err := client.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		Content:   "e2e test post " + fmt.Sprint(time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created post has no ID")
	}

	got, err := client.GetPost(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if got.Content != created.Content {
		t.Errorf("Content = %q, want %q", got.Content, created.Content)
	}
}

func TestGetChannelPosts(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	channelID := mustGetTownSquareID(t, client)

	_, err := client.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		Content:   "channel posts test " + fmt.Sprint(time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	pl, err := client.GetChannelPosts(ctx, channelID, 0, 60)
	if err != nil {
		t.Fatalf("GetChannelPosts: %v", err)
	}
	if len(pl.Order) < 1 {
		t.Error("expected at least 1 post in channel")
	}
}

func TestPinUnpinPost(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	channelID := mustGetTownSquareID(t, client)

	post, err := client.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		Content:   "pin test " + fmt.Sprint(time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	if err := client.PinPost(ctx, post.ID); err != nil {
		t.Fatalf("PinPost: %v", err)
	}

	pinned, err := client.GetPost(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetPost after pin: %v", err)
	}
	if !pinned.IsPinned {
		t.Error("expected IsPinned=true after PinPost")
	}

	if err := client.UnpinPost(ctx, post.ID); err != nil {
		t.Fatalf("UnpinPost: %v", err)
	}

	unpinned, err := client.GetPost(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetPost after unpin: %v", err)
	}
	if unpinned.IsPinned {
		t.Error("expected IsPinned=false after UnpinPost")
	}
}

func TestGetThread(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	channelID := mustGetTownSquareID(t, client)

	root, err := client.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		Content:   "thread root " + fmt.Sprint(time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("CreatePost (root): %v", err)
	}

	_, err = client.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		RootID:    root.ID,
		Content:   "thread reply",
	})
	if err != nil {
		t.Fatalf("CreatePost (reply): %v", err)
	}

	tr, err := client.GetThread(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(tr.Order) < 2 {
		t.Errorf("Order count = %d, want >= 2", len(tr.Order))
	}
}

func TestSearchPosts(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	channelID := mustGetTownSquareID(t, client)

	unique := fmt.Sprintf("xyzzy%d", time.Now().UnixNano())
	_, err := client.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		Content:   unique,
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// With ZincSearch configured a post is searchable once indexed: the
	// indexer runs every 5s and holds back posts younger than 3s. Poll rather
	// than guess a pause; without Zinc the first attempt finds it.
	deadline := time.Now().Add(20 * time.Second)
	for {
		pl, err := client.SearchPosts(ctx, channelID, unique, nil)
		if err != nil {
			t.Fatalf("SearchPosts: %v", err)
		}
		if len(pl.Order) >= 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the post never became searchable")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func TestGetUsersByIDs(t *testing.T) {
	alice := newTestClient(aliceKratosID)
	bob := newTestClient(bobKratosID)
	ctx := context.Background()

	aliceUser, err := alice.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe (alice): %v", err)
	}
	bobUser, err := bob.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe (bob): %v", err)
	}

	users, err := alice.GetUsersByIDs(ctx, []string{aliceUser.ID, bobUser.ID})
	if err != nil {
		t.Fatalf("GetUsersByIDs: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestViewChannel(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	channelID := mustGetTownSquareID(t, client)

	if err := client.ViewChannel(ctx, channelID); err != nil {
		t.Fatalf("ViewChannel: %v", err)
	}
}

func TestCreateDMAndPost(t *testing.T) {
	alice := newTestClient(aliceKratosID)
	bob := newTestClient(bobKratosID)
	ctx := context.Background()

	aliceUser, err := alice.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe (alice): %v", err)
	}
	bobUser, err := bob.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe (bob): %v", err)
	}

	dm, err := alice.CreateDirectChannel(ctx, aliceUser.ID, bobUser.ID)
	if err != nil {
		t.Fatalf("CreateDirectChannel: %v", err)
	}
	if dm.ID == "" {
		t.Fatal("DM channel has no ID")
	}
	if dm.Type != "D" {
		t.Errorf("channel Type = %q, want %q", dm.Type, "D")
	}

	post, err := alice.CreatePost(ctx, &model.Post{
		ChannelID: dm.ID,
		Content:   "DM test " + fmt.Sprint(time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("CreatePost in DM: %v", err)
	}
	if post.ChannelID != dm.ID {
		t.Errorf("post ChannelID = %q, want %q", post.ChannelID, dm.ID)
	}
}

func TestGetMyDirectChannels(t *testing.T) {
	alice := newTestClient(aliceKratosID)
	bob := newTestClient(bobKratosID)
	ctx := context.Background()

	aliceUser, err := alice.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe (alice): %v", err)
	}
	bobUser, err := bob.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe (bob): %v", err)
	}

	// Create a DM channel
	dm, err := alice.CreateDirectChannel(ctx, aliceUser.ID, bobUser.ID)
	if err != nil {
		t.Fatalf("CreateDirectChannel: %v", err)
	}

	// Fetch direct channels
	channels, err := alice.GetMyDirectChannels(ctx)
	if err != nil {
		t.Fatalf("GetMyDirectChannels: %v", err)
	}

	var found bool
	for _, ch := range channels {
		if ch.ID == dm.ID {
			found = true
			if ch.Type != "D" {
				t.Errorf("channel Type = %q, want %q", ch.Type, "D")
			}
			break
		}
	}
	if !found {
		t.Error("DM channel not found in GetMyDirectChannels results")
	}
}

func TestSearchUsers(t *testing.T) {
	client := newTestClient(aliceKratosID)
	ctx := context.Background()

	users, err := client.SearchUsers(ctx, "bob", 0, 25)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	var found bool
	for _, u := range users {
		if u.Username == "bob" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'bob' in search results")
	}
}

// mustGetTownSquareID returns the town-square channel ID or fails the test.
func mustGetTownSquareID(t *testing.T, client api.ChitClient) string {
	t.Helper()
	ctx := context.Background()

	teams, err := client.GetMyTeams(ctx)
	if err != nil {
		t.Fatalf("GetMyTeams: %v", err)
	}
	var teamID string
	for _, tm := range teams {
		if tm.Name == "uat-team" {
			teamID = tm.ID
			break
		}
	}
	if teamID == "" {
		t.Fatal("uat-team not found")
	}

	channels, err := client.GetMyChannels(ctx, teamID)
	if err != nil {
		t.Fatalf("GetMyChannels: %v", err)
	}
	for _, ch := range channels {
		if ch.Name == "town-square" {
			return ch.ID
		}
	}
	t.Fatal("town-square not found")
	return ""
}
