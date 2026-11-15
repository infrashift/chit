package main

import (
	"strings"
	"testing"
)

// The seed writes channel members past the membership hook, so it does not
// run where the hook is on. It refuses before it opens the database.
func TestRun_RefusesWithTheMembershipHookOn(t *testing.T) {
	t.Setenv("CHIT_DATABASE_URL", "postgres://nobody@127.0.0.1:1/none?connect_timeout=1")
	t.Setenv("CHIT_MEMBERSHIP_HOOK_URL", "http://127.0.0.1:1/membership-check")
	err := run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "CHIT_MEMBERSHIP_HOOK_URL is set") {
		t.Fatalf("run = %v, want a refusal naming CHIT_MEMBERSHIP_HOOK_URL", err)
	}
}
