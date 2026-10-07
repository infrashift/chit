package workers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

func TestNewSearchIndexer(t *testing.T) {
	si := NewSearchIndexer("http://localhost:4080", "admin", "pass123")
	if si.zincURL != "http://localhost:4080" {
		t.Fatalf("expected zincURL=%q, got %q", "http://localhost:4080", si.zincURL)
	}
	if si.zincUser != "admin" {
		t.Fatalf("expected zincUser=%q, got %q", "admin", si.zincUser)
	}
	if si.zincPass != "pass123" {
		t.Fatalf("expected zincPass=%q, got %q", "pass123", si.zincPass)
	}
	if si.client == nil {
		t.Fatal("expected non-nil http client")
	}
}

func TestSearchIndexer_StartStop(t *testing.T) {
	si := NewSearchIndexer("http://localhost:4080", "admin", "pass")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- si.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestSearchIndexer_IndexPost_Success(t *testing.T) {
	var (
		gotURL         string
		gotAuth        string
		gotContentType string
		gotBody        []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.Path
		user, _, _ := r.BasicAuth()
		gotAuth = user
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"doc-1"}`))
	}))
	defer srv.Close()

	si := NewSearchIndexer(srv.URL, "admin", "secret")
	doc := map[string]any{
		"id":      "post-123",
		"content": "hello world",
	}

	err := si.IndexPost(context.Background(), "post-123", doc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if gotURL != "/api/chit-posts/_doc/post-123" {
		t.Fatalf("expected URL=/api/chit-posts/_doc/post-123, got %q", gotURL)
	}
	if gotAuth != "admin" {
		t.Fatalf("expected basic auth user=%q, got %q", "admin", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected Content-Type=application/json, got %q", gotContentType)
	}
	if !strings.Contains(string(gotBody), "post-123") {
		t.Fatalf("expected body to contain post-123, got %q", string(gotBody))
	}
}

func TestSearchIndexer_IndexPost_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	si := NewSearchIndexer(srv.URL, "admin", "secret")
	err := si.IndexPost(context.Background(), "1", map[string]any{"id": "1"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSearchIndexer_Search_Success(t *testing.T) {
	response := map[string]any{
		"hits": map[string]any{
			"hits": []map[string]any{
				{"_id": "post-1"},
				{"_id": "post-2"},
				{"_id": "post-3"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chit-posts/_search" {
			t.Errorf("expected URL=/api/chit-posts/_search, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer srv.Close()

	si := NewSearchIndexer(srv.URL, "admin", "secret")
	ids, err := si.Search(context.Background(), "hello", 0, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != "post-1" || ids[1] != "post-2" || ids[2] != "post-3" {
		t.Fatalf("unexpected IDs: %v", ids)
	}
}

func TestSearchIndexer_Search_EmptyHits(t *testing.T) {
	response := map[string]any{
		"hits": map[string]any{
			"hits": []map[string]any{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer srv.Close()

	si := NewSearchIndexer(srv.URL, "admin", "secret")
	ids, err := si.Search(context.Background(), "nothing", 0, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ids == nil {
		t.Fatal("expected non-nil slice for empty results")
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 IDs, got %d", len(ids))
	}
}

// fakePostStore is a PostSource over a fixed slice.
type fakePostStore struct {
	posts []*model.Post
}

func (f *fakePostStore) GetPostsSince(_ context.Context, since int64, limit int) ([]*model.Post, error) {
	var out []*model.Post
	for _, p := range f.posts {
		if p.UpdateAt > since {
			out = append(out, p)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func TestSearchIndexer_RunOnce_IndexesAndDeletes(t *testing.T) {
	var mu sync.Mutex
	indexed := map[string]string{} // id -> method

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		parts := strings.Split(r.URL.Path, "/")
		id := parts[len(parts)-1]
		indexed[id] = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ps := &fakePostStore{posts: []*model.Post{
		{ID: "p1", ChannelID: "ch", UserID: "u", Content: "hello", UpdateAt: 100},
		{ID: "p2", ChannelID: "ch", UserID: "u", Content: "gone", UpdateAt: 200, DeleteAt: 150},
		{ID: "p3", ChannelID: "ch", UserID: "u", Content: "world", UpdateAt: 300},
	}}

	si := NewSearchIndexer(srv.URL, "admin", "secret").WithPostStore(ps)
	si.runOnce(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if indexed["p1"] != http.MethodPut || indexed["p3"] != http.MethodPut {
		t.Fatalf("expected p1/p3 indexed via PUT, got %v", indexed)
	}
	if indexed["p2"] != http.MethodDelete {
		t.Fatalf("expected soft-deleted p2 removed via DELETE, got %v", indexed)
	}
	if si.watermark != 300 {
		t.Fatalf("expected watermark=300, got %d", si.watermark)
	}
}

func TestSearchIndexer_RunOnce_StopsOnErrorAndRetries(t *testing.T) {
	var mu sync.Mutex
	fail := true
	var puts []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		puts = append(puts, parts[len(parts)-1])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ps := &fakePostStore{posts: []*model.Post{
		{ID: "p1", ChannelID: "ch", UserID: "u", Content: "hello", UpdateAt: 100},
	}}
	si := NewSearchIndexer(srv.URL, "admin", "secret").WithPostStore(ps)

	si.runOnce(context.Background())
	if si.watermark != 0 {
		t.Fatalf("watermark must not advance on failure, got %d", si.watermark)
	}

	mu.Lock()
	fail = false
	mu.Unlock()

	si.runOnce(context.Background())
	if si.watermark != 100 {
		t.Fatalf("expected watermark=100 after retry, got %d", si.watermark)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(puts) != 1 || puts[0] != "p1" {
		t.Fatalf("expected p1 indexed on retry, got %v", puts)
	}
}

// ZincSearch answers 400 (not 404) when deleting a document that was never
// indexed; that must not stall the watermark.
func TestSearchIndexer_DeleteMissingDocIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ps := &fakePostStore{posts: []*model.Post{
		{ID: "gone", ChannelID: "ch", UserID: "u", Content: "x", UpdateAt: 100, DeleteAt: 50},
		{ID: "live", ChannelID: "ch", UserID: "u", Content: "y", UpdateAt: 200},
	}}
	si := NewSearchIndexer(srv.URL, "admin", "secret").WithPostStore(ps)

	si.runOnce(context.Background())
	if si.watermark != 200 {
		t.Fatalf("watermark stalled on missing-doc delete: got %d, want 200", si.watermark)
	}
}
