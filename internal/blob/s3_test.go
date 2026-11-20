package blob

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The example in the AWS documentation for a signed GET Object ("Example:
// GET Object", Signature Version 4, authenticating requests with the
// Authorization header).
func TestSign_MatchesTheAWSExample(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"https://examplebucket.s3.amazonaws.com/test.txt", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-9")
	sign(req, nil, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1",
		time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, " +
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, " +
		"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization\n got %s\nwant %s", got, want)
	}
}

func TestURIEncode(t *testing.T) {
	if got := uriEncodePath("/b/a%20b/c~d"); got != "/b/a%20b/c~d" {
		t.Errorf("path: %s", got)
	}
	if got := uriEncode("a/b=c", false); got != "a%2Fb%3Dc" {
		t.Errorf("query: %s", got)
	}
}

// fakeS3 is a path-style bucket that insists on a SigV4 Authorization
// header from the right key.
type fakeS3 struct {
	t       *testing.T
	mu      sync.Mutex
	bucket  bool
	objects map[string]string
	types   map[string]string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AK/") ||
		r.Header.Get("X-Amz-Date") == "" || r.Header.Get("X-Amz-Content-Sha256") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	key, isObject := strings.CutPrefix(r.URL.Path, "/chit-files/")
	if r.URL.Path == "/chit-files" {
		switch r.Method {
		case http.MethodHead:
			if !f.bucket {
				w.WriteHeader(http.StatusNotFound)
			}
		case http.MethodPut:
			f.bucket = true
		}
		return
	}
	if !isObject || !f.bucket {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPut:
		b, err := io.ReadAll(r.Body)
		if err != nil || r.Header.Get("X-Amz-Content-Sha256") != sha256Hex(b) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.objects[key] = string(b)
		f.types[key] = r.Header.Get("Content-Type")
	case http.MethodGet:
		v, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, err := io.WriteString(w, v); err != nil {
			f.t.Errorf("fake S3: write object %s: %v", key, err) // the server's goroutine: not t.Fatal
		}
	case http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	}
}

func TestS3_RoundTrip(t *testing.T) {
	fake := &fakeS3{t: t, objects: map[string]string{}, types: map[string]string{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s, err := NewS3(&S3Config{Endpoint: srv.URL, Bucket: "chit-files", Region: "us-east-1", AccessKeyID: "AK", SecretAccessKey: "SK"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if err = s.Put(ctx, "c/f", "image/png", []byte("x")); err == nil {
		t.Fatal("put before the bucket exists succeeded")
	}
	for range 2 { // idempotent
		if err = s.EnsureBucket(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Put(ctx, "c/f", "image/png", []byte("bytes")); err != nil {
		t.Fatal(err)
	}
	if fake.types["c/f"] != "image/png" {
		t.Errorf("content type %q", fake.types["c/f"])
	}
	rc, n, err := s.Get(ctx, "c/f")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("close object: %v", err)
	}
	if string(b) != "bytes" || n != 5 {
		t.Fatalf("got %q (%d)", b, n)
	}
	if err := s.Delete(ctx, "c/f"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get(ctx, "c/f"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	if err := s.Delete(ctx, "c/f"); err != nil {
		t.Fatalf("deleting a missing object: %v", err)
	}
}

func TestNewS3_RefusesAnIncompleteConfig(t *testing.T) {
	for _, cfg := range []*S3Config{
		{Endpoint: "localhost:8333", Bucket: "b", Region: "r", AccessKeyID: "a", SecretAccessKey: "s"},
		{Endpoint: "http://localhost:8333", Bucket: "b", Region: "r", AccessKeyID: "a"},
		{Endpoint: "http://localhost:8333", Region: "r", AccessKeyID: "a", SecretAccessKey: "s"},
	} {
		if _, err := NewS3(cfg); err == nil {
			t.Errorf("accepted %+v", cfg)
		}
	}
}
