package bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/infrashift/chit/internal/model"
)

// Control words, sent in a thread to the agent (named, or in a thread it has
// joined), act on that thread rather than going to Claude:
//
//	!new [message]  start a new session; with a message, run it in the new one
//	!stop           stop the run in progress and drop the queued messages
//	!status         say what the agent is doing in this thread
//
// !new waits its turn in the thread's queue, so a run already under way
// cannot overwrite the reset when it finishes. !stop and !status act at once,
// since they are about the run that holds the queue.
const (
	controlNew    = "!new"
	controlStop   = "!stop"
	controlStatus = "!status"
)

// errStopped is the cause given to a run cancelled by !stop.
var errStopped = errors.New("stopped on request")

// runState is the thread's run in progress, for !stop and !status.
type runState struct {
	cancel  context.CancelCauseFunc
	started time.Time
}

// parseControl returns the control word a post opens with, after any leading
// mention of the agent, and the text after it.
func (b *Bridge) parseControl(content string) (word, rest string) {
	s := strings.TrimSpace(content)
	if b.agentUsername != "" {
		lower := strings.ToLower(s)
		for prefix := "@" + strings.ToLower(b.agentUsername); strings.HasPrefix(lower, prefix); {
			s = strings.TrimSpace(s[len(prefix):])
			lower = strings.ToLower(s)
		}
	}
	word, rest, _ = strings.Cut(s, " ")
	switch word = strings.ToLower(strings.TrimSpace(word)); word {
	case controlNew, controlStop, controlStatus:
		return word, strings.TrimSpace(rest)
	}
	return "", ""
}

// controlNow handles the control words that do not wait in the queue, and
// reports whether it handled the post. It runs on the WebSocket read loop, so
// its replies go out from their own goroutine.
func (b *Bridge) controlNow(ctx context.Context, root string, post *model.Post, word string) bool {
	var msg string
	switch word {
	case controlStop:
		msg = b.stop(root)
	case controlStatus:
		msg = b.status(root)
	default:
		return false
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.reply(ctx, post.ChannelID, root, msg, map[string]any{hopsProp: agentHops(post)})
	}()
	return true
}

// stop cancels the thread's run and empties its queue.
func (b *Bridge) stop(root string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	dropped := 0
	if queue, ok := b.queues[root]; ok {
		for drained := false; !drained; {
			select {
			case <-queue:
				dropped++
			default:
				drained = true
			}
		}
	}
	run, running := b.running[root]
	if running {
		run.cancel(errStopped)
	}
	switch {
	case running && dropped > 0:
		return fmt.Sprintf("⏹ Stopped the run in progress and dropped %d queued message(s).", dropped)
	case running:
		return "⏹ Stopped the run in progress."
	case dropped > 0:
		return fmt.Sprintf("⏹ Dropped %d queued message(s).", dropped)
	}
	return "Nothing is running in this thread."
}

// status describes the thread's run, queue and session from memory.
func (b *Bridge) status(root string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	parts := []string{"idle"}
	if run, ok := b.running[root]; ok {
		parts[0] = "running for " + time.Since(run.started).Round(time.Second).String()
	}
	if queue, ok := b.queues[root]; ok && len(queue) > 0 {
		parts = append(parts, fmt.Sprintf("%d queued", len(queue)))
	}
	switch session, ok := b.sessions[root]; {
	case ok && session != "":
		parts = append(parts, "next reply resumes session `"+session+"`")
	case ok:
		parts = append(parts, "next reply starts a new session")
	}
	return "ℹ️ " + strings.Join(parts, " · ")
}

// newSession forgets the thread's session, ahead of anything after it in the
// queue. When announce is set (a bare !new), it says so in the thread,
// marking the reply so that a restart does not resume the old session either;
// otherwise the run that follows replies, with its new session.
func (b *Bridge) newSession(ctx context.Context, root string, post *model.Post, announce bool) {
	b.mu.Lock()
	b.sessions[root] = ""
	b.cleanStart.add(root)
	b.mu.Unlock()
	if !announce {
		return
	}
	b.reply(ctx, post.ChannelID, root, "🆕 The next message in this thread starts a new Claude session.",
		map[string]any{hopsProp: agentHops(post), sessionResetProp: true})
}
