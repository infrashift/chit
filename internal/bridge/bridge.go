package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/model"
)

const sessionProp = "claude_session_id"

// hopsProp counts consecutive agent→agent turns in a thread. A post by a human
// carries no such prop (hops 0), so every human turn resets the chain; each
// agent reply to an agent increments it. Capped by Config.MaxAgentHops, this
// guarantees an agent↔agent exchange terminates. It is a cooperation
// mechanism between trusted bridges, not a security boundary — any client can
// write the prop.
const hopsProp = "claude_agent_hops"

// perThreadQueueSize bounds how many pending messages a single thread can
// accumulate while a claude run is in progress.
const perThreadQueueSize = 16

// mentionRe matches @username patterns at word boundaries. Copied verbatim
// from internal/app/mention.go, which is the source of truth — the bridge
// cannot import internal/app without pulling the store (and pgx) in and
// failing `make check-deps`.
var mentionRe = regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])@([a-z0-9][a-z0-9._-]{0,62}[a-z0-9])`)

// Bridge wires Chit posts to headless Claude Code runs.
type Bridge struct {
	cfg    *Config
	client *chitclient.Client
	runner *ClaudeRunner

	agentUserID   string
	agentUsername string
	channels      map[string]struct{}

	mu         sync.Mutex
	sessions   map[string]string           // thread root ID → claude session ID
	queues     map[string]chan *model.Post // thread root ID → pending posts
	actorTypes map[string]string           // author user ID → actor_type ("" = lookup failed)
	offlist    map[string]struct{}         // channels already warned about
	wg         sync.WaitGroup
}

// New creates a Bridge from configuration.
func New(cfg *Config) *Bridge {
	channels := make(map[string]struct{}, len(cfg.Channels))
	for _, id := range cfg.Channels {
		channels[id] = struct{}{}
	}
	client := chitclient.New(cfg.ServerURL, cfg.AgentKratosID, cfg.ProxySecret)
	if cfg.UseOAuth() {
		client = chitclient.NewOAuth(cfg.ServerURL, cfg.OAuth())
	}
	return &Bridge{
		cfg:        cfg,
		client:     client,
		runner:     NewClaudeRunner(cfg),
		channels:   channels,
		sessions:   make(map[string]string),
		queues:     make(map[string]chan *model.Post),
		actorTypes: make(map[string]string),
		offlist:    make(map[string]struct{}),
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
	b.agentUsername = me.Username
	b.mu.Lock()
	b.actorTypes[me.ID] = model.ActorTypeAgent
	b.mu.Unlock()
	if me.ActorType != model.ActorTypeAgent {
		// Not fatal, but worth shouting about: other bridges sharing this
		// channel resolve actor_type to decide whether to answer, so a
		// mis-typed agent user defeats the loop guard for all of them.
		slog.Warn("agent user is not marked actor_type=agent; other agents will treat its posts as human",
			"user_id", me.ID, "actor_type", me.ActorType)
	}
	slog.Info("bridge started",
		"agent", me.Username, "agent_user_id", me.ID,
		"channels", b.cfg.Channels, "workdir", b.cfg.WorkDir,
		"model", b.cfg.Model, "require_mention", b.cfg.RequireMention,
		"reply_to_agents", b.cfg.ReplyToAgents, "max_agent_hops", b.cfg.MaxAgentHops)

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
	if !b.shouldEnqueue(post) {
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

// shouldEnqueue applies the cheap event filters: configured channel, not
// authored by the agent itself, not an ephemeral command response, and
// (optionally) the agent must be mentioned. It runs inline on the WebSocket
// read loop, so it must never make a network call — anything needing a lookup
// belongs in shouldRun.
func (b *Bridge) shouldEnqueue(post *model.Post) bool {
	if _, ok := b.channels[post.ChannelID]; !ok {
		b.warnOffAllowlist(post.ChannelID)
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

// warnOffAllowlist reports, once per channel, that the agent is receiving
// posts from a channel outside CHIT_CLAUDE_CHANNELS.
//
// The event was already dropped — the allowlist is the bridge's access-control
// boundary and it held. The warning exists because reaching this point means
// the agent user was made a member of a channel it does not serve, and that is
// not under the operator's control: any team member can add the agent to an
// open channel, creating an open channel auto-adds every team member, and
// anyone can open a DM or group channel with it. Silently dropping those posts
// would hide the fact that the agent was pulled somewhere unexpected.
func (b *Bridge) warnOffAllowlist(channelID string) {
	b.mu.Lock()
	_, seen := b.offlist[channelID]
	if !seen {
		b.offlist[channelID] = struct{}{}
	}
	b.mu.Unlock()
	if seen {
		return
	}
	slog.Warn("ignoring post from a channel outside CHIT_CLAUDE_CHANNELS; this agent is a member of a channel it does not serve",
		"channel_id", channelID, "agent_user_id", b.agentUserID)
}

// shouldRun decides whether a queued post actually warrants a claude run, and
// returns the author's actor_type alongside the verdict. It may call the REST
// API, so it runs on the per-thread worker rather than the WebSocket read
// loop. Posts by humans (and by authors whose type could not be resolved) are
// always run; posts by other agents or bots are dropped unless ReplyToAgents
// is on, this agent is named directly, and the thread is under the hop cap.
func (b *Bridge) shouldRun(ctx context.Context, post *model.Post) (run bool, authorActor string) {
	actor := b.actorType(ctx, post.UserID)
	if actor != model.ActorTypeAgent && actor != model.ActorTypeBot {
		return true, actor
	}
	if !b.cfg.ReplyToAgents {
		return false, actor
	}
	if hops := agentHops(post); hops >= b.cfg.MaxAgentHops {
		slog.Info("agent hop limit reached, not replying",
			"post_id", post.ID, "hops", hops, "max", b.cfg.MaxAgentHops)
		return false, actor
	}
	// Deliberately not post.Props["mentions"]: the server merges @all/@channel
	// expansion into that list (internal/app/mention.go), so it cannot tell a
	// broadcast from a direct mention. Agent-to-agent requires being named.
	if !mentionsUsernameDirectly(post.Content, b.agentUsername) {
		return false, actor
	}
	return true, actor
}

// actorType returns the author's actor_type, caching every lookup for the
// process lifetime. Failures are cached as "" so an unreachable or deleted
// user is not re-fetched on every post; "" is treated as "not an agent",
// failing open toward humans — silently ignoring a person is worse than one
// extra agent turn, which the hop cap bounds anyway.
func (b *Bridge) actorType(ctx context.Context, userID string) string {
	b.mu.Lock()
	actor, ok := b.actorTypes[userID]
	b.mu.Unlock()
	if ok {
		return actor
	}

	user, err := b.client.GetUser(ctx, userID)
	if err != nil {
		slog.Warn("failed to resolve author actor_type; treating as human",
			"user_id", userID, "error", err)
		actor = ""
	} else {
		actor = user.ActorType
	}

	b.mu.Lock()
	b.actorTypes[userID] = actor
	b.mu.Unlock()
	return actor
}

// mentionsUsernameDirectly reports whether content names username with an
// @mention. @all and @channel never count: a broadcast is not an instruction
// to a specific agent, and treating it as one would let one @all restart an
// agent-to-agent exchange.
func mentionsUsernameDirectly(content, username string) bool {
	if username == "" {
		return false
	}
	want := strings.ToLower(username)
	if want == "all" || want == "channel" {
		return false
	}
	for _, m := range mentionRe.FindAllStringSubmatch(content, -1) {
		if strings.EqualFold(m[1], want) {
			return true
		}
	}
	return false
}

// agentHops reads the agent→agent hop count carried in a post's props. Props
// arriving over the wire are JSON, so the value is a float64; the int case
// covers props this process built in memory.
func agentHops(post *model.Post) int {
	if post.Props == nil {
		return 0
	}
	switch v := post.Props[hopsProp].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
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
	run, authorActor := b.shouldRun(ctx, post)
	if !run {
		slog.Debug("post filtered out before running claude",
			"post_id", post.ID, "author_actor_type", authorActor)
		return
	}

	// Replying to a human resets the chain; replying to an agent extends it.
	// Error replies carry the count too — they echo a stderr tail, which can
	// contain an @mention that would otherwise restart an exchange at hop 0.
	hops := 0
	if authorActor == model.ActorTypeAgent || authorActor == model.ActorTypeBot {
		hops = agentHops(post) + 1
	}

	sessionID := b.lookupSession(ctx, root)

	slog.Info("running claude", "root_id", root, "post_id", post.ID,
		"resume", sessionID != "", "content_len", len(post.Content))

	result, err := b.runner.Run(ctx, post.Content, sessionID)
	if err != nil {
		slog.Error("claude run failed", "root_id", root, "error", err)
		b.reply(ctx, post.ChannelID, root,
			fmt.Sprintf("⚠️ Claude run failed: %v", err),
			map[string]any{hopsProp: hops})
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
		hopsProp:    hops,
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
