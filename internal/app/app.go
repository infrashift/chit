package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/infrashift/chit/internal/jobs/workers"
	"github.com/infrashift/chit/internal/keto"

	"github.com/infrashift/chit/internal/cache"
	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

// App contains all dependencies for business logic.
type App struct {
	Store  store.Store
	Hub    *websocket.Hub
	PubSub pubsub.PubSub
	Config *config.Config

	// Slash commands (nil when commands are disabled).
	CommandRegistry *command.Registry
	CommandHandlers map[string]command.Handler
	AuditLogger     *command.AuditLogger
	WebhookCh       chan *command.WebhookEvent

	keto       *keto.Client
	httpClient *http.Client

	zincOnce   sync.Once
	zincClient *workers.SearchIndexer

	// userCache caches ProvisionUser lookups (kratos ID → user) so every
	// authenticated request does not hit the database.
	userCache *cache.LRU[string, *model.User]

	// background tracks work started off the request path, so shutdown can
	// let it finish.
	background sync.WaitGroup
}

// ketoAsyncTimeout bounds one background batch of Keto writes.
const ketoAsyncTimeout = 30 * time.Second

// writeKetoMembersAsync records channel membership tuples in Keto off the
// request path. channel_members is the access check and Keto a best-effort
// mirror of it, and writing inline cost one synchronous HTTP round trip (up
// to five seconds when Keto is slow) per member: for an open channel on a
// large team, longer than the request's write timeout.
func (a *App) writeKetoMembersAsync(channelID string, userIDs []string) {
	if len(userIDs) == 0 {
		return
	}
	ids := slices.Clone(userIDs)
	a.background.Add(1)
	go func() {
		defer a.background.Done()
		ctx, cancel := context.WithTimeout(context.Background(), ketoAsyncTimeout)
		defer cancel()
		for _, uid := range ids {
			if err := a.keto.WriteRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, uid); err != nil {
				slog.Warn("keto: failed to record channel membership",
					"channel_id", channelID, "user_id", uid, "error", err)
				if ctx.Err() != nil {
					return
				}
			}
		}
	}()
}

// WaitBackground blocks until background work started by the App, such as
// Keto writes, has finished.
func (a *App) WaitBackground() {
	a.background.Wait()
}

// searchClient returns a shared ZincSearch client (the SearchIndexer type
// doubles as the query client), created on first use.
func (a *App) searchClient() *workers.SearchIndexer {
	a.zincOnce.Do(func() {
		a.zincClient = workers.NewSearchIndexer(
			a.Config.ZincSearchURL,
			a.Config.ZincSearchUser,
			a.Config.ZincSearchPassword,
		)
	})
	return a.zincClient
}

// New creates a new App instance.
func New(s store.Store, hub *websocket.Hub, ps pubsub.PubSub, cfg *config.Config) *App {
	userCache, err := cache.NewLRU[string, *model.User](cfg.CacheSize, cfg.CacheTTL)
	if err != nil {
		// Invalid cache config (size <= 0): run uncached rather than fail.
		slog.Warn("user cache disabled", "error", err)
		userCache = nil
	}

	return &App{
		Store:      s,
		Hub:        hub,
		PubSub:     ps,
		Config:     cfg,
		keto:       keto.New(cfg.KetoReadURL, cfg.KetoWriteURL, &http.Client{Timeout: 5 * time.Second}),
		httpClient: &http.Client{Timeout: 5 * time.Second},
		userCache:  userCache,
	}
}

// FetchKratosIdentity retrieves identity traits from Kratos Admin API.
func (a *App) FetchKratosIdentity(ctx context.Context, kratosID string) (username, displayName, email string, err error) {
	url := fmt.Sprintf("%s/admin/identities/%s", a.Config.KratosAdminURL, kratosID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", "", "", fmt.Errorf("create kratos request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("fetch kratos identity: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("kratos identity fetch failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// TWO SCHEMAS, ONE READER. chit's own deploy/kratos/identity.schema.json
	// calls the human-readable name `display_name`; the shared cluster schema
	// this deployment authenticates against
	// (terraform/live/ory-identity/config/kratos/identity.schema.json.tftpl)
	// calls it `name` and additionally carries `role` and `kind`.
	//
	// Both are decoded and display_name wins, so neither deployment needs a
	// build flag. Getting this wrong is not a visible error: an identity that
	// authenticates perfectly would be provisioned with an EMPTY display name,
	// and the first thing anyone would notice is a blank author on a post.
	var identity struct {
		Traits struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Name        string `json:"name"`
			Email       string `json:"email"`
			Kind        string `json:"kind"`
		} `json:"traits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return "", "", "", fmt.Errorf("decode kratos identity: %w", err)
	}

	displayName = identity.Traits.DisplayName
	if displayName == "" {
		displayName = identity.Traits.Name
	}
	if displayName == "" {
		// Never provision a nameless user: the username is always present and
		// is a far better fallback than a blank byline.
		displayName = identity.Traits.Username
	}

	slog.Info("fetched kratos identity", "kratos_id", kratosID, "username", identity.Traits.Username)

	return identity.Traits.Username, displayName, identity.Traits.Email, nil
}
