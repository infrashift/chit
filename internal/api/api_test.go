package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	chitapi "github.com/infrashift/chit/api"
)

// registeredRoutes returns every "METHOD /path" the router exposes.
func registeredRoutes(t *testing.T, h http.Handler) []string {
	t.Helper()

	r, ok := h.(chi.Routes)
	if !ok {
		t.Fatalf("router does not expose its routes (%T)", h)
	}

	var routes []string
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	return routes
}

// The batch tag endpoint first lived at POST /posts/tags, which collides with
// the /posts/{id} routes: chi binds id="tags" and answers 405 for a method
// those routes do not define. It must not sit under /posts.
//
// The earlier check only counted requests in the server log and never looked
// at their status, which is how a route returning 405 on every call was
// mistaken for a working one.
func TestRoutes_BatchTagsDoesNotSitUnderPosts(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	routes := registeredRoutes(t, New(a))

	var found string
	for _, r := range routes {
		if strings.HasSuffix(r, "/tags/posts") && strings.HasPrefix(r, "POST") {
			found = r
		}
		if strings.HasSuffix(r, "/posts/tags") {
			t.Errorf("batch tags is registered at %q, which collides with /posts/{id}", r)
		}
	}
	if found == "" {
		t.Errorf("no POST route ending in /tags/posts:\n%s", strings.Join(routes, "\n"))
	}
}

// chiToSpecPath converts a chi pattern to its OpenAPI equivalent: chi writes
// wildcards as {id}, and so does the spec, but chi patterns carry the /api/v1
// mount prefix that spec paths omit.
func chiToSpecPath(route string) string {
	return strings.TrimSuffix(strings.TrimPrefix(route, "/api/v1"), "/")
}

// specParamNames rewrites every {param} to {} so a route and a spec entry that
// name the same positional parameter differently still compare equal —
// /posts/{id}/tags vs /posts/{post_id}/tags is a naming choice, not a
// different endpoint.
var specParamNames = regexp.MustCompile(`\{[^}]*\}`)

func normalizePath(p string) string {
	return specParamNames.ReplaceAllString(p, "{}")
}

// Two independent misses let a batch endpoint ship that answered 405 on every
// call: it was mounted at a path that collides with /posts/{id}, and it was
// never added to the OpenAPI spec, so the request validator rejected it as an
// unknown operation. Neither was visible from the handler or its unit tests.
//
// This asserts the router and the spec describe the same API.
func TestRoutes_MatchTheOpenAPISpec(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(chitapi.OpenAPISpec, &spec); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	// method+path pairs the spec declares, normalized for comparison.
	declared := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			declared[strings.ToUpper(method)+" "+normalizePath(path)] = true
		}
	}

	// Routes that exist to serve the docs and the spec itself are not part of
	// the described API and are not expected to appear in it.
	skip := map[string]bool{
		"GET /openapi.yaml": true,
		"GET /docs/*":       true,
		"GET /docs":         true,
	}

	for _, r := range registeredRoutes(t, New(a)) {
		method, route, ok := strings.Cut(r, " ")
		if !ok {
			continue
		}
		p := chiToSpecPath(route)
		if skip[method+" "+p] || strings.HasPrefix(p, "/docs") {
			continue
		}
		// chi registers a catch-all for unmatched methods; ignore it.
		if method == "" || p == "" {
			continue
		}
		if !declared[method+" "+normalizePath(p)] {
			t.Errorf("route %s %s is served but absent from api/openapi.yaml, "+
				"so the request validator will reject it", method, p)
		}
	}
}

// An empty origin allowlist admits no browser origin. CORS used to fall back
// to "*" for it while the WebSocket check refused everything.
func TestRouter_EmptyOriginListSendsNoCORSHeaders(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	a.Config.AllowedOrigins = nil

	r, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/system/ping", http.NoBody)
	r.Header.Set("Origin", "https://elsewhere.example")
	w := httptest.NewRecorder()
	New(a).ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q with an empty allowlist", got)
	}
}
