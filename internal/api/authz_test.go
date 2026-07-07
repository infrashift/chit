package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// charlie (thirdUserID) is seeded as a user but is neither a team nor a
// channel member; alice (extraUserID) is a team member but not a channel
// member. testUser is a member of both and owns testRootPost.

func charlieUser() *model.User {
	return &model.User{ID: thirdUserID, Username: "charlie", Roles: "system_user"}
}

func aliceUser() *model.User {
	return &model.User{ID: extraUserID, Username: "alice", Roles: "system_user"}
}

func TestAuthz_NonMemberCannotPostToChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createPost(a)
	body := `{"channel_id":"` + testChannelID + `","content":"sneaky"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonMemberCannotReadChannelPosts(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelPosts(a)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonMemberCannotReadPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getPost(a)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonOwnerCannotEditPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updatePost(a)
	body := `{"content":"defaced"}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r = withChiParam(r, "id", testRootPost) // owned by testUser
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonOwnerCannotDeletePost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := deletePost(a)
	r := httptest.NewRequest(http.MethodDelete, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost) // owned by testUser
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_OwnerCanEditOwnPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updatePost(a)
	body := `{"content":"edited by owner"}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_SystemAdminCanDeleteAnyPost(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	admin := &model.User{ID: "019421a0-0000-7000-8000-0000000000aa", Username: "admin", Roles: "system_user system_admin"}
	ms.user.seed(admin)

	handler := deletePost(a)
	r := httptest.NewRequest(http.MethodDelete, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost) // owned by testUser
	r = authedRequest(r, admin)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonMemberCannotPinPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := pinPost(a)
	r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonMemberCannotListChannelMembers(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelMembers(a)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonMemberCannotReadThread(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getThread(a)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonTeamMemberCannotCreateChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createChannel(a)
	body := `{"team_id":"` + testTeamID + `","name":"intruder","display_name":"Intruder","type":"O"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/channels", strings.NewReader(body))
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonCreatorCannotDeleteChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := deleteChannel(a)
	r := httptest.NewRequest(http.MethodDelete, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID) // created by testUser
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_CannotCreateDMForOtherUsers(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createDirectChannel(a)
	body := `["` + testUserID + `","` + extraUserID + `"]` // charlie not included
	r := httptest.NewRequest(http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_NonMemberCannotInviteThemselves(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	// Make the seeded channel private so team membership is not sufficient.
	ms.channel.seed(&model.Channel{
		ID:          testChannelID,
		TeamID:      testTeamID,
		CreatorID:   testUserID,
		Name:        "general",
		DisplayName: "General",
		Type:        "P",
		CreateAt:    1000,
		UpdateAt:    1000,
	})

	handler := addChannelMember(a)
	body := `{"user_id":"` + thirdUserID + `"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r = withChiParam(r, "id", testChannelID)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}
