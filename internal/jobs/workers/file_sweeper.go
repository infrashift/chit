package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/infrashift/chit/internal/assert"
)

// Sweeper runs one bounded pass of the attachment sweep. *app.App is the
// implementation (App.SweepFiles).
type Sweeper interface {
	SweepFiles(ctx context.Context) (expired, purged int, err error)
}

// FileSweeper runs the attachment sweep every interval. Each pass is bounded
// by its batch size, so a backlog drains over several passes instead of in
// one long one.
type FileSweeper struct {
	sweeper  Sweeper
	interval time.Duration
	stop     chan struct{}
	once     sync.Once
}

// NewFileSweeper returns a sweeper that calls sweeper.SweepFiles every
// interval.
func NewFileSweeper(sweeper Sweeper, interval time.Duration) *FileSweeper {
	assert.That(sweeper != nil, "NewFileSweeper: nil sweeper")
	assert.That(interval > 0, "NewFileSweeper: interval must be positive")
	fs := &FileSweeper{sweeper: sweeper, interval: interval, stop: make(chan struct{})}
	assert.That(fs.stop != nil, "NewFileSweeper: no stop channel")
	return fs
}

// Start runs passes until ctx ends or Stop is called.
func (fs *FileSweeper) Start(ctx context.Context) error {
	assert.That(fs.sweeper != nil && fs.stop != nil, "FileSweeper.Start: not built by NewFileSweeper")
	assert.That(fs.interval > 0, "FileSweeper.Start: interval must be positive")
	slog.Info("file sweeper started", "interval", fs.interval)
	ticker := time.NewTicker(fs.interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil
		case <-fs.stop:
			return nil
		case <-ticker.C:
			expired, purged, err := fs.sweeper.SweepFiles(ctx)
			if err != nil {
				slog.Warn("file sweep failed; will retry", "error", err)
				continue
			}
			assert.That(expired >= 0 && purged >= 0, "FileSweeper.Start: negative sweep counts")
			if expired > 0 || purged > 0 {
				slog.Info("file sweep", "expired", expired, "purged", purged)
			}
		}
	}
	return nil
}

// Stop ends the loop.
func (fs *FileSweeper) Stop() {
	assert.That(fs.stop != nil, "FileSweeper.Stop: not built by NewFileSweeper")
	fs.once.Do(func() { close(fs.stop) })
}
