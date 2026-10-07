package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed []string
		want    bool
	}{
		{"no origin header (non-browser client)", "", []string{"https://chat.example.com"}, true},
		{"wildcard allows anything", "https://evil.example.com", []string{"*"}, true},
		{"exact match", "https://chat.example.com", []string{"https://chat.example.com"}, true},
		{"case-insensitive match", "https://Chat.Example.com", []string{"https://chat.example.com"}, true},
		{"mismatch rejected", "https://evil.example.com", []string{"https://chat.example.com"}, false},
		{"empty allowlist rejects browsers", "https://chat.example.com", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := originAllowed(r, tt.allowed); got != tt.want {
				t.Errorf("originAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowed, got, tt.want)
			}
		})
	}
}
