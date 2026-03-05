package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/infrashift/chit/internal/command"
)

// WebhookDispatcher is a worker pool that delivers CloudEvents to a webhook target.
type WebhookDispatcher struct {
	eventCh    <-chan *command.WebhookEvent
	targetURL  string
	secret     string
	numWorkers int
	client     *http.Client
	stop       chan struct{}
	wg         sync.WaitGroup
}

// NewWebhookDispatcher creates a dispatcher that reads from eventCh and POSTs
// CloudEvents to targetURL, signing payloads with secret.
func NewWebhookDispatcher(
	eventCh <-chan *command.WebhookEvent,
	targetURL, secret string,
	numWorkers int,
	timeout time.Duration,
) *WebhookDispatcher {
	return &WebhookDispatcher{
		eventCh:    eventCh,
		targetURL:  targetURL,
		secret:     secret,
		numWorkers: numWorkers,
		client:     &http.Client{Timeout: timeout},
		stop:       make(chan struct{}),
	}
}

// Start launches the worker goroutines. It blocks until ctx is cancelled or Stop is called.
func (d *WebhookDispatcher) Start(ctx context.Context) error {
	slog.Info("webhook dispatcher started", "workers", d.numWorkers, "target", d.targetURL)

	for i := range d.numWorkers {
		d.wg.Add(1)
		go d.worker(ctx, i)
	}

	// Block until done.
	select {
	case <-ctx.Done():
	case <-d.stop:
	}

	d.wg.Wait()
	slog.Info("webhook dispatcher stopped")
	return nil
}

// Stop signals all workers to drain and exit.
func (d *WebhookDispatcher) Stop() {
	close(d.stop)
	d.wg.Wait()
}

func (d *WebhookDispatcher) worker(ctx context.Context, id int) {
	defer d.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case evt, ok := <-d.eventCh:
			if !ok {
				return
			}
			if err := d.deliver(ctx, evt); err != nil {
				slog.Error("webhook delivery failed",
					"worker", id, "event_id", evt.EventID, "error", err)
			}
		}
	}
}

func (d *WebhookDispatcher) deliver(ctx context.Context, evt *command.WebhookEvent) error {
	ce := command.NewCloudEvent(evt)
	payload, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("marshal cloudevent: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.targetURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/cloudevents+json")
	req.Header.Set("ce-specversion", ce.SpecVersion)
	req.Header.Set("ce-type", ce.Type)
	req.Header.Set("ce-source", ce.Source)
	req.Header.Set("ce-id", ce.ID)

	if d.secret != "" {
		sig := command.SignPayload(payload, d.secret)
		req.Header.Set("X-Chit-Signature", sig)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook target returned status %d", resp.StatusCode)
	}

	slog.Debug("webhook delivered", "event_id", evt.EventID, "status", resp.StatusCode)
	return nil
}
