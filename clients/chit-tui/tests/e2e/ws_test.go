//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

func TestWSConnectAndReceiveEvent(t *testing.T) {
	aliceWS := newTestWSClient(aliceKratosID, 64)
	if err := aliceWS.Connect(); err != nil {
		t.Fatalf("WS Connect (alice): %v", err)
	}
	defer func() { _ = aliceWS.Close() }()

	// Post as Bob so Alice receives the event via WS.
	bob := newTestClient(bobKratosID)
	channelID := mustGetTownSquareID(t, newTestClient(aliceKratosID))

	unique := fmt.Sprintf("ws-e2e-%d", time.Now().UnixNano())
	_, err := bob.CreatePost(context.Background(), &model.Post{
		ChannelID: channelID,
		Content:   unique,
	})
	if err != nil {
		t.Fatalf("CreatePost (bob): %v", err)
	}

	// Wait for the posted event on Alice's WS connection.
	timeout := time.After(10 * time.Second)
	for {
		select {
		case evt := <-aliceWS.Events():
			if evt.Event == model.WebSocketEventPosted {
				// Successfully received a posted event.
				return
			}
			// Other events are fine, keep waiting.
		case <-timeout:
			t.Fatal("timeout waiting for posted event on WebSocket")
		}
	}
}
