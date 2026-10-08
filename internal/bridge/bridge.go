package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/infrashift/chit/internal/chitclient"
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
	channels := make(map[string]struct{}, len(cfg.Channels))
	for _, id := range cfg.Channels {
		channels[id] = struct{}{}
	}
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
	me, err := b.client.Me(ctx)
	if err != nil {
		return fmt.Errorf("resolve agent identity: %w", err)
	}
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
	b.client.Listen(ctx, func(event *model.WebSocketEvent) {
		b.handleEvent(workCtx, event)
	})

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
	for _, id := range b.cfg.Channels {
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
	channel, err := b.client.GetChannel(ctx, channelID)
	if err != nil {
		return false, err
	}
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
	addressed, ok := b.shouldEnqueue(post)
	if !ok || !b.firstSighting(post.ID) {
		return
	}

	root := post.RootID
	if root == "" {
		root = post.ID
	}

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
	sent := false
	select {
	case queue <- item:
		sent = true
	default:
	}
	// The agent is about to answer here, so the thread is joined now: a
	// follow-up sent before that answer lands is for the agent too.
	if sent && addressed {
		b.joined.add(root)
		b.bystander.remove(root)
	}
	b.mu.Unlock()

	if !sent {
		slog.Warn("thread queue full, dropping message", "root_id", root, "post_id", post.ID)
		if addressed {
			b.noticeQueueFull(ctx, root, post)
		}
	}
}

// firstSighting records postID and reports whether it is new. A post delivered
// twice would otherwise run claude twice and answer twice.
func (b *Bridge) firstSighting(postID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.seen.add(postID)
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
// resolved.
func (b *Bridge) actorType(ctx context.Context, userID string) string {
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
	defer b.wg.Done()
	idle := time.NewTimer(b.idleTimeout)
	defer idle.Stop()
	for {
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

// retire removes an idle thread's worker state, unless a post arrived in the
// meantime. Sends happen under mu, so once the queue is gone from the map
// nothing more can arrive on it.
func (b *Bridge) retire(root string, queue chan queued) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(queue) > 0 {
		return false
	}
	delete(b.queues, root)
	delete(b.sessions, root)
	delete(b.busy, root)
	return true
}

// abandon tells a thread, once, that the addressed posts left in its queue
// (and taken, if non-nil) were never run.
func (b *Bridge) abandon(ctx context.Context, root string, queue chan queued, taken *queued) {
	var last *model.Post
	if taken != nil && taken.addressed {
		last = taken.post
	}
	for {
		select {
		case q := <-queue:
			if q.addressed {
				last = q.post
			}
			continue
		default:
		}
		break
	}
	if last == nil {
		return
	}
	slog.Info("shutting down with unrun posts in thread", "root_id", root, "last_post_id", last.ID)
	b.reply(ctx, last.ChannelID, root, shutdownNotice, map[string]any{hopsProp: agentHops(last)})
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

	// A new session in an existing thread is given the thread so far,
	// unless !new or a retirement asked for a clean start.
	sessionID, seed := "", false
	if root != post.ID {
		var clean bool
		sessionID, _, clean = b.threadState(ctx, root)
		seed = sessionID == "" && !clean
	}

	// The run gets its own context so !stop can cancel it alone.
	runCtx, cancelRun := context.WithCancelCause(ctx)
	b.mu.Lock()
	b.running[root] = runState{cancel: cancelRun, started: time.Now()}
	b.mu.Unlock()
	progressDone := b.showProgress(ctx, post.ChannelID, root, hops)
	defer progressDone()
	result, notice, err := b.runClaude(runCtx, root, post, sessionID, seed)
	b.mu.Lock()
	delete(b.running, root)
	b.joined.add(root)
	b.bystander.remove(root)
	b.mu.Unlock()
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
	select {
	case b.runSlots <- struct{}{}:
	case <-b.stopping:
		return nil, "", errShuttingDown
	case <-ctx.Done():
		return nil, "", errShuttingDown
	}
	defer func() { <-b.runSlots }()
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
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
	defer cancel()
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

// threadState returns the claude session to resume in a thread, whether this
// agent has replied there, and whether its next session was asked to start
// clean (by !new, or by outgrowing MaxSessionTokens). It answers from memory
// first, then (after a restart or an idle retirement) from the newest of the
// agent's replies in the thread that names a session; a reply marked with
// sessionResetProp ends the search, since the session before it was retired.
func (b *Bridge) threadState(ctx context.Context, root string) (sessionID string, joined, clean bool) {
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
		if reset, _ := p.Props[sessionResetProp].(bool); reset {
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
	return sessionID, joined, clean
}

// participates reports whether this agent has replied in a thread, so a
// follow-up there needs no mention. The answer is remembered either way; the
// agent's own replies keep it current.
func (b *Bridge) participates(ctx context.Context, root string) bool {
	_, joined, _ := b.threadState(ctx, root)
	b.mu.Lock()
	defer b.mu.Unlock()
	if joined {
		b.joined.add(root)
		b.bystander.remove(root)
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
