//go:build integration

package pubsub

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Round-trip regression test: Subscribe must actually receive what Publish
// sends (the previous implementation LISTENed on a connection it immediately
// released, so nothing was ever delivered), and payloads must arrive verbatim
// (not double-JSON-encoded).
func TestPGNotifyIntegration_PublishSubscribeRoundTrip(t *testing.T) {
	dbURL := os.Getenv("CHIT_DATABASE_URL")
	if dbURL == "" {
		t.Fatal("CHIT_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	ps, err := NewPGNotify(pool)
	if err != nil {
		t.Fatalf("NewPGNotify: %v", err)
	}
	defer func() { _ = ps.Close() }()

	received := make(chan []byte, 1)
	if err := ps.Subscribe(context.Background(), "chit_test_topic", func(data []byte) {
		received <- data
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	payload := []byte(`{"event":"posted","post_id":"p1"}`)

	// The LISTEN is executed asynchronously by the listener loop; retry the
	// publish until the subscription is live or we time out.
	deadline := time.After(10 * time.Second)
	for {
		if err := ps.Publish(context.Background(), "chit_test_topic", payload); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		select {
		case got := <-received:
			if !bytes.Equal(got, payload) {
				t.Fatalf("payload: got %q, want %q (double-encoded?)", got, payload)
			}
			return
		case <-time.After(500 * time.Millisecond):
		case <-deadline:
			t.Fatal("notification never received")
		}
	}
}
