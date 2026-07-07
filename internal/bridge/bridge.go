package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/model"
)

const sessionProp = "claude_session_id"

// perThreadQueueSize bounds how many pending messages a single thread can
// accumulate while a claude run is in progress.
const perThreadQueueSize = 16

// Bridge wires Chit posts to headless Claude Code runs.
type Bridge struct {
	cfg    *Config
	client *chitclient.Client
	runner *ClaudeRunner

	agentUserID string
	channels    map[string]struct{}

	mu       sync.Mutex
	sessions map[string]string           // thread root ID → claude session ID
	queues   map[string]chan *model.Post // thread root ID → pending posts
	wg       sync.WaitGroup
}

// New creates a Bridge from configuration.
func New(cfg *Config) *Bridge {
	channels := make(map[string]struct{}, len(cfg.Channels))
	for _, id := range cfg.Channels {
		channels[id] = struct{}{}
	}
	return &Bridge{
		cfg:      cfg,
		client:   chitclient.New(cfg.ServerURL, cfg.AgentKratosID, cfg.ProxySecret),
		runner:   NewClaudeRunner(cfg),
		channels: channels,
		sessions: make(map[string]string),
		queues:   make(map[string]chan *model.Post),
	}
}

// Run resolves the agent identity, then listens for posts until ctx is
// cancelled. It blocks.
func (b *Bridge) Run(ctx context.Context) error {
	me, err := b.client.Me(ctx)
	if err != nil {
		return fmt.Errorf("resolve agent identity: %w", err)
	}
	b.agentUserID = me.ID
	if me.ActorType != model.ActorTypeAgent {
		slog.Warn("agent user is not marked actor_type=agent",
			"user_id", me.ID, "actor_type", me.ActorType)
	}
	slog.Info("bridge started",
		"agent", me.Username, "agent_user_id", me.ID,
		"channels", b.cfg.Channels, "workdir", b.cfg.WorkDir,
		"require_mention", b.cfg.RequireMention)

	b.client.Listen(ctx, func(event *model.WebSocketEvent) {
		b.handleEvent(ctx, event)
	})
	b.wg.Wait()
	return nil
}

// handleEvent filters incoming events down to posts the bridge should answer
// and enqueues them on their thread's worker.
func (b *Bridge) handleEvent(ctx context.Context, event *model.WebSocketEvent) {
	if event.Event != model.WebSocketEventPosted {
		return
	}
	post := postFromEventData(event.Data)
	if post == nil {
		return
	}
	if !b.shouldHandle(post) {
		return
	}

	root := post.RootID
	if root == "" {
		root = post.ID
	}

	b.mu.Lock()
	queue, ok := b.queues[root]
	if !ok {
		queue = make(chan *model.Post, perThreadQueueSize)
		b.queues[root] = queue
		b.wg.Add(1)
		go b.threadWorker(ctx, root, queue)
	}
	b.mu.Unlock()

	select {
	case queue <- post:
	default:
		slog.Warn("thread queue full, dropping message", "root_id", root, "post_id", post.ID)
	}
}

// shouldHandle applies the event filters: configured channel, not authored by
// the agent itself, not an ephemeral command response, and (optionally) the
// agent must be mentioned.
func (b *Bridge) shouldHandle(post *model.Post) bool {
	if _, ok := b.channels[post.ChannelID]; !ok {
		return false
	}
	if post.UserID == b.agentUserID {
		return false
	}
	if post.Type == "command_response" {
		return false
	}
	if b.cfg.RequireMention && !mentionsUser(post, b.agentUserID) {
		return false
	}
	return true
}

// threadWorker serializes claude runs for one thread: one process per session
// at a time, messages queued in arrival order.
func (b *Bridge) threadWorker(ctx context.Context, root string, queue chan *model.Post) {
	defer b.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case post := <-queue:
			b.processPost(ctx, root, post)
		}
	}
}

func (b *Bridge) processPost(ctx context.Context, root string, post *model.Post) {
	sessionID := b.lookupSession(ctx, root)

	slog.Info("running claude", "root_id", root, "post_id", post.ID,
		"resume", sessionID != "", "content_len", len(post.Content))

	result, err := b.runner.Run(ctx, post.Content, sessionID)
	if err != nil {
		slog.Error("claude run failed", "root_id", root, "error", err)
		b.reply(ctx, post.ChannelID, root,
			fmt.Sprintf("⚠️ Claude run failed: %v", err), nil)
		return
	}

	b.mu.Lock()
	b.sessions[root] = result.SessionID
	b.mu.Unlock()

	content := result.Result
	if content == "" {
		content = "_(empty response)_"
	}
	props := map[string]any{
		sessionProp: result.SessionID,
		"claude_usage": map[string]any{
			"input_tokens":            result.Usage.InputTokens,
			"output_tokens":           result.Usage.OutputTokens,
			"cache_read_input_tokens": result.Usage.CacheReadInputTokens,
			"total_cost_usd":          result.TotalCostUSD,
			"num_turns":               result.NumTurns,
		},
	}
	b.reply(ctx, post.ChannelID, root, content+b.runner.Footer(result), props)
}

func (b *Bridge) reply(ctx context.Context, channelID, root, content string, props map[string]any) {
	if _, err := b.client.CreatePost(ctx, channelID, root, content, props); err != nil {
		slog.Error("failed to post reply", "root_id", root, "error", err)
	}
}

// lookupSession returns the claude session for a thread: the in-memory map
// first, then (after a restart) the newest agent reply's props in the thread.
func (b *Bridge) lookupSession(ctx context.Context, root string) string {
	b.mu.Lock()
	sessionID, ok := b.sessions[root]
	b.mu.Unlock()
	if ok {
		return sessionID
	}

	posts, err := b.client.GetThread(ctx, root)
	if err != nil {
		slog.Warn("failed to fetch thread for session recovery; starting fresh session",
			"root_id", root, "error", err)
		return ""
	}
	// Thread posts are oldest-first; scan newest-first for the latest session.
	for i := len(posts) - 1; i >= 0; i-- {
		p := posts[i]
		if p.UserID != b.agentUserID || p.Props == nil {
			continue
		}
		if id, ok := p.Props[sessionProp].(string); ok && id != "" {
			b.mu.Lock()
			b.sessions[root] = id
			b.mu.Unlock()
			return id
		}
	}
	return ""
}

// postFromEventData decodes the full post carried in a `posted` event's data.
func postFromEventData(data map[string]any) *model.Post {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var post model.Post
	if err := json.Unmarshal(raw, &post); err != nil || post.ID == "" {
		return nil
	}
	return &post
}

// mentionsUser reports whether the post's parsed mentions include userID.
func mentionsUser(post *model.Post, userID string) bool {
	if post.Props == nil {
		return false
	}
	mentions, ok := post.Props["mentions"].([]any)
	if !ok {
		return false
	}
	for _, m := range mentions {
		if id, ok := m.(string); ok && id == userID {
			return true
		}
	}
	return false
}
