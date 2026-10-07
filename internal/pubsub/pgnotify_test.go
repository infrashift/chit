package pubsub

import (
	"context"
	"testing"
)

// Topics become LISTEN/NOTIFY channel names, so anything that is not a plain
// identifier is refused before it reaches the database. Validation happens
// first, so this needs no connection.
func TestPGNotify_RejectsInvalidTopics(t *testing.T) {
	ps := &PGNotify{}
	for _, topic := range []string{"bad; DROP TABLE users", "bad topic", "", "1starts_with_digit"} {
		if err := ps.Publish(context.Background(), topic, []byte("x")); err == nil {
			t.Errorf("Publish(%q): expected an error", topic)
		}
		if err := ps.Subscribe(context.Background(), topic, func([]byte) {}); err == nil {
			t.Errorf("Subscribe(%q): expected an error", topic)
		}
	}
}
