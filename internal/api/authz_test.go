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

// dana is the only seeded system_admin, and is a member of testTeamID.
func adminUser() *model.User {
	return &model.User{ID: adminUserID, Username: "dana", Roles: "system_user system_admin"}
}

func TestAuthz_NonMemberCannotPostToChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createPost(a)
	body := `{"channel_id":"` + testChannelID + `","content":"sneaky"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/posts", strings.NewReader(body))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(body))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(body))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels", strings.NewReader(body))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r = withChiParam(r, "id", testChannelID)
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ─── POST /users — privilege escalation ──────────────────────────
//
// createUser had NO authorization check of any kind. It decoded a model.User
// straight from the body — including `roles` and `oauth_client_id` — and saved
// it. Any authenticated caller could therefore mint a system_admin and bind an
// OAuth2 client to it, on the endpoint the docs point at for provisioning
// agents. The suite did not catch it because TestCreateUser sent the request
// UNAUTHENTICATED and asserted 201, so the hole was pinned open by a passing
// test.

func TestAuthz_CreateUserRequiresAdmin(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createUser(a)
	body := `{"kratos_id":"escalated","username":"mallory","display_name":"Mallory",` +
		`"email":"mallory@test.com","roles":"system_admin"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/users", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
	// The status code alone would pass if the handler 403'd AFTER writing.
	if _, err := ms.user.GetByUsername(t.Context(), "mallory"); err == nil {
		t.Fatal("a refused createUser still persisted the user")
	}
}

func TestAuthz_CreateUserUnauthenticatedIsRejected(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createUser(a)
	body := `{"kratos_id":"anon","username":"anon","display_name":"Anon","email":"a@test.com"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/users", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// No actor in context. This must be a 401, never a nil-pointer panic.
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body: %s", w.Code, w.Body.String())
	}
}

// A non-admin must not be able to bind an OAuth2 client to a user row.
// users.oauth_client_id is what ResolveOAuthClient matches on, so a caller who
// can write it can issue itself a machine credential that maps to any identity
// it likes — including one carrying system_admin.
func TestAuthz_CreateUserCannotBindOAuthClientWithoutAdmin(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createUser(a)
	body := `{"kratos_id":"k-bind","username":"botty","display_name":"Botty",` +
		`"email":"botty@test.com","roles":"system_admin","oauth_client_id":"stolen-client"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/users", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
	if _, err := ms.user.GetByOAuthClientID(t.Context(), "stolen-client"); err == nil {
		t.Fatal("a refused createUser still bound the OAuth2 client")
	}
}

// ─── Team membership ─────────────────────────────────────────────
//
// addTeamMember and removeTeamMember took no actor at all. Team membership is
// what AddChannelMember consults to authorize joining an OPEN channel, so
// "anyone can add anyone to any team" was also "anyone can join any open
// channel on it".

func TestAuthz_AddTeamMemberRequiresMembership(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := addTeamMember(a)
	body := `{"user_id":"` + thirdUserID + `"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/teams/"+testTeamID+"/members",
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testTeamID)
	// charlie is not on testTeamID and is adding himself to it.
	r = authedRequest(r, charlieUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_RemoveTeamMemberRequiresAdmin(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := removeTeamMember(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	r = withChiParams(r, map[string]string{"id": testTeamID, "user_id": testUserID})
	// alice IS a team member, but evicting a peer is an admin action.
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

// Leaving is not evicting. A member may always remove themselves, and breaking
// this while adding the admin gate would trap everyone on every team.
func TestAuthz_RemoveTeamMemberSelfIsAllowed(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := removeTeamMember(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	r = withChiParams(r, map[string]string{"id": testTeamID, "user_id": extraUserID})
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthz_TeamMemberHandlersRejectAnonymous(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	add := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"user_id":"x"}`))
	add = withChiParam(add, "id", testTeamID)
	addW := httptest.NewRecorder()
	addTeamMember(a).ServeHTTP(addW, add)
	if addW.Code != http.StatusUnauthorized {
		t.Fatalf("addTeamMember: expected 401, got %d", addW.Code)
	}

	rm := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	rm = withChiParams(rm, map[string]string{"id": testTeamID, "user_id": extraUserID})
	rmW := httptest.NewRecorder()
	removeTeamMember(a).ServeHTTP(rmW, rm)
	if rmW.Code != http.StatusUnauthorized {
		t.Fatalf("removeTeamMember: expected 401, got %d", rmW.Code)
	}
}
