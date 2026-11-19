package api

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	chitapi "github.com/infrashift/chit/api"
	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
)

// New creates and configures the chi router with all routes and middleware.
func New(a *app.App) http.Handler {
	assert.That(a != nil, "api.New: nil app")
	assert.That(a.Config != nil, "api.New: nil config")
	r := chi.NewRouter()

	// Global middleware
	// No RealIP: chitd only ever sees Oathkeeper, and RealIP would let any
	// caller rewrite RemoteAddr with a forged X-Forwarded-For. The logger
	// records the header separately, as the unverified value it is.
	r.Use(chimiddleware.RequestID)
	r.Use(StructuredLogger)
	r.Use(chimiddleware.Recoverer)
	useCORS(r, a.Config.AllowedOrigins)

	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no auth required)
		r.Group(func(r chi.Router) {
			r.Get("/system/ping", systemPing)
			r.Get("/system/config/client", systemClientConfig())

			// OpenAPI spec + Swagger UI
			MountDocs(r, chitapi.OpenAPISpec)
		})

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			useAuthenticated(r, a)
			routeUsers(r, a)
			routeTeams(r, a)
			routeChannels(r, a)
			routePosts(r, a)
			routeThreads(r, a)
			routeTags(r, a)
			routeAttachments(r, a)
			routeCommandsSearchAndWebSocket(r, a)
		})
	})

	return r
}

// useCORS installs the CORS handler for allowedOrigins.
//
// The same allowlist governs CORS and WebSocket origins. An empty list
// allows no browser origin: no CORS handler is installed, as the
// WebSocket check refuses every Origin. It used to mean "*" for CORS
// only, since go-chi/cors treats an empty list as allow-all.
func useCORS(r chi.Router, allowedOrigins []string) {
	assert.That(r != nil, "useCORS: nil router")
	if slices.Contains(allowedOrigins, "*") {
		slog.Warn("CORS and WebSocket origins are open to all sites; set CHIT_ALLOWED_ORIGINS in production")
	}
	if len(allowedOrigins) > 0 {
		// Note: the trusted proxy header (X-User-Id) is intentionally NOT an
		// allowed CORS header — only the auth proxy may set it, server-side.
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}
}

// useAuthenticated installs the middleware of the authenticated group:
// the attachment headers, authentication, rate limiting and, when enabled,
// OpenAPI validation.
func useAuthenticated(r chi.Router, a *app.App) {
	assert.That(r != nil, "useAuthenticated: nil router")
	assert.That(a != nil && a.Config != nil, "useAuthenticated: nil app or config")
	// First, so that a refusal by AuthExtract on a file route carries the
	// attachment headers too.
	r.Use(FileResponseHeaders)
	r.Use(AuthExtract(a))
	// After AuthExtract, which names the person and, on behalf of them,
	// the client: see ratelimit.go.
	r.Use(RateLimit(RateLimits{
		UserPerSecond:   a.Config.RateLimitUserPerSecond,
		UserBurst:       a.Config.RateLimitUserBurst,
		ClientPerSecond: a.Config.RateLimitClientPerSecond,
		ClientBurst:     a.Config.RateLimitClientBurst,
	}, a))

	// OpenAPI request validation (optional)
	if a.Config.EnableOpenAPIValidation {
		validationMW, err := NewValidationMiddleware(chitapi.OpenAPISpec)
		if err != nil {
			slog.Error("failed to init openapi validation middleware, skipping", "error", err)
		} else {
			r.Use(validationMW)
		}
	}
}

// routeUsers registers the user routes.
func routeUsers(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeUsers: nil router")
	assert.That(a != nil, "routeUsers: nil app")
	// Users
	r.Post("/users", createUser(a))
	r.Get("/users/me", getMe())
	r.Put("/users/me", updateMe(a))
	r.Get("/users/{id}", getUser(a))
	r.Get("/users/username/{username}", getUserByUsername(a))
	r.Get("/users", searchUsers(a))
	r.Post("/users/ids", getUsersByIDs(a))
}

// routeTeams registers the team routes.
func routeTeams(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeTeams: nil router")
	assert.That(a != nil, "routeTeams: nil app")
	// Teams
	r.Post("/teams", createTeam(a))
	r.Get("/teams/{id}", getTeam(a))
	r.Put("/teams/{id}", updateTeam(a))
	r.Delete("/teams/{id}", deleteTeam(a))
	r.Get("/teams", getAllTeams(a))
	r.Get("/users/me/teams", getMyTeams(a))
	r.Post("/teams/{id}/members", addTeamMember(a))
	r.Delete("/teams/{id}/members/{user_id}", removeTeamMember(a))
	r.Get("/teams/{id}/members", getTeamMembers(a))
}

// routeChannels registers the channel routes.
func routeChannels(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeChannels: nil router")
	assert.That(a != nil, "routeChannels: nil app")
	// Channels
	r.Post("/channels", createChannel(a))
	r.Get("/channels/{id}", getChannel(a))
	r.Put("/channels/{id}", updateChannel(a))
	r.Delete("/channels/{id}", deleteChannel(a))
	r.Get("/teams/{id}/channels", getChannelsForTeam(a))
	r.Get("/users/me/teams/{id}/channels", getMyChannels(a))
	r.Get("/users/me/teams/{id}/channels/members", getMyChannelMembers(a))
	r.Get("/users/me/channels/direct", getMyDirectChannels(a))
	r.Post("/channels/direct", createDirectChannel(a))
	r.Post("/channels/group", createGroupChannel(a))
	r.Post("/channels/{id}/members", addChannelMember(a))
	r.Delete("/channels/{id}/members/{user_id}", removeChannelMember(a))
	r.Get("/channels/{id}/members", getChannelMembers(a))
	r.Post("/channels/{id}/members/me/view", viewChannel(a))
	// Not persisted: relayed to the channel's WebSocket clients.
	r.Post("/channels/{id}/stream_status", sendStreamStatus(a))
}

// routePosts registers the post routes.
func routePosts(r chi.Router, a *app.App) {
	assert.That(r != nil, "routePosts: nil router")
	assert.That(a != nil, "routePosts: nil app")
	// Posts
	r.Post("/posts", createPost(a))
	r.Get("/posts/{id}", getPost(a))
	r.Put("/posts/{id}", updatePost(a))
	r.Delete("/posts/{id}", deletePost(a))
	r.Post("/posts/{id}/pin", pinPost(a))
	r.Post("/posts/{id}/unpin", unpinPost(a))
	r.Get("/channels/{id}/posts", getChannelPosts(a))
	r.Get("/channels/{id}/pinned", getPinnedPosts(a))
}

// routeThreads registers the thread routes.
func routeThreads(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeThreads: nil router")
	assert.That(a != nil, "routeThreads: nil app")
	// Threads
	r.Get("/posts/{id}/thread", getThread(a))
	r.Get("/users/me/teams/{id}/threads", getMyThreads(a))
	r.Put("/users/me/teams/{team_id}/threads/{id}/read", markThreadAsRead(a))
	r.Put("/users/me/teams/{team_id}/threads/{id}/following", updateThreadFollowing(a))
	// Threads in direct and group channels belong to no team. The
	// read and follow routes ignore team_id, so they also exist
	// without it, rather than make a client invent one for a DM.
	r.Get("/users/me/threads/direct", getMyDirectThreads(a))
	r.Put("/users/me/threads/{id}/read", markThreadAsRead(a))
	r.Put("/users/me/threads/{id}/following", updateThreadFollowing(a))
}

// routeTags registers the tag routes.
func routeTags(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeTags: nil router")
	assert.That(a != nil, "routeTags: nil app")
	// Tags
	r.Post("/tags", createTag(a))
	r.Get("/tags", getAllTags(a))
	r.Put("/tags/{id}", renameTag(a))
	r.Delete("/tags/{id}", deleteTag(a))
	r.Post("/channels/{id}/tags", addTagToChannel(a))
	r.Delete("/channels/{id}/tags/{tag_id}", removeTagFromChannel(a))
	r.Get("/channels/{id}/tags", getTagsForChannel(a))
	r.Post("/tags/channels", filterChannelsByTags(a))
	r.Delete("/users/{id}/tags", deleteUserTags(a))
	r.Post("/posts/{id}/tags", addTagToPost(a))
	r.Delete("/posts/{id}/tags/{tag_id}", removeTagFromPost(a))
	r.Get("/posts/{id}/tags", getTagsForPost(a))
	// Registered under /tags, not /posts: "/posts/tags" collides with
	// the "/posts/{id}" routes, so chi binds id="tags" and answers
	// 405 for a method those routes do not define.
	r.Post("/tags/posts", getTagsForPosts(a))
}

// routeAttachments registers the attachment routes.
func routeAttachments(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeAttachments: nil router")
	assert.That(a != nil, "routeAttachments: nil app")
	r.Post("/channels/{id}/files", uploadFile(a))
	r.Get("/files/{id}", downloadFile(a))
	r.Get("/files/{id}/info", getFileInfo(a))
	r.Post("/files/search", searchFiles(a))
}

// routeCommandsSearchAndWebSocket registers the command, search and
// WebSocket routes.
func routeCommandsSearchAndWebSocket(r chi.Router, a *app.App) {
	assert.That(r != nil, "routeCommandsSearchAndWebSocket: nil router")
	assert.That(a != nil, "routeCommandsSearchAndWebSocket: nil app")
	// Commands
	r.Get("/commands", listCommands(a))

	// Search
	r.Post("/posts/search", searchPosts(a, searchEverywhere))
	r.Post("/teams/{id}/posts/search", searchPosts(a, searchTeam))
	r.Post("/channels/{id}/posts/search", searchPosts(a, searchChannel))

	// WebSocket
	r.Get("/websocket", handleWebSocket(a))
}
