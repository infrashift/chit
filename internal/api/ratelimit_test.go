package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

// The callers of the rate-limit tests. halod is on the on-behalf-of
// allowlist; the bot is a machine actor that is not.
var (
	rlHalod  = &model.User{ID: "halod-user", OAuthClientID: "halod-client", ActorType: model.ActorTypeAgent}
	rlBot    = &model.User{ID: "bot-user", OAuthClientID: "bot-client", ActorType: model.ActorTypeBot}
	rlAlice  = &model.User{ID: "alice", ActorType: model.ActorTypeUser}
	rlBob    = &model.User{ID: "bob", ActorType: model.ActorTypeUser}
	rlCarol  = &model.User{ID: "carol", ActorType: model.ActorTypeUser}
	rlDave   = &model.User{ID: "dave", ActorType: model.ActorTypeUser}
	rlClient = func(u *model.User) bool { return u.OAuthClientID == rlHalod.OAuthClientID }
)

// limiterRig drives the rate-limit middleware with a clock of its own.
type limiterRig struct {
	now     time.Time
	handler http.Handler
}

func newLimiterRig(limits RateLimits) *limiterRig {
	rig := &limiterRig{now: time.Unix(1_000_000, 0)}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	rig.handler = rateLimit(limits, rlClient, func() time.Time { return rig.now })(ok)
	return rig
}

// as sends one request from user, on behalf of nobody.
func (rig *limiterRig) as(t *testing.T, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	return rig.send(t, user, nil)
}

// forPerson sends one request from actor on behalf of principal, as
// AuthExtract leaves it.
func (rig *limiterRig) forPerson(t *testing.T, actor, principal *model.User) *httptest.ResponseRecorder {
	t.Helper()
	return rig.send(t, principal, actor)
}

func (rig *limiterRig) send(t *testing.T, user, actor *model.User) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/posts", http.NoBody)
	r = ContextSetUser(r, user)
	if actor != nil {
		r = ContextSetActor(r, actor)
	}
	w := httptest.NewRecorder()
	rig.handler.ServeHTTP(w, r)
	return w
}

func wantCode(t *testing.T, what string, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("%s: got %d, want %d: %s", what, w.Code, code, w.Body.String())
	}
}

// Limits where only the bucket under test can run out.
func userLimits(burst int) RateLimits {
	return RateLimits{UserPerSecond: 1, UserBurst: burst, ClientPerSecond: 1, ClientBurst: 1000}
}

func clientLimits(burst int) RateLimits {
	return RateLimits{UserPerSecond: 1, UserBurst: 1000, ClientPerSecond: 1, ClientBurst: burst}
}

func TestRateLimit_APersonHasTheirOwnBucket(t *testing.T) {
	rig := newLimiterRig(userLimits(2))
	for range 2 {
		wantCode(t, "within the burst", rig.as(t, rlAlice), http.StatusOK)
	}
	w := rig.as(t, rlAlice)
	wantCode(t, "past the burst", w, http.StatusTooManyRequests)
	if got := w.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1 (one token a second)", got)
	}
	wantCode(t, "another person", rig.as(t, rlBob), http.StatusOK)

	rig.now = rig.now.Add(time.Second)
	wantCode(t, "a second later", rig.as(t, rlAlice), http.StatusOK)
}

// The fix for P-CHIT-8: on behalf of a person, the client spends that
// person's budget, not one shared by everyone it acts for.
func TestRateLimit_OnBehalfOfIsKeyedByThePrincipal(t *testing.T) {
	rig := newLimiterRig(userLimits(2))
	for range 2 {
		wantCode(t, "for alice", rig.forPerson(t, rlHalod, rlAlice), http.StatusOK)
	}
	wantCode(t, "for alice past her burst", rig.forPerson(t, rlHalod, rlAlice), http.StatusTooManyRequests)
	// Alice's own requests share her bucket: the client cannot be used to
	// give anyone more than their own budget.
	wantCode(t, "alice herself", rig.as(t, rlAlice), http.StatusTooManyRequests)
	for range 2 {
		wantCode(t, "for bob", rig.forPerson(t, rlHalod, rlBob), http.StatusOK)
	}
	// Nor did the client's own requests come out of a per-user bucket.
	for range 5 {
		wantCode(t, "halod as itself", rig.as(t, rlHalod), http.StatusOK)
	}
}

// The aggregate cap: everything a client does on behalf of anyone counts
// against its own bucket too.
func TestRateLimit_OnBehalfOfCountsAgainstTheClient(t *testing.T) {
	rig := newLimiterRig(clientLimits(3))
	for _, p := range []*model.User{rlAlice, rlBob, rlCarol} {
		wantCode(t, "for "+p.ID, rig.forPerson(t, rlHalod, p), http.StatusOK)
	}
	wantCode(t, "for dave, past the client's burst", rig.forPerson(t, rlHalod, rlDave), http.StatusTooManyRequests)
	wantCode(t, "halod as itself, past its burst", rig.as(t, rlHalod), http.StatusTooManyRequests)
	// The people still have their own budgets for themselves.
	wantCode(t, "dave himself", rig.as(t, rlDave), http.StatusOK)
}

func TestRateLimit_AnAllowlistedClientAsItselfUsesTheAggregateBucket(t *testing.T) {
	rig := newLimiterRig(RateLimits{UserPerSecond: 1, UserBurst: 2, ClientPerSecond: 1, ClientBurst: 10})
	for range 10 {
		wantCode(t, "halod as itself, within the aggregate burst", rig.as(t, rlHalod), http.StatusOK)
	}
	wantCode(t, "halod as itself, past it", rig.as(t, rlHalod), http.StatusTooManyRequests)
}

// A machine actor that may not act on behalf of anyone is one caller, and
// keeps the per-user limit.
func TestRateLimit_AMachineActorNotAllowlistedKeepsThePerUserLimit(t *testing.T) {
	rig := newLimiterRig(RateLimits{UserPerSecond: 1, UserBurst: 2, ClientPerSecond: 1, ClientBurst: 10})
	for range 2 {
		wantCode(t, "bot within the user burst", rig.as(t, rlBot), http.StatusOK)
	}
	wantCode(t, "bot past the user burst", rig.as(t, rlBot), http.StatusTooManyRequests)
}

// A request one bucket refuses spends nothing from the other.
func TestRateLimit_ARefusalSpendsNoOtherBucket(t *testing.T) {
	t.Run("refused for the person, the client keeps its token", func(t *testing.T) {
		rig := newLimiterRig(RateLimits{UserPerSecond: 1, UserBurst: 1, ClientPerSecond: 1, ClientBurst: 2})
		wantCode(t, "for alice", rig.forPerson(t, rlHalod, rlAlice), http.StatusOK)
		wantCode(t, "for alice again", rig.forPerson(t, rlHalod, rlAlice), http.StatusTooManyRequests)
		wantCode(t, "halod's last token", rig.as(t, rlHalod), http.StatusOK)
	})
	t.Run("refused for the client, the person keeps their token", func(t *testing.T) {
		rig := newLimiterRig(RateLimits{UserPerSecond: 1, UserBurst: 1, ClientPerSecond: 1, ClientBurst: 1})
		wantCode(t, "halod as itself", rig.as(t, rlHalod), http.StatusOK)
		wantCode(t, "for alice", rig.forPerson(t, rlHalod, rlAlice), http.StatusTooManyRequests)
		wantCode(t, "alice's own token", rig.as(t, rlAlice), http.StatusOK)
	})
}

// Retry-After is the wait for the bucket that refused, rounded up to whole
// seconds.
func TestRateLimit_RetryAfter(t *testing.T) {
	cases := []struct {
		name   string
		limits RateLimits
		who    func(*limiterRig, *testing.T) *httptest.ResponseRecorder
		want   string
	}{
		{"person, 0.3 a second", RateLimits{UserPerSecond: 0.3, UserBurst: 1, ClientPerSecond: 100, ClientBurst: 100},
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.as(t, rlAlice) }, "4"},
		{"client, 0.25 a second", RateLimits{UserPerSecond: 100, UserBurst: 100, ClientPerSecond: 0.25, ClientBurst: 1},
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.forPerson(t, rlHalod, rlAlice) }, "4"},
		{"client, 1500 a second", RateLimits{UserPerSecond: 100, UserBurst: 100, ClientPerSecond: 1500, ClientBurst: 1},
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.as(t, rlHalod) }, "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newLimiterRig(tc.limits)
			wantCode(t, "first", tc.who(rig, t), http.StatusOK)
			w := tc.who(rig, t)
			wantCode(t, "second", w, http.StatusTooManyRequests)
			if got := w.Header().Get("Retry-After"); got != tc.want {
				t.Fatalf("Retry-After = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRateLimit_AnUnauthenticatedRequestIsLimitedByAddress(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := RateLimit(userLimits(1), rlClient)(ok)
	codes := make([]int, 0, 3)
	for _, addr := range []string{"10.0.0.1:9999", "10.0.0.1:9999", "10.0.0.2:9999"} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		r.RemoteAddr = addr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		codes = append(codes, w.Code)
	}
	if want := []int{http.StatusOK, http.StatusTooManyRequests, http.StatusOK}; !slices.Equal(codes, want) {
		t.Fatalf("codes = %v, want %v", codes, want)
	}
}

// Each refusal is logged with the bucket and who: for a request on behalf of
// a person, the actor, its client ID and the principal.
func TestRateLimit_LogsTheRefusal(t *testing.T) {
	cases := []struct {
		name   string
		limits RateLimits
		send   func(*limiterRig, *testing.T) *httptest.ResponseRecorder
		want   map[string]any
		absent []string
	}{
		{"on behalf of, the person's bucket", userLimits(1),
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.forPerson(t, rlHalod, rlAlice) },
			map[string]any{"bucket": "user", "actor_id": "halod-user", "client_id": "halod-client", "principal_id": "alice"},
			[]string{"user_id"}},
		{"on behalf of, the client's bucket", clientLimits(1),
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.forPerson(t, rlHalod, rlAlice) },
			map[string]any{"bucket": "client", "actor_id": "halod-user", "client_id": "halod-client", "principal_id": "alice"},
			[]string{"user_id"}},
		{"the client as itself", clientLimits(1),
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.as(t, rlHalod) },
			map[string]any{"bucket": "client", "actor_id": "halod-user", "client_id": "halod-client"},
			[]string{"principal_id", "user_id"}},
		{"a person", userLimits(1),
			func(r *limiterRig, t *testing.T) *httptest.ResponseRecorder { return r.as(t, rlAlice) },
			map[string]any{"bucket": "user", "user_id": "alice"},
			[]string{"actor_id", "principal_id"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			rig := newLimiterRig(tc.limits)
			wantCode(t, "first", tc.send(rig, t), http.StatusOK)
			if buf.Len() != 0 {
				t.Fatalf("an allowed request was logged: %s", buf.String())
			}
			wantCode(t, "second", tc.send(rig, t), http.StatusTooManyRequests)

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("want one JSON log line, got %q", buf.String())
			}
			if line["level"] != "WARN" || line["msg"] != "rate limit exceeded" {
				t.Fatalf("level %v, msg %v", line["level"], line["msg"])
			}
			for k, v := range tc.want {
				if line[k] != v {
					t.Errorf("%s = %v, want %v", k, line[k], v)
				}
			}
			for _, k := range tc.absent {
				if _, ok := line[k]; ok {
					t.Errorf("%s = %v, want it absent", k, line[k])
				}
			}
			if line["path"] != "/api/v1/posts" || line["method"] != http.MethodPost {
				t.Errorf("method %v, path %v", line["method"], line["path"])
			}
		})
	}
}

// Through the router: the limits come from the configuration, and halod as
// itself is not held to the per-user limit.
func TestRateLimit_RouterUsesTheConfiguredLimits(t *testing.T) {
	newEnv := func(t *testing.T, userBurst, clientBurst int) *bridgeEnv {
		e := newBridgeEnv(t)
		// Slow refills, so the clock cannot hand out a token mid-test.
		e.a.Config.RateLimitUserPerSecond, e.a.Config.RateLimitUserBurst = 0.001, userBurst
		e.a.Config.RateLimitClientPerSecond, e.a.Config.RateLimitClientBurst = 0.001, clientBurst
		e.router = New(e.a)
		return e
	}
	me := "/api/v1/users/me"

	t.Run("per user", func(t *testing.T) {
		e := newEnv(t, 2, 100)
		for range 2 {
			wantCode(t, "for testuser", e.do(t, http.MethodGet, me, "client:"+haloClientID, "", obo(testUserID)), http.StatusOK)
		}
		wantCode(t, "for testuser past the burst",
			e.do(t, http.MethodGet, me, "client:"+haloClientID, "", obo(testUserID)), http.StatusTooManyRequests)
		wantCode(t, "testuser herself", e.do(t, http.MethodGet, me, "user:"+testKratosID, "", nil), http.StatusTooManyRequests)
		wantCode(t, "for alice", e.do(t, http.MethodGet, me, "client:"+haloClientID, "", obo(extraUserID)), http.StatusOK)
		for range 5 {
			wantCode(t, "halod as itself", e.do(t, http.MethodGet, me, "client:"+haloClientID, "", nil), http.StatusOK)
		}
		// Not on the allowlist: one caller, the per-user limit.
		for range 2 {
			wantCode(t, "other agent", e.do(t, http.MethodGet, me, "client:"+otherClientID, "", nil), http.StatusOK)
		}
		wantCode(t, "other agent past the burst",
			e.do(t, http.MethodGet, me, "client:"+otherClientID, "", nil), http.StatusTooManyRequests)
	})
	t.Run("per client", func(t *testing.T) {
		e := newEnv(t, 100, 3)
		wantCode(t, "for testuser", e.do(t, http.MethodGet, me, "client:"+haloClientID, "", obo(testUserID)), http.StatusOK)
		for range 2 {
			wantCode(t, "halod as itself", e.do(t, http.MethodGet, me, "client:"+haloClientID, "", nil), http.StatusOK)
		}
		w := e.do(t, http.MethodGet, me, "client:"+haloClientID, "", obo(extraUserID))
		wantCode(t, "for alice past the client's burst", w, http.StatusTooManyRequests)
		if w.Header().Get("Retry-After") == "" {
			t.Fatal("a 429 without Retry-After")
		}
		var body model.AppError
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !strings.Contains(body.Message, "rate limit") {
			t.Fatalf("body %q: %v", w.Body.String(), err)
		}
	})
}

// The defaults are the ones in config.Defaults, which the router reads.
func TestRateLimit_DefaultsLetHalodPastThePerUserBurst(t *testing.T) {
	e := newBridgeEnv(t)
	for i := range e.a.Config.RateLimitUserBurst + 10 {
		if w := e.do(t, http.MethodGet, "/api/v1/users/me", "client:"+haloClientID, "", nil); w.Code != http.StatusOK {
			t.Fatalf("request %d of halod as itself: got %d", i+1, w.Code)
		}
	}
}

// Idle buckets are evicted once they would have refilled anyway; a busy
// caller's bucket is never reset (the old janitor wiped every bucket every
// ten minutes, refilling the burst of whoever was active).
func TestUserLimiters_EvictsOnlyIdleBuckets(t *testing.T) {
	now := time.Unix(0, 0)
	u := newUserLimiters(1, 2, now)
	allow := func(key string) bool {
		res := u.reserve(key, now)
		if res.DelayFrom(now) > 0 {
			res.CancelAt(now)
			return false
		}
		return true
	}

	for i := range 2 {
		if !allow("busy") {
			t.Fatalf("request %d of a burst of 2 refused", i+1)
		}
	}
	if !allow("idle") {
		t.Fatal("first request refused")
	}
	if allow("busy") {
		t.Fatal("a third request inside a second was allowed with a burst of 2")
	}

	// Long enough for "idle" to be evicted; "busy" keeps calling.
	for range 3 {
		now = now.Add(u.idleAfter / 2)
		allow("busy")
	}
	u.mu.Lock()
	_, idleKept := u.entries["idle"]
	_, busyKept := u.entries["busy"]
	u.mu.Unlock()
	if idleKept || !busyKept {
		t.Fatalf("idle kept=%v busy kept=%v, want only the busy bucket", idleKept, busyKept)
	}
}
