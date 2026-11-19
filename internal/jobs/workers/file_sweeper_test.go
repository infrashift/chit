package workers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// countingSweeper counts its passes.
type countingSweeper struct{ calls *atomic.Int32 }

func (c countingSweeper) SweepFiles(context.Context) (expired, purged int, err error) {
	c.calls.Add(1)
	return 0, 0, nil
}

func TestFileSweeper_RunsEachIntervalUntilStopped(t *testing.T) {
	var calls atomic.Int32
	fs := NewFileSweeper(countingSweeper{calls: &calls}, 5*time.Millisecond)
	done := make(chan struct{})
	go func() {
		if err := fs.Start(t.Context()); err != nil {
			t.Errorf("Start: %v", err)
		}
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	fs.Stop()
	fs.Stop() // idempotent
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not end the loop")
	}
	if calls.Load() < 2 {
		t.Fatalf("%d passes", calls.Load())
	}
}
