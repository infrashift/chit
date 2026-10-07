package api

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

// AuthExtract reads the trusted proxy header (set by Oathkeeper) and provisions
// or retrieves the local user, storing it in the request context.
// When CHIT_TRUSTED_PROXY_SECRET is set, the proxy must also present it in
// X-Proxy-Secret; this prevents header spoofing if the backend is reachable
// without going through the proxy.
func AuthExtract(a *app.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret := a.Config.TrustedProxySecret; secret != "" {
				presented := r.Header.Get("X-Proxy-Secret")
				if subtle.ConstantTimeCompare([]byte(presented), []byte(secret)) != 1 {
					WriteError(w, model.NewUnauthorizedError("AuthExtract", "invalid proxy credentials"))
					return
				}
			}

			// A machine actor authenticated by client-credentials wins over
			// the Kratos header. Oathkeeper sets exactly one of these per
			// request and blanks the other, so honouring the client header
			// first means a forged X-User-Id cannot ride along with a
			// legitimately introspected token.
			if clientID := r.Header.Get(a.Config.TrustedClientHeader); clientID != "" {
				user, err := a.ResolveOAuthClient(r.Context(), clientID)
				if err != nil {
					slog.Warn("auth: unknown oauth client", "client_id", clientID, "error", err)
					WriteError(w, model.NewUnauthorizedError("AuthExtract", "unknown oauth client"))
					return
				}
				r = ContextSetUser(r, user)
				next.ServeHTTP(w, r)
				return
			}

			kratosID := r.Header.Get(a.Config.TrustedProxyHeader)
			if kratosID == "" {
				WriteError(w, model.NewUnauthorizedError("AuthExtract", "missing authentication header"))
				return
			}

			user, err := a.ProvisionUser(r.Context(), kratosID)
			if err != nil {
				slog.Error("auth: failed to provision user", "kratos_id", kratosID, "error", err)
				WriteAppError(w, "AuthExtract", err)
				return
			}

			r = ContextSetUser(r, user)
			next.ServeHTTP(w, r)
		})
	}
}

// StructuredLogger logs each request using slog.
func StructuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"forwarded_for", r.Header.Get("X-Forwarded-For"),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker so that WebSocket upgrades work through
// the logging middleware.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// RateLimit applies per-user rate limiting: requestsPerSecond sustained,
// with bursts up to burst.
func RateLimit(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	limiters := newUserLimiters(rate.Limit(requestsPerSecond), burst, time.Now)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if user := ContextGetUser(r); user != nil {
				key = user.ID
			}
			if !limiters.allow(key) {
				WriteError(w, model.NewAppError("RateLimit", "rate limit exceeded", "", http.StatusTooManyRequests))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
	now       func() time.Time
	lastSweep time.Time
	entries   map[string]*limiterEntry
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newUserLimiters(limit rate.Limit, burst int, now func() time.Time) *userLimiters {
	// Time for an empty bucket to refill, with margin; at least a minute.
	idle := time.Minute
	if limit > 0 {
		idle = max(idle, 2*time.Duration(float64(burst)/float64(limit)*float64(time.Second)))
	}
	return &userLimiters{
		limit: limit, burst: burst, idleAfter: idle, now: now,
		lastSweep: now(), entries: map[string]*limiterEntry{},
	}
}

func (u *userLimiters) allow(key string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now()
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
	return e.limiter.AllowN(now, 1)
}
