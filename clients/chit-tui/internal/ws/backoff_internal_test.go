package ws

import "testing"

// A new client redials on the default schedule, and each session starts it
// from the first attempt.
func TestClientUsesDefaultBackoff(t *testing.T) {
	c, ok := NewWSClient("ws://example.invalid", "tok", 0).(*wsClient)
	if !ok {
		t.Fatal("NewWSClient did not return a *wsClient")
	}
	first := c.newBackoff()
	if want := NewBackoff(); *first != *want {
		t.Errorf("newBackoff() = %+v, want %+v", *first, *want)
	}
	first.Next()
	if second := c.newBackoff(); second == first || *second != *NewBackoff() {
		t.Errorf("newBackoff() reused a schedule already in use: %+v", *second)
	}
}
