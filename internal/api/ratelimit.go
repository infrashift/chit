package api

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/infrashift/chit/internal/model"
)

// Rate limits (P-CHIT-8).
//
// A person, and any caller that is not on CHIT_ON_BEHALF_OF_CLIENTS, has one
// token bucket, keyed by their user ID: the per-user limit.
//
// A client on CHIT_ON_BEHALF_OF_CLIENTS is one caller for many people. HALO's
// halod posts each Subject's message on their behalf, and reads history and
// members, sends stream_status and posts replies in each Conversation as
// itself. With one per-user bucket for it, every Subject would share ten
// requests a second. So such a client has a bucket of its own instead, the
// aggregate limit, which counts everything it does:
//
//   - On behalf of a person, a request counts against that person's per-user
//     bucket (the one their own browser requests use) and against the
//     client's aggregate bucket.
//   - As itself, a request counts against the client's aggregate bucket.
//
// The aggregate bucket bounds the whole client, so a compromised or buggy one
// still cannot flood chitd, and no person can be made to spend more than
// their own budget through it.
//
// A refusal is a 429 with Retry-After, and a warning in the log that names the
// bucket, the person and, for a client, the actor and its OAuth2 client ID.

// RateLimits are the two limits, from CHIT_RATE_LIMIT_*.
type RateLimits struct {
	UserPerSecond   float64
	UserBurst       int
	ClientPerSecond float64
	ClientBurst     int
}

// Names of the buckets, in the refusal log line.
const (
	bucketUser   = "user"
	bucketClient = "client"
)

// RateLimit applies limits to each request. isClient reports whether a user
// is an allowlisted on-behalf-of client (app.App.OnBehalfOfAllowed).
func RateLimit(limits RateLimits, isClient func(*model.User) bool) func(http.Handler) http.Handler {
	return rateLimit(limits, isClient, time.Now)
}

func rateLimit(limits RateLimits, isClient func(*model.User) bool, now func() time.Time) func(http.Handler) http.Handler {
	users := newUserLimiters(rate.Limit(limits.UserPerSecond), limits.UserBurst, now())
	clients := newUserLimiters(rate.Limit(limits.ClientPerSecond), limits.ClientBurst, now())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buckets := bucketsFor(r, users, clients, isClient)
			refused, wait := take(now(), buckets)
			if refused == nil {
				next.ServeHTTP(w, r)
				return
			}
			logRefusal(r, refused.name, wait)
			setRetryAfter(w, wait)
			WriteError(w, model.NewAppError("RateLimit", "rate limit exceeded", "", http.StatusTooManyRequests))
		})
	}
}

// bucket is one token bucket that a request counts against.
type bucket struct {
	name     string
	limiters *userLimiters
	key      string
}

// bucketsFor returns the buckets that r counts against.
func bucketsFor(r *http.Request, users, clients *userLimiters, isClient func(*model.User) bool) []bucket {
	user := ContextGetUser(r)
	switch {
	case user == nil:
		// Not reached behind AuthExtract, which refuses a request it cannot
		// put a user on; kept so the limiter never lets one through unlimited.
		return []bucket{{bucketUser, users, r.RemoteAddr}}
	case ContextGetActor(r) != nil:
		// On behalf of user: AuthExtract set the actor only for an
		// allowlisted client.
		return []bucket{
			{bucketUser, users, user.ID},
			{bucketClient, clients, ContextGetActor(r).ID},
		}
	case isClient(user):
		return []bucket{{bucketClient, clients, user.ID}}
	default:
		return []bucket{{bucketUser, users, user.ID}}
	}
}

// take takes one token from every bucket, or from none: a request that one
// bucket refuses spends nothing from the others, so a person refused for
// their own budget does not use up the client's, nor the other way round. It
// returns the bucket that refused, and how long until it would allow the
// request.
func take(now time.Time, buckets []bucket) (*bucket, time.Duration) {
	held := make([]*rate.Reservation, 0, len(buckets))
	for i := range buckets {
		res := buckets[i].limiters.reserve(buckets[i].key, now)
		wait := res.DelayFrom(now)
		if res.OK() && wait == 0 {
			held = append(held, res)
			continue
		}
		res.CancelAt(now)
		for _, h := range held {
			h.CancelAt(now)
		}
		if !res.OK() {
			wait = 0 // never: no bucket has a burst below 1
		}
		return &buckets[i], wait
	}
	return nil, 0
}

// setRetryAfter sets Retry-After to wait in whole seconds, rounded up.
func setRetryAfter(w http.ResponseWriter, wait time.Duration) {
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(int64(math.Ceil(wait.Seconds())), 10))
	}
}

// logRefusal records a refusal: which bucket, and who. For a request on
// behalf of a person, both the actor and the principal.
func logRefusal(r *http.Request, name string, wait time.Duration) {
	attrs := []any{"bucket", name, "method", r.Method, "path", r.URL.Path,
		"retry_after_ms", wait.Milliseconds()}
	user := ContextGetUser(r)
	switch actor := ContextGetActor(r); {
	case user == nil:
		attrs = append(attrs, "remote_addr", r.RemoteAddr)
	case actor != nil:
		attrs = append(attrs, "actor_id", actor.ID, "client_id", actor.OAuthClientID, "principal_id", user.ID)
	case user.OAuthClientID != "":
		attrs = append(attrs, "actor_id", user.ID, "client_id", user.OAuthClientID)
	default:
		attrs = append(attrs, "user_id", user.ID)
	}
	slog.Warn("rate limit exceeded", attrs...)
}

// userLimiters holds one token bucket per caller.
//
// Idle buckets are evicted as requests arrive, not by a background goroutine.
// The old janitor was started once per RateLimit call and never stopped, and
// it wiped every bucket every ten minutes, which refilled the burst of
// whoever was busy at that moment. A bucket idle for idleAfter has refilled
// completely, so evicting it changes nothing.
type userLimiters struct {
	mu        sync.Mutex
	limit     rate.Limit
	burst     int
	idleAfter time.Duration
	lastSweep time.Time
	entries   map[string]*limiterEntry
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newUserLimiters(limit rate.Limit, burst int, now time.Time) *userLimiters {
	// Time for an empty bucket to refill, with margin; at least a minute.
	idle := time.Minute
	if limit > 0 {
		idle = max(idle, 2*time.Duration(float64(burst)/float64(limit)*float64(time.Second)))
	}
	return &userLimiters{
		limit: limit, burst: burst, idleAfter: idle,
		lastSweep: now, entries: map[string]*limiterEntry{},
	}
}

// reserve reserves a token from key's bucket; the caller cancels the
// reservation if it does not go ahead.
func (u *userLimiters) reserve(key string, now time.Time) *rate.Reservation {
	return u.limiter(key, now).ReserveN(now, 1)
}

// limiter returns key's bucket, made on first use, and evicts idle buckets.
func (u *userLimiters) limiter(key string, now time.Time) *rate.Limiter {
	u.mu.Lock()
	defer u.mu.Unlock()
	if now.Sub(u.lastSweep) > u.idleAfter {
		for k, e := range u.entries {
			if now.Sub(e.lastSeen) > u.idleAfter {
				delete(u.entries, k)
			}
		}
		u.lastSweep = now
	}
	e, ok := u.entries[key]
	if !ok {
		e = &limiterEntry{limiter: rate.NewLimiter(u.limit, u.burst)}
		u.entries[key] = e
	}
	e.lastSeen = now
	return e.limiter
}
