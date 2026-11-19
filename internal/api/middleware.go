package api

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
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
//
// semscope: allow(NASA-P10-09) chi's Router.Use requires func(http.Handler) http.Handler.
func AuthExtract(a *app.App) func(http.Handler) http.Handler {
	// The routes are compiled once, here, and shared by every request.
	routes := onBehalfOfRoutes()
	routeList := onBehalfOfRouteList(routes)
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
				if principalID, ok := onBehalfOf(r); ok {
					if !onBehalfOfRoute(routes, r) {
						WriteError(w, model.NewBadRequestError("AuthExtract",
							OnBehalfOfHeader+" is accepted only on "+routeList))
						return
					}
					principal, err := a.ResolvePrincipal(r.Context(), user, principalID)
					if err != nil {
						slog.Warn("auth: on-behalf-of refused", "client_id", clientID, "principal_id", principalID, "error", err)
						WriteAppError(w, "AuthExtract", err)
						return
					}
					// Authorized as the person from here on; the client is
					// kept beside them for the record.
					r = ContextSetActor(ContextSetUser(r, principal), user)
					next.ServeHTTP(w, r)
					return
				}
				r = ContextSetUser(r, user)
				next.ServeHTTP(w, r)
				return
			}

			// A person's session never acts for somebody else. Refused rather
			// than ignored, so a caller that meant it learns it did not happen.
			if _, ok := onBehalfOf(r); ok {
				WriteError(w, model.NewForbiddenError("AuthExtract",
					OnBehalfOfHeader+" is only for allowlisted OAuth2 clients"))
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

// OnBehalfOfHeader names the person an allowlisted OAuth2 client acts for.
const OnBehalfOfHeader = "X-On-Behalf-Of"

// onBehalfOf returns the X-On-Behalf-Of value and whether the header is
// present at all: present and empty is a request to act for nobody, and is
// refused, not read as "no header".
func onBehalfOf(r *http.Request) (string, bool) {
	values, ok := r.Header[http.CanonicalHeaderKey(OnBehalfOfHeader)]
	if !ok {
		return "", false
	}
	if len(values) != 1 {
		return "", true
	}
	return values[0], true
}

// onBehalfOfRouteSpec is one route on which a client may act for a person.
type onBehalfOfRouteSpec struct {
	method, name string
	path         *regexp.Regexp
}

// onBehalfOfRoutes returns the only requests that may act on behalf of a
// person. Everything else a client does, it does as itself. Keep in step with
// the routes in api.go and with OnBehalfOfHeader in api/openapi.yaml.
// It compiles each pattern; call it once, when the middleware is built.
func onBehalfOfRoutes() []onBehalfOfRouteSpec {
	return []onBehalfOfRouteSpec{
		// Posting, and saying that a reply is being produced.
		{http.MethodPost, "POST /posts", regexp.MustCompile(`^/api/v1/posts$`)},
		{http.MethodPost, "POST /channels/{id}/stream_status", regexp.MustCompile(`^/api/v1/channels/[^/]+/stream_status$`)},
		// Tags: halod creates and applies a person's profile/ Tags for them.
		{http.MethodPost, "POST /tags", regexp.MustCompile(`^/api/v1/tags$`)},
		{http.MethodPost, "POST /posts/{id}/tags", regexp.MustCompile(`^/api/v1/posts/[^/]+/tags$`)},
		{http.MethodPost, "POST /channels/{id}/tags", regexp.MustCompile(`^/api/v1/channels/[^/]+/tags$`)},
		// Conversations: halod creates a person's private channel and adds
		// itself to it, as the person. GET /users/me returns the person, which
		// is how halod learns the user ID of a kratos: principal.
		{http.MethodPost, "POST /channels", regexp.MustCompile(`^/api/v1/channels$`)},
		{http.MethodPost, "POST /channels/{id}/members", regexp.MustCompile(`^/api/v1/channels/[^/]+/members$`)},
		{http.MethodGet, "GET /users/me", regexp.MustCompile(`^/api/v1/users/me$`)},
	}
}

// onBehalfOfRouteList names the routes, for the refusal message.
func onBehalfOfRouteList(routes []onBehalfOfRouteSpec) string {
	names := make([]string, len(routes))
	for i, route := range routes {
		names[i] = route.name
	}
	return strings.Join(names, ", ")
}

// onBehalfOfRoute reports whether r is one of routes.
func onBehalfOfRoute(routes []onBehalfOfRouteSpec, r *http.Request) bool {
	for _, route := range routes {
		if r.Method == route.method && route.path.MatchString(r.URL.Path) {
			return true
		}
	}
	return false
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
//
// semscope: allow(NASA-P10-09) chi's Router.Use requires func(http.Handler) http.Handler.
func RateLimit(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	limiters := newUserLimiters(rate.Limit(requestsPerSecond), burst, systemClock{})

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
	clock     clock
	lastSweep time.Time
	entries   map[string]*limiterEntry
}

// clock supplies the current time; tests substitute a fake.
type clock interface {
	Now() time.Time
}

// systemClock is the production clock.
type systemClock struct{}

// Now returns time.Now().
func (systemClock) Now() time.Time { return time.Now() }

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newUserLimiters(limit rate.Limit, burst int, c clock) *userLimiters {
	// Time for an empty bucket to refill, with margin; at least a minute.
	idle := time.Minute
	if limit > 0 {
		idle = max(idle, 2*time.Duration(float64(burst)/float64(limit)*float64(time.Second)))
	}
	return &userLimiters{
		limit: limit, burst: burst, idleAfter: idle, clock: c,
		lastSweep: c.Now(), entries: map[string]*limiterEntry{},
	}
}

func (u *userLimiters) allow(key string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.clock.Now()
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
