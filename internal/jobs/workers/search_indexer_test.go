package workers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		w.Write([]byte(`{"id":"doc-1"}`))
	}))
	defer srv.Close()

	si := NewSearchIndexer(srv.URL, "admin", "secret")
	doc := map[string]any{
		"id":      "post-123",
		"content": "hello world",
	}

	err := si.IndexPost(context.Background(), doc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if gotURL != "/api/chit-posts/_doc" {
		t.Fatalf("expected URL=/api/chit-posts/_doc, got %q", gotURL)
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
	err := si.IndexPost(context.Background(), map[string]any{"id": "1"})
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
		json.NewEncoder(w).Encode(response)
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
		json.NewEncoder(w).Encode(response)
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
