package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type seedIdentity struct {
	Email       string
	Username    string
	DisplayName string
	Password    string
}

// The password for all UAT users is "Password1!" — suitable for manual testing only.
var identities = []seedIdentity{
	{"alice@example.com", "alice", "Alice Anderson", "Password1!"},
	{"bob@example.com", "bob", "Bob Baker", "Password1!"},
	{"chad@example.com", "chad", "Chad Cooper", "Password1!"},
	{"diana@example.com", "diana", "Diana Drake", "Password1!"},
	{"eve@example.com", "eve", "Eve Ellis", "Password1!"},
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// run does the seeding; it is split from main so the deferred db.Close runs
// on every exit path, including failures.
func run(ctx context.Context) error {
	kratosAdminURL := os.Getenv("KRATOS_ADMIN_URL")
	if kratosAdminURL == "" {
		kratosAdminURL = "http://localhost:4434"
	}

	dbURL := os.Getenv("CHIT_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://chit:chit@localhost:5432/chit?sslmode=disable"
	}

	// Wait for Kratos admin API to be ready.
	fmt.Println("Waiting for Kratos admin API...")
	client := &http.Client{Timeout: 5 * time.Second}
	for i := range 30 {
		resp, err := httpDo(ctx, client, http.MethodGet, kratosAdminURL+"/health/ready", nil)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Printf("Kratos admin is ready (%ds).\n", i)
				break
			}
		}
		if i == 29 {
			return errors.New("kratos admin API not ready after 30s")
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to Chit database to sync kratos_id values.
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err = db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	fmt.Println("Connected to Chit database.")

	fmt.Println()
	fmt.Println("Creating Kratos identities...")
	for _, id := range identities {
		kratosID, err := ensureIdentity(ctx, client, kratosAdminURL, id)
		if err != nil {
			return fmt.Errorf("ensure identity for %s: %w", id.Username, err)
		}
		fmt.Printf("  %-8s  kratos_id=%s\n", id.Username, kratosID)

		// Sync kratos_id into the Chit database users table.
		result, err := db.ExecContext(ctx,
			"UPDATE users SET kratos_id = $1 WHERE email = $2 AND delete_at = 0",
			kratosID, id.Email,
		)
		if err != nil {
			return fmt.Errorf("update kratos_id for %s: %w", id.Username, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			fmt.Printf("  WARNING: no Chit user found with email %s\n", id.Email)
		}
	}

	fmt.Println()
	fmt.Println("=== UAT Login Credentials ===")
	fmt.Println()
	fmt.Printf("  %-10s %-24s %s\n", "Username", "Email (login identifier)", "Password")
	fmt.Printf("  %-10s %-24s %s\n", "--------", "-----------------------", "--------")
	for _, id := range identities {
		fmt.Printf("  %-10s %-24s %s\n", id.Username, id.Email, id.Password)
	}
	fmt.Println()
	fmt.Println("All users can log in via chit-tui using their email and password.")
	fmt.Println()
	return nil
}

// httpDo sends a request bound to ctx. body may be nil.
func httpDo(ctx context.Context, client *http.Client, method, target string, body []byte) (*http.Response, error) {
	var rdr io.Reader = http.NoBody
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

// ensureIdentity creates a Kratos identity or returns the existing one's ID.
func ensureIdentity(ctx context.Context, client *http.Client, baseURL string, id seedIdentity) (string, error) {
	// Try to find an existing identity by credential identifier (email).
	kratosID, err := findIdentityByEmail(ctx, client, baseURL, id.Email)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	if kratosID != "" {
		// Identity exists — update password to ensure it matches.
		if err = updatePassword(ctx, client, baseURL, kratosID, id); err != nil {
			return "", fmt.Errorf("update password: %w", err)
		}
		fmt.Printf("  Identity %-8s already exists, updated password.\n", id.Username)
		return kratosID, nil
	}

	// Create new identity.
	kratosID, err = createIdentity(ctx, client, baseURL, id)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	fmt.Printf("  Created identity %-8s\n", id.Username)
	return kratosID, nil
}

func findIdentityByEmail(ctx context.Context, client *http.Client, baseURL, email string) (string, error) {
	target := fmt.Sprintf("%s/admin/identities?credentials_identifier=%s", baseURL, url.QueryEscape(email))
	resp, err := httpDo(ctx, client, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var results []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}
	return results[0].ID, nil
}

func createIdentity(ctx context.Context, client *http.Client, baseURL string, id seedIdentity) (string, error) {
	body := map[string]any{
		"schema_id": "default",
		"traits": map[string]string{
			"email":        id.Email,
			"username":     id.Username,
			"display_name": id.DisplayName,
		},
		"credentials": map[string]any{
			"password": map[string]any{
				"config": map[string]string{
					"password": id.Password,
				},
			},
		},
		"state": "active",
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	resp, err := httpDo(ctx, client, http.MethodPost, baseURL+"/admin/identities", data)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.ID, nil
}

func updatePassword(ctx context.Context, client *http.Client, baseURL, kratosID string, id seedIdentity) error {
	body := map[string]any{
		"schema_id": "default",
		"traits": map[string]string{
			"email":        id.Email,
			"username":     id.Username,
			"display_name": id.DisplayName,
		},
		"credentials": map[string]any{
			"password": map[string]any{
				"config": map[string]string{
					"password": id.Password,
				},
			},
		},
		"state": "active",
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := httpDo(ctx, client, http.MethodPut, baseURL+"/admin/identities/"+url.PathEscape(kratosID), data)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
