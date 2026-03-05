package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
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

	ketoReadURL  string
	ketoWriteURL string
	vaultAddr    string
	vaultToken   string
	vaultKey     string
	httpClient   *http.Client
}

// New creates a new App instance.
func New(s store.Store, hub *websocket.Hub, ps pubsub.PubSub, cfg *config.Config) *App {
	return &App{
		Store:        s,
		Hub:          hub,
		PubSub:       ps,
		Config:       cfg,
		ketoReadURL:  cfg.KetoReadURL,
		ketoWriteURL: cfg.KetoWriteURL,
		vaultAddr:    cfg.VaultAddr,
		vaultToken:   cfg.VaultToken,
		vaultKey:     cfg.VaultTransitKey,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}
}

// CheckChannelPermission verifies a user has a specific relation on a channel via Keto.
func (a *App) CheckChannelPermission(ctx context.Context, channelID, userID, relation string) (bool, error) {
	url := fmt.Sprintf("%s/relation-tuples/check", a.ketoReadURL)

	body := map[string]string{
		"namespace":  "chit/channel",
		"object":     channelID,
		"relation":   relation,
		"subject_id": userID,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("marshal keto check: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return false, fmt.Errorf("create keto request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("keto check: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode keto response: %w", err)
	}

	return result.Allowed, nil
}

// WriteKetoRelation writes a relation tuple to Keto.
func (a *App) WriteKetoRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	url := fmt.Sprintf("%s/admin/relation-tuples", a.ketoWriteURL)

	body := map[string]string{
		"namespace":  namespace,
		"object":     object,
		"relation":   relation,
		"subject_id": subjectID,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal keto write: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create keto write request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keto write: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("keto write: status %d", resp.StatusCode)
	}

	return nil
}

// DeleteKetoRelation removes a relation tuple from Keto.
func (a *App) DeleteKetoRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	url := fmt.Sprintf("%s/admin/relation-tuples?namespace=%s&object=%s&relation=%s&subject_id=%s",
		a.ketoWriteURL, namespace, object, relation, subjectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create keto delete request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keto delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("keto delete: status %d", resp.StatusCode)
	}

	return nil
}

// EncryptContent encrypts content via Vault Transit if enabled.
func (a *App) EncryptContent(ctx context.Context, plaintext string) ([]byte, error) {
	if !a.Config.VaultEnabled {
		return nil, nil
	}

	url := fmt.Sprintf("%s/v1/transit/encrypt/%s", a.vaultAddr, a.vaultKey)
	body := map[string]string{
		"plaintext": plaintext,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal vault encrypt: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", a.vaultToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault encrypt: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Ciphertext string `json:"ciphertext"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode vault response: %w", err)
	}

	return []byte(result.Data.Ciphertext), nil
}

// DecryptContent decrypts content via Vault Transit.
func (a *App) DecryptContent(ctx context.Context, ciphertext []byte) (string, error) {
	if !a.Config.VaultEnabled || len(ciphertext) == 0 {
		return "", nil
	}

	url := fmt.Sprintf("%s/v1/transit/decrypt/%s", a.vaultAddr, a.vaultKey)
	body := map[string]string{
		"ciphertext": string(ciphertext),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal vault decrypt: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", a.vaultToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault decrypt: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode vault response: %w", err)
	}

	return result.Data.Plaintext, nil
}

// FetchKratosIdentity retrieves identity traits from Kratos Admin API.
func (a *App) FetchKratosIdentity(ctx context.Context, kratosID string) (username, displayName, email string, err error) {
	url := fmt.Sprintf("%s/admin/identities/%s", a.Config.KratosAdminURL, kratosID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("create kratos request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("fetch kratos identity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("kratos identity fetch failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var identity struct {
		Traits struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"traits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return "", "", "", fmt.Errorf("decode kratos identity: %w", err)
	}

	slog.Info("fetched kratos identity", "kratos_id", kratosID, "username", identity.Traits.Username)

	return identity.Traits.Username, identity.Traits.DisplayName, identity.Traits.Email, nil
}
