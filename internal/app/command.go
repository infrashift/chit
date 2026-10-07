package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/model"
)

// CommandResponse is the result of intercepting a slash command.
type CommandResponse struct {
	Text string
}

// CheckCommandPermission verifies an actor can execute a command via Keto.
func (a *App) CheckCommandPermission(ctx context.Context, actorID, commandID string) (bool, error) {
	url := fmt.Sprintf("%s/relation-tuples/check", a.ketoReadURL)

	body := map[string]string{
		"namespace":  model.KetoNamespaceCommand,
		"object":     fmt.Sprintf("Command:%s", commandID),
		"relation":   model.KetoRelationExecute,
		"subject_id": fmt.Sprintf("Actor:%s", actorID),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("marshal keto check: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return false, fmt.Errorf("create keto request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("keto check command: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode keto response: %w", err)
	}

	return result.Allowed, nil
}

// InterceptSlashCommand checks whether content is a slash command, authorizes
// and executes it if so. Returns (response, handled, error). If handled is
// true the caller should NOT persist the post.
func (a *App) InterceptSlashCommand(ctx context.Context, userID, channelID, content string) (*CommandResponse, bool, error) {
	parsed, ok := command.Parse(content)
	if !ok {
		return nil, false, nil
	}

	cmd, ok := a.CommandRegistry.Lookup(parsed.Slug)
	if !ok {
		return &CommandResponse{
			Text: fmt.Sprintf("Unknown command `/%s`. Type `/help` for available commands.", parsed.Slug),
		}, true, nil
	}

	eventID := model.NewID()

	// Determine actor type for audit purposes: human, agent, and bot actors
	// all execute commands the same way; only the audit record differs.
	actorType := model.ActorTypeUser
	if u, err := a.Store.User().Get(ctx, userID); err == nil && u.ActorType != "" {
		actorType = u.ActorType
	}

	// AuthZ check.
	allowed, err := a.CheckCommandPermission(ctx, userID, cmd.ID)
	if err != nil {
		slog.Warn("command authz check failed, denying",
			"command", cmd.Slug, "user_id", userID, "error", err)
		if a.AuditLogger != nil {
			a.AuditLogger.LogCommandAttempt(eventID, userID, actorType, cmd.Slug, false)
		}
		return &CommandResponse{
			Text: fmt.Sprintf("Permission check failed for `/%s`.", cmd.Slug),
		}, true, nil
	}

	if a.AuditLogger != nil {
		a.AuditLogger.LogCommandAttempt(eventID, userID, actorType, cmd.Slug, allowed)
	}

	if !allowed {
		return &CommandResponse{
			Text: fmt.Sprintf("You do not have permission to use `/%s`.", cmd.Slug),
		}, true, nil
	}

	// Execute the handler.
	handler, ok := a.CommandHandlers[cmd.ID]
	if !ok {
		return &CommandResponse{Text: fmt.Sprintf("`/%s` is registered but has no handler.", cmd.Slug)}, true, nil
	}

	result, err := handler.Execute(ctx, userID, channelID, parsed.Args)
	if err != nil {
		slog.Error("command handler error",
			"command", cmd.Slug, "user_id", userID, "error", err)
		return &CommandResponse{Text: fmt.Sprintf("Error executing `/%s`: %s", cmd.Slug, err.Error())}, true, nil
	}

	// Dispatch webhook if enabled and handler flagged it.
	if a.WebhookCh != nil && result.Triggered {
		evt := &command.WebhookEvent{
			EventID:     eventID,
			ActorID:     userID,
			CommandSlug: cmd.Slug,
			Args:        parsed.Args,
			ChannelID:   channelID,
			Status:      "executed",
		}
		select {
		case a.WebhookCh <- evt:
		default:
			slog.Warn("webhook channel full, dropping event", "event_id", eventID)
		}
	}

	// Broadcast ephemeral command response to the invoking user only.
	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: model.WebSocketEventCommandResponse,
		Data: map[string]any{
			"text":         result.ResponseText,
			"command_slug": cmd.Slug,
			"channel_id":   channelID,
		},
		Broadcast: &model.WebSocketBroadcast{
			UserID: userID,
		},
	})

	return &CommandResponse{Text: result.ResponseText}, true, nil
}
