package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

const (
	testAgentID       = "agent-user-1"
	testAgentUsername = "chit-agent"
	testKratosID      = "kratos-agent-1"
	testChannelID     = "chan-1"
	testOtherUser     = "human-user-1"
	testAgentID2      = "agent-user-2"
	fakeSessionID     = "11111111-2222-3333-4444-555555555555"
	fakeSessionID2    = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// writeFakeClaude creates an executable script that records its argv (one per
// line) into argvFile and prints the given JSON on stdout.
func writeFakeClaude(t *testing.T, dir, argvFile, stdout string, exitCode int) string {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
cat <<'FAKEEOF'
%s
FAKEEOF
exit %d
`, argvFile, stdout, exitCode)
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	return path
}

func fakeResultJSON(sessionID string) string {
	return fmt.Sprintf(`{
		"result": "Here is the plan.",
		"session_id": %q,
		"is_error": false,
		"num_turns": 3,
		"total_cost_usd": 0.42,
		"usage": {
			"input_tokens": 4500,
			"output_tokens": 1200,
			"cache_read_input_tokens": 38000,
			"cache_creation_input_tokens": 900
		}
	}`, sessionID)
}

// fakeChitd is a minimal chitd stand-in: /users/me, GET /users/{id},
// POST /posts (recorded), and GET /posts/{id}/thread.
type fakeChitd struct {
	mu           sync.Mutex
	posts        []*model.Post
	thread       []*model.Post
	users        map[string]*model.User // user ID → user; absent → 404
	lastAuth     string
	getUserCalls int
}

func (f *fakeChitd) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lastAuth = r.Header.Get("X-User-Id")
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(&model.User{
			ID:        testAgentID,
			KratosID:  testKratosID,
			Username:  testAgentUsername,
			ActorType: model.ActorTypeAgent,
		})
	})
	mux.HandleFunc("GET /api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.getUserCalls++
		user, ok := f.users[r.PathValue("id")]
		f.mu.Unlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(user)
	})
	mux.HandleFunc("POST /api/v1/posts", func(w http.ResponseWriter, r *http.Request) {
		var post model.Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		post.ID = fmt.Sprintf("post-%d", len(f.posts)+1)
		post.UserID = testAgentID
		f.mu.Lock()
		f.posts = append(f.posts, &post)
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(&post)
	})
	mux.HandleFunc("GET /api/v1/posts/{id}/thread", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(&model.PostList{Order: f.thread})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (f *fakeChitd) createdPosts() []*model.Post {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*model.Post, len(f.posts))
	copy(out, f.posts)
	return out
}

func (f *fakeChitd) userLookups() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getUserCalls
}

// humanUsers is the default user table: one ordinary human author.
func humanUsers() map[string]*model.User {
	return map[string]*model.User{
		testOtherUser: {ID: testOtherUser, Username: "alice", ActorType: model.ActorTypeUser},
	}
}

func testConfig(t *testing.T, serverURL, claudeBin string) *Config {
	t.Helper()
	cfg := Defaults()
	cfg.ServerURL = serverURL
	cfg.AgentKratosID = testKratosID
	cfg.Channels = []string{testChannelID}
	cfg.ClaudeBin = claudeBin
	cfg.WorkDir = t.TempDir()
	return cfg
}

func postedEvent(post *model.Post) *model.WebSocketEvent {
	data, _ := json.Marshal(post)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	return &model.WebSocketEvent{Event: model.WebSocketEventPosted, Data: m}
}

func waitForPosts(t *testing.T, f *fakeChitd, n int) []*model.Post {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if posts := f.createdPosts(); len(posts) >= n {
			return posts
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d posts; got %d", n, len(f.createdPosts()))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// ─── ClaudeRunner ────────────────────────────────────────────────

func TestClaudeRunner_NewSessionAndResumeArgv(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	cfg := testConfig(t, "http://unused", bin)
	runner := NewClaudeRunner(cfg)

	// New session: no --resume.
	res, err := runner.Run(context.Background(), "plan the migration", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.SessionID != fakeSessionID {
		t.Errorf("SessionID: got %q", res.SessionID)
	}
	argv, _ := os.ReadFile(argvFile)
	if strings.Contains(string(argv), "--resume") {
		t.Errorf("new session must not pass --resume; argv:\n%s", argv)
	}
	if !strings.Contains(string(argv), "plan the migration") {
		t.Errorf("prompt missing from argv:\n%s", argv)
	}
	if !strings.Contains(string(argv), "dontAsk") {
		t.Errorf("permission mode missing from argv:\n%s", argv)
	}

	// Resume: --resume <id> present.
	if _, err := runner.Run(context.Background(), "continue", fakeSessionID); err != nil {
		t.Fatalf("Run (resume): %v", err)
	}
	argv, _ = os.ReadFile(argvFile)
	lines := strings.Split(strings.TrimSpace(string(argv)), "\n")
	found := false
	for i, l := range lines {
		if l == "--resume" && i+1 < len(lines) && lines[i+1] == fakeSessionID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --resume %s in argv:\n%s", fakeSessionID, argv)
	}
}

func TestClaudeRunner_ModelFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	// Configured model is passed through as --model <id>.
	cfg := testConfig(t, "http://unused", bin)
	cfg.Model = "claude-fable-5"
	if _, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	argv, _ := os.ReadFile(argvFile)
	lines := strings.Split(strings.TrimSpace(string(argv)), "\n")
	found := false
	for i, l := range lines {
		if l == "--model" && i+1 < len(lines) && lines[i+1] == "claude-fable-5" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --model claude-fable-5 in argv:\n%s", argv)
	}

	// Unset model omits the flag entirely, leaving the CLI default in place.
	cfg.Model = ""
	if _, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	argv, _ = os.ReadFile(argvFile)
	if strings.Contains(string(argv), "--model") {
		t.Errorf("empty model must not pass --model; argv:\n%s", argv)
	}
}

func TestClaudeRunner_FailureIncludesStderr(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	script := "#!/bin/sh\necho 'boom: credentials missing' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := NewClaudeRunner(testConfig(t, "http://unused", bin))
	_, err := runner.Run(context.Background(), "hi", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "credentials missing") {
		t.Errorf("error should carry stderr tail, got: %v", err)
	}
}

func TestClaudeRunner_Footer(t *testing.T) {
	cfg := testConfig(t, "http://unused", "claude")
	runner := NewClaudeRunner(cfg)

	var res ClaudeResult
	if err := json.Unmarshal([]byte(fakeResultJSON(fakeSessionID)), &res); err != nil {
		t.Fatal(err)
	}
	footer := runner.Footer(&res)

	// context = (38000 + 4500) / 200000 = 21%
	for _, want := range []string{"in 4.5k", "out 1.2k", "cache 38.0k", "$0.42", "context ~21%"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer missing %q: %s", want, footer)
		}
	}
}

// ─── Event filtering ─────────────────────────────────────────────

func TestShouldEnqueue(t *testing.T) {
	cfg := testConfig(t, "http://unused", "claude")
	b := New(cfg)
	b.agentUserID = testAgentID

	tests := []struct {
		name string
		post *model.Post
		want bool
	}{
		{"normal post in configured channel", &model.Post{ChannelID: testChannelID, UserID: testOtherUser}, true},
		{"other channel ignored", &model.Post{ChannelID: "elsewhere", UserID: testOtherUser}, false},
		{"own post ignored (no loops)", &model.Post{ChannelID: testChannelID, UserID: testAgentID}, false},
		{"command response ignored", &model.Post{ChannelID: testChannelID, UserID: testOtherUser, Type: "command_response"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := b.shouldEnqueue(tc.post); got != tc.want {
				t.Errorf("shouldEnqueue = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("mention gating", func(t *testing.T) {
		cfg := testConfig(t, "http://unused", "claude")
		cfg.RequireMention = true
		b := New(cfg)
		b.agentUserID = testAgentID

		without := &model.Post{ChannelID: testChannelID, UserID: testOtherUser}
		if b.shouldEnqueue(without) {
			t.Error("post without mention should be ignored when RequireMention")
		}
		with := &model.Post{ChannelID: testChannelID, UserID: testOtherUser,
			Props: map[string]any{"mentions": []any{testAgentID}}}
		if !b.shouldEnqueue(with) {
			t.Error("post mentioning the agent should be handled")
		}
	})
}

// syncBuffer is a concurrency-safe io.Writer for capturing slog output.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// Membership in a channel the bridge does not serve is not under the
// operator's control (open-channel auto-add, DMs, any teammate inviting the
// agent), so those posts are dropped — but reported once per channel so the
// operator can see the agent was pulled somewhere unexpected.
func TestBridge_WarnsOncePerOffAllowlistChannel(t *testing.T) {
	var logs syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	b := New(testConfig(t, "http://unused", "claude"))
	b.agentUserID = testAgentID

	// Repeated posts from one unserved channel warn exactly once.
	stray := &model.Post{ChannelID: "stray-chan", UserID: testOtherUser}
	for range 3 {
		if b.shouldEnqueue(stray) {
			t.Fatal("post from an unserved channel must be dropped")
		}
	}
	// A second unserved channel gets its own warning.
	b.shouldEnqueue(&model.Post{ChannelID: "dm-chan", UserID: testOtherUser})
	// The configured channel never warns.
	if !b.shouldEnqueue(&model.Post{ChannelID: testChannelID, UserID: testOtherUser}) {
		t.Fatal("post in the configured channel must be handled")
	}

	out := logs.String()
	if n := strings.Count(out, "stray-chan"); n != 1 {
		t.Errorf("want exactly 1 warning for stray-chan, got %d:\n%s", n, out)
	}
	if n := strings.Count(out, "dm-chan"); n != 1 {
		t.Errorf("want exactly 1 warning for dm-chan, got %d:\n%s", n, out)
	}
	if strings.Contains(out, testChannelID) {
		t.Errorf("configured channel must not warn:\n%s", out)
	}
}

// ─── Multi-agent gating ──────────────────────────────────────────

func TestShouldRun(t *testing.T) {
	tests := []struct {
		name          string
		authorID      string
		replyToAgents bool
		maxHops       int
		content       string
		props         map[string]any
		want          bool
	}{
		{name: "human author always runs", authorID: testOtherUser, want: true},
		{name: "agent author dropped by default", authorID: testAgentID2, want: false},
		{name: "bot author dropped by default", authorID: "bot-user-1", want: false},
		{name: "agent author without mention", authorID: testAgentID2,
			replyToAgents: true, maxHops: 2, content: "done, shipping it", want: false},
		{name: "agent @all is not a direct mention", authorID: testAgentID2,
			replyToAgents: true, maxHops: 2, content: "@all ship it", want: false},
		{name: "agent naming this agent runs", authorID: testAgentID2,
			replyToAgents: true, maxHops: 2, content: "@chit-agent thoughts?", want: true},
		{name: "hop cap reached", authorID: testAgentID2,
			replyToAgents: true, maxHops: 2, content: "@chit-agent again?",
			props: map[string]any{hopsProp: float64(2)}, want: false},
		{name: "under hop cap", authorID: testAgentID2,
			replyToAgents: true, maxHops: 2, content: "@chit-agent again?",
			props: map[string]any{hopsProp: float64(1)}, want: true},
		{name: "unresolvable author treated as human", authorID: "ghost-user",
			content: "hello", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chitd := &fakeChitd{users: map[string]*model.User{
				testOtherUser: {ID: testOtherUser, Username: "alice", ActorType: model.ActorTypeUser},
				testAgentID2:  {ID: testAgentID2, Username: "other-agent", ActorType: model.ActorTypeAgent},
				"bot-user-1":  {ID: "bot-user-1", Username: "helper-bot", ActorType: model.ActorTypeBot},
			}}
			srv := chitd.server(t)

			cfg := testConfig(t, srv.URL, "claude")
			cfg.ReplyToAgents = tc.replyToAgents
			cfg.MaxAgentHops = tc.maxHops
			b := New(cfg)
			b.agentUserID = testAgentID
			b.agentUsername = testAgentUsername

			post := &model.Post{ID: "p-1", ChannelID: testChannelID,
				UserID: tc.authorID, Content: tc.content, Props: tc.props}
			got, _ := b.shouldRun(context.Background(), post)
			if got != tc.want {
				t.Errorf("shouldRun = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestActorTypeCacheIsSingleFetch(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)

	b := New(testConfig(t, srv.URL, "claude"))
	b.agentUserID = testAgentID
	b.agentUsername = testAgentUsername

	ctx := context.Background()
	for range 3 {
		if got := b.actorType(ctx, testOtherUser); got != model.ActorTypeUser {
			t.Fatalf("actorType = %q, want %q", got, model.ActorTypeUser)
		}
	}
	if n := chitd.userLookups(); n != 1 {
		t.Errorf("resolved author should be fetched once, got %d lookups", n)
	}

	// Failures are cached too, so an unknown author is not re-fetched forever.
	for range 3 {
		if got := b.actorType(ctx, "ghost-user"); got != "" {
			t.Fatalf("failed lookup should yield %q, got %q", "", got)
		}
	}
	if n := chitd.userLookups(); n != 2 {
		t.Errorf("failed lookup should be negative-cached; got %d total lookups, want 2", n)
	}
}

func TestMentionsUsernameDirectly(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"@chit-agent please review", true},
		{"hey @chit-agent, thoughts?", true},
		{"@CHIT-AGENT shouting", true},
		{"@all ship it", false},
		{"@channel heads up", false},
		{"@chit-agent-two is someone else", false},
		{"no mention at all", false},
		{"email chit-agent@example.com is not a mention", false},
	}
	for _, tc := range tests {
		t.Run(tc.content, func(t *testing.T) {
			if got := mentionsUsernameDirectly(tc.content, testAgentUsername); got != tc.want {
				t.Errorf("mentionsUsernameDirectly(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

// ─── End-to-end through handleEvent ──────────────────────────────

func TestBridge_PostTriggersRunAndThreadedReply(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	cfg := testConfig(t, srv.URL, bin)
	b := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	me, err := b.client.Me(ctx)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	b.agentUserID = me.ID

	rootPost := &model.Post{ID: "root-1", ChannelID: testChannelID, UserID: testOtherUser, Content: "plan service X"}
	b.handleEvent(ctx, postedEvent(rootPost))

	posts := waitForPosts(t, chitd, 1)
	reply := posts[0]
	if reply.RootID != "root-1" {
		t.Errorf("reply must thread on the root post: got root_id=%q", reply.RootID)
	}
	if !strings.Contains(reply.Content, "Here is the plan.") {
		t.Errorf("reply missing claude result: %s", reply.Content)
	}
	if !strings.Contains(reply.Content, "context ~21%") {
		t.Errorf("reply missing footer: %s", reply.Content)
	}
	if got := reply.Props[sessionProp]; got != fakeSessionID {
		t.Errorf("session prop: got %v, want %s", got, fakeSessionID)
	}

	// A thread reply resumes the same session.
	followUp := &model.Post{ID: "msg-2", ChannelID: testChannelID, UserID: testOtherUser,
		RootID: "root-1", Content: "looks good, proceed"}
	b.handleEvent(ctx, postedEvent(followUp))

	waitForPosts(t, chitd, 2)
	argv, _ := os.ReadFile(argvFile)
	if !strings.Contains(string(argv), "--resume\n"+fakeSessionID) {
		t.Errorf("follow-up must resume session %s; argv:\n%s", fakeSessionID, argv)
	}
}

func TestBridge_SessionRecoveryFromThreadProps(t *testing.T) {
	chitd := &fakeChitd{
		thread: []*model.Post{
			{ID: "root-1", ChannelID: testChannelID, UserID: testOtherUser, Content: "task"},
			{ID: "old-reply", ChannelID: testChannelID, UserID: testAgentID, RootID: "root-1",
				Props: map[string]any{sessionProp: fakeSessionID2}},
		},
	}
	srv := chitd.server(t)

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID2), 0)

	b := New(testConfig(t, srv.URL, bin))
	b.agentUserID = testAgentID

	// Fresh bridge (empty in-memory map) must recover the session from props.
	got := b.lookupSession(context.Background(), "root-1")
	if got != fakeSessionID2 {
		t.Fatalf("lookupSession: got %q, want %q", got, fakeSessionID2)
	}
}

func TestBridge_ReplyCarriesHopCount(t *testing.T) {
	users := humanUsers()
	users[testAgentID2] = &model.User{ID: testAgentID2, Username: "other-agent", ActorType: model.ActorTypeAgent}
	chitd := &fakeChitd{users: users}
	srv := chitd.server(t)

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	cfg := testConfig(t, srv.URL, bin)
	cfg.ReplyToAgents = true
	b := New(cfg)
	b.agentUserID = testAgentID
	b.agentUsername = testAgentUsername

	ctx := context.Background()

	// A human turn resets the chain: the reply starts at hop 0.
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "plan service X"}))
	posts := waitForPosts(t, chitd, 1)
	// Props round-trip through JSON, so read via agentHops rather than
	// comparing the any-typed float64 the decoder produces.
	if got := agentHops(posts[0]); got != 0 {
		t.Errorf("reply to a human: hops = %d, want 0", got)
	}

	// An agent naming this one extends the chain to hop 1.
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-2", ChannelID: testChannelID,
		UserID: testAgentID2, Content: "@chit-agent can you review?"}))
	posts = waitForPosts(t, chitd, 2)
	if got := agentHops(posts[1]); got != 1 {
		t.Errorf("reply to an agent: hops = %d, want 1", got)
	}

	// At the cap the chain stops: no third post is ever created.
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-3", ChannelID: testChannelID,
		UserID: testAgentID2, Content: "@chit-agent one more?",
		Props: map[string]any{hopsProp: float64(2)}}))
	time.Sleep(200 * time.Millisecond)
	if n := len(chitd.createdPosts()); n != 2 {
		t.Errorf("hop cap should suppress the reply; got %d posts, want 2", n)
	}
}

// Two bridges serving one thread must each resume their own claude session.
func TestBridge_TwoAgentsIndependentSessions(t *testing.T) {
	chitd := &fakeChitd{
		users: humanUsers(),
		thread: []*model.Post{
			{ID: "root-1", ChannelID: testChannelID, UserID: testOtherUser, Content: "task"},
			{ID: "reply-a", ChannelID: testChannelID, UserID: testAgentID, RootID: "root-1",
				Props: map[string]any{sessionProp: fakeSessionID}},
			{ID: "reply-b", ChannelID: testChannelID, UserID: testAgentID2, RootID: "root-1",
				Props: map[string]any{sessionProp: fakeSessionID2}},
		},
	}
	srv := chitd.server(t)

	agentA := New(testConfig(t, srv.URL, "claude"))
	agentA.agentUserID = testAgentID
	agentB := New(testConfig(t, srv.URL, "claude"))
	agentB.agentUserID = testAgentID2

	ctx := context.Background()
	if got := agentA.lookupSession(ctx, "root-1"); got != fakeSessionID {
		t.Errorf("agent A session: got %q, want %q", got, fakeSessionID)
	}
	if got := agentB.lookupSession(ctx, "root-1"); got != fakeSessionID2 {
		t.Errorf("agent B session: got %q, want %q", got, fakeSessionID2)
	}
}

func TestBridge_RunFailurePostsErrorReply(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)

	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'auth expired' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	b := New(testConfig(t, srv.URL, bin))
	b.agentUserID = testAgentID

	ctx := context.Background()
	post := &model.Post{ID: "root-9", ChannelID: testChannelID, UserID: testOtherUser, Content: "do a thing"}
	b.handleEvent(ctx, postedEvent(post))

	posts := waitForPosts(t, chitd, 1)
	if !strings.Contains(posts[0].Content, "Claude run failed") || !strings.Contains(posts[0].Content, "auth expired") {
		t.Errorf("expected error reply carrying stderr, got: %s", posts[0].Content)
	}
	if posts[0].RootID != "root-9" {
		t.Errorf("error reply must stay in the thread: root_id=%q", posts[0].RootID)
	}
}
