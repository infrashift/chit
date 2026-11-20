package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// errorWith answers with status, an optional Retry-After and body as is.
func errorWith(status int, retryAfter, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(status)
		if _, err := io.WriteString(w, body); err != nil {
			panic(err)
		}
	}
}

// getMeError calls GetMe against a server that answers with h.
func getMeError(t *testing.T, h http.HandlerFunc) error {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", h)
	client, _ := setupTestClient(t, mux)
	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe: no error")
	}
	return err
}

func TestRetryAfter(t *testing.T) {
	future := time.Now().Add(20 * time.Second).UTC().Format(http.TimeFormat)
	cases := []struct {
		header  string
		min     time.Duration
		max     time.Duration
		message string
	}{
		{"3", 3 * time.Second, 3 * time.Second, "try again in 3s"},
		{"3600", api.MaxRetryAfter, api.MaxRetryAfter, "try again in 60s"},
		{"-5", 0, 0, "try again soon"},
		{"soon", 0, 0, "try again soon"},
		{"", 0, 0, "try again soon"},
		{future, 15 * time.Second, 20 * time.Second, "try again in"},
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			err := getMeError(t, errorWith(429, tc.header, `{"id":"RateLimit","message":"rate limit"}`))
			wait, ok := api.RetryAfterOf(err)
			if !ok {
				t.Fatalf("RetryAfterOf(%v): not a 429", err)
			}
			if wait < tc.min || wait > tc.max {
				t.Errorf("wait = %v, want %v to %v", wait, tc.min, tc.max)
			}
			if msg := api.UserMessage(err); !strings.Contains(msg, tc.message) {
				t.Errorf("UserMessage = %q, want %q", msg, tc.message)
			}
		})
	}
	if _, ok := api.RetryAfterOf(errors.New("plain")); ok {
		t.Error("RetryAfterOf: a plain error is a 429")
	}
	if _, ok := api.RetryAfterOf(&api.APIError{StatusCode: 500}); ok {
		t.Error("RetryAfterOf: a 500 is a 429")
	}
}

func TestUserMessage(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"refusal detail", 403, `{"id":"channel.membership_refused","message":"refused","reason":"clearance","detail":"Bob lacks clearance"}`, "the membership policy refused this: Bob lacks clearance"},
		{"refusal reason", 403, `{"id":"channel.membership_refused","message":"refused","reason":"clearance"}`, "refused this (clearance)"},
		{"refusal bare", 403, `{"id":"channel.membership_refused","message":"refused"}`, "the membership policy refused this"},
		{"file off", 503, `{"id":"file.disabled","message":"x"}`, "attachments are off"},
		{"file large", 413, `{"id":"file.too_large","message":"x"}`, "too large"},
		{"quota", 413, `{"id":"file.quota_exceeded","message":"x"}`, "quota"},
		{"svg", 415, `{"id":"file.svg","message":"x"}`, "only PNG, JPEG, WebP and GIF"},
		{"other file", 422, `{"id":"file.polyglot","message":"the image holds another format"}`, "refused the image: the image holds another format"},
		{"conflict", 409, `{"id":"SqlChannelStore.Save","message":"a channel with that name exists"}`, "that name is in use: a channel with that name exists"},
		{"conflict no body", 409, ``, "that name is in use: conflict"},
		{"other", 404, `{"id":"x","message":"not found"}`, "api: 404 not found"},
		{"text body", 502, "bad gateway\n", "api: 502 bad gateway"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := getMeError(t, errorWith(tc.status, "", tc.body))
			if got := api.UserMessage(err); !strings.Contains(got, tc.want) {
				t.Errorf("UserMessage = %q, want %q", got, tc.want)
			}
			if got := api.StatusOf(err); got != tc.status {
				t.Errorf("StatusOf = %d, want %d", got, tc.status)
			}
		})
	}
	if got := api.UserMessage(errors.New("plain")); got != "plain" {
		t.Errorf("UserMessage(plain) = %q", got)
	}
	if got := api.StatusOf(errors.New("plain")); got != 0 {
		t.Errorf("StatusOf(plain) = %d", got)
	}
}

func TestAPIError_ErrorAndID(t *testing.T) {
	e := &api.APIError{StatusCode: 403, AppError: &model.AppError{ID: "x", Message: "refused", Detail: "why"}}
	if got := e.Error(); got != "api: 403 refused: why" {
		t.Errorf("Error = %q", got)
	}
	if e.ID() != "x" {
		t.Errorf("ID = %q", e.ID())
	}
	bare := &api.APIError{StatusCode: 500}
	if bare.Error() != "api: 500" || bare.ID() != "" {
		t.Errorf("bare error: %q %q", bare.Error(), bare.ID())
	}
	if api.UserMessage(bare) != "api: 500" {
		t.Errorf("UserMessage(bare) = %q", api.UserMessage(bare))
	}
}

// tagRoutesMux serves the tag and channel-tag routes and records each
// call as "METHOD URI BODY" in calls.
func tagRoutesMux(t *testing.T, calls *[]string) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	record := func(r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*calls = append(*calls, r.Method+" "+r.URL.RequestURI()+" "+strings.TrimSpace(string(body)))
	}
	ok := func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(t, w, map[string]string{"status": "OK"})
	}
	mux.HandleFunc("GET /api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(t, w, []*model.Tag{{ID: "t1", Name: "a"}})
	})
	mux.HandleFunc("PUT /api/v1/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(t, w, model.Tag{ID: r.PathValue("id"), Name: "renamed"})
	})
	mux.HandleFunc("GET /api/v1/channels/{id}/tags", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(t, w, []*model.Tag{{ID: "t2"}})
	})
	mux.HandleFunc("POST /api/v1/tags/channels", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(t, w, []*model.TaggedChannel{{Channel: &model.Channel{ID: "c9"}}})
	})
	mux.HandleFunc("DELETE /api/v1/tags/{id}", ok)
	mux.HandleFunc("POST /api/v1/channels/{id}/tags", ok)
	mux.HandleFunc("DELETE /api/v1/channels/{id}/tags/{tag}", ok)
	return mux
}

func TestTagRoutes(t *testing.T) {
	var calls []string
	mux := tagRoutesMux(t, &calls)
	client, _ := setupTestClient(t, mux)
	ctx := context.Background()

	tags, err := client.GetTags(ctx, "c 1")
	if err != nil || len(tags) != 1 {
		t.Fatalf("GetTags = %v, %v", tags, err)
	}
	tag, err := client.RenameTag(ctx, "t1", "renamed")
	if err != nil || tag.Name != "renamed" {
		t.Fatalf("RenameTag = %+v, %v", tag, err)
	}
	if err := client.DeleteTag(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	ctags, err := client.GetChannelTags(ctx, "c1")
	if err != nil || len(ctags) != 1 {
		t.Fatalf("GetChannelTags = %v, %v", ctags, err)
	}
	if err := client.AddChannelTag(ctx, "c1", "t2"); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveChannelTag(ctx, "c1", "t2"); err != nil {
		t.Fatal(err)
	}
	found, err := client.FindChannelsByTags(ctx, []string{"t2"}, "team1")
	if err != nil || len(found) != 1 || found[0].Channel.ID != "c9" {
		t.Fatalf("FindChannelsByTags = %v, %v", found, err)
	}
	if _, err := client.FindChannelsByTags(ctx, []string{"t2"}, ""); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /api/v1/tags?channel_id=c+1 ",
		`PUT /api/v1/tags/t1 {"name":"renamed"}`,
		"DELETE /api/v1/tags/t1 ",
		"GET /api/v1/channels/c1/tags ",
		`POST /api/v1/channels/c1/tags {"tag_id":"t2"}`,
		"DELETE /api/v1/channels/c1/tags/t2 ",
		`POST /api/v1/tags/channels {"tag_ids":["t2"],"team_id":"team1"}`,
		`POST /api/v1/tags/channels {"tag_ids":["t2"]}`,
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestRenameTag_Conflict(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/tags/{id}", errorWith(409, "", `{"id":"x","message":"a tag with that name exists"}`))
	client, _ := setupTestClient(t, mux)
	tag, err := client.RenameTag(context.Background(), "t1", "dup")
	if tag != nil || api.StatusOf(err) != 409 {
		t.Errorf("RenameTag = %v, %v; want nil and a 409", tag, err)
	}
}

func TestUploadFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse form: %v", err)
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Errorf("form file: %v", err)
			return
		}
		data, err := io.ReadAll(f)
		if err != nil {
			t.Errorf("read part: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(t, w, model.File{
			ID: "f1", ChannelID: r.PathValue("id"), Name: h.Filename,
			ContentType: h.Header.Get("Content-Type"), SizeBytes: int64(len(data)),
			AltText: r.FormValue("alt_text"),
		})
	})
	client, _ := setupTestClient(t, mux)

	f, err := client.UploadFile(context.Background(), "c1", "cat.png", "image/png", " a cat ", strings.NewReader("PNGDATA"))
	if err != nil {
		t.Fatal(err)
	}
	want := model.File{ID: "f1", ChannelID: "c1", Name: "cat.png", ContentType: "image/png", SizeBytes: 7, AltText: "a cat"}
	if *f != want {
		t.Errorf("File = %+v, want %+v", *f, want)
	}
	f, err = client.UploadFile(context.Background(), "c1", "cat.png", "image/png", "", strings.NewReader("x"))
	if err != nil || f.AltText != "" {
		t.Errorf("an upload with no alt text = %+v, %v", f, err)
	}
}

func TestUploadFile_TooLarge(t *testing.T) {
	mux := http.NewServeMux()
	reached := false
	mux.HandleFunc("POST /api/v1/channels/{id}/files", func(_ http.ResponseWriter, _ *http.Request) { reached = true })
	client, _ := setupTestClient(t, mux)
	big := io.LimitReader(zeros{}, model.MaxFileBytes+1)
	_, err := client.UploadFile(context.Background(), "c1", "big.png", "image/png", "", big)
	if !errors.Is(err, api.ErrFileTooLarge) {
		t.Errorf("err = %v, want ErrFileTooLarge", err)
	}
	if reached {
		t.Error("the client sent a file that is too large")
	}
}

func TestUploadFile_ServerRefuses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/{id}/files", errorWith(415, "", `{"id":"file.svg","message":"svg"}`))
	client, _ := setupTestClient(t, mux)
	f, err := client.UploadFile(context.Background(), "c1", "x.svg", "image/svg+xml", "", strings.NewReader("<svg/>"))
	if f != nil || !strings.Contains(api.UserMessage(err), "only PNG") {
		t.Errorf("UploadFile = %v, %v", f, err)
	}
}

// zeros is an endless reader of zero bytes.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestFileInfoAndDownload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/{id}/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, model.File{ID: r.PathValue("id"), Name: "cat.png"})
	})
	mux.HandleFunc("GET /api/v1/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "gone" {
			errorWith(404, "", `{"id":"x","message":"no such file"}`)(w, r)
			return
		}
		if _, err := io.WriteString(w, "0123456789"); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	client, _ := setupTestClient(t, mux)
	ctx := context.Background()

	f, err := client.GetFileInfo(ctx, "f1")
	if err != nil || f.ID != "f1" || f.Name != "cat.png" {
		t.Fatalf("GetFileInfo = %+v, %v", f, err)
	}
	var buf bytes.Buffer
	n, err := client.DownloadFile(ctx, "f1", &buf, 10)
	if err != nil || n != 10 || buf.String() != "0123456789" {
		t.Errorf("DownloadFile = %d, %v, %q", n, err, buf.String())
	}
	buf.Reset()
	if _, err := client.DownloadFile(ctx, "f1", &buf, 9); !errors.Is(err, api.ErrFileTooLarge) {
		t.Errorf("a file past the limit: err = %v", err)
	}
	if _, err := client.DownloadFile(ctx, "gone", &buf, 10); api.StatusOf(err) != 404 {
		t.Errorf("a missing file: err = %v", err)
	}
}

func TestGetFileInfo_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/{id}/info", errorWith(403, "", `{"id":"x","message":"not yours"}`))
	client, _ := setupTestClient(t, mux)
	f, err := client.GetFileInfo(context.Background(), "f1")
	if f != nil || api.StatusOf(err) != 403 {
		t.Errorf("GetFileInfo = %v, %v", f, err)
	}
}

func TestCreatePost_SendsFileIDs(t *testing.T) {
	mux := http.NewServeMux()
	var got map[string]any
	mux.HandleFunc("POST /api/v1/posts", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		writeJSON(t, w, model.Post{ID: "p1"})
	})
	client, _ := setupTestClient(t, mux)
	if _, err := client.CreatePost(context.Background(), &model.Post{ChannelID: "c1", FileIDs: []string{"f1"}}); err != nil {
		t.Fatal(err)
	}
	ids, ok := got["file_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "f1" {
		t.Errorf("file_ids = %v", got["file_ids"])
	}
}
