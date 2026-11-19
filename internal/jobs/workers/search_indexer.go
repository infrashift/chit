package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/model"
)

const (
	indexBatchSize = 200
	// indexSettleLag holds back posts updated in the last few seconds.
	// update_at is stamped in Go before the INSERT commits, so a transaction
	// that commits late can appear behind posts already indexed; reading
	// only settled rows keeps the cursor from passing it.
	indexSettleLag = 3 * time.Second
	// indexMaxAttempts is how many times one post is retried before the
	// indexer gives up on it and moves on. Without a limit one document the
	// index refuses stopped indexing for everything after it, forever.
	indexMaxAttempts = 3

	// indexMaxBatchesPerTick caps one tick at 100 batches (20,000 posts), so
	// runOnce has a fixed loop bound (Power of 10 rule 2). The next tick goes
	// on from the cursor.
	indexMaxBatchesPerTick = 100
)

// PostSource is what the indexer reads posts from: the one PostStore method
// it uses, so tests and callers need not supply the rest.
type PostSource interface {
	GetPostsSince(ctx context.Context, after model.PostCursor, until int64, limit int) ([]*model.Post, error)
}

// SearchIndexer is a background worker that syncs posts to ZincSearch.
// It is also used as a plain ZincSearch client by the search path (posts nil).
type SearchIndexer struct {
	zincURL  string
	zincUser string
	zincPass string
	client   *http.Client
	stop     chan struct{}

	posts    PostSource
	cursor   model.PostCursor // the last post synced
	failures map[string]int   // post ID → consecutive failed attempts
	clock    clock
}

// clock supplies the current time; tests substitute a fake.
type clock interface {
	Now() time.Time
}

// systemClock is the production clock.
type systemClock struct{}

// Now returns time.Now().
func (systemClock) Now() time.Time { return time.Now() }

// NewSearchIndexer creates a new search indexer worker.
func NewSearchIndexer(zincURL, zincUser, zincPass string) *SearchIndexer {
	si := &SearchIndexer{
		zincURL:  zincURL,
		zincUser: zincUser,
		zincPass: zincPass,
		client:   &http.Client{Timeout: 10 * time.Second},
		stop:     make(chan struct{}),
		failures: map[string]int{},
		clock:    systemClock{},
	}
	assert.That(si.client != nil && si.stop != nil, "NewSearchIndexer: no client or stop channel")
	assert.That(si.failures != nil && si.clock != nil, "NewSearchIndexer: no failure map or clock")
	return si
}

// WithPostStore enables the background indexing loop by giving the worker a
// source of posts to poll.
func (si *SearchIndexer) WithPostStore(ps PostSource) *SearchIndexer {
	assert.That(ps != nil, "SearchIndexer.WithPostStore: nil post source")
	assert.That(si.posts == nil, "SearchIndexer.WithPostStore: post source already set")
	si.posts = ps
	return si
}

// Start begins the indexing loop. The cursor is in-memory only: a restart
// re-indexes from the beginning, which is safe because documents are written
// by post ID (upserts).
func (si *SearchIndexer) Start(ctx context.Context) error {
	if si.posts == nil {
		slog.Warn("search indexer started without a post store; indexing disabled")
		return nil
	}
	slog.Info("search indexer started")
	assert.That(si.stop != nil, "SearchIndexer.Start: not built by NewSearchIndexer")
	assert.That(si.clock != nil && si.failures != nil, "SearchIndexer.Start: no clock or failure map")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil
		case <-si.stop:
			return nil
		case <-ticker.C:
			si.runOnce(ctx)
		}
	}
	return nil
}

// runOnce syncs every settled post after the cursor. A post that fails is
// retried on the next tick; after indexMaxAttempts it is logged and skipped.
func (si *SearchIndexer) runOnce(ctx context.Context) {
	assert.That(si.posts != nil, "SearchIndexer.runOnce: no post source")
	assert.That(si.clock != nil && si.failures != nil, "SearchIndexer.runOnce: not built by NewSearchIndexer")
	until := si.clock.Now().Add(-indexSettleLag).UnixMilli()
	for range indexMaxBatchesPerTick {
		posts, err := si.posts.GetPostsSince(ctx, si.cursor, until, indexBatchSize)
		if err != nil {
			slog.Warn("search indexer: failed to fetch posts", "error", err)
			return
		}
		assert.That(len(posts) <= indexBatchSize, "SearchIndexer.runOnce: more posts than the batch size")

		for _, p := range posts {
			if err := si.sync(ctx, p); err != nil {
				si.failures[p.ID]++
				if si.failures[p.ID] < indexMaxAttempts {
					slog.Warn("search indexer: failed to sync post; will retry",
						"post_id", p.ID, "attempt", si.failures[p.ID], "error", err)
					return
				}
				slog.Error("search indexer: giving up on post; it will be missing from search",
					"post_id", p.ID, "attempts", si.failures[p.ID], "error", err)
			}
			delete(si.failures, p.ID)
			si.cursor = model.PostCursor{UpdateAt: p.UpdateAt, ID: p.ID}
		}

		if len(posts) < indexBatchSize {
			return
		}
	}
}

// sync writes one post's current state to the index.
func (si *SearchIndexer) sync(ctx context.Context, p *model.Post) error {
	assert.That(p != nil, "SearchIndexer.sync: nil post")
	assert.That(si.client != nil, "SearchIndexer.sync: nil HTTP client")
	if p.DeleteAt > 0 {
		return si.DeletePost(ctx, p.ID)
	}
	return si.IndexPost(ctx, p.ID, map[string]any{
		"channel_id": p.ChannelID,
		"user_id":    p.UserID,
		"content":    p.Content,
		"create_at":  p.CreateAt,
	})
}

// Stop signals the indexer to shut down.
func (si *SearchIndexer) Stop() {
	assert.That(si.stop != nil, "SearchIndexer.Stop: not built by NewSearchIndexer")
	close(si.stop)
}

// IndexPost upserts a single post document in ZincSearch, keyed by post ID so
// re-indexing is idempotent and search hits map straight back to posts.
func (si *SearchIndexer) IndexPost(ctx context.Context, id string, doc map[string]any) error {
	assert.That(si.client != nil, "SearchIndexer.IndexPost: nil HTTP client")
	assert.That(doc != nil, "SearchIndexer.IndexPost: nil document")
	assert.That(ctx != nil, "SearchIndexer.IndexPost: nil context")
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}

	url := fmt.Sprintf("%s/api/%s/_doc/%s", si.zincURL, "chit-posts", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(si.zincUser, si.zincPass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := si.client.Do(req)
	if err != nil {
		return fmt.Errorf("index post: %w", err)
	}
	defer func() { errlog.Debug(resp.Body.Close(), "close search index response body") }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("index post: status %d", resp.StatusCode)
	}

	return nil
}

// DeletePost removes a post document from the index. Missing documents are
// not an error — the post may never have been indexed. ZincSearch reports
// that as 400 ("id not found") rather than 404, so both are tolerated.
func (si *SearchIndexer) DeletePost(ctx context.Context, id string) error {
	assert.That(si.client != nil, "SearchIndexer.DeletePost: nil HTTP client")
	assert.That(ctx != nil, "SearchIndexer.DeletePost: nil context")
	url := fmt.Sprintf("%s/api/%s/_doc/%s", si.zincURL, "chit-posts", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	req.SetBasicAuth(si.zincUser, si.zincPass)

	resp, err := si.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete post from index: %w", err)
	}
	defer func() { errlog.Debug(resp.Body.Close(), "close search delete response body") }()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("delete post from index: status %d", resp.StatusCode)
	}

	return nil
}

// Search queries ZincSearch and returns matching document IDs.
func (si *SearchIndexer) Search(ctx context.Context, query string, from, size int) ([]string, error) {
	assert.That(si.client != nil, "SearchIndexer.Search: nil HTTP client")
	assert.That(from >= 0 && size > 0, "SearchIndexer.Search: bad window")
	searchReq := map[string]any{
		"search_type": "match",
		"query": map[string]any{
			"term": query,
		},
		"from":        from,
		"max_results": size,
		"_source":     []string{"_id"},
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("marshal search: %w", err)
	}

	url := fmt.Sprintf("%s/api/%s/_search", si.zincURL, "chit-posts")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	req.SetBasicAuth(si.zincUser, si.zincPass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := si.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer func() { errlog.Debug(resp.Body.Close(), "close search response body") }()

	// Without this a 401 or 500 decoded as an empty hit list: search said
	// "nothing found" when it had not looked.
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search: status %d", resp.StatusCode)
	}

	var result struct {
		Hits struct {
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search result: %w", err)
	}

	ids := make([]string, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		ids = append(ids, hit.ID)
	}
	assert.That(len(ids) == len(result.Hits.Hits), "SearchIndexer.Search: lost a hit")
	return ids, nil
}
