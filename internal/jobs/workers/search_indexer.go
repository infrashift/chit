package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// SearchIndexer is a background worker that syncs posts to ZincSearch.
type SearchIndexer struct {
	zincURL  string
	zincUser string
	zincPass string
	client   *http.Client
	stop     chan struct{}
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

// Start begins the indexing loop.
func (si *SearchIndexer) Start(ctx context.Context) error {
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
			// TODO: poll for new/updated posts from store and index them
		}
	}
}

// Stop signals the indexer to shut down.
func (si *SearchIndexer) Stop() {
	close(si.stop)
}

// IndexPost sends a single post to ZincSearch for indexing.
func (si *SearchIndexer) IndexPost(ctx context.Context, doc map[string]any) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}

	url := fmt.Sprintf("%s/api/%s/_doc", si.zincURL, "chit-posts")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(si.zincUser, si.zincPass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := si.client.Do(req)
	if err != nil {
		return fmt.Errorf("index post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("index post: status %d", resp.StatusCode)
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
		"from": from,
		"max_results": size,
		"_source": []string{"_id"},
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
	defer resp.Body.Close()

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
