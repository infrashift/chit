package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestCreateChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createChannel(a)
	body := `{"team_id":"` + testTeamID + `","name":"newchan","display_name":"New Channel","type":"O"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var ch model.Channel
	decodeJSON(t, w.Body, &ch)
	if ch.CreatorID != testUserID {
		t.Fatalf("expected creator_id=%q, got %q", testUserID, ch.CreatorID)
	}
}

func TestCreateChannel_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createChannel(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels", strings.NewReader("{bad"))
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannel(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetChannel_NotFound(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannel(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", "nonexistent")
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updateChannel(a)
	body := `{"display_name":"Updated Channel"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := deleteChannel(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetChannelsForTeam(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelsForTeam(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMyChannels(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMyChannels(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testTeamID)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateDirectChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createDirectChannel(a)
	body := `["` + testUserID + `","` + extraUserID + `"]`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCreateDirectChannel_Idempotent(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createDirectChannel(a)
	body := `["` + testUserID + `","` + extraUserID + `"]`

	// First call — creates the DM
	r1 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r1 = authedRequest(r1, testUser())
	handler.ServeHTTP(w1, r1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first call: expected 201, got %d; body: %s", w1.Code, w1.Body.String())
	}
	var ch1 model.Channel
	decodeJSON(t, w1.Body, &ch1)

	// Second call — should return same channel, no 500
	r2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r2 = authedRequest(r2, testUser())
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("second call: expected 201, got %d; body: %s", w2.Code, w2.Body.String())
	}
	var ch2 model.Channel
	decodeJSON(t, w2.Body, &ch2)

	if ch1.ID != ch2.ID {
		t.Fatalf("expected same channel ID, got %q and %q", ch1.ID, ch2.ID)
	}
}

func TestCreateDirectChannel_WrongCount(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createDirectChannel(a)
	body := `["` + testUserID + `"]`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddChannelMember(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := addChannelMember(a)
	body := `{"user_id":"` + extraUserID + `"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestRemoveChannelMember(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := removeChannelMember(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	r = withChiParams(r, map[string]string{"id": testChannelID, "user_id": testUserID})
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetChannelMembers(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelMembers(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMyDirectChannels(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	// Seed a direct channel with membership for the test user
	dmChannel := &model.Channel{
		ID:          "019421a0-0000-7000-8000-000000000099",
		Type:        "D",
		Name:        testUserID + "__" + extraUserID,
		DisplayName: testUserID + ", " + extraUserID,
		CreateAt:    1000,
		UpdateAt:    1000,
	}
	ms.Channels.Seed(dmChannel)
	ms.Channels.SeedMember(&model.ChannelMember{
		ChannelID: dmChannel.ID,
		UserID:    testUserID,
		CreateAt:  1000,
	})
	ms.Channels.SeedMember(&model.ChannelMember{
		ChannelID: dmChannel.ID,
		UserID:    extraUserID,
		CreateAt:  1000,
	})

	handler := getMyDirectChannels(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/users/me/channels/direct", http.NoBody)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var channels []*model.Channel
	decodeJSON(t, w.Body, &channels)

	if len(channels) != 1 {
		t.Fatalf("expected 1 direct channel, got %d", len(channels))
	}
	if channels[0].ID != dmChannel.ID {
		t.Fatalf("expected channel ID %q, got %q", dmChannel.ID, channels[0].ID)
	}
}

func TestGetMyDirectChannels_Empty(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMyDirectChannels(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/users/me/channels/direct", http.NoBody)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var channels []*model.Channel
	decodeJSON(t, w.Body, &channels)

	if len(channels) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(channels))
	}
}

func TestCreateGroupChannel_Idempotent(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createGroupChannel(a)
	body := `["` + testUserID + `","` + extraUserID + `","` + thirdUserID + `"]`

	// First call — creates the GM
	r1 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/group", strings.NewReader(body))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r1 = authedRequest(r1, testUser())
	handler.ServeHTTP(w1, r1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first call: expected 201, got %d; body: %s", w1.Code, w1.Body.String())
	}
	var ch1 model.Channel
	decodeJSON(t, w1.Body, &ch1)

	// Second call — should return same channel, no 500
	r2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/group", strings.NewReader(body))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r2 = authedRequest(r2, testUser())
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("second call: expected 201, got %d; body: %s", w2.Code, w2.Body.String())
	}
	var ch2 model.Channel
	decodeJSON(t, w2.Body, &ch2)

	if ch1.ID != ch2.ID {
		t.Fatalf("expected same channel ID, got %q and %q", ch1.ID, ch2.ID)
	}
}

func TestViewChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := viewChannel(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
	r = withChiParam(r, "id", testChannelID)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
