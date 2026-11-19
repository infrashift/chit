package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/model"
)

// CommandResponse is the result of intercepting a slash command.
type CommandResponse struct {
	Text string
}

// CheckCommandPermission verifies an actor can execute a command via Keto.
func (a *App) CheckCommandPermission(ctx context.Context, actorID, commandID string) (bool, error) {
	return a.keto.Check(ctx, model.KetoNamespaceCommand,
		"Command:"+commandID, model.KetoRelationExecute, "Actor:"+actorID)
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

	if denied := a.authorizeCommand(ctx, eventID, userID, cmd); denied != nil {
		return denied, true, nil
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
		a.dispatchCommandWebhook(&command.WebhookEvent{
			EventID:     eventID,
			ActorID:     userID,
			CommandSlug: cmd.Slug,
			Args:        parsed.Args,
			ChannelID:   channelID,
			Status:      "executed",
		})
	}

	a.broadcastCommandResponse(userID, channelID, cmd.Slug, result.ResponseText)

	return &CommandResponse{Text: result.ResponseText}, true, nil
}

// authorizeCommand checks with Keto that userID may execute cmd and records
// the attempt in the audit log. It returns nil when the command is allowed,
// otherwise the response to show the user.
func (a *App) authorizeCommand(ctx context.Context, eventID, userID string, cmd *command.Command) *CommandResponse {
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
		}
	}

	if a.AuditLogger != nil {
		a.AuditLogger.LogCommandAttempt(eventID, userID, actorType, cmd.Slug, allowed)
	}

	if !allowed {
		return &CommandResponse{
			Text: fmt.Sprintf("You do not have permission to use `/%s`.", cmd.Slug),
		}
	}
	return nil
}

// dispatchCommandWebhook queues evt for the webhook dispatcher without
// blocking; the event is dropped when the queue is full.
func (a *App) dispatchCommandWebhook(evt *command.WebhookEvent) {
	select {
	case a.WebhookCh <- evt:
	default:
		slog.Warn("webhook channel full, dropping event", "event_id", evt.EventID)
	}
}

// broadcastCommandResponse sends the ephemeral command response to the
// invoking user only.
func (a *App) broadcastCommandResponse(userID, channelID, slug, text string) {
	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: model.WebSocketEventCommandResponse,
		Data: map[string]any{
			"text":         text,
			"command_slug": slug,
			"channel_id":   channelID,
		},
		Broadcast: &model.WebSocketBroadcast{
			UserID: userID,
		},
	})
}
