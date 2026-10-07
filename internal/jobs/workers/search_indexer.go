package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/infrashift/chit/internal/model"
)

const indexBatchSize = 200

// PostSource is what the indexer reads posts from: the one PostStore method
// it uses, so tests and callers need not supply the rest.
type PostSource interface {
	GetPostsSince(ctx context.Context, sinceUpdateAt int64, limit int) ([]*model.Post, error)
}

// SearchIndexer is a background worker that syncs posts to ZincSearch.
// It is also used as a plain ZincSearch client by the search path (posts nil).
type SearchIndexer struct {
	zincURL  string
	zincUser string
	zincPass string
	client   *http.Client
	stop     chan struct{}

	posts     PostSource
	watermark int64 // update_at of the last indexed post
}

// NewSearchIndexer creates a new search indexer worker.
func NewSearchIndexer(zincURL, zincUser, zincPass string) *SearchIndexer {
	return &SearchIndexer{
		zincURL:  zincURL,
		zincUser: zincUser,
		zincPass: zincPass,
		client:   &http.Client{Timeout: 10 * time.Second},
		stop:     make(chan struct{}),
	}
}

// WithPostStore enables the background indexing loop by giving the worker a
// source of posts to poll.
func (si *SearchIndexer) WithPostStore(ps PostSource) *SearchIndexer {
	si.posts = ps
	return si
}

// Start begins the indexing loop. The watermark is in-memory only: a restart
// re-indexes from the beginning, which is safe because documents are written
// by post ID (upserts).
func (si *SearchIndexer) Start(ctx context.Context) error {
	if si.posts == nil {
		slog.Warn("search indexer started without a post store; indexing disabled")
		return nil
	}
	slog.Info("search indexer started")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-si.stop:
			return nil
		case <-ticker.C:
			si.runOnce(ctx)
		}
	}
}

// runOnce drains all posts updated since the watermark. On any indexing error
// the batch stops and the watermark stays put, so the next tick retries.
func (si *SearchIndexer) runOnce(ctx context.Context) {
	for {
		posts, err := si.posts.GetPostsSince(ctx, si.watermark, indexBatchSize)
		if err != nil {
			slog.Warn("search indexer: failed to fetch posts", "error", err)
			return
		}
		if len(posts) == 0 {
			return
		}

		for _, p := range posts {
			if p.DeleteAt > 0 {
				err = si.DeletePost(ctx, p.ID)
			} else {
				err = si.IndexPost(ctx, p.ID, map[string]any{
					"channel_id": p.ChannelID,
					"user_id":    p.UserID,
					"content":    p.Content,
					"create_at":  p.CreateAt,
				})
			}
			if err != nil {
				slog.Warn("search indexer: failed to sync post", "post_id", p.ID, "error", err)
				return
			}
			si.watermark = p.UpdateAt
		}

		if len(posts) < indexBatchSize {
			return
		}
	}
}

// Stop signals the indexer to shut down.
func (si *SearchIndexer) Stop() {
	close(si.stop)
}

// IndexPost upserts a single post document in ZincSearch, keyed by post ID so
// re-indexing is idempotent and search hits map straight back to posts.
func (si *SearchIndexer) IndexPost(ctx context.Context, id string, doc map[string]any) error {
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("index post: status %d", resp.StatusCode)
	}

	return nil
}

// DeletePost removes a post document from the index. Missing documents are
// not an error — the post may never have been indexed. ZincSearch reports
// that as 400 ("id not found") rather than 404, so both are tolerated.
func (si *SearchIndexer) DeletePost(ctx context.Context, id string) error {
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("delete post from index: status %d", resp.StatusCode)
	}

	return nil
}

// Search queries ZincSearch and returns matching document IDs.
func (si *SearchIndexer) Search(ctx context.Context, query string, from, size int) ([]string, error) {
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
	defer func() { _ = resp.Body.Close() }()

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
	return ids, nil
}
