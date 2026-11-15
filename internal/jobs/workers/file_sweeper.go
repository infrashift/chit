package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// SweepFunc runs one bounded pass of the attachment sweep (app.SweepFiles).
type SweepFunc func(ctx context.Context) (expired, purged int, err error)

// FileSweeper runs the attachment sweep every interval. Each pass is bounded
// by its batch size, so a backlog drains over several passes instead of in
// one long one.
type FileSweeper struct {
	sweep    SweepFunc
	interval time.Duration
	stop     chan struct{}
	once     sync.Once
}

// NewFileSweeper returns a sweeper that calls sweep every interval.
func NewFileSweeper(sweep SweepFunc, interval time.Duration) *FileSweeper {
	return &FileSweeper{sweep: sweep, interval: interval, stop: make(chan struct{})}
}

// Start runs passes until ctx ends or Stop is called.
func (fs *FileSweeper) Start(ctx context.Context) error {
	slog.Info("file sweeper started", "interval", fs.interval)
	ticker := time.NewTicker(fs.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-fs.stop:
			return nil
		case <-ticker.C:
			expired, purged, err := fs.sweep(ctx)
			if err != nil {
				slog.Warn("file sweep failed; will retry", "error", err)
				continue
			}
			if expired > 0 || purged > 0 {
				slog.Info("file sweep", "expired", expired, "purged", purged)
			}
		}
	}
}

// Stop ends the loop.
func (fs *FileSweeper) Stop() {
	fs.once.Do(func() { close(fs.stop) })
}
