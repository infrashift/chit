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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

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
// line) into argvFile and its stdin into argvFile+".stdin", then prints the
// given JSON on stdout.
func writeFakeClaude(t *testing.T, dir, argvFile, stdout string, exitCode int) string {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
cat > %q
cat <<'FAKEEOF'
%s
FAKEEOF
exit %d
`, argvFile, argvFile+".stdin", stdout, exitCode)
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
	threadCalls  int

	// For Run: channels the agent can read (absent → 404, or channelStatus
	// when set), the channel IDs it is a member of, and the events the
	// WebSocket delivers.
	channels      map[string]*model.Channel
	channelStatus int
	memberOf      []string
	events        chan *model.WebSocketEvent
	wsConnects    int
	deleted       []string
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
		f.mu.Lock()
		post.ID = fmt.Sprintf("post-%d", len(f.posts)+1)
		post.UserID = testAgentID
		f.posts = append(f.posts, &post)
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(&post)
	})
	mux.HandleFunc("DELETE /api/v1/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.deleted = append(f.deleted, r.PathValue("id"))
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	})
	mux.HandleFunc("GET /api/v1/posts/{id}/thread", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.threadCalls++
		_ = json.NewEncoder(w).Encode(&model.PostList{Order: f.thread})
	})
	mux.HandleFunc("GET /api/v1/channels/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		channel, ok := f.channels[r.PathValue("id")]
		status := f.channelStatus
		f.mu.Unlock()
		switch {
		case status != 0:
			http.Error(w, "unavailable", status)
		case !ok:
			http.Error(w, "not found", http.StatusNotFound)
		default:
			_ = json.NewEncoder(w).Encode(channel)
		}
	})
	mux.HandleFunc("GET /api/v1/users/me/teams/{id}/channels/members", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		members := []*model.ChannelMember{}
		for _, id := range f.memberOf {
			members = append(members, &model.ChannelMember{ChannelID: id, UserID: testAgentID})
		}
		_ = json.NewEncoder(w).Encode(members)
	})
	mux.HandleFunc("GET /api/v1/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		f.mu.Lock()
		f.wsConnects++
		f.mu.Unlock()
		closed := make(chan struct{})
		go func() {
			defer close(closed)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		for {
			select {
			case ev := <-f.events:
				if err := conn.WriteJSON(ev); err != nil {
					return
				}
			case <-closed:
				return
			}
		}
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

// testConfig answers every post, so tests about running and replying need
// not mention the agent; the mention gating has tests of its own.
func testConfig(t *testing.T, serverURL, claudeBin string) *Config {
	t.Helper()
	cfg := Defaults()
	cfg.RequireMention = false
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

// readArgv returns the argv the fake claude recorded, one element per entry.
func readArgv(t *testing.T, argvFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// hasFlag reports whether argv holds flag immediately followed by value.
func hasFlag(argv []string, flag, value string) bool {
	for i, a := range argv {
		if a == flag && i+1 < len(argv) && argv[i+1] == value {
			return true
		}
	}
	return false
}

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
	argv := readArgv(t, argvFile)
	if slices.Contains(argv, "--resume") {
		t.Errorf("new session must not pass --resume; argv: %q", argv)
	}
	if !hasFlag(argv, "--permission-mode", "dontAsk") {
		t.Errorf("permission mode missing from argv: %q", argv)
	}

	// Resume: --resume <id> present.
	if _, err := runner.Run(context.Background(), "continue", fakeSessionID); err != nil {
		t.Fatalf("Run (resume): %v", err)
	}
	if argv := readArgv(t, argvFile); !hasFlag(argv, "--resume", fakeSessionID) {
		t.Errorf("expected --resume %s in argv: %q", fakeSessionID, argv)
	}
}

// The prompt is the CLI's positional argument and -p is a boolean, so a
// prompt in argv that reads like an option is parsed as one: a post saying
// "--version" would print the version instead of reaching Claude. The prompt
// therefore travels on stdin, and argv carries only the bridge's own flags.
func TestClaudeRunner_PromptOnStdinNotArgv(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	for _, prompt := range []string{"--version", "--continue", "plan the migration"} {
		if _, err := NewClaudeRunner(testConfig(t, "http://unused", bin)).Run(context.Background(), prompt, ""); err != nil {
			t.Fatalf("Run(%q): %v", prompt, err)
		}
		if argv := readArgv(t, argvFile); slices.Contains(argv, prompt) {
			t.Errorf("prompt %q must not be in argv: %q", prompt, argv)
		}
		stdin, _ := os.ReadFile(argvFile + ".stdin")
		if string(stdin) != prompt {
			t.Errorf("stdin = %q, want %q", stdin, prompt)
		}
	}
}

// Runs are isolated from the host's Claude Code setup: no MCP server loads
// unless configured, and only the configured settings sources are read.
func TestClaudeRunner_IsolatedFromHostConfig(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	cfg := testConfig(t, "http://unused", bin)
	if _, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	argv := readArgv(t, argvFile)
	if !slices.Contains(argv, "--strict-mcp-config") {
		t.Errorf("--strict-mcp-config missing: %q", argv)
	}
	if slices.Contains(argv, "--mcp-config") {
		t.Errorf("no --mcp-config unless configured: %q", argv)
	}
	if !hasFlag(argv, "--setting-sources", "project") {
		t.Errorf("default --setting-sources project missing: %q", argv)
	}

	cfg.MCPConfig = "/etc/chit-claude/mcp.json"
	cfg.SettingSources = SettingSourcesAll
	if _, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	argv = readArgv(t, argvFile)
	if !slices.Contains(argv, "--strict-mcp-config") || !hasFlag(argv, "--mcp-config", cfg.MCPConfig) {
		t.Errorf("configured MCP servers must be the only ones: %q", argv)
	}
	if slices.Contains(argv, "--setting-sources") {
		t.Errorf("%q must omit --setting-sources: %q", SettingSourcesAll, argv)
	}
}

func TestClaudeRunner_ModelFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)

	// Configured model is passed through as --model <id>.
	cfg := testConfig(t, "http://unused", bin)
	cfg.Model = "claude-fable-5-1"
	if _, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if argv := readArgv(t, argvFile); !hasFlag(argv, "--model", "claude-fable-5-1") {
		t.Errorf("expected --model claude-fable-5-1 in argv: %q", argv)
	}

	// Unset model omits the flag entirely, leaving the CLI default in place.
	cfg.Model = ""
	if _, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if argv := readArgv(t, argvFile); slices.Contains(argv, "--model") {
		t.Errorf("empty model must not pass --model; argv: %q", argv)
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

	// No per-call breakdown and no reported window: context is the run's
	// whole input, cache writes included, over CHIT_CLAUDE_CONTEXT_WINDOW:
	// (4500 + 38000 + 900) / 200000 = 21%.
	for _, want := range []string{"in 4.5k", "out 1.2k", "cache read 38.0k", "write 900", "$0.42", "context ~21%"} {
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
			if _, got := b.shouldEnqueue(tc.post); got != tc.want {
				t.Errorf("shouldEnqueue = %v, want %v", got, tc.want)
			}
		})
	}
}

// With RequireMention (the default), a post must name the agent; a broadcast
// counts only with AnswerBroadcasts; and with FollowThreads (the default) a
// reply in a thread the agent has joined needs no mention, while a reply in a
// thread it knows nothing about goes to the worker to check.
func TestShouldEnqueue_MentionGating(t *testing.T) {
	broadcast := map[string]any{"mentions": []any{testAgentID}} // as the server expands @channel
	cases := []struct {
		name             string
		followThreads    bool
		answerBroadcasts bool
		post             *model.Post
		wantOK           bool
		wantAddressed    bool
	}{
		{name: "no mention", followThreads: true,
			post: &model.Post{Content: "anyone around?"}},
		{name: "direct mention", followThreads: true,
			post: &model.Post{Content: "@chit-agent plan it"}, wantOK: true, wantAddressed: true},
		{name: "@channel is not addressed to the agent", followThreads: true,
			post: &model.Post{Content: "@channel standup", Props: broadcast}},
		{name: "@channel with AnswerBroadcasts", followThreads: true, answerBroadcasts: true,
			post: &model.Post{Content: "@channel standup", Props: broadcast}, wantOK: true, wantAddressed: true},
		{name: "follow-up in a joined thread", followThreads: true,
			post: &model.Post{RootID: "joined-root", Content: "and then?"}, wantOK: true, wantAddressed: true},
		{name: "follow-up in a bystander thread", followThreads: true,
			post: &model.Post{RootID: "other-root", Content: "lol"}},
		{name: "follow-up in an unknown thread is checked later", followThreads: true,
			post: &model.Post{RootID: "new-root", Content: "and then?"}, wantOK: true},
		{name: "joined thread without FollowThreads",
			post: &model.Post{RootID: "joined-root", Content: "and then?"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(t, "http://unused", "claude")
			cfg.RequireMention = true
			cfg.FollowThreads = tc.followThreads
			cfg.AnswerBroadcasts = tc.answerBroadcasts
			b := New(cfg)
			b.agentUserID = testAgentID
			b.agentUsername = testAgentUsername
			b.joined.add("joined-root")
			b.bystander.add("other-root")

			tc.post.ChannelID = testChannelID
			tc.post.UserID = testOtherUser
			addressed, ok := b.shouldEnqueue(tc.post)
			if ok != tc.wantOK || addressed != tc.wantAddressed {
				t.Errorf("shouldEnqueue = (addressed %v, ok %v), want (%v, %v)",
					addressed, ok, tc.wantAddressed, tc.wantOK)
			}
		})
	}
}

// End to end with the default gating: an unmentioned reply in a thread the
// agent has not joined is checked against the thread once and then ignored
// without a lookup; once the agent is named there, later replies need no
// mention.
func TestBridge_FollowsThreadsItJoined(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers(), thread: []*model.Post{
		{ID: "root-1", ChannelID: testChannelID, UserID: testOtherUser, Content: "humans talking"},
	}}
	srv := chitd.server(t)
	cfg := testConfig(t, srv.URL, writeEchoClaude(t, t.TempDir()))
	cfg.RequireMention = true
	b, ctx := liveBridge(t, cfg)

	say := func(id, content string) {
		b.handleEvent(ctx, postedEvent(&model.Post{ID: id, ChannelID: testChannelID,
			UserID: testOtherUser, RootID: "root-1", Content: content}))
	}
	threadCalls := func() int {
		chitd.mu.Lock()
		defer chitd.mu.Unlock()
		return chitd.threadCalls
	}

	say("msg-1", "chatting")
	waitFor(t, "the thread to be checked", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.bystander.has("root-1")
	})
	say("msg-2", "still chatting")
	say("msg-3", "@chit-agent summarise")
	// Queued behind msg-3 in the same thread, so its answer lands second.
	say("msg-4", "and the risks")

	posts := waitForPosts(t, chitd, 2)
	if !strings.HasPrefix(posts[0].Content, "re: [@alice] @chit-agent summarise") ||
		!strings.HasPrefix(posts[1].Content, "re: [@alice] and the risks") {
		t.Errorf("want answers to the mention and the follow-up, got %q, %q", posts[0].Content, posts[1].Content)
	}
	if n := threadCalls(); n != 3 {
		// One check for msg-1; for msg-3, one session lookup and one fetch to
		// seed its new session with the thread so far. msg-2 was dropped on
		// the read loop, and msg-4 resumed the session msg-3 started.
		t.Errorf("thread fetched %d times, want 3", n)
	}
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
		if _, ok := b.shouldEnqueue(stray); ok {
			t.Fatal("post from an unserved channel must be dropped")
		}
	}
	// A second unserved channel gets its own warning.
	b.shouldEnqueue(&model.Post{ChannelID: "dm-chan", UserID: testOtherUser})
	// The configured channel never warns.
	if _, ok := b.shouldEnqueue(&model.Post{ChannelID: testChannelID, UserID: testOtherUser}); !ok {
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
		// Only bridges write the hop count, so a post carrying one is an
		// agent's even when its author cannot be looked up. Reading it as
		// human would reset the count and lift the hop cap.
		{name: "unresolvable author with a hop count is an agent", authorID: "ghost-user",
			content: "@chit-agent thoughts?", props: map[string]any{hopsProp: float64(1)}, want: false},
		{name: "unresolvable agent still obeys the hop cap", authorID: "ghost-user",
			replyToAgents: true, maxHops: 2, content: "@chit-agent again?",
			props: map[string]any{hopsProp: float64(2)}, want: false},
		{name: "unresolvable agent under the cap may be answered", authorID: "ghost-user",
			replyToAgents: true, maxHops: 2, content: "@chit-agent again?",
			props: map[string]any{hopsProp: float64(1)}, want: true},
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

	// Failures are not cached: a lookup that failed once (a chitd restart,
	// a timeout) is retried, so the author is not misclassified for good.
	for range 3 {
		if got := b.actorType(ctx, "ghost-user"); got != "" {
			t.Fatalf("failed lookup should yield %q, got %q", "", got)
		}
	}
	if n := chitd.userLookups(); n != 4 {
		t.Errorf("failed lookups must be retried; got %d total lookups, want 4", n)
	}

	// Once the author resolves, the answer is cached like any other.
	chitd.mu.Lock()
	chitd.users["ghost-user"] = &model.User{ID: "ghost-user", ActorType: model.ActorTypeAgent}
	chitd.mu.Unlock()
	for range 2 {
		if got := b.actorType(ctx, "ghost-user"); got != model.ActorTypeAgent {
			t.Fatalf("actorType = %q after recovery, want %q", got, model.ActorTypeAgent)
		}
	}
	if n := chitd.userLookups(); n != 5 {
		t.Errorf("recovered author should be fetched once more, then cached; got %d lookups, want 5", n)
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
	t.Cleanup(func() { cancel(); b.wg.Wait() })

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
	if got := readArgv(t, argvFile); slices.Contains(got, "--resume") {
		t.Errorf("a root post starts a new session: %q", got)
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
	got, joined, _ := b.threadState(context.Background(), "root-1")
	if got != fakeSessionID2 || !joined {
		t.Fatalf("threadState: got (%q, %v), want (%q, true)", got, joined, fakeSessionID2)
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
	b, ctx := liveBridge(t, cfg)

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

	// At the cap the chain stops. A human post queued behind the capped one
	// in the same thread is answered only after the capped one was handled,
	// so once that answer lands, a reply to the capped post would have too.
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-3", ChannelID: testChannelID,
		UserID: testAgentID2, Content: "@chit-agent one more?",
		Props: map[string]any{hopsProp: float64(2)}}))
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-4", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-3", Content: "never mind"}))
	posts = waitForPosts(t, chitd, 3)
	if n := len(posts); n != 3 {
		t.Errorf("hop cap should suppress the reply; got %d posts, want 3", n)
	}
	if got := agentHops(posts[2]); got != 0 {
		t.Errorf("the third post should answer the human (hops 0), got hops %d", got)
	}
}

// Runs across threads are capped by MaxConcurrentRuns: each thread has its
// own worker, but they share the run slots.
func TestBridge_ConcurrentRunsAreCapped(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)

	// Each run marks itself live in a directory for a moment; the most marks
	// ever seen at once is the concurrency the bridge allowed.
	dir := t.TempDir()
	live := filepath.Join(dir, "live")
	if err := os.Mkdir(live, 0o755); err != nil {
		t.Fatal(err)
	}
	peak := filepath.Join(dir, "peak")
	bin := filepath.Join(dir, "claude")
	script := fmt.Sprintf(`#!/bin/sh
cat > /dev/null
touch %[1]s/$$
n=$(ls %[1]s | wc -l)
if [ "$n" -gt "$(cat %[2]s 2>/dev/null || echo 0)" ]; then echo "$n" > %[2]s; fi
sleep 0.3
rm %[1]s/$$
cat <<'FAKEEOF'
%[3]s
FAKEEOF
`, live, peak, fakeResultJSON(fakeSessionID))
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(t, srv.URL, bin)
	cfg.MaxConcurrentRuns = 1
	b := New(cfg)
	b.agentUserID = testAgentID

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); b.wg.Wait() })
	for i := range 3 {
		b.handleEvent(ctx, postedEvent(&model.Post{ID: fmt.Sprintf("root-%d", i),
			ChannelID: testChannelID, UserID: testOtherUser, Content: "go"}))
	}
	waitForPosts(t, chitd, 3)

	got, _ := os.ReadFile(peak)
	if strings.TrimSpace(string(got)) != "1" {
		t.Errorf("peak concurrent runs = %s, want 1", got)
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
	if got, _, _ := agentA.threadState(ctx, "root-1"); got != fakeSessionID {
		t.Errorf("agent A session: got %q, want %q", got, fakeSessionID)
	}
	if got, _, _ := agentB.threadState(ctx, "root-1"); got != fakeSessionID2 {
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

	b, ctx := liveBridge(t, testConfig(t, srv.URL, bin))
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

// writeEchoClaude writes a fake claude whose result echoes the last line of
// its prompt (the message being answered, tagged with its author), so each
// reply names the post it answers. Messages must not contain characters that
// need escaping in JSON.
func writeEchoClaude(t *testing.T, dir string) string {
	t.Helper()
	return writeScript(t, dir, `p=$(tail -n 1)
printf '{"result":"re: %s","session_id":"`+fakeSessionID+`"}\n' "$p"
`)
}

// liveBridge returns a bridge talking to chitd, resolved as the test agent,
// whose workers stop when the test ends.
func liveBridge(t *testing.T, cfg *Config) (*Bridge, context.Context) {
	t.Helper()
	b := New(cfg)
	b.agentUserID = testAgentID
	b.agentUsername = testAgentUsername
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); b.wg.Wait() })
	return b, ctx
}

// When the thread's session is gone (WORKDIR or ~/.claude changed), the reply
// starts a new session and says so, rather than failing every later reply.
func TestBridge_LostSessionStartsFresh(t *testing.T) {
	chitd := &fakeChitd{
		users: humanUsers(),
		thread: []*model.Post{
			{ID: "root-1", ChannelID: testChannelID, UserID: testOtherUser, Content: "task"},
			{ID: "old-reply", ChannelID: testChannelID, UserID: testAgentID, RootID: "root-1",
				Props: map[string]any{sessionProp: "gone-session"}},
		},
	}
	srv := chitd.server(t)

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeScript(t, dir, `printf '%s\n' "$@" > `+argvFile+`
cat > /dev/null
case "$*" in
*--resume*) echo "No conversation found with session ID: gone-session" >&2; exit 1 ;;
esac
echo '{"result":"fresh answer","session_id":"`+fakeSessionID2+`"}'
`)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, bin))
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-2", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-1", Content: "carry on"}))

	reply := waitForPosts(t, chitd, 1)[0]
	if !strings.Contains(reply.Content, "could not be found") || !strings.Contains(reply.Content, "fresh answer") {
		t.Errorf("reply should explain the new session and answer: %s", reply.Content)
	}
	if got := reply.Props[sessionProp]; got != fakeSessionID2 {
		t.Errorf("reply must record the new session, got %v", got)
	}
	if argv := readArgv(t, argvFile); slices.Contains(argv, "--resume") {
		t.Errorf("the retry must not resume: %q", argv)
	}
}

// A root post cannot have a session yet, so looking one up is a wasted call.
func TestBridge_RootPostSkipsSessionLookup(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, writeEchoClaude(t, t.TempDir())))

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "hello"}))
	waitForPosts(t, chitd, 1)
	chitd.mu.Lock()
	defer chitd.mu.Unlock()
	if chitd.threadCalls != 0 {
		t.Errorf("root post fetched the thread %d times, want 0", chitd.threadCalls)
	}
}

// A reply longer than one post may be is split, not lost; the session and
// usage props ride on the last part, the hop count on every part.
func TestBridge_LongReplyIsSplit(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	bin := writeScript(t, t.TempDir(), `cat > /dev/null
printf '{"result":"'
i=0; while [ $i -lt 1100 ]; do printf '%s\\n' "0123456789012345678901234567890123456789012345678901234567890123"; i=$((i+1)); done
printf '","session_id":"`+fakeSessionID+`"}'
`)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, bin))
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "write a lot"}))

	// ~71.5k bytes is two parts. Were it split into more, the second would
	// not be the last and the session check below would fail.
	posts := waitForPosts(t, chitd, 2)[:2]
	for i, p := range posts {
		if len(p.Content) > model.PostMaxContentSize {
			t.Errorf("part %d is %d bytes", i, len(p.Content))
		}
		if _, ok := p.Props[hopsProp]; !ok {
			t.Errorf("part %d lacks the hop count", i)
		}
	}
	if _, ok := posts[0].Props[sessionProp]; ok {
		t.Error("the session prop belongs on the last part only")
	}
	if posts[1].Props[sessionProp] != fakeSessionID || !strings.Contains(posts[1].Content, "⚙") {
		t.Errorf("last part must carry the session and the footer: %v", posts[1].Props)
	}
}

// A post delivered twice is answered once.
func TestBridge_DuplicateDeliveryRunsOnce(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, writeEchoClaude(t, t.TempDir())))

	first := &model.Post{ID: "root-1", ChannelID: testChannelID, UserID: testOtherUser, Content: "alpha"}
	b.handleEvent(ctx, postedEvent(first))
	b.handleEvent(ctx, postedEvent(first))
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-2", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-1", Content: "beta"}))

	// The thread's queue is FIFO, so a duplicate run would land before beta.
	posts := waitForPosts(t, chitd, 2)
	if !strings.HasPrefix(posts[0].Content, "re: [@alice] alpha") || !strings.HasPrefix(posts[1].Content, "re: [@alice] beta") {
		t.Errorf("want replies to alpha then beta, got %q, %q", posts[0].Content, posts[1].Content)
	}
}

func TestFirstSightingForgetsOldest(t *testing.T) {
	b := New(testConfig(t, "http://unused", "claude"))
	if !b.firstSighting("p-0") || b.firstSighting("p-0") {
		t.Fatal("a post is new once")
	}
	for i := 1; i <= seenPostsSize; i++ {
		b.firstSighting(fmt.Sprintf("p-%d", i))
	}
	if !b.firstSighting("p-0") {
		t.Error("the oldest ID should have been forgotten")
	}
	if n := b.seen.len(); n != seenPostsSize {
		t.Errorf("seen holds %d, want %d", n, seenPostsSize)
	}
}

// A thread whose queue overflows says so once, rather than dropping posts in
// silence or answering every dropped post.
func TestBridge_QueueFullIsAnnouncedOnce(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)

	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	started := filepath.Join(dir, "started")
	bin := writeScript(t, dir, `cat > /dev/null
touch `+started+`
while [ ! -f `+release+` ]; do sleep 0.02; done
echo '{"result":"done","session_id":"`+fakeSessionID+`"}'
`)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, bin))

	post := func(i int) {
		b.handleEvent(ctx, postedEvent(&model.Post{ID: fmt.Sprintf("msg-%d", i), ChannelID: testChannelID,
			UserID: testOtherUser, RootID: "root-1", Content: "more"}))
	}
	// The first post is taken off the queue and blocks in claude ...
	post(0)
	for deadline := time.Now().Add(5 * time.Second); ; {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first run never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// ... the next perThreadQueueSize fill the queue, and two more overflow.
	for i := 1; i <= perThreadQueueSize+2; i++ {
		post(i)
	}
	notice := waitForPosts(t, chitd, 1)[0]
	if notice.Content != queueFullNotice || notice.RootID != "root-1" {
		t.Errorf("want the queue-full notice in the thread, got %+v", notice)
	}

	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	posts := waitForPosts(t, chitd, 1+1+perThreadQueueSize)
	n := 0
	for _, p := range posts {
		if p.Content == queueFullNotice {
			n++
		}
	}
	if n != 1 {
		t.Errorf("queue-full notice posted %d times, want 1", n)
	}
}

// A session whose context reaches MaxSessionTokens is retired: the reply says
// so and carries the reset marker, and the next reply, even after a restart,
// starts a new session.
func TestBridge_SessionRetiredAtTokenCeiling(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	bin := writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0) // context 43.4k
	cfg := testConfig(t, srv.URL, bin)
	cfg.MaxSessionTokens = 40_000
	b, ctx := liveBridge(t, cfg)

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "big task"}))
	first := waitForPosts(t, chitd, 1)[0]
	if !strings.Contains(first.Content, "starts a new session") || first.Props[sessionResetProp] != true {
		t.Errorf("the reply should announce and mark the reset: %q %v", first.Content, first.Props)
	}

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-2", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-1", Content: "next"}))
	waitForPosts(t, chitd, 2)
	if argv := readArgv(t, argvFile); slices.Contains(argv, "--resume") {
		t.Errorf("the retired session must not be resumed: %q", argv)
	}

	// After a restart the marker, not the older session prop, decides.
	chitd.mu.Lock()
	chitd.thread = []*model.Post{
		{ID: "root-1", UserID: testOtherUser},
		{ID: "a-1", UserID: testAgentID, Props: map[string]any{sessionProp: "old"}},
		{ID: "a-2", UserID: testAgentID, Props: map[string]any{sessionProp: "old", sessionResetProp: true}},
	}
	chitd.mu.Unlock()
	fresh := New(cfg)
	fresh.agentUserID = testAgentID
	if got, joined, clean := fresh.threadState(context.Background(), "root-1"); got != "" || !joined || !clean {
		t.Errorf("threadState after a reset = (%q, %v, %v), want (\"\", true, true)", got, joined, clean)
	}
}

func TestBridge_FooterCanBeOff(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	dir := t.TempDir()
	cfg := testConfig(t, srv.URL, writeFakeClaude(t, dir, filepath.Join(dir, "argv"), fakeResultJSON(fakeSessionID), 0))
	cfg.Footer = false
	b, ctx := liveBridge(t, cfg)

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "hi"}))
	reply := waitForPosts(t, chitd, 1)[0]
	if strings.Contains(reply.Content, "⚙") {
		t.Errorf("footer should be off: %q", reply.Content)
	}
	if _, ok := reply.Props["claude_usage"]; !ok {
		t.Error("usage still belongs in the props")
	}
}

// Each run leaves one run_complete line with its token figures, the record
// operators total usage from.
func TestBridge_LogsRunComplete(t *testing.T) {
	var logs syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	dir := t.TempDir()
	b, ctx := liveBridge(t, testConfig(t, srv.URL,
		writeFakeClaude(t, dir, filepath.Join(dir, "argv"), fakeResultJSON(fakeSessionID), 0)))
	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "hi"}))
	waitForPosts(t, chitd, 1)

	var line map[string]any
	for l := range strings.SplitSeq(logs.String(), "\n") {
		if strings.Contains(l, `"msg":"run_complete"`) {
			if err := json.Unmarshal([]byte(l), &line); err != nil {
				t.Fatal(err)
			}
		}
	}
	if line == nil {
		t.Fatalf("no run_complete line in:\n%s", logs.String())
	}
	for k, want := range map[string]float64{"cache_write_tokens": 900, "context_tokens": 43400, "input_tokens": 4500} {
		if line[k] != want {
			t.Errorf("%s = %v, want %v", k, line[k], want)
		}
	}
}
