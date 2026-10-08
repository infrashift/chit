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

// showProgress posts progressNotice in the thread if the run is still going
// after b.progressDelay, and returns the function that ends it: called once
// the reply is posted, it cancels the note or deletes it.
//
// The note is deleted rather than edited into the reply. The reply has to be
// a new post: other bridges, notifications and mention handling act on
// `posted` events, and an @mention that arrived by edit would hand work to
// another agent that never hears of it.
func (b *Bridge) showProgress(ctx context.Context, channelID, root string, hops int) (done func()) {
	var noteID string
	created := make(chan struct{})
	timer := time.AfterFunc(b.progressDelay, func() {
		defer close(created)
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
		defer cancel()
		note, err := b.client.CreatePost(rctx, channelID, root, progressNotice, map[string]any{hopsProp: hops})
		if err != nil {
			slog.Warn("failed to post progress note", "root_id", root, "error", err)
			return
		}
		noteID = note.ID
	})
	return func() {
		if timer.Stop() {
			return
		}
		<-created
		if noteID == "" {
			return
		}
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
		defer cancel()
		if err := b.client.DeletePost(rctx, noteID); err != nil {
			slog.Warn("failed to remove progress note", "root_id", root, "post_id", noteID, "error", err)
		}
	}
}
