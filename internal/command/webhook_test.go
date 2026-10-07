package command

import (
	"encoding/json"
	"testing"
)

func TestNewCloudEvent(t *testing.T) {
	evt := &WebhookEvent{
		EventID:     "evt-1",
		ActorID:     "user-abc",
		CommandSlug: "help",
		Args:        "",
		ChannelID:   "ch-1",
		Status:      "executed",
	}

	ce := NewCloudEvent(evt)

	if ce.SpecVersion != "1.0" {
		t.Errorf("specversion=%s, want 1.0", ce.SpecVersion)
	}
	if ce.Type != "com.chit.command.executed" {
		t.Errorf("type=%s", ce.Type)
	}
	if ce.Source != "/chit/commands" {
		t.Errorf("source=%s", ce.Source)
	}
	// The id is the audit log's event_id, so a receiver can correlate.
	if ce.ID != "evt-1" {
		t.Errorf("id=%q, want the event's id evt-1", ce.ID)
	}
	if NewCloudEvent(&WebhookEvent{}).ID == "" {
		t.Error("an event without an id still needs one: CloudEvents requires it")
	}
	if ce.DataContentType != "application/json" {
		t.Errorf("datacontenttype=%s", ce.DataContentType)
	}

	// Verify data marshals correctly.
	payload, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatal("data field missing or wrong type")
	}
	if data["actor_id"] != "user-abc" {
		t.Errorf("data.actor_id=%v", data["actor_id"])
	}
	if data["command_slug"] != "help" {
		t.Errorf("data.command_slug=%v", data["command_slug"])
	}
}

func TestSignPayload(t *testing.T) {
	payload := []byte(`{"test": true}`)
	secret := "my-secret"

	sig := SignPayload(payload, secret)
	if len(sig) != 64 { // SHA256 hex = 64 chars
		t.Errorf("signature length=%d, want 64", len(sig))
	}

	// Same input produces same output.
	sig2 := SignPayload(payload, secret)
	if sig != sig2 {
		t.Error("deterministic signature mismatch")
	}

	// Different secret produces different output.
	sig3 := SignPayload(payload, "other-secret")
	if sig == sig3 {
		t.Error("different secrets produced same signature")
	}
}
