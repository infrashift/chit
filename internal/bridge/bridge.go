package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/model"
)

const sessionProp = "claude_session_id"

// sessionResetProp marks an agent reply after which the thread's session is
// not resumed (it outgrew CHIT_CLAUDE_MAX_SESSION_TOKENS). Session recovery
// stops at it, so a restart does not resume the session it retired.
const sessionResetProp = "claude_session_reset"

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

// threadMemorySize is how many threads' participation (has this agent replied
// there or not) is remembered, to answer follow-ups without a lookup.
const threadMemorySize = 4096

// queued is a post waiting on its thread's worker. A post that did not name
// the agent is queued only when it might be a follow-up in a thread the agent
// has replied in; the worker checks that before running it.
type queued struct {
	post      *model.Post
	addressed bool
	// newSession is a !new: forget the thread's session first, and run the
	// post (its text after the control word) only when runAfter is set.
	newSession bool
	runAfter   bool
}

// idleWorkerTimeout is how long a thread's worker waits for another post
// before it exits. Its session is forgotten too; it is recovered from the
// thread's props if the thread wakes up.
const idleWorkerTimeout = 10 * time.Minute

// replyTimeout bounds posting one reply. Replies get their own deadline, not
// the run's context, so a run stopped by shutdown can still say so.
const replyTimeout = 15 * time.Second

// errShuttingDown is returned for a post the bridge stopped before running.
//
// semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity
var errShuttingDown = errors.New("the bridge is shutting down")

// shutdownNotice answers posts a shutting-down bridge did not get to.
const shutdownNotice = "⚠️ The bridge is restarting and did not get to your latest message " +
	"in this thread. Please send it again once I am back."

// queueFullNotice answers a post dropped because its thread's queue is full.
const queueFullNotice = "⚠️ I'm still working through earlier messages in this thread " +
	"and could not queue this one. Please send it again once I have replied."

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
	// stopping closes when shutdown begins: no new run starts after it.
	stopping      chan struct{}
	idleTimeout   time.Duration
	progressDelay time.Duration

	mu         sync.Mutex
	sessions   map[string]string      // thread root ID → claude session ID ("" = start fresh)
	queues     map[string]chan queued // thread root ID → pending posts
	running    map[string]runState    // thread root ID → its run in progress
	authors    map[string]*model.User // author user ID → resolved user
	offlist    map[string]struct{}    // channels already warned about
	busy       map[string]struct{}    // threads told their queue is full
	seen       *recentSet             // recently enqueued post IDs
	joined     *recentSet             // threads this agent has replied in
	bystander  *recentSet             // threads checked and found without its replies
	cleanStart *recentSet             // threads whose next session was asked to start clean
	wg         sync.WaitGroup
}

// New creates a Bridge from configuration.
func New(cfg *Config) *Bridge {
	assert.That(cfg != nil, "New: nil config")
	assert.That(cfg.MaxConcurrentRuns >= 1, "New: no run slots; Load validates the limit")
	channels := make(map[string]struct{}, len(cfg.Channels))
	for _, id := range cfg.Channels {
		channels[id] = struct{}{}
	}
	assert.That(len(channels) <= len(cfg.Channels), "New: more allowed channels than configured")
	client := chitclient.New(cfg.ServerURL, cfg.AgentKratosID, cfg.ProxySecret)
	if cfg.UseOAuth() {
		client = chitclient.NewOAuth(cfg.ServerURL, cfg.OAuth())
	}
	return &Bridge{
		cfg:           cfg,
		client:        client,
		runner:        NewClaudeRunner(cfg),
		runSlots:      make(chan struct{}, cfg.MaxConcurrentRuns),
		stopping:      make(chan struct{}),
		idleTimeout:   idleWorkerTimeout,
		progressDelay: progressDelay,
		channels:      channels,
		sessions:      make(map[string]string),
		queues:        make(map[string]chan queued),
		running:       make(map[string]runState),
		authors:       make(map[string]*model.User),
		offlist:       make(map[string]struct{}),
		busy:          make(map[string]struct{}),
		seen:          newRecentSet(seenPostsSize),
		joined:        newRecentSet(threadMemorySize),
		bystander:     newRecentSet(threadMemorySize),
		cleanStart:    newRecentSet(threadMemorySize),
	}
}

// Run resolves the agent identity, checks it can serve its channels, then
// listens for posts until ctx is cancelled. It blocks until shutdown is done:
// runs in progress get ShutdownGrace to finish, and threads with posts that
// never started are told to send them again.
func (b *Bridge) Run(ctx context.Context) error {
	assert.That(ctx != nil, "Bridge.Run: nil context")
	assert.That(b.client != nil && b.cfg != nil, "Bridge.Run: bridge not built by New")
	me, err := b.client.Me(ctx)
	if err != nil {
		return fmt.Errorf("resolve agent identity: %w", err)
	}
	assert.That(me != nil, "Bridge.Run: nil agent user without an error")
	b.agentUserID = me.ID
	b.agentUsername = me.Username
	b.mu.Lock()
	b.authors[me.ID] = me
	b.mu.Unlock()
	if me.ActorType != model.ActorTypeAgent {
		// Not fatal, but worth shouting about: other bridges sharing this
		// channel resolve actor_type to decide whether to answer, so a
		// mis-typed agent user defeats the loop guard for all of them.
		slog.Warn("agent user is not marked actor_type=agent; other agents will treat its posts as human",
			"user_id", me.ID, "actor_type", me.ActorType)
	}
	if err := b.checkChannels(ctx); err != nil {
		return err
	}
	slog.Info("bridge started",
		"agent", me.Username, "agent_user_id", me.ID,
		"channels", b.cfg.Channels, "workdir", b.cfg.WorkDir,
		"model", b.cfg.Model, "require_mention", b.cfg.RequireMention,
		"reply_to_agents", b.cfg.ReplyToAgents, "max_agent_hops", b.cfg.MaxAgentHops,
		"max_concurrent_runs", b.cfg.MaxConcurrentRuns, "setting_sources", b.cfg.SettingSources)

	// Runs outlive ctx, which only says to stop listening; workCtx is
	// cancelled when the grace period is up.
	workCtx, stopWork := context.WithCancel(context.WithoutCancel(ctx))
	defer stopWork()
	b.client.Listen(ctx, eventListener{b: b, ctx: workCtx})

	assert.That(b.cfg.ShutdownGrace >= 0, "Bridge.Run: negative shutdown grace; Load validates it")
	close(b.stopping)
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	slog.Info("bridge stopping; waiting for runs in progress", "grace", b.cfg.ShutdownGrace)
	select {
	case <-done:
	case <-time.After(b.cfg.ShutdownGrace):
		slog.Warn("shutdown grace period over; stopping runs in progress")
		stopWork()
		<-done
	}
	slog.Info("bridge stopped")
	return nil
}

// checkChannels confirms the agent is a member of every channel it serves.
// The hub delivers events only to members, so a bridge serving a channel its
// agent is not in would start cleanly and never hear a thing. A refusal from
// chitd (the channel does not exist, or the agent may not see it) stops the
// bridge; a failure to get an answer only warns, so a chitd restart does not
// keep the bridge down.
func (b *Bridge) checkChannels(ctx context.Context) error {
	assert.That(ctx != nil, "Bridge.checkChannels: nil context")
	assert.That(len(b.channels) <= len(b.cfg.Channels), "Bridge.checkChannels: allowlist differs from the config")
	for _, id := range b.cfg.Channels {
		_, allowed := b.channels[id]
		assert.That(allowed, "Bridge.checkChannels: configured channel missing from the allowlist")
		member, err := b.isMember(ctx, id)
		var status *chitclient.StatusError
		switch {
		case errors.As(err, &status) && status.Status < 500:
			return fmt.Errorf("channel %s in CHIT_CLAUDE_CHANNELS: %w", id, err)
		case err != nil:
			slog.Warn("could not check channel membership", "channel_id", id, "error", err)
		case !member:
			return fmt.Errorf("channel %s in CHIT_CLAUDE_CHANNELS: agent %s is not a member, "+
				"so it would never receive its posts; add it to the channel", id, b.agentUsername)
		}
	}
	return nil
}

// isMember reports whether the agent belongs to a channel. A direct or group
// channel can only be read by its members; an open channel can be read by
// anyone on its team, so for a team channel the agent's memberships decide.
func (b *Bridge) isMember(ctx context.Context, channelID string) (bool, error) {
	assert.That(ctx != nil, "Bridge.isMember: nil context")
	assert.That(b.client != nil, "Bridge.isMember: nil chitd client")
	channel, err := b.client.GetChannel(ctx, channelID)
	if err != nil {
		return false, err
	}
	assert.That(channel != nil, "Bridge.isMember: nil channel without an error")
	if channel.TeamID == "" {
		return true, nil
	}
	members, err := b.client.GetMyChannelMembers(ctx, channel.TeamID)
	if err != nil {
		return false, err
	}
	for _, m := range members {
		if m.ChannelID == channelID {
			return true, nil
		}
	}
	return false, nil
}

// eventListener hands the WebSocket's events to the bridge under the work
// context, which outlives the listening context.
type eventListener struct {
	b   *Bridge
	ctx context.Context
}

// HandleEvent passes event to the bridge.
func (l eventListener) HandleEvent(event *model.WebSocketEvent) {
	assert.That(l.b != nil, "eventListener.HandleEvent: nil bridge")
	assert.That(l.ctx != nil, "eventListener.HandleEvent: nil work context")
	l.b.handleEvent(l.ctx, event)
}

// handleEvent filters incoming events down to posts the bridge should answer
// and enqueues them on their thread's worker.
func (b *Bridge) handleEvent(ctx context.Context, event *model.WebSocketEvent) {
	assert.That(ctx != nil, "Bridge.handleEvent: nil context")
	assert.That(event != nil, "Bridge.handleEvent: nil event")
	if event.Event != model.WebSocketEventPosted {
		return
	}
	post := postFromEventData(event.Data)
	if post == nil {
		return
	}
	assert.That(post.ID != "", "Bridge.handleEvent: decoded post has no ID")
	addressed, ok := b.shouldEnqueue(post)
	if !ok || !b.firstSighting(post.ID) {
		return
	}

	root := post.RootID
	if root == "" {
		root = post.ID
	}
	_, allowed := b.channels[post.ChannelID]
	assert.That(allowed, "Bridge.handleEvent: post outside the allowlist passed the filter")

	item := queued{post: post, addressed: addressed}
	if addressed {
		switch word, rest := b.parseControl(post.Content); word {
		case controlStop, controlStatus:
			b.controlNow(ctx, root, post, word)
			return
		case controlNew:
			item.newSession, item.runAfter = true, rest != ""
			if rest != "" {
				withRest := *post
				withRest.Content = rest
				item.post = &withRest
			}
		}
	}

	if !b.enqueue(ctx, root, item) {
		slog.Warn("thread queue full, dropping message", "root_id", root, "post_id", post.ID)
		if addressed {
			b.noticeQueueFull(ctx, root, post)
		}
	}
}

// enqueue sends item to root's worker, starting the worker if the thread has
// none, and reports whether the queue had room.
func (b *Bridge) enqueue(ctx context.Context, root string, item queued) bool {
	assert.That(root != "", "Bridge.enqueue: empty thread root")
	assert.That(item.post != nil && item.post.ID != "", "Bridge.enqueue: queued item without a post")
	assert.That(!item.runAfter || item.newSession, "Bridge.enqueue: runAfter set without newSession")
	// The send happens under mu, so a worker retiring for idleness (which
	// also takes mu) can never leave a post in a queue nobody reads.
	b.mu.Lock()
	queue, ok := b.queues[root]
	if !ok {
		queue = make(chan queued, perThreadQueueSize)
		b.queues[root] = queue
		b.wg.Add(1)
		go b.threadWorker(ctx, root, queue)
	}
	assert.That(cap(queue) == perThreadQueueSize, "Bridge.enqueue: thread queue has the wrong capacity")
	sent := false
	select {
	case queue <- item:
		sent = true
	default:
	}
	// The agent is about to answer here, so the thread is joined now: a
	// follow-up sent before that answer lands is for the agent too.
	if sent && item.addressed {
		b.joined.add(root)
		b.bystander.remove(root)
	}
	b.mu.Unlock()
	return sent
}

// firstSighting records postID and reports whether it is new. A post delivered
// twice would otherwise run claude twice and answer twice.
func (b *Bridge) firstSighting(postID string) bool {
	assert.That(postID != "", "Bridge.firstSighting: empty post ID")
	assert.That(b.seen != nil, "Bridge.firstSighting: nil seen set")
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.seen.add(postID)
}

// noticeQueueFull tells the thread, once until its worker catches up, that a
// post was dropped. It posts from its own goroutine: the caller is the
// WebSocket read loop, which must not wait on the network.
func (b *Bridge) noticeQueueFull(ctx context.Context, root string, post *model.Post) {
	assert.That(root != "", "Bridge.noticeQueueFull: empty thread root")
	assert.That(post != nil, "Bridge.noticeQueueFull: nil post")
	assert.That(b.busy != nil, "Bridge.noticeQueueFull: nil busy set")
	b.mu.Lock()
	_, told := b.busy[root]
	b.busy[root] = struct{}{}
	b.mu.Unlock()
	if told {
		return
	}
	b.wg.Add(1)
	go func() {
		defer errlog.Recover("bridge queue-full reply")
		defer b.wg.Done()
		b.reply(ctx, post.ChannelID, root, queueFullNotice, map[string]any{hopsProp: agentHops(post)})
	}()
}

// shouldEnqueue applies the cheap event filters: configured channel, not
// authored by the agent itself, not an ephemeral command response, and, with
// RequireMention, addressed to the agent. It runs inline on the WebSocket read
// loop, so it must never make a network call — anything needing a lookup
// belongs on the worker.
//
// With RequireMention, a post is addressed when it names the agent, or (with
// FollowThreads) when it replies in a thread the agent has replied in. A reply
// in a thread the bridge knows nothing about yet is queued unaddressed, for
// the worker to check.
func (b *Bridge) shouldEnqueue(post *model.Post) (addressed, ok bool) {
	assert.That(post != nil, "Bridge.shouldEnqueue: nil post")
	assert.That(b.cfg != nil && b.channels != nil, "Bridge.shouldEnqueue: bridge not built by New")
	if _, ok := b.channels[post.ChannelID]; !ok {
		b.warnOffAllowlist(post.ChannelID)
		return false, false
	}
	if post.UserID == b.agentUserID {
		return false, false
	}
	if post.Type == "command_response" {
		return false, false
	}
	if !b.cfg.RequireMention || b.mentioned(post) {
		return true, true
	}
	if !b.cfg.FollowThreads || post.RootID == "" {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	assert.That(b.joined != nil && b.bystander != nil, "Bridge.shouldEnqueue: nil thread memory")
	switch {
	case b.joined.has(post.RootID):
		return true, true
	case b.bystander.has(post.RootID):
		return false, false
	}
	return false, true
}

// mentioned reports whether a post names this agent. @all and @channel count
// only with AnswerBroadcasts: the server folds them into props["mentions"],
// and one @channel would otherwise start a run in every persona serving the
// channel.
func (b *Bridge) mentioned(post *model.Post) bool {
	assert.That(post != nil, "Bridge.mentioned: nil post")
	assert.That(b.cfg != nil, "Bridge.mentioned: nil config")
	if b.cfg.AnswerBroadcasts {
		return mentionsUser(post, b.agentUserID)
	}
	return mentionsUsernameDirectly(post.Content, b.agentUsername)
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
	_, served := b.channels[channelID]
	assert.That(!served, "Bridge.warnOffAllowlist: channel is on the allowlist")
	b.mu.Lock()
	assert.That(b.offlist != nil, "Bridge.warnOffAllowlist: nil offlist")
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
	assert.That(ctx != nil, "Bridge.shouldRun: nil context")
	assert.That(post != nil, "Bridge.shouldRun: nil post")
	assert.That(b.cfg.MaxAgentHops >= 0, "Bridge.shouldRun: negative hop cap; Load validates it")
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
// resolved.
func (b *Bridge) actorType(ctx context.Context, userID string) string {
	assert.That(ctx != nil, "Bridge.actorType: nil context")
	if user := b.author(ctx, userID); user != nil {
		return user.ActorType
	}
	return ""
}

// author returns a post author's user record, or nil when it cannot be
// resolved. Successful lookups are cached for the process lifetime; failures
// are not, because a failure is usually transient, and caching one would
// misclassify that author for good — an agent read as a human escapes the
// loop guard.
func (b *Bridge) author(ctx context.Context, userID string) *model.User {
	assert.That(ctx != nil, "Bridge.author: nil context")
	assert.That(b.client != nil, "Bridge.author: nil chitd client")
	b.mu.Lock()
	user, ok := b.authors[userID]
	b.mu.Unlock()
	if ok {
		return user
	}

	user, err := b.client.GetUser(ctx, userID)
	if err != nil {
		slog.Warn("failed to resolve post author", "user_id", userID, "error", err)
		return nil
	}
	assert.That(user != nil, "Bridge.author: nil user without an error")

	b.mu.Lock()
	b.authors[userID] = user
	b.mu.Unlock()
	return user
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
	return slices.Contains(model.ParseMentions(content), want)
}

// agentHops reads the agent→agent hop count carried in a post's props. Props
// arriving over the wire are JSON, so the value is a float64; the int case
// covers props this process built in memory.
func agentHops(post *model.Post) int {
	assert.That(post != nil, "agentHops: nil post")
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
// at a time, messages queued in arrival order. It exits when the thread has
// been idle for idleTimeout, and on shutdown, after telling the thread about
// any post it did not get to.
func (b *Bridge) threadWorker(ctx context.Context, root string, queue chan queued) {
	defer b.recoverThread(root)
	defer b.wg.Done()
	assert.That(root != "", "Bridge.threadWorker: empty thread root")
	assert.That(queue != nil, "Bridge.threadWorker: nil queue")
	assert.That(b.idleTimeout > 0, "Bridge.threadWorker: idle timeout not positive")
	idle := time.NewTimer(b.idleTimeout)
	defer idle.Stop()
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-b.stopping:
			b.abandon(ctx, root, queue, nil)
			return
		case <-idle.C:
			if b.retire(root, queue) {
				return
			}
			idle.Reset(b.idleTimeout)
		case q := <-queue:
			assert.That(q.post != nil, "Bridge.threadWorker: queued item without a post")
			select {
			case <-b.stopping:
				b.abandon(ctx, root, queue, &q)
				return
			default:
			}
			b.mu.Lock()
			delete(b.busy, root)
			b.mu.Unlock()
			if q.newSession {
				b.newSession(ctx, root, q.post, !q.runAfter)
			}
			if (!q.newSession || q.runAfter) && (q.addressed || b.participates(ctx, root)) {
				b.processPost(ctx, root, q.post)
			}
			idle.Reset(b.idleTimeout)
		}
	}
}

// recoverThread stops a panic in the worker of one thread (ADR-0011). It
// logs the panic and removes the state of the thread, so the next post
// starts a new worker. The other threads go on.
func (b *Bridge) recoverThread(root string) {
	r := recover()
	if r == nil {
		return
	}
	slog.Error("bridge thread worker stopped after a panic",
		"root_id", root, "panic", r, "stack", string(debug.Stack()))
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.queues, root)
	delete(b.sessions, root)
	delete(b.busy, root)
}

// retire removes an idle thread's worker state, unless a post arrived in the
// meantime. Sends happen under mu, so once the queue is gone from the map
// nothing more can arrive on it.
func (b *Bridge) retire(root string, queue chan queued) bool {
	assert.That(queue != nil, "Bridge.retire: nil queue")
	b.mu.Lock()
	defer b.mu.Unlock()
	assert.That(b.queues[root] == queue, "Bridge.retire: worker's queue is not the thread's queue")
	if len(queue) > 0 {
		return false
	}
	delete(b.queues, root)
	delete(b.sessions, root)
	delete(b.busy, root)
	_, still := b.queues[root]
	assert.That(!still, "Bridge.retire: queue still registered")
	return true
}

// abandon tells a thread, once, that the addressed posts left in its queue
// (and taken, if non-nil) were never run.
func (b *Bridge) abandon(ctx context.Context, root string, queue chan queued, taken *queued) {
	assert.That(queue != nil, "Bridge.abandon: nil queue")
	assert.That(taken == nil || taken.post != nil, "Bridge.abandon: taken item without a post")
	var last *model.Post
	if taken != nil && taken.addressed {
		last = taken.post
	}
	// This worker is the only receiver, so a receive after len > 0 does not
	// block, and the capacity bounds the loop (Power of 10 rule 2).
	for range cap(queue) {
		if len(queue) == 0 {
			break
		}
		q := <-queue
		assert.That(q.post != nil, "Bridge.abandon: queued item without a post")
		if q.addressed {
			last = q.post
		}
	}
	if last == nil {
		return
	}
	slog.Info("shutting down with unrun posts in thread", "root_id", root, "last_post_id", last.ID)
	b.reply(ctx, last.ChannelID, root, shutdownNotice, map[string]any{hopsProp: agentHops(last)})
}

func (b *Bridge) processPost(ctx context.Context, root string, post *model.Post) {
	assert.That(root != "", "Bridge.processPost: empty thread root")
	assert.That(post != nil && post.ID != "", "Bridge.processPost: post without an ID")
	run, authorActor := b.shouldRun(ctx, post)
	if !run {
		slog.Debug("post filtered out before running claude",
			"post_id", post.ID, "author_actor_type", authorActor)
		return
	}

	hops := replyHops(post, authorActor)
	sessionID, seed := b.resumeSession(ctx, root, post)

	runCtx, cancelRun := b.startRun(ctx, root)
	assert.That(!seed || sessionID == "", "Bridge.processPost: seeding a resumed session")
	progress := b.showProgress(ctx, post.ChannelID, root, hops)
	defer progress.Stop(ctx)
	result, notice, err := b.runClaude(runCtx, root, post, sessionID, seed)
	b.finishRun(root)
	stopped := errors.Is(context.Cause(runCtx), errStopped)
	cancelRun(nil)
	if stopped {
		// !stop already answered in the thread.
		slog.Info("run stopped on request", "root_id", root, "post_id", post.ID)
		return
	}
	if errors.Is(err, errShuttingDown) {
		b.reply(ctx, post.ChannelID, root, shutdownNotice, map[string]any{hopsProp: hops})
		return
	}
	if result != nil && result.SessionID != "" {
		b.mu.Lock()
		b.sessions[root] = result.SessionID
		b.cleanStart.remove(root)
		b.mu.Unlock()
	}
	if err != nil {
		b.replyRunFailed(ctx, root, post, hops, result, notice, err)
		return
	}
	b.replyResult(ctx, root, post, hops, result, notice)
}

// replyHops is the agent-hop count a reply to post carries.
func replyHops(post *model.Post, authorActor string) int {
	assert.That(post != nil, "replyHops: nil post")
	// Replying to a human resets the chain; replying to an agent extends it.
	// Error replies carry the count too: an error text could contain an
	// @mention that would otherwise restart an exchange at hop 0.
	if authorActor == model.ActorTypeAgent || authorActor == model.ActorTypeBot {
		return agentHops(post) + 1
	}
	return 0
}

// resumeSession returns the session a run for post resumes, and whether a new
// session should be seeded with the thread so far.
func (b *Bridge) resumeSession(ctx context.Context, root string, post *model.Post) (string, bool) {
	assert.That(root != "", "Bridge.resumeSession: empty thread root")
	assert.That(post != nil && post.ID != "", "Bridge.resumeSession: post without an ID")
	// A new session in an existing thread is given the thread so far,
	// unless !new or a retirement asked for a clean start.
	if root == post.ID {
		return "", false
	}
	t := b.thread(ctx, root)
	assert.That(t.sessionID == "" || t.joined, "Bridge.resumeSession: session in a thread the agent has not joined")
	return t.sessionID, t.sessionID == "" && !t.clean
}

// startRun records a run in root and returns its context.
func (b *Bridge) startRun(ctx context.Context, root string) (context.Context, context.CancelCauseFunc) {
	assert.That(ctx != nil, "Bridge.startRun: nil context")
	assert.That(root != "", "Bridge.startRun: empty thread root")
	// The run gets its own context so !stop can cancel it alone.
	runCtx, cancelRun := context.WithCancelCause(ctx)
	b.mu.Lock()
	assert.That(b.running != nil, "Bridge.startRun: nil run map")
	b.running[root] = runState{cancel: cancelRun, started: time.Now()}
	b.mu.Unlock()
	return runCtx, cancelRun
}

// finishRun clears root's run and marks the thread as joined.
func (b *Bridge) finishRun(root string) {
	assert.That(root != "", "Bridge.finishRun: empty thread root")
	b.mu.Lock()
	delete(b.running, root)
	b.joined.add(root)
	b.bystander.remove(root)
	assert.That(b.joined.has(root), "Bridge.finishRun: thread not marked as joined")
	b.mu.Unlock()
}

// replyRunFailed logs a failed run and answers in the thread with the error.
func (b *Bridge) replyRunFailed(ctx context.Context, root string, post *model.Post, hops int, result *ClaudeResult, notice string, err error) {
	assert.That(err != nil, "Bridge.replyRunFailed: nil error")
	assert.That(post != nil, "Bridge.replyRunFailed: nil post")
	assert.That(root != "", "Bridge.replyRunFailed: empty thread root")
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
}

// replyResult answers in the thread with a successful run's result, retiring
// the session when it has grown past the configured limit.
func (b *Bridge) replyResult(ctx context.Context, root string, post *model.Post, hops int, result *ClaudeResult, notice string) {
	assert.That(result != nil, "Bridge.replyResult: nil result")
	assert.That(post != nil, "Bridge.replyResult: nil post")
	assert.That(root != "", "Bridge.replyResult: empty thread root")
	content := result.Result
	if content == "" {
		content = "_(empty response)_"
	}
	retired := b.cfg.MaxSessionTokens > 0 && result.Usage.ContextTokens() >= b.cfg.MaxSessionTokens
	if retired {
		content += fmt.Sprintf(sessionFullNotice, humanTokens(result.Usage.ContextTokens()))
		b.mu.Lock()
		b.sessions[root] = ""
		b.cleanStart.add(root)
		b.mu.Unlock()
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
	if retired {
		props[sessionResetProp] = true
	}
	if b.cfg.Footer {
		content += b.runner.Footer(result)
	}
	b.reply(ctx, post.ChannelID, root, notice+content, props)
}

// sessionFullNotice ends a reply after which the thread starts a new session.
const sessionFullNotice = "\n\n_This thread's Claude session has grown to %s tokens " +
	"(CHIT_CLAUDE_MAX_SESSION_TOKENS), so my next reply here starts a new session " +
	"without this context. Restate anything it should know when you write again._"

// sessionLostNotice opens a reply whose thread's session could not be resumed.
const sessionLostNotice = "_The Claude session for this thread could not be found " +
	"(the bridge's working directory or Claude's state changed), " +
	"so this reply starts a new one without the earlier context._\n\n"

// runClaude runs one turn under a concurrency slot, seeding the prompt with
// the thread when seed is set. When the thread's session no longer exists it
// retries once as a new, seeded session — otherwise every later reply in the
// thread would fail the same way — and returns a notice saying so, for the
// reply to lead with.
func (b *Bridge) runClaude(ctx context.Context, root string, post *model.Post, sessionID string, seed bool) (*ClaudeResult, string, error) {
	assert.That(post != nil, "Bridge.runClaude: nil post")
	assert.That(b.runner != nil, "Bridge.runClaude: nil runner")
	assert.That(!seed || sessionID == "", "Bridge.runClaude: seeding a resumed session")
	select {
	case b.runSlots <- struct{}{}:
	case <-b.stopping:
		return nil, "", errShuttingDown
	case <-ctx.Done():
		return nil, "", errShuttingDown
	}
	defer func() { <-b.runSlots }()
	assert.That(len(b.runSlots) >= 1, "Bridge.runClaude: running without a slot")
	select {
	case <-b.stopping:
		return nil, "", errShuttingDown
	default:
	}

	prompt := b.prompt(ctx, root, post, seed)
	slog.Info("running claude", "root_id", root, "post_id", post.ID,
		"resume", sessionID != "", "prompt_len", len(prompt))
	start := time.Now()
	result, err := b.runner.Run(ctx, prompt, sessionID)
	if !errors.Is(err, ErrSessionNotFound) {
		logRun(root, post.ID, sessionID != "", start, result, err)
		return result, "", err
	}
	slog.Warn("thread's claude session not found; starting a new one",
		"root_id", root, "session_id", sessionID)
	b.mu.Lock()
	delete(b.sessions, root)
	b.mu.Unlock()
	start = time.Now()
	result, err = b.runner.Run(ctx, b.prompt(ctx, root, post, true), "")
	logRun(root, post.ID, false, start, result, err)
	return result, sessionLostNotice, err
}

// logRun writes one run_complete line per run: the record to total token use
// from, per thread or per bridge.
func logRun(root, postID string, resumed bool, start time.Time, result *ClaudeResult, err error) {
	assert.That(root != "" && postID != "", "logRun: empty thread root or post ID")
	assert.That(!start.IsZero(), "logRun: zero start time")
	attrs := []any{"root_id", root, "post_id", postID, "resumed", resumed,
		"duration_ms", time.Since(start).Milliseconds(), "ok", err == nil}
	if result != nil {
		attrs = append(attrs,
			"input_tokens", result.Usage.InputTokens,
			"output_tokens", result.Usage.OutputTokens,
			"cache_read_tokens", result.Usage.CacheReadInputTokens,
			"cache_write_tokens", result.Usage.CacheCreationInputTokens,
			"context_tokens", result.Usage.ContextTokens(),
			"cost_usd", result.TotalCostUSD,
			"num_turns", result.NumTurns)
	}
	slog.Info("run_complete", attrs...)
}

// reply posts content into the thread, split across several posts when it is
// longer than one post may be. Every part carries the hop count, since another
// agent may answer any of them; the remaining props go on the last part, which
// is where session recovery looks first.
func (b *Bridge) reply(ctx context.Context, channelID, root, content string, props map[string]any) {
	_, allowed := b.channels[channelID]
	assert.That(allowed, "Bridge.reply: channel outside the allowlist")
	assert.That(root != "", "Bridge.reply: empty thread root")
	assert.That(content != "", "Bridge.reply: empty content")
	assert.That(props != nil, "Bridge.reply: nil props")
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
	defer cancel()
	parts := splitContent(content, model.PostMaxContentSize)
	for i, part := range parts {
		assert.That(part != "" && len(part) <= model.PostMaxContentSize, "Bridge.reply: part empty or too long")
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
	assert.That(limit >= utf8.UTFMax, "splitContent: limit too small to hold a rune")
	var parts []string
	for len(s) > limit {
		cut := strings.LastIndexByte(s[:limit], '\n') + 1
		if cut <= limit/2 {
			cut = limit
			for cut > 0 && !utf8.RuneStart(s[cut]) {
				cut--
			}
		}
		assert.That(cut <= limit, "splitContent: piece longer than the limit")
		parts = append(parts, s[:cut])
		s = s[cut:]
	}
	return append(parts, s)
}

// threadInfo is threadState's answer as one value, for a caller that needs
// only part of it.
type threadInfo struct {
	sessionID     string
	joined, clean bool
}

// thread returns threadState's answer as a threadInfo.
func (b *Bridge) thread(ctx context.Context, root string) threadInfo {
	assert.That(ctx != nil, "Bridge.thread: nil context")
	assert.That(root != "", "Bridge.thread: empty thread root")
	sessionID, joined, clean := b.threadState(ctx, root)
	return threadInfo{sessionID: sessionID, joined: joined, clean: clean}
}

// threadState returns the claude session to resume in a thread, whether this
// agent has replied there, and whether its next session was asked to start
// clean (by !new, or by outgrowing MaxSessionTokens). It answers from memory
// first, then (after a restart or an idle retirement) from the newest of the
// agent's replies in the thread that names a session; a reply marked with
// sessionResetProp ends the search, since the session before it was retired.
func (b *Bridge) threadState(ctx context.Context, root string) (sessionID string, joined, clean bool) {
	assert.That(root != "", "Bridge.threadState: empty thread root")
	assert.That(b.client != nil, "Bridge.threadState: nil chitd client")
	b.mu.Lock()
	sessionID, ok := b.sessions[root]
	clean = b.cleanStart.has(root)
	b.mu.Unlock()
	if ok {
		return sessionID, true, clean
	}

	posts, err := b.client.GetThread(ctx, root)
	if err != nil {
		slog.Warn("failed to fetch thread for session recovery; starting fresh session",
			"root_id", root, "error", err)
		return "", false, false
	}
	// Thread posts are oldest-first; scan newest-first for the latest session.
	for i := len(posts) - 1; i >= 0; i-- {
		p := posts[i]
		if p.UserID != b.agentUserID {
			continue
		}
		joined = true
		if reset, ok := p.Props[sessionResetProp].(bool); ok && reset {
			clean = true
			break
		}
		if id, ok := p.Props[sessionProp].(string); ok && id != "" {
			sessionID = id
			break
		}
	}
	if joined {
		b.mu.Lock()
		b.sessions[root] = sessionID
		if clean {
			b.cleanStart.add(root)
		}
		b.mu.Unlock()
	}
	assert.That(sessionID == "" || joined, "Bridge.threadState: session found without the agent's reply")
	assert.That(!clean || joined, "Bridge.threadState: clean start without the agent's reply")
	return sessionID, joined, clean
}

// participates reports whether this agent has replied in a thread, so a
// follow-up there needs no mention. The answer is remembered either way; the
// agent's own replies keep it current.
func (b *Bridge) participates(ctx context.Context, root string) bool {
	assert.That(root != "", "Bridge.participates: empty thread root")
	joined := b.thread(ctx, root).joined
	b.mu.Lock()
	defer b.mu.Unlock()
	if joined {
		b.joined.add(root)
		b.bystander.remove(root)
		assert.That(b.joined.has(root), "Bridge.participates: joined thread not remembered")
	} else {
		b.bystander.add(root)
	}
	return joined
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
	assert.That(len(raw) > 0, "postFromEventData: empty encoding decoded to a post")
	return &post
}

// mentionsUser reports whether the post's parsed mentions include userID.
func mentionsUser(post *model.Post, userID string) bool {
	assert.That(post != nil, "mentionsUser: nil post")
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
