package auth_test

import (
	"sync"
	"testing"

	"github.com/infrashift/chit-tui/internal/auth"
)

func TestTokenStore_InitialValue(t *testing.T) {
	ts := auth.NewTokenStore("initial-token")
	if got := ts.Get(); got != "initial-token" {
		t.Errorf("Get() = %q, want %q", got, "initial-token")
	}
}

func TestTokenStore_SetAndGet(t *testing.T) {
	ts := auth.NewTokenStore("")
	ts.Set("new-token")
	if got := ts.Get(); got != "new-token" {
		t.Errorf("Get() = %q, want %q", got, "new-token")
	}
}

func TestTokenStore_EmptyInitial(t *testing.T) {
	ts := auth.NewTokenStore("")
	if got := ts.Get(); got != "" {
		t.Errorf("Get() = %q, want empty", got)
	}
}

func TestTokenStore_ConcurrentAccess(t *testing.T) {
	ts := auth.NewTokenStore("start")
	var wg sync.WaitGroup

	// Writers
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 100 {
				ts.Set("token-from-writer")
			}
		}(i)
	}

	// Readers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_ = ts.Get()
			}
		}()
	}

	wg.Wait()
	// If we get here without a race condition, the test passes.
	got := ts.Get()
	if got != "token-from-writer" {
		t.Errorf("Get() = %q, want %q", got, "token-from-writer")
	}
}
