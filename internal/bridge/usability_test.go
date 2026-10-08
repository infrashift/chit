package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

// A run still going after progressDelay shows a note in the thread, which is
// deleted once the reply (a new post, so mentions and handoffs work) lands.
func TestBridge_ProgressNoteForLongRuns(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	bin, started, release := writeBlockingClaude(t, t.TempDir())
	b, ctx := liveBridge(t, testConfig(t, srv.URL, bin))
	b.progressDelay = 20 * time.Millisecond

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "long task"}))
	waitFor(t, "the run to start", func() bool { return fileExists(started) })
	note := waitForPosts(t, chitd, 1)[0]
	if note.Content != progressNotice || note.RootID != "root-1" {
		t.Fatalf("want the progress note in the thread, got %+v", note)
	}

	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	reply := waitForPosts(t, chitd, 2)[1]
	if !strings.HasPrefix(reply.Content, "finished") {
		t.Errorf("want the answer as a new post, got %q", reply.Content)
	}
	waitFor(t, "the note to be deleted", func() bool {
		chitd.mu.Lock()
		defer chitd.mu.Unlock()
		return slices.Equal(chitd.deleted, []string{note.ID})
	})
}

func TestBridge_NoProgressNoteForQuickRuns(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, writeEchoClaude(t, t.TempDir())))

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "quick"}))
	if reply := waitForPosts(t, chitd, 1)[0]; reply.Content == progressNotice {
		t.Error("a quick run should not show a progress note")
	}
	chitd.mu.Lock()
	defer chitd.mu.Unlock()
	if len(chitd.deleted) != 0 {
		t.Errorf("nothing should be deleted, got %v", chitd.deleted)
	}
}

// writeStdinClaude writes a fake claude that saves its prompt to stdinFile.
func writeStdinClaude(t *testing.T, dir, stdinFile string) string {
	t.Helper()
	return writeScript(t, dir, `cat > `+stdinFile+`
echo '{"result":"ok","session_id":"`+fakeSessionID+`"}'
`)
}

// Named in the middle of a conversation, the agent starts its session with
// the thread so far, each post tagged with its author and its own replies
// without their footer.
func TestBridge_NewSessionMidThreadIsSeeded(t *testing.T) {
	users := humanUsers()
	users[testAgentID] = &model.User{ID: testAgentID, Username: testAgentUsername, ActorType: model.ActorTypeAgent}
	users["bob-1"] = &model.User{ID: "bob-1", Username: "bob", ActorType: model.ActorTypeUser}
	chitd := &fakeChitd{users: users, thread: []*model.Post{
		{ID: "root-1", UserID: testOtherUser, Content: "should we shard the users table?"},
		{ID: "p-2", UserID: "bob-1", RootID: "root-1", Content: "only past 50M rows"},
		{ID: "p-3", UserID: testAgentID, RootID: "root-1", Content: "an older answer" + footerSeparator + " in 1 · out 2`"},
		{ID: "msg-4", UserID: testOtherUser, RootID: "root-1", Content: "@chit-agent what do you think?"},
	}}
	srv := chitd.server(t)
	dir := t.TempDir()
	stdin := filepath.Join(dir, "stdin")
	b, ctx := liveBridge(t, testConfig(t, srv.URL, writeStdinClaude(t, dir, stdin)))

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-4", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-1", Content: "@chit-agent what do you think?"}))
	waitForPosts(t, chitd, 1)

	got, _ := os.ReadFile(stdin)
	want := "Earlier in this thread:\n\n" +
		"[@alice] should we shard the users table?\n\n" +
		"[@bob] only past 50M rows\n\n" +
		"[@chit-agent] an older answer\n\n" +
		"The message to answer:\n\n" +
		"[@alice] @chit-agent what do you think?"
	if string(got) != want {
		t.Errorf("prompt:\n%s\nwant:\n%s", got, want)
	}
}

func TestThreadHistoryKeepsTheNewest(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers(), thread: []*model.Post{
		{ID: "p-1", UserID: testOtherUser, Content: strings.Repeat("old ", 20)},
		{ID: "p-2", UserID: testOtherUser, Content: "recent"},
		{ID: "p-3", UserID: testOtherUser, Content: "now"},
	}}
	srv := chitd.server(t)
	cfg := testConfig(t, srv.URL, "claude")
	cfg.SeedMaxChars = 40
	b := New(cfg)
	b.agentUserID = testAgentID

	got := b.threadHistory(t.Context(), "p-1", "p-3")
	if got != "(earlier messages omitted)\n\n[@alice] recent" {
		t.Errorf("history = %q", got)
	}
}

func TestParseControl(t *testing.T) {
	b := New(testConfig(t, "http://unused", "claude"))
	b.agentUsername = testAgentUsername
	cases := []struct{ content, word, rest string }{
		{"!stop", controlStop, ""},
		{"@chit-agent !STATUS", controlStatus, ""},
		{"  @Chit-Agent   !new  plan the next step ", controlNew, "plan the next step"},
		{"@chit-agent please !stop", "", ""},
		{"!stopwatch", "", ""},
		{"@someone-else !stop", "", ""},
	}
	for _, tc := range cases {
		word, rest := b.parseControl(tc.content)
		if word != tc.word || rest != tc.rest {
			t.Errorf("parseControl(%q) = (%q, %q), want (%q, %q)", tc.content, word, rest, tc.word, tc.rest)
		}
	}
}

// !stop cancels the thread's run and its queue at once, without the run
// posting an error of its own; !status reports while the run is going.
func TestBridge_StopAndStatus(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	bin := writeScript(t, dir, "cat > /dev/null\ntouch "+started+"\nexec sleep 30\n")
	b, ctx := liveBridge(t, testConfig(t, srv.URL, bin))
	b.agentUsername = testAgentUsername

	say := func(id, content string) {
		b.handleEvent(ctx, postedEvent(&model.Post{ID: id, ChannelID: testChannelID,
			UserID: testOtherUser, RootID: "root-1", Content: content}))
	}
	say("msg-1", "long task")
	waitFor(t, "the run to start", func() bool { return fileExists(started) })
	say("msg-2", "and another")

	say("msg-3", "!status")
	status := waitForPosts(t, chitd, 1)[0]
	if !strings.Contains(status.Content, "running for") || !strings.Contains(status.Content, "1 queued") {
		t.Errorf("status = %q", status.Content)
	}

	say("msg-4", "@chit-agent !stop")
	stop := waitForPosts(t, chitd, 2)[1]
	if stop.Content != "⏹ Stopped the run in progress and dropped 1 queued message(s)." {
		t.Errorf("stop reply = %q", stop.Content)
	}
	waitFor(t, "the run to end", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(b.running) == 0
	})
	// Prove the stopped run posted nothing: a later post in the thread is
	// answered next.
	say("msg-5", "!status")
	if next := waitForPosts(t, chitd, 3)[2]; !strings.HasPrefix(next.Content, "ℹ️ idle") {
		t.Errorf("after !stop the thread should be idle with nothing else posted, got %q", next.Content)
	}
}

// !new forgets the session, in queue order; with a message it runs that
// message in the new session.
func TestBridge_NewStartsAFreshSession(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	b, ctx := liveBridge(t, testConfig(t, srv.URL,
		writeFakeClaude(t, dir, argvFile, fakeResultJSON(fakeSessionID), 0)))
	b.agentUsername = testAgentUsername

	say := func(id, content string) {
		b.handleEvent(ctx, postedEvent(&model.Post{ID: id, ChannelID: testChannelID,
			UserID: testOtherUser, RootID: "root-1", Content: content}))
	}
	say("msg-1", "first")
	waitForPosts(t, chitd, 1)

	say("msg-2", "!new")
	ack := waitForPosts(t, chitd, 2)[1]
	if !strings.Contains(ack.Content, "new Claude session") || ack.Props[sessionResetProp] != true {
		t.Errorf("want an acknowledgement marked as a reset, got %q %v", ack.Content, ack.Props)
	}

	say("msg-3", "@chit-agent !new second")
	waitForPosts(t, chitd, 3)
	if argv := readArgv(t, argvFile); slices.Contains(argv, "--resume") {
		t.Errorf("the message after !new must start a new session: %q", argv)
	}
	stdin, _ := os.ReadFile(argvFile + ".stdin")
	if !strings.HasSuffix(string(stdin), "[@alice] second") {
		t.Errorf("only the text after !new goes to Claude, got %q", stdin)
	}
}

// !new promises a clean start, so its session is not seeded with the thread.
func TestBridge_NewIsNotSeeded(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers(), thread: []*model.Post{
		{ID: "root-1", UserID: testOtherUser, Content: "old context"},
		{ID: "p-2", UserID: testAgentID, RootID: "root-1", Content: "old answer",
			Props: map[string]any{sessionProp: fakeSessionID}},
	}}
	srv := chitd.server(t)
	dir := t.TempDir()
	stdin := filepath.Join(dir, "stdin")
	b, ctx := liveBridge(t, testConfig(t, srv.URL, writeStdinClaude(t, dir, stdin)))
	b.agentUsername = testAgentUsername

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-3", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-1", Content: "!new fresh question"}))
	waitForPosts(t, chitd, 1)
	if got, _ := os.ReadFile(stdin); string(got) != "[@alice] fresh question" {
		t.Errorf("prompt after !new = %q, want only the message", got)
	}
}
