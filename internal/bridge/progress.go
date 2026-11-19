package bridge

import (
	"context"
	"log/slog"
	"time"
)

// progressDelay is how long a run may go before the thread is shown that the
// agent is working on it. Quick answers never show the note.
const progressDelay = 10 * time.Second

// progressNotice stands in the thread while a long run is under way.
const progressNotice = "⏳ Working on it… (this note goes away when I reply)"

// progressNote is a progress note that may be in the thread. Stop ends it.
type progressNote struct {
	b       *Bridge
	root    string
	timer   *time.Timer
	created chan struct{} // closed once the timer's post attempt has finished
	noteID  string        // set before created is closed; empty if posting failed
}

// showProgress posts progressNotice in the thread if the run is still going
// after b.progressDelay, and returns the note: Stop, called once the reply is
// posted, cancels the note or deletes it.
//
// The note is deleted rather than edited into the reply. The reply has to be
// a new post: other bridges, notifications and mention handling act on
// `posted` events, and an @mention that arrived by edit would hand work to
// another agent that never hears of it.
func (b *Bridge) showProgress(ctx context.Context, channelID, root string, hops int) *progressNote {
	p := &progressNote{b: b, root: root, created: make(chan struct{})}
	p.timer = time.AfterFunc(b.progressDelay, func() {
		defer close(p.created)
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
		defer cancel()
		note, err := b.client.CreatePost(rctx, channelID, root, progressNotice, map[string]any{hopsProp: hops})
		if err != nil {
			slog.Warn("failed to post progress note", "root_id", root, "error", err)
			return
		}
		p.noteID = note.ID
	})
	return p
}

// Stop cancels the note if it has not been posted yet, or deletes it. ctx is
// the run's context; the delete outlives its cancellation.
func (p *progressNote) Stop(ctx context.Context) {
	if p.timer.Stop() {
		return
	}
	<-p.created
	if p.noteID == "" {
		return
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
	defer cancel()
	if err := p.b.client.DeletePost(rctx, p.noteID); err != nil {
		slog.Warn("failed to remove progress note", "root_id", p.root, "post_id", p.noteID, "error", err)
	}
}
