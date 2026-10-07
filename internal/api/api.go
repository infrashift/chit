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
)

// New creates and configures the chi router with all routes and middleware.
func New(a *app.App) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	// No RealIP: chitd only ever sees Oathkeeper, and RealIP would let any
	// caller rewrite RemoteAddr with a forged X-Forwarded-For. The logger
	// records the header separately, as the unverified value it is.
	r.Use(chimiddleware.RequestID)
	r.Use(StructuredLogger)
	r.Use(chimiddleware.Recoverer)

	// The same allowlist governs CORS and WebSocket origins. An empty list
	// allows no browser origin: no CORS handler is installed, as the
	// WebSocket check refuses every Origin. It used to mean "*" for CORS
	// only, since go-chi/cors treats an empty list as allow-all.
	allowedOrigins := a.Config.AllowedOrigins
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

	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no auth required)
		r.Group(func(r chi.Router) {
			r.Get("/system/ping", systemPing)
			r.Get("/system/config/client", systemClientConfig(a))

			// OpenAPI spec + Swagger UI
			MountDocs(r, chitapi.OpenAPISpec)
		})

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(AuthExtract(a))
			r.Use(RateLimit(10, 50))

			// OpenAPI request validation (optional)
			if a.Config.EnableOpenAPIValidation {
				validationMW, err := NewValidationMiddleware(chitapi.OpenAPISpec)
				if err != nil {
					slog.Error("failed to init openapi validation middleware, skipping", "error", err)
				} else {
					r.Use(validationMW)
				}
			}

			// Users
			r.Post("/users", createUser(a))
			r.Get("/users/me", getMe(a))
			r.Put("/users/me", updateMe(a))
			r.Get("/users/{id}", getUser(a))
			r.Get("/users/username/{username}", getUserByUsername(a))
			r.Get("/users", searchUsers(a))
			r.Post("/users/ids", getUsersByIDs(a))

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

			// Channels
			r.Post("/channels", createChannel(a))
			r.Get("/channels/{id}", getChannel(a))
			r.Put("/channels/{id}", updateChannel(a))
			r.Delete("/channels/{id}", deleteChannel(a))
			r.Get("/teams/{id}/channels", getChannelsForTeam(a))
			r.Get("/users/me/teams/{id}/channels", getMyChannels(a))
			r.Get("/users/me/channels/direct", getMyDirectChannels(a))
			r.Post("/channels/direct", createDirectChannel(a))
			r.Post("/channels/group", createGroupChannel(a))
			r.Post("/channels/{id}/members", addChannelMember(a))
			r.Delete("/channels/{id}/members/{user_id}", removeChannelMember(a))
			r.Get("/channels/{id}/members", getChannelMembers(a))
			r.Post("/channels/{id}/members/me/view", viewChannel(a))

			// Posts
			r.Post("/posts", createPost(a))
			r.Get("/posts/{id}", getPost(a))
			r.Put("/posts/{id}", updatePost(a))
			r.Delete("/posts/{id}", deletePost(a))
			r.Post("/posts/{id}/pin", pinPost(a))
			r.Post("/posts/{id}/unpin", unpinPost(a))
			r.Get("/channels/{id}/posts", getChannelPosts(a))
			r.Get("/channels/{id}/pinned", getPinnedPosts(a))

			// Threads
			r.Get("/posts/{id}/thread", getThread(a))
			r.Get("/users/me/teams/{id}/threads", getMyThreads(a))
			r.Put("/users/me/teams/{team_id}/threads/{id}/read", markThreadAsRead(a))
			r.Put("/users/me/teams/{team_id}/threads/{id}/following", updateThreadFollowing(a))

			// Tags
			r.Post("/tags", createTag(a))
			r.Get("/tags", getAllTags(a))
			r.Post("/posts/{id}/tags", addTagToPost(a))
			r.Delete("/posts/{id}/tags/{tag_id}", removeTagFromPost(a))
			r.Get("/posts/{id}/tags", getTagsForPost(a))
			// Registered under /tags, not /posts: "/posts/tags" collides with
			// the "/posts/{id}" routes, so chi binds id="tags" and answers
			// 405 for a method those routes do not define.
			r.Post("/tags/posts", getTagsForPosts(a))

			// Commands
			r.Get("/commands", listCommands(a))

			// Search
			r.Post("/posts/search", searchPosts(a, searchEverywhere))
			r.Post("/teams/{id}/posts/search", searchPosts(a, searchTeam))
			r.Post("/channels/{id}/posts/search", searchPosts(a, searchChannel))

			// WebSocket
			r.Get("/websocket", handleWebSocket(a))
		})
	})

	return r
}
