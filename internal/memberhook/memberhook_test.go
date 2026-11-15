package memberhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/memberhook"
	"github.com/infrashift/chit/internal/memberhook/memberhooktest"
)

func sampleRequest() *memberhook.Request {
	return &memberhook.Request{
		ChannelID:        "019421a0-0000-7000-8000-00000000c001",
		UserID:           "019421a0-0000-7000-8000-00000000a001",
		KratosIdentityID: "9b4c2a71-7a49-4d55-9f0a-2a8f3c1d0e11",
		ActorType:        memberhook.ActorHuman,
		AddedBy:          "019421a0-0000-7000-8000-00000000a002",
		ViaActorID:       "019421a0-0000-7000-8000-00000000a003",
	}
}

func fixed(a memberhooktest.Answer) func(*memberhook.Request) memberhooktest.Answer {
	return func(*memberhook.Request) memberhooktest.Answer { return a }
}

func TestCheck_Allow(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	d, err := srv.Client(t).Check(t.Context(), sampleRequest())
	if err != nil || !d.Allow {
		t.Fatalf("Check = %+v, %v; want allow", d, err)
	}
	calls := srv.Calls()
	if len(calls) != 1 || calls[0] != *sampleRequest() {
		t.Fatalf("hook received %+v, want the request", calls)
	}
}

func TestCheck_RefuseCarriesReasonAndDetail(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Refuse("tool_access", "Bob cannot use the Tool web-search.")))
	d, err := srv.Client(t).Check(t.Context(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if d.Allow || d.Reason != "tool_access" || d.Detail != "Bob cannot use the Tool web-search." {
		t.Fatalf("Check = %+v", d)
	}
}

// A refusal with no reason still gets a code; an overlong reason or detail
// is cut, on a rune boundary.
func TestCheck_RefusalReasonDefaultsAndBounds(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Answer{Status: 200, Body: `{"allow":false}`}))
	d, err := srv.Client(t).Check(t.Context(), sampleRequest())
	if err != nil || d.Allow || d.Reason != memberhook.DefaultReason || d.Detail != "" {
		t.Fatalf("Check = %+v, %v", d, err)
	}

	long := strings.Repeat("é", memberhook.MaxDetailBytes)
	srv.SetPolicy(fixed(memberhooktest.Refuse(strings.Repeat("r", 100), long)))
	if d, err = srv.Client(t).Check(t.Context(), sampleRequest()); err != nil {
		t.Fatal(err)
	}
	if len(d.Reason) != memberhook.MaxReasonBytes || len(d.Detail) != memberhook.MaxDetailBytes ||
		!strings.HasPrefix(long, d.Detail) {
		t.Fatalf("reason %d bytes, detail %d bytes", len(d.Reason), len(d.Detail))
	}
}

// Every answer that is not 200 with {"allow": <bool>} is an error, which the
// caller turns into a refusal.
func TestCheck_FailsClosed(t *testing.T) {
	cases := map[string]memberhooktest.Answer{
		"500":                {Status: 500, Body: `{"allow":true}`},
		"403":                {Status: 403, Body: `{"allow":true}`},
		"204":                {Status: 204, Body: ``},
		"invalid JSON":       {Status: 200, Body: `{"allow":tru`},
		"not an object":      {Status: 200, Body: `true`},
		"no allow":           {Status: 200, Body: `{"reason":"x"}`},
		"allow is a string":  {Status: 200, Body: `{"allow":"true"}`},
		"allow is null":      {Status: 200, Body: `{"allow":null}`},
		"empty body":         {Status: 200, Body: ``},
		// Valid JSON in its first 64 KiB: only the size bound refuses it.
		"answer is too long": {Status: 200, Body: `{"allow":true}` + strings.Repeat(" ", 70<<10)},
	}
	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			srv := memberhooktest.New(t, fixed(answer))
			d, err := srv.Client(t).Check(t.Context(), sampleRequest())
			if err == nil || d.Allow {
				t.Fatalf("Check = %+v, %v; want an error", d, err)
			}
		})
	}
}

func TestCheck_TimeoutFailsClosed(t *testing.T) {
	release := make(chan struct{})
	srv := memberhooktest.New(t, func(*memberhook.Request) memberhooktest.Answer {
		<-release
		return memberhooktest.Allow
	})
	t.Cleanup(func() { close(release) })
	h, err := memberhook.New(&memberhook.Config{
		URL: srv.Hook.URL, Timeout: 200 * time.Millisecond,
		TokenURL: srv.Token.URL, ClientID: memberhooktest.ClientID, ClientSecret: memberhooktest.ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	d, err := h.Check(t.Context(), sampleRequest())
	if err == nil || d.Allow {
		t.Fatalf("Check = %+v, %v; want a timeout error", d, err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("Check took %v; the timeout is 200ms", elapsed)
	}
}

func TestCheck_NetworkErrorFailsClosed(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	h, err := memberhook.New(&memberhook.Config{
		URL: "http://127.0.0.1:1/check", Timeout: time.Second,
		TokenURL: srv.Token.URL, ClientID: memberhooktest.ClientID, ClientSecret: memberhooktest.ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := h.Check(t.Context(), sampleRequest()); err == nil || d.Allow {
		t.Fatalf("Check = %+v, %v; want an error", d, err)
	}
}

func TestCheck_TokenErrorFailsClosed(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	h, err := memberhook.New(&memberhook.Config{
		URL: srv.Hook.URL, Timeout: time.Second,
		TokenURL: srv.Token.URL, ClientID: memberhooktest.ClientID, ClientSecret: "wrong",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := h.Check(t.Context(), sampleRequest()); err == nil || d.Allow {
		t.Fatalf("Check = %+v, %v; want an error", d, err)
	}
	if len(srv.Calls()) != 0 {
		t.Fatal("the hook was called without a token")
	}
}

// The token is fetched once and reused.
func TestCheck_CachesTheToken(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	h := srv.Client(t)
	for range 3 {
		if _, err := h.Check(t.Context(), sampleRequest()); err != nil {
			t.Fatal(err)
		}
	}
	if srv.Tokens() != 1 || len(srv.Calls()) != 3 {
		t.Fatalf("tokens %d, calls %d; want 1 and 3", srv.Tokens(), len(srv.Calls()))
	}
}

// A 401 drops the cached token: Chit fetches a new one and tries once more.
func TestCheck_401RefreshesTheTokenOnce(t *testing.T) {
	var hookCalls, tokenCalls atomic.Int32
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-" + string(rune('0'+n)), "token_type": "bearer", "expires_in": 3600,
		})
	}))
	defer token.Close()
	var revoked atomic.Bool
	revoked.Store(true) // token-1 is refused, as if halod rotated its key
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hookCalls.Add(1)
		if r.Header.Get("Authorization") == "Bearer token-1" && revoked.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"allow":true}`)
	}))
	defer hook.Close()

	h, err := memberhook.New(&memberhook.Config{
		URL: hook.URL, Timeout: 5 * time.Second, TokenURL: token.URL, ClientID: "c", ClientSecret: "s",
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := h.Check(t.Context(), sampleRequest())
	if err != nil || !d.Allow {
		t.Fatalf("Check = %+v, %v; want allow after one refresh", d, err)
	}
	if tokenCalls.Load() != 2 || hookCalls.Load() != 2 {
		t.Fatalf("token requests %d, hook calls %d; want 2 and 2", tokenCalls.Load(), hookCalls.Load())
	}
	// The new token is kept.
	if _, err := h.Check(t.Context(), sampleRequest()); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 2 {
		t.Fatalf("token requests %d after a third call; want the refreshed token reused", tokenCalls.Load())
	}
}

// A hook that refuses every token is a refusal after exactly one retry.
func TestCheck_401TwiceFailsClosed(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	var hookCalls atomic.Int32
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hookCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer hook.Close()
	h, err := memberhook.New(&memberhook.Config{
		URL: hook.URL, Timeout: time.Second,
		TokenURL: srv.Token.URL, ClientID: memberhooktest.ClientID, ClientSecret: memberhooktest.ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := h.Check(t.Context(), sampleRequest()); err == nil || d.Allow {
		t.Fatalf("Check = %+v, %v; want an error", d, err)
	}
	if hookCalls.Load() != 2 || srv.Tokens() != 2 {
		t.Fatalf("hook calls %d, tokens %d; want 2 and 2", hookCalls.Load(), srv.Tokens())
	}
}

// The request is a POST of JSON with the bearer token, and the body has
// exactly the contract's fields.
func TestCheck_RequestShape(t *testing.T) {
	var got struct {
		method, auth, contentType string
		body                      map[string]any
	}
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.auth, got.contentType = r.Method, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		_, _ = io.WriteString(w, `{"allow":true}`)
	}))
	defer hook.Close()
	h, err := memberhook.New(&memberhook.Config{
		URL: hook.URL + "/internal/chit/membership-check", Timeout: time.Second,
		TokenURL: srv.Token.URL, ClientID: memberhooktest.ClientID, ClientSecret: memberhooktest.ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Check(t.Context(), sampleRequest()); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.auth != "Bearer token-1" || got.contentType != "application/json" {
		t.Fatalf("method %q, auth %q, content type %q", got.method, got.auth, got.contentType)
	}
	want := map[string]any{
		"channel_id": "019421a0-0000-7000-8000-00000000c001", "user_id": "019421a0-0000-7000-8000-00000000a001",
		"kratos_identity_id": "9b4c2a71-7a49-4d55-9f0a-2a8f3c1d0e11", "actor_type": "human",
		"added_by": "019421a0-0000-7000-8000-00000000a002", "via_actor_id": "019421a0-0000-7000-8000-00000000a003",
	}
	if len(got.body) != len(want) {
		t.Fatalf("body %v, want %v", got.body, want)
	}
	for k, v := range want {
		if got.body[k] != v {
			t.Fatalf("body %v, want %v", got.body, want)
		}
	}

	// Empty Kratos and via IDs are sent as empty strings, not left out.
	req := sampleRequest()
	req.KratosIdentityID, req.ViaActorID = "", ""
	if _, err := h.Check(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	if v, ok := got.body["kratos_identity_id"]; !ok || v != "" {
		t.Fatalf("kratos_identity_id = %v, %v", v, ok)
	}
	if v, ok := got.body["via_actor_id"]; !ok || v != "" {
		t.Fatalf("via_actor_id = %v, %v", v, ok)
	}
}

// A redirect is not followed: it would carry the token elsewhere.
func TestCheck_DoesNotFollowRedirects(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	var followed atomic.Bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
		_, _ = io.WriteString(w, `{"allow":true}`)
	}))
	defer elsewhere.Close()
	hook := httptest.NewServer(http.RedirectHandler(elsewhere.URL, http.StatusTemporaryRedirect))
	defer hook.Close()
	h, err := memberhook.New(&memberhook.Config{
		URL: hook.URL, Timeout: time.Second,
		TokenURL: srv.Token.URL, ClientID: memberhooktest.ClientID, ClientSecret: memberhooktest.ClientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := h.Check(t.Context(), sampleRequest()); err == nil || d.Allow {
		t.Fatalf("Check = %+v, %v; want an error", d, err)
	}
	if followed.Load() {
		t.Fatal("the redirect was followed")
	}
}

// A cancelled caller is not an allow.
func TestCheck_CancelledContext(t *testing.T) {
	srv := memberhooktest.New(t, fixed(memberhooktest.Allow))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if d, err := srv.Client(t).Check(ctx, sampleRequest()); err == nil || d.Allow {
		t.Fatalf("Check = %+v, %v; want an error", d, err)
	}
}

func TestNew_RefusesIncompleteConfig(t *testing.T) {
	ok := memberhook.Config{URL: "http://h/check", Timeout: time.Second, TokenURL: "http://t", ClientID: "c", ClientSecret: "s"}
	if _, err := memberhook.New(&ok); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*memberhook.Config){
		"relative URL":    func(c *memberhook.Config) { c.URL = "/check" },
		"other scheme":    func(c *memberhook.Config) { c.URL = "ftp://h/check" },
		"no host":         func(c *memberhook.Config) { c.URL = "http:///check" },
		"zero timeout":    func(c *memberhook.Config) { c.Timeout = 0 },
		"no token URL":    func(c *memberhook.Config) { c.TokenURL = "" },
		"no client ID":    func(c *memberhook.Config) { c.ClientID = "" },
		"no secret":       func(c *memberhook.Config) { c.ClientSecret = "" },
		"unparseable URL": func(c *memberhook.Config) { c.URL = "http://h/%zz" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := ok
			mutate(&cfg)
			if _, err := memberhook.New(&cfg); err == nil {
				t.Fatal("New accepted it")
			}
		})
	}
}
