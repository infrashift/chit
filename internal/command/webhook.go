package command

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// WebhookEvent contains the data to be dispatched as a CloudEvent.
type WebhookEvent struct {
	EventID     string
	ActorID     string
	CommandSlug string
	Args        string
	ChannelID   string
	Status      string // "executed", "denied", etc.
}

// CloudEvent is a CloudEvents v1.0 envelope for command webhook delivery.
type CloudEvent struct {
	SpecVersion     string    `json:"specversion"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	ID              string    `json:"id"`
	Time            time.Time `json:"time"`
	DataContentType string    `json:"datacontenttype"`
	Data            any       `json:"data"`
}

// CloudEventData is the data payload inside a CloudEvent.
type CloudEventData struct {
	ActorID     string `json:"actor_id"`
	CommandSlug string `json:"command_slug"`
	Args        string `json:"args,omitempty"`
	ChannelID   string `json:"channel_id"`
	Status      string `json:"status"`
}

// NewCloudEvent builds a CloudEvents v1.0 envelope from a WebhookEvent.
func NewCloudEvent(evt *WebhookEvent) *CloudEvent {
	return &CloudEvent{
		SpecVersion:     "1.0",
		Type:            "com.chit.command.executed",
		Source:          "/chit/commands",
		ID:              uuid.Must(uuid.NewV7()).String(),
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
		Data: CloudEventData{
			ActorID:     evt.ActorID,
			CommandSlug: evt.CommandSlug,
			Args:        evt.Args,
			ChannelID:   evt.ChannelID,
			Status:      evt.Status,
		},
	}
}

// SignPayload computes an HMAC-SHA256 hex digest of payload using secret.
func SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
