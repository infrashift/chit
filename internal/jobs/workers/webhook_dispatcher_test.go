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

// deliveryRecorder is a webhook receiver that records the last delivery.
type deliveryRecorder struct {
	t           *testing.T
	mu          sync.Mutex
	contentType string
	signature   string
	body        []byte
	calls       int
}

func (rec *deliveryRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.calls++
	rec.contentType = r.Header.Get("Content-Type")
	rec.signature = r.Header.Get("X-Chit-Signature")
	var err error
	if rec.body, err = io.ReadAll(r.Body); err != nil {
		rec.t.Errorf("read body: %v", err)
	}
	w.WriteHeader(http.StatusOK)
}

func TestWebhookDispatcher_Delivery(t *testing.T) {
	rec := &deliveryRecorder{t: t}
	srv := httptest.NewServer(rec)
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

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if rec.calls != 1 {
		t.Fatalf("expected 1 delivery, got %d", rec.calls)
	}
	checkSignedDelivery(t, rec.contentType, rec.signature, rec.body)
	checkCloudEvent(t, rec.body)
}

// checkSignedDelivery checks the content type and the signature of a
// delivery signed with "test-secret".
func checkSignedDelivery(t *testing.T, contentType, signature string, body []byte) {
	t.Helper()
	if contentType != "application/cloudevents+json" {
		t.Errorf("expected Content-Type=application/cloudevents+json, got %q", contentType)
	}

	if signature == "" {
		t.Error("expected X-Chit-Signature header")
	}

	// Verify the signature matches.
	expectedSig := command.SignPayload(body, "test-secret")
	if signature != expectedSig {
		t.Errorf("signature mismatch: got %q, expected %q", signature, expectedSig)
	}
}

// checkCloudEvent checks that body is the CloudEvent of an executed help
// command by user-001.
func checkCloudEvent(t *testing.T, body []byte) {
	t.Helper()
	// Verify CloudEvent structure.
	var ce map[string]any
	if err := json.Unmarshal(body, &ce); err != nil {
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
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
