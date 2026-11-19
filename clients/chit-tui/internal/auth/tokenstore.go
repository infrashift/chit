package auth

import "sync"

// TokenStore is a thread-safe mutable token holder.
// It is the api.TokenSource the HTTP client reads on every request, so a
// token set after sign-in takes effect without rebuilding the client.
type TokenStore struct {
	mu    sync.RWMutex
	token string
}

// NewTokenStore creates a TokenStore with an initial token value.
func NewTokenStore(initial string) *TokenStore {
	return &TokenStore{token: initial}
}

// Get returns the current token.
func (ts *TokenStore) Get() string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.token
}

// Set updates the stored token.
func (ts *TokenStore) Set(token string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.token = token
}
