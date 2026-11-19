package memberhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefreshTime(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		expiry time.Time
		want   time.Time
	}{
		{"an hour: the margin before expiry", now.Add(time.Hour), now.Add(time.Hour - tokenRefreshMargin)},
		{"a short life: halfway", now.Add(20 * time.Second), now.Add(10 * time.Second)},
		{"no expiry: kept", time.Time{}, now.Add(100 * 365 * 24 * time.Hour)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refreshTime(now, tc.expiry); !got.Equal(tc.want) {
				t.Fatalf("refreshTime = %v, want %v", got, tc.want)
			}
		})
	}
}

// A token near its expiry is replaced before it is used.
func TestAccessToken_RefetchedNearExpiry(t *testing.T) {
	var tokens atomic.Int32
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokens.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "token_type": "bearer", "expires_in": 3600}); err != nil {
			t.Errorf("write the token: %v", err)
		}
	}))
	defer token.Close()
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, `{"allow":true}`); err != nil {
			t.Errorf("write the hook answer: %v", err)
		}
	}))
	defer hook.Close()
	h, err := New(&Config{URL: hook.URL, Timeout: time.Second, TokenURL: token.URL, ClientID: "c", ClientSecret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Check(t.Context(), &Request{}); err != nil {
		t.Fatal(err)
	}
	if want := time.Now().Add(time.Hour - tokenRefreshMargin); h.refreshAt.After(want) || h.refreshAt.Before(want.Add(-time.Minute)) {
		t.Fatalf("refreshAt %v, want about %v", h.refreshAt, want)
	}
	h.refreshAt = time.Now().Add(-time.Millisecond) // as if the margin was reached
	if _, err := h.Check(t.Context(), &Request{}); err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 2 {
		t.Fatalf("token requests %d, want 2", tokens.Load())
	}
}

func TestCut(t *testing.T) {
	if got := cut("abc", 5); got != "abc" {
		t.Fatalf("cut = %q", got)
	}
	if got := cut("aé", 2); got != "a" {
		t.Fatalf("cut split a rune: %q", got)
	}
	if got := cut("a\xffb", 5); got != "ab" {
		t.Fatalf("cut kept invalid UTF-8: %q", got)
	}
}
