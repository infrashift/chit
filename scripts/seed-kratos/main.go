package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		resp, err := client.Get(kratosAdminURL + "/health/ready")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Printf("Kratos admin is ready (%ds).\n", i)
				break
			}
		}
		if i == 29 {
			log.Fatal("Kratos admin API not ready after 30s")
		}
		time.Sleep(1 * time.Second)
	}

	// Connect to Chit database to sync kratos_id values.
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Connected to Chit database.")

	fmt.Println()
	fmt.Println("Creating Kratos identities...")
	for _, id := range identities {
		kratosID, err := ensureIdentity(client, kratosAdminURL, id)
		if err != nil {
			log.Fatalf("Failed to ensure identity for %s: %v", id.Username, err)
		}
		fmt.Printf("  %-8s  kratos_id=%s\n", id.Username, kratosID)

		// Sync kratos_id into the Chit database users table.
		result, err := db.Exec(
			"UPDATE users SET kratos_id = $1 WHERE email = $2 AND delete_at = 0",
			kratosID, id.Email,
		)
		if err != nil {
			log.Fatalf("Failed to update kratos_id for %s: %v", id.Username, err)
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
}

// ensureIdentity creates a Kratos identity or returns the existing one's ID.
func ensureIdentity(client *http.Client, baseURL string, id seedIdentity) (string, error) {
	// Try to find an existing identity by credential identifier (email).
	kratosID, err := findIdentityByEmail(client, baseURL, id.Email)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	if kratosID != "" {
		// Identity exists — update password to ensure it matches.
		if err := updatePassword(client, baseURL, kratosID, id); err != nil {
			return "", fmt.Errorf("update password: %w", err)
		}
		fmt.Printf("  Identity %-8s already exists, updated password.\n", id.Username)
		return kratosID, nil
	}

	// Create new identity.
	kratosID, err = createIdentity(client, baseURL, id)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	fmt.Printf("  Created identity %-8s\n", id.Username)
	return kratosID, nil
}

func findIdentityByEmail(client *http.Client, baseURL, email string) (string, error) {
	url := fmt.Sprintf("%s/admin/identities?credentials_identifier=%s", baseURL, email)
	resp, err := client.Get(url)
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

func createIdentity(client *http.Client, baseURL string, id seedIdentity) (string, error) {
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

	req, err := http.NewRequest("POST", baseURL+"/admin/identities", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
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

func updatePassword(client *http.Client, baseURL, kratosID string, id seedIdentity) error {
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

	url := fmt.Sprintf("%s/admin/identities/%s", baseURL, kratosID)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
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
