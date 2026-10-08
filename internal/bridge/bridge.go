package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

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

// seenPostsSize is how many recent post IDs are remembered to drop a post
// delivered twice (around a WebSocket reconnect) instead of running it twice.
const seenPostsSize = 1024

// queueFullNotice answers a post dropped because its thread's queue is full.
const queueFullNotice = "⚠️ I'm still working through earlier messages in this thread " +
	"and could not queue this one. Please send it again once I have replied."

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

	// runSlots bounds concurrent claude processes across all threads.
	runSlots chan struct{}

	mu         sync.Mutex
	sessions   map[string]string           // thread root ID → claude session ID
	queues     map[string]chan *model.Post // thread root ID → pending posts
	actorTypes map[string]string           // author user ID → resolved actor_type
	offlist    map[string]struct{}         // channels already warned about
	busy       map[string]struct{}         // threads told their queue is full
	seen       map[string]struct{}         // recently enqueued post IDs
	seenOrder  []string                    // seen, oldest first
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
		runSlots:   make(chan struct{}, cfg.MaxConcurrentRuns),
		channels:   channels,
		sessions:   make(map[string]string),
		queues:     make(map[string]chan *model.Post),
		actorTypes: make(map[string]string),
		offlist:    make(map[string]struct{}),
		busy:       make(map[string]struct{}),
		seen:       make(map[string]struct{}),
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
		"reply_to_agents", b.cfg.ReplyToAgents, "max_agent_hops", b.cfg.MaxAgentHops,
		"max_concurrent_runs", b.cfg.MaxConcurrentRuns, "setting_sources", b.cfg.SettingSources)

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
	if !b.shouldEnqueue(post) || !b.firstSighting(post.ID) {
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
		b.noticeQueueFull(ctx, root, post)
	}
}

// firstSighting records postID and reports whether it is new. A post delivered
// twice would otherwise run claude twice and answer twice.
func (b *Bridge) firstSighting(postID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.seen[postID]; ok {
		return false
	}
	b.seen[postID] = struct{}{}
	b.seenOrder = append(b.seenOrder, postID)
	if len(b.seenOrder) > seenPostsSize {
		delete(b.seen, b.seenOrder[0])
		b.seenOrder = b.seenOrder[1:]
	}
	return true
}

// noticeQueueFull tells the thread, once until its worker catches up, that a
// post was dropped. It posts from its own goroutine: the caller is the
// WebSocket read loop, which must not wait on the network.
func (b *Bridge) noticeQueueFull(ctx context.Context, root string, post *model.Post) {
	b.mu.Lock()
	_, told := b.busy[root]
	b.busy[root] = struct{}{}
	b.mu.Unlock()
	if told {
		return
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.reply(ctx, post.ChannelID, root, queueFullNotice, map[string]any{hopsProp: agentHops(post)})
	}()
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
// loop. Posts by humans are always run; posts by other agents or bots are
// dropped unless ReplyToAgents is on, this agent is named directly, and the
// thread is under the hop cap.
//
// An author whose type cannot be resolved is treated as human — silently
// ignoring a person is worse than one extra agent turn — unless the post
// carries a hop count. Only bridges write that prop, so a post with one came
// from an agent, and treating it as human would reset the count to zero and
// lift the hop cap for as long as the lookup kept failing.
func (b *Bridge) shouldRun(ctx context.Context, post *model.Post) (run bool, authorActor string) {
	actor := b.actorType(ctx, post.UserID)
	if actor == "" && agentHops(post) > 0 {
		actor = model.ActorTypeAgent
	}
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

// actorType returns the author's actor_type, or "" when it cannot be
// resolved. Successful lookups are cached for the process lifetime; failures
// are not, because a failure is usually transient, and caching one would
// misclassify that author for good — an agent read as a human escapes the
// loop guard.
func (b *Bridge) actorType(ctx context.Context, userID string) string {
	b.mu.Lock()
	actor, ok := b.actorTypes[userID]
	b.mu.Unlock()
	if ok {
		return actor
	}

	user, err := b.client.GetUser(ctx, userID)
	if err != nil {
		slog.Warn("failed to resolve author actor_type",
			"user_id", userID, "error", err)
		return ""
	}

	b.mu.Lock()
	b.actorTypes[userID] = user.ActorType
	b.mu.Unlock()
	return user.ActorType
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
			b.mu.Lock()
			delete(b.busy, root)
			b.mu.Unlock()
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
	// Error replies carry the count too: an error text could contain an
	// @mention that would otherwise restart an exchange at hop 0.
	hops := 0
	if authorActor == model.ActorTypeAgent || authorActor == model.ActorTypeBot {
		hops = agentHops(post) + 1
	}

	sessionID := ""
	if root != post.ID {
		sessionID = b.lookupSession(ctx, root)
	}

	result, notice, err := b.runClaude(ctx, root, post, sessionID)
	if result != nil && result.SessionID != "" {
		b.mu.Lock()
		b.sessions[root] = result.SessionID
		b.mu.Unlock()
	}
	if err != nil {
		var runErr *RunError
		if errors.As(err, &runErr) && runErr.Detail != "" {
			slog.Error("claude run failed", "root_id", root, "error", err, "detail", runErr.Detail)
		} else {
			slog.Error("claude run failed", "root_id", root, "error", err)
		}
		props := map[string]any{hopsProp: hops}
		// A failed run can still have started a session worth resuming
		// (an API error mid-task leaves the work done so far).
		if result != nil && result.SessionID != "" {
			props[sessionProp] = result.SessionID
		}
		b.reply(ctx, post.ChannelID, root, notice+"⚠️ Claude run failed: "+err.Error(), props)
		return
	}

	content := result.Result
	if content == "" {
		content = "_(empty response)_"
	}
	props := map[string]any{
		sessionProp: result.SessionID,
		hopsProp:    hops,
		"claude_usage": map[string]any{
			"input_tokens":                result.Usage.InputTokens,
			"output_tokens":               result.Usage.OutputTokens,
			"cache_read_input_tokens":     result.Usage.CacheReadInputTokens,
			"cache_creation_input_tokens": result.Usage.CacheCreationInputTokens,
			"total_cost_usd":              result.TotalCostUSD,
			"num_turns":                   result.NumTurns,
		},
	}
	b.reply(ctx, post.ChannelID, root, notice+content+b.runner.Footer(result), props)
}

// sessionLostNotice opens a reply whose thread's session could not be resumed.
const sessionLostNotice = "_The Claude session for this thread could not be found " +
	"(the bridge's working directory or Claude's state changed), " +
	"so this reply starts a new one without the earlier context._\n\n"

// runClaude runs one turn under a concurrency slot. When the thread's session
// no longer exists it retries once as a new session — otherwise every later
// reply in the thread would fail the same way — and returns a notice saying
// so, for the reply to lead with.
func (b *Bridge) runClaude(ctx context.Context, root string, post *model.Post, sessionID string) (*ClaudeResult, string, error) {
	select {
	case b.runSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	defer func() { <-b.runSlots }()

	slog.Info("running claude", "root_id", root, "post_id", post.ID,
		"resume", sessionID != "", "content_len", len(post.Content))
	result, err := b.runner.Run(ctx, post.Content, sessionID)
	if !errors.Is(err, ErrSessionNotFound) {
		return result, "", err
	}
	slog.Warn("thread's claude session not found; starting a new one",
		"root_id", root, "session_id", sessionID)
	b.mu.Lock()
	delete(b.sessions, root)
	b.mu.Unlock()
	result, err = b.runner.Run(ctx, post.Content, "")
	return result, sessionLostNotice, err
}

// reply posts content into the thread, split across several posts when it is
// longer than one post may be. Every part carries the hop count, since another
// agent may answer any of them; the remaining props go on the last part, which
// is where session recovery looks first.
func (b *Bridge) reply(ctx context.Context, channelID, root, content string, props map[string]any) {
	parts := splitContent(content, model.PostMaxContentSize)
	for i, part := range parts {
		partProps := props
		if i < len(parts)-1 {
			partProps = map[string]any{hopsProp: props[hopsProp]}
		}
		if _, err := b.client.CreatePost(ctx, channelID, root, part, partProps); err != nil {
			slog.Error("failed to post reply", "root_id", root,
				"part", i+1, "parts", len(parts), "error", err)
			return
		}
	}
}

// splitContent cuts s into pieces of at most limit bytes, preferring to break
// at a line end in the second half of each piece and never splitting a UTF-8
// sequence.
func splitContent(s string, limit int) []string {
	var parts []string
	for len(s) > limit {
		cut := strings.LastIndexByte(s[:limit], '\n') + 1
		if cut <= limit/2 {
			cut = limit
			for cut > 0 && !utf8.RuneStart(s[cut]) {
				cut--
			}
		}
		parts = append(parts, s[:cut])
		s = s[cut:]
	}
	return append(parts, s)
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
