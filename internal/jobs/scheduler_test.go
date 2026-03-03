package jobs

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockWorker struct {
	mu       sync.Mutex
	started  bool
	stopped  bool
	startCtx context.Context
}

func (w *mockWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	w.started = true
	w.startCtx = ctx
	w.mu.Unlock()

	<-ctx.Done()
	return nil
}

func (w *mockWorker) Stop() {
	w.mu.Lock()
	w.stopped = true
	w.mu.Unlock()
}

func (w *mockWorker) wasStarted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.started
}

func (w *mockWorker) wasStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

func TestNewScheduler(t *testing.T) {
	s := NewScheduler()
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if len(s.workers) != 0 {
		t.Fatalf("expected empty workers, got %d", len(s.workers))
	}
}

func TestAddWorker(t *testing.T) {
	s := NewScheduler()
	w := &mockWorker{}
	s.AddWorker(w)

	if len(s.workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(s.workers))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler()
	w := &mockWorker{}
	s.AddWorker(w)

	s.Start()

	// Give the goroutine time to run
	time.Sleep(50 * time.Millisecond)

	if !w.wasStarted() {
		t.Fatal("expected worker to be started")
	}

	s.Stop()

	if !w.wasStopped() {
		t.Fatal("expected worker to be stopped")
	}
}

func TestScheduler_StopBeforeStart(t *testing.T) {
	s := NewScheduler()
	// Should not panic
	s.Stop()
}

func TestScheduler_MultipleWorkers(t *testing.T) {
	s := NewScheduler()
	workers := make([]*mockWorker, 3)
	for i := range workers {
		workers[i] = &mockWorker{}
		s.AddWorker(workers[i])
	}

	s.Start()
	time.Sleep(50 * time.Millisecond)

	for i, w := range workers {
		if !w.wasStarted() {
			t.Fatalf("expected worker %d to be started", i)
		}
	}

	s.Stop()

	for i, w := range workers {
		if !w.wasStopped() {
			t.Fatalf("expected worker %d to be stopped", i)
		}
	}
}
