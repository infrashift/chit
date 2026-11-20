package jobs

import (
	"context"
	"log/slog"
	"sync"

	"github.com/infrashift/chit/internal/assert"
)

// Worker represents a background job that can be started and stopped.
type Worker interface {
	Start(ctx context.Context) error
	Stop()
}

// Scheduler manages background workers.
type Scheduler struct {
	workers []Worker
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewScheduler creates a new job scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// AddWorker registers a worker with the scheduler.
func (s *Scheduler) AddWorker(w Worker) {
	assert.That(w != nil, "Scheduler.AddWorker: nil worker")
	assert.That(s.cancel == nil, "Scheduler.AddWorker: scheduler already started")
	s.workers = append(s.workers, w)
}

// Start launches all registered workers.
func (s *Scheduler) Start() {
	assert.That(s.cancel == nil, "Scheduler.Start: already started")
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	for _, w := range s.workers {
		assert.That(w != nil, "Scheduler.Start: nil worker")
		s.wg.Add(1)
		go func(worker Worker) {
			defer s.wg.Done()
			if err := worker.Start(ctx); err != nil {
				slog.Error("worker failed", "error", err)
			}
		}(w)
	}

	slog.Info("job scheduler started", "workers", len(s.workers))
}

// Stop signals all workers to stop and waits for them to finish.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	for _, w := range s.workers {
		assert.That(w != nil, "Scheduler.Stop: nil worker")
		w.Stop()
	}
	s.wg.Wait()
	slog.Info("job scheduler stopped")
}
