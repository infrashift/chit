package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

// Start must return when its context ends. It needs a post source to run the
// loop at all: without one it returns at once ("indexing disabled"), which
// is what this test used to exercise without noticing.
func TestSearchIndexer_StartStop(t *testing.T) {
	si := newTestIndexer("http://127.0.0.1:1", &fakePostStore{})
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

// fakePostStore is a PostSource over a fixed slice, in (update_at, id) order.
type fakePostStore struct {
	posts []*model.Post
}

func (f *fakePostStore) GetPostsSince(_ context.Context, after model.PostCursor, until int64, limit int) ([]*model.Post, error) {
	var out []*model.Post
	for _, p := range f.posts {
		if p.UpdateAt <= until && (p.UpdateAt > after.UpdateAt || (p.UpdateAt == after.UpdateAt && p.ID > after.ID)) {
			out = append(out, p)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// newTestIndexer indexes into zincURL from ps, with every post already
// settled.
func newTestIndexer(zincURL string, ps PostSource) *SearchIndexer {
	si := NewSearchIndexer(zincURL, "admin", "secret").WithPostStore(ps)
	si.now = func() time.Time { return time.UnixMilli(1 << 50) }
	return si
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

	si := newTestIndexer(srv.URL, ps)
	si.runOnce(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if indexed["p1"] != http.MethodPut || indexed["p3"] != http.MethodPut {
		t.Fatalf("expected p1/p3 indexed via PUT, got %v", indexed)
	}
	if indexed["p2"] != http.MethodDelete {
		t.Fatalf("expected soft-deleted p2 removed via DELETE, got %v", indexed)
	}
	if si.cursor.UpdateAt != 300 || si.cursor.ID != "p3" {
		t.Fatalf("cursor = %+v, want p3 at 300", si.cursor)
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
	si := newTestIndexer(srv.URL, ps)

	si.runOnce(context.Background())
	if si.cursor.UpdateAt != 0 {
		t.Fatalf("cursor must not advance on failure, got %+v", si.cursor)
	}

	mu.Lock()
	fail = false
	mu.Unlock()

	si.runOnce(context.Background())
	if si.cursor.UpdateAt != 100 {
		t.Fatalf("cursor = %+v after retry, want 100", si.cursor)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(puts) != 1 || puts[0] != "p1" {
		t.Fatalf("expected p1 indexed on retry, got %v", puts)
	}
}

// ZincSearch answers 400 (not 404) when deleting a document that was never
// indexed; that must not stall the cursor.
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
	si := newTestIndexer(srv.URL, ps)

	si.runOnce(context.Background())
	if si.cursor.UpdateAt != 200 {
		t.Fatalf("cursor stalled on missing-doc delete: got %+v, want 200", si.cursor)
	}
}

// The cursor used to be update_at alone. More than a batch of posts sharing
// one update_at left the rest past the batch limit and behind the cursor,
// never indexed.
func TestSearchIndexer_TiesAcrossABatchAreAllIndexed(t *testing.T) {
	var mu sync.Mutex
	indexed := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		parts := strings.Split(r.URL.Path, "/")
		indexed[parts[len(parts)-1]] = true
	}))
	defer srv.Close()

	var posts []*model.Post
	for i := range indexBatchSize + 5 {
		posts = append(posts, &model.Post{ID: fmt.Sprintf("p%04d", i), Content: "x", UpdateAt: 100})
	}
	si := newTestIndexer(srv.URL, &fakePostStore{posts: posts})
	si.runOnce(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(indexed) != len(posts) {
		t.Fatalf("indexed %d of %d posts sharing one update_at", len(indexed), len(posts))
	}
}

// A post updated within the settle lag waits for a later tick: its
// transaction may not have committed, and the posts around it may not all
// be visible yet.
func TestSearchIndexer_RecentPostsWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	now := time.UnixMilli(1_700_000_000_000)
	si := newTestIndexer(srv.URL, &fakePostStore{posts: []*model.Post{
		{ID: "old", Content: "x", UpdateAt: now.Add(-time.Minute).UnixMilli()},
		{ID: "fresh", Content: "x", UpdateAt: now.UnixMilli()},
	}})
	si.now = func() time.Time { return now }
	si.runOnce(context.Background())
	if si.cursor.ID != "old" {
		t.Fatalf("cursor = %+v, want it stopped before the unsettled post", si.cursor)
	}
}

// One document the index refuses used to stop indexing for every post after
// it, retried every tick forever.
func TestSearchIndexer_GivesUpOnAPoisonPost(t *testing.T) {
	var mu sync.Mutex
	var synced []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		id := parts[len(parts)-1]
		if id == "poison" {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		mu.Lock()
		synced = append(synced, id)
		mu.Unlock()
	}))
	defer srv.Close()

	si := newTestIndexer(srv.URL, &fakePostStore{posts: []*model.Post{
		{ID: "poison", Content: "x", UpdateAt: 100},
		{ID: "after", Content: "y", UpdateAt: 200},
	}})
	for range indexMaxAttempts {
		si.runOnce(context.Background())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(synced) != 1 || synced[0] != "after" {
		t.Fatalf("synced %v, want the post after the poison one", synced)
	}
}
