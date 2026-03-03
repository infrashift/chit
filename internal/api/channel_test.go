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
	r := httptest.NewRequest(http.MethodPost, "/api/v1/channels", strings.NewReader(body))
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
	r := httptest.NewRequest(http.MethodPost, "/api/v1/channels", strings.NewReader("{bad"))
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
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetChannel_NotFound(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannel(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", "nonexistent")
	w := httptest.NewRecorder()

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
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := deleteChannel(a)
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetChannelsForTeam(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelsForTeam(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMyChannels(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMyChannels(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
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
	r := httptest.NewRequest(http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCreateDirectChannel_WrongCount(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createDirectChannel(a)
	body := `["` + testUserID + `"]`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/channels/direct", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

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
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestRemoveChannelMember(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := removeChannelMember(a)
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"id": testChannelID, "user_id": testUserID})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetChannelMembers(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelMembers(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestViewChannel(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := viewChannel(a)
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParam(r, "id", testChannelID)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
