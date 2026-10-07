package workers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/command"
)

func TestWebhookDispatcher_Delivery(t *testing.T) {
	var (
		mu             sync.Mutex
		gotContentType string
		gotSignature   string
		gotBody        []byte
		callCount      int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		gotContentType = r.Header.Get("Content-Type")
		gotSignature = r.Header.Get("X-Chit-Signature")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := make(chan *command.WebhookEvent, 10)
	d := NewWebhookDispatcher(ch, srv.URL, "test-secret", 1, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	// Send an event.
	ch <- &command.WebhookEvent{
		EventID:     "evt-1",
		ActorID:     "user-001",
		CommandSlug: "help",
		Args:        "",
		ChannelID:   "ch-001",
		Status:      "executed",
	}

	// Wait for delivery.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not stop")
	}

	mu.Lock()
	defer mu.Unlock()

	if callCount != 1 {
		t.Fatalf("expected 1 delivery, got %d", callCount)
	}

	if gotContentType != "application/cloudevents+json" {
		t.Errorf("expected Content-Type=application/cloudevents+json, got %q", gotContentType)
	}

	if gotSignature == "" {
		t.Error("expected X-Chit-Signature header")
	}

	// Verify the signature matches.
	expectedSig := command.SignPayload(gotBody, "test-secret")
	if gotSignature != expectedSig {
		t.Errorf("signature mismatch: got %q, expected %q", gotSignature, expectedSig)
	}

	// Verify CloudEvent structure.
	var ce map[string]any
	if err := json.Unmarshal(gotBody, &ce); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if ce["specversion"] != "1.0" {
		t.Errorf("expected specversion=1.0, got %v", ce["specversion"])
	}
	if ce["type"] != "com.chit.command.executed" {
		t.Errorf("expected type=com.chit.command.executed, got %v", ce["type"])
	}
	data, ok := ce["data"].(map[string]any)
	if !ok {
		t.Fatal("data field missing or wrong type")
	}
	if data["actor_id"] != "user-001" {
		t.Errorf("expected data.actor_id=user-001, got %v", data["actor_id"])
	}
	if data["command_slug"] != "help" {
		t.Errorf("expected data.command_slug=help, got %v", data["command_slug"])
	}
}

func TestWebhookDispatcher_NoSignatureWithoutSecret(t *testing.T) {
	var gotSignature string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-Chit-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := make(chan *command.WebhookEvent, 10)
	d := NewWebhookDispatcher(ch, srv.URL, "", 1, 5*time.Second) // No secret

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	ch <- &command.WebhookEvent{
		EventID:     "evt-2",
		ActorID:     "user-002",
		CommandSlug: "kick",
		ChannelID:   "ch-002",
		Status:      "executed",
	}

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	if gotSignature != "" {
		t.Errorf("expected empty signature without secret, got %q", gotSignature)
	}
}

func TestWebhookDispatcher_StartStop(t *testing.T) {
	ch := make(chan *command.WebhookEvent, 10)
	d := NewWebhookDispatcher(ch, "http://localhost:9999", "", 2, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not stop after context cancel")
	}
}

// A receiver answering 500 is logged, not fatal: the worker must deliver,
// survive the error, and stop cleanly. This used to assert nothing beyond
// "did not crash", not even that a delivery was attempted.
func TestWebhookDispatcher_ServerError(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ch := make(chan *command.WebhookEvent, 10)
	d := NewWebhookDispatcher(ch, srv.URL, "", 1, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	ch <- &command.WebhookEvent{
		EventID:     "evt-3",
		ActorID:     "user-003",
		CommandSlug: "topic",
		ChannelID:   "ch-003",
		Status:      "executed",
	}

	deadline := time.Now().Add(2 * time.Second)
	for attempts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if attempts.Load() != 1 {
		t.Fatalf("delivery attempts = %d, want 1", attempts.Load())
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not stop")
	}
}
