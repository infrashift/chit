//go:build integration

package sqlstore

import (
	"context"
	"os"
	"testing"
)

// testStore creates a SqlStore connected to the integration-test database.
// It truncates all tables before returning and closes the store on cleanup.
func testStore(t *testing.T) *SqlStore {
	t.Helper()

	dbURL := os.Getenv("CHIT_DATABASE_URL")
	if dbURL == "" {
		t.Fatal("CHIT_DATABASE_URL is not set")
	}

	ctx := context.Background()
	ss, err := New(ctx, dbURL, 5, 2)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Truncate all tables for per-test isolation (order respects FK constraints).
	_, err = ss.pool.Exec(ctx, `
		TRUNCATE message_tags, thread_memberships, threads, posts,
		         channel_members, channels, team_members, teams, tags, users
		CASCADE
	`)
	if err != nil {
		ss.Close()
		t.Fatalf("failed to truncate tables: %v", err)
	}

	t.Cleanup(func() { ss.Close() })
	return ss
}
