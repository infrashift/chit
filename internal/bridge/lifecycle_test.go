package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

const testTeamID = "team-1"

// servingChitd is a fake chitd in which the agent is a member of the test
// channel, an open channel on a team, with a WebSocket feed.
func servingChitd() *fakeChitd {
	return &fakeChitd{
		users: humanUsers(),
		channels: map[string]*model.Channel{
			testChannelID: {ID: testChannelID, TeamID: testTeamID, Type: model.ChannelOpen},
		},
		memberOf: []string{testChannelID},
		events:   make(chan *model.WebSocketEvent, 64),
	}
}

// startRun runs the bridge until the returned stop is called, then reports
// what Run returned. stop may be called from any goroutine.
func startRun(t *testing.T, b *Bridge) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- b.Run(ctx) }()
	stopped := false
	stop = func() error {
		if stopped {
			return nil
		}
		stopped = true
		cancel()
		select {
		case err := <-errc:
			return err
		case <-time.After(15 * time.Second):
			return errors.New("Run did not return after cancel")
		}
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

// waitFor polls cond until it holds or a few seconds pass.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForConnect(t *testing.T, f *fakeChitd) {
	t.Helper()
	waitFor(t, "the WebSocket to connect", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.wsConnects > 0
	})
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Run end to end: identity, channel check, a post over the WebSocket, a
// threaded reply, and a clean return on cancel.
func TestBridge_RunServesOverWebSocket(t *testing.T) {
	chitd := servingChitd()
	srv := chitd.server(t)
	b := New(testConfig(t, srv.URL, writeEchoClaude(t, t.TempDir())))
	stop := startRun(t, b)
	waitForConnect(t, chitd)

	chitd.events <- postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "hello"})
	reply := waitForPosts(t, chitd, 1)[0]
	if reply.RootID != "root-1" || !strings.HasPrefix(reply.Content, "re: hello") {
		t.Errorf("unexpected reply %+v", reply)
	}
	if err := stop(); err != nil {
		t.Errorf("Run returned %v", err)
	}
}

// A bridge whose agent cannot hear a configured channel refuses to start
// instead of running deaf; a chitd that cannot answer only warns.
func TestBridge_RunChecksChannelMembership(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*fakeChitd)
		wantErr string
	}{
		{"not a member of the open channel", func(f *fakeChitd) { f.memberOf = nil }, "not a member"},
		{"channel unknown or hidden", func(f *fakeChitd) { f.channels = nil }, "status 404"},
		{"direct channel readable means member", func(f *fakeChitd) {
			f.channels[testChannelID] = &model.Channel{ID: testChannelID, Type: model.ChannelDirect}
			f.memberOf = nil
		}, ""},
		{"chitd unavailable only warns", func(f *fakeChitd) { f.channelStatus = 503 }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chitd := servingChitd()
			tc.mutate(chitd)
			srv := chitd.server(t)
			b := New(testConfig(t, srv.URL, "claude"))

			if tc.wantErr != "" {
				err := b.Run(context.Background())
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Run = %v, want an error containing %q", err, tc.wantErr)
				}
				return
			}
			stop := startRun(t, b)
			waitForConnect(t, chitd)
			if err := stop(); err != nil {
				t.Errorf("Run returned %v", err)
			}
		})
	}
}

// writeBlockingClaude writes a fake claude that marks started, then waits for
// release before answering. With exec, cancellation kills the waiting
// process itself.
func writeBlockingClaude(t *testing.T, dir string) (bin, started, release string) {
	t.Helper()
	started = filepath.Join(dir, "started")
	release = filepath.Join(dir, "release")
	bin = writeScript(t, dir, `cat > /dev/null
touch `+started+`
while [ ! -f `+release+` ]; do sleep 0.02; done
echo '{"result":"finished","session_id":"`+fakeSessionID+`"}'
`)
	return bin, started, release
}

// On shutdown a run in progress finishes and posts its answer; posts queued
// behind it are not started, and the thread is told once to resend.
func TestBridge_ShutdownFinishesRunInProgress(t *testing.T) {
	chitd := servingChitd()
	srv := chitd.server(t)
	dir := t.TempDir()
	bin, started, release := writeBlockingClaude(t, dir)
	b := New(testConfig(t, srv.URL, bin))
	stop := startRun(t, b)
	waitForConnect(t, chitd)

	for i, id := range []string{"msg-0", "msg-1", "msg-2"} {
		chitd.events <- postedEvent(&model.Post{ID: id, ChannelID: testChannelID,
			UserID: testOtherUser, RootID: "root-1", Content: "step"})
		if i == 0 {
			waitFor(t, "the first run to start", func() bool { return fileExists(started) })
		}
	}
	waitFor(t, "the two posts to queue", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(b.queues["root-1"]) == 2
	})

	// Stop, and let the run finish only once shutdown has begun.
	errc := make(chan error, 1)
	go func() { errc <- stop() }()
	waitFor(t, "shutdown to begin", func() bool {
		select {
		case <-b.stopping:
			return true
		default:
			return false
		}
	})
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("Run returned %v", err)
	}

	posts := chitd.createdPosts()
	if len(posts) != 2 {
		t.Fatalf("want the answer and one notice, got %d posts: %+v", len(posts), posts)
	}
	if !strings.HasPrefix(posts[0].Content, "finished") {
		t.Errorf("the run in progress should have answered, got %q", posts[0].Content)
	}
	if posts[1].Content != shutdownNotice || posts[1].RootID != "root-1" {
		t.Errorf("want the shutdown notice in the thread, got %+v", posts[1])
	}
}

// A run still going when the grace period ends is stopped, and the thread is
// told why rather than left waiting.
func TestBridge_ShutdownGraceStopsRuns(t *testing.T) {
	chitd := servingChitd()
	srv := chitd.server(t)
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	bin := writeScript(t, dir, "cat > /dev/null\ntouch "+started+"\nexec sleep 30\n")
	cfg := testConfig(t, srv.URL, bin)
	cfg.ShutdownGrace = 100 * time.Millisecond
	b := New(cfg)
	stop := startRun(t, b)
	waitForConnect(t, chitd)

	chitd.events <- postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "long task"})
	waitFor(t, "the run to start", func() bool { return fileExists(started) })

	start := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("shutdown took %s with a 100ms grace", took)
	}
	posts := chitd.createdPosts()
	if len(posts) != 1 || !strings.Contains(posts[0].Content, "bridge is shutting down") {
		t.Errorf("want one reply saying the run was stopped, got %+v", posts)
	}
}

// An idle thread's worker exits and its state is dropped; the thread still
// works when it wakes up.
func TestBridge_IdleWorkerRetires(t *testing.T) {
	chitd := &fakeChitd{users: humanUsers()}
	srv := chitd.server(t)
	b, ctx := liveBridge(t, testConfig(t, srv.URL, writeEchoClaude(t, t.TempDir())))
	b.idleTimeout = 50 * time.Millisecond

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "root-1", ChannelID: testChannelID,
		UserID: testOtherUser, Content: "first"}))
	waitForPosts(t, chitd, 1)
	waitFor(t, "the idle worker to retire", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		_, queued := b.queues["root-1"]
		_, session := b.sessions["root-1"]
		return !queued && !session
	})

	b.handleEvent(ctx, postedEvent(&model.Post{ID: "msg-2", ChannelID: testChannelID,
		UserID: testOtherUser, RootID: "root-1", Content: "second"}))
	if reply := waitForPosts(t, chitd, 2)[1]; !strings.HasPrefix(reply.Content, "re: second") {
		t.Errorf("a woken thread should be answered, got %q", reply.Content)
	}
}
