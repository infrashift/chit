package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes ANSI escape codes from a string.
func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// NewTestUser returns a User fixture.
func NewTestUser() *model.User {
	return &model.User{
		ID:          "user-00000000-0000-7000-0000-000000000001",
		KratosID:    "kratos-00000000-0000-0000-0000-000000000001",
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Roles:       "system_user",
		CreateAt:    1700000000000,
		UpdateAt:    1700000000000,
	}
}

// NewTestTeam returns a Team fixture.
func NewTestTeam() *model.Team {
	return &model.Team{
		ID:          "team-00000000-0000-7000-0000-000000000001",
		Name:        "test-team",
		DisplayName: "Test Team",
		Description: "A test team",
		Type:        model.TeamOpen,
		CreatorID:   "user-00000000-0000-7000-0000-000000000001",
		CreateAt:    1700000000000,
		UpdateAt:    1700000000000,
	}
}

// NewTestChannel returns a Channel fixture.
func NewTestChannel() *model.Channel {
	return &model.Channel{
		ID:            "chan-00000000-0000-7000-0000-000000000001",
		TeamID:        "team-00000000-0000-7000-0000-000000000001",
		CreatorID:     "user-00000000-0000-7000-0000-000000000001",
		Name:          "general",
		DisplayName:   "General",
		Header:        "Welcome to General",
		Purpose:       "General discussion",
		Type:          model.ChannelOpen,
		TotalMsgCount: 10,
		LastPostAt:    1700000000000,
		CreateAt:      1700000000000,
		UpdateAt:      1700000000000,
	}
}

// NewTestPost returns a Post fixture.
func NewTestPost() *model.Post {
	return &model.Post{
		ID:        "post-00000000-0000-7000-0000-000000000001",
		ChannelID: "chan-00000000-0000-7000-0000-000000000001",
		UserID:    "user-00000000-0000-7000-0000-000000000001",
		Content:   "Hello, world!",
		CreateAt:  1700000000000,
		UpdateAt:  1700000000000,
	}
}

// NewTestCommand returns a Command fixture.
func NewTestCommand() *model.Command {
	return &model.Command{
		ID:          "cmd-00000000-0000-7000-0000-000000000001",
		Slug:        "remind",
		Description: "Set a reminder",
		Category:    "workflow",
	}
}

// NewTestThread returns a Thread fixture.
func NewTestThread() *model.Thread {
	return &model.Thread{
		PostID:       "post-00000000-0000-7000-0000-000000000001",
		ChannelID:    "chan-00000000-0000-7000-0000-000000000001",
		ReplyCount:   2,
		LastReplyAt:  1700000000000,
		Participants: []string{"user-00000000-0000-7000-0000-000000000001"},
	}
}

// MockHandler returns an http.HandlerFunc that responds with the given status and body.
func MockHandler(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// NewMockServer creates a test HTTP server with a custom handler.
func NewMockServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

// NewMockServerMux creates a test HTTP server with a ServeMux.
// Register routes on the returned mux before making requests.
func NewMockServerMux() (*httptest.Server, *http.ServeMux) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	return srv, mux
}
