package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store/sqlstore"
)

type seedUser struct {
	Username    string
	DisplayName string
	Email       string
	KratosID    string
	Roles       string
}

var users = []seedUser{
	{"alice", "Alice Anderson", "alice@example.com", "a11ce000-0000-4000-a000-000000000001", "system_admin"},
	{"bob", "Bob Baker", "bob@example.com", "b0b00000-0000-4000-a000-000000000002", "system_user"},
	{"chad", "Chad Cooper", "chad@example.com", "c4ad0000-0000-4000-a000-000000000003", "system_user"},
	{"diana", "Diana Drake", "diana@example.com", "d1a4a000-0000-4000-a000-000000000004", "system_user"},
	{"eve", "Eve Ellis", "eve@example.com", "e0e00000-0000-4000-a000-000000000005", "system_user"},
}

func isNotFound(err error) bool {
	var appErr *model.AppError
	return errors.As(err, &appErr) && appErr.StatusCode == 404
}

func main() {
	dbURL := os.Getenv("CHIT_DATABASE_URL")
	if dbURL == "" {
		log.Fatal("CHIT_DATABASE_URL is not set")
	}

	ctx := context.Background()
	store, err := sqlstore.New(ctx, dbURL, 5, 2)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	// Create users (idempotent — skip if already exists).
	userIDs := make([]string, len(users))
	for i, u := range users {
		existing, err := store.User().GetByKratosID(u.KratosID)
		if err == nil {
			userIDs[i] = existing.ID
			fmt.Printf("  User %-8s already exists  id=%s\n", u.Username, existing.ID)
			continue
		}
		if !isNotFound(err) {
			log.Fatalf("Failed to check user %s: %v", u.Username, err)
		}

		saved, err := store.User().Save(&model.User{
			KratosID:    u.KratosID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Roles:       u.Roles,
		})
		if err != nil {
			log.Fatalf("Failed to create user %s: %v", u.Username, err)
		}
		userIDs[i] = saved.ID
		fmt.Printf("  Created user %-8s  id=%s\n", u.Username, saved.ID)
	}

	// Create team (idempotent).
	var team *model.Team
	team, err = store.Team().GetByName("uat-team")
	if err != nil && !isNotFound(err) {
		log.Fatalf("Failed to check team: %v", err)
	}
	if team == nil {
		team, err = store.Team().Save(&model.Team{
			Name:        "uat-team",
			DisplayName: "UAT Team",
			Type:        model.TeamOpen,
			CreatorID:   userIDs[0],
		})
		if err != nil {
			log.Fatalf("Failed to create team: %v", err)
		}
		fmt.Printf("  Created team %-8s  id=%s\n", team.Name, team.ID)
	} else {
		fmt.Printf("  Team %-8s already exists  id=%s\n", team.Name, team.ID)
	}

	// Add all users to team (SaveMember is already idempotent via ON CONFLICT).
	for i, uid := range userIDs {
		_, err := store.Team().SaveMember(&model.TeamMember{
			TeamID: team.ID,
			UserID: uid,
		})
		if err != nil {
			log.Fatalf("Failed to add user %s to team: %v", users[i].Username, err)
		}
	}
	fmt.Printf("  Ensured %d members in team\n", len(userIDs))

	// Create channel (idempotent).
	var channel *model.Channel
	channel, err = store.Channel().GetByName(team.ID, "town-square")
	if err != nil && !isNotFound(err) {
		log.Fatalf("Failed to check channel: %v", err)
	}
	if channel == nil {
		channel, err = store.Channel().Save(&model.Channel{
			TeamID:      team.ID,
			CreatorID:   userIDs[0],
			Name:        "town-square",
			DisplayName: "Town Square",
			Type:        model.ChannelOpen,
		})
		if err != nil {
			log.Fatalf("Failed to create channel: %v", err)
		}
		fmt.Printf("  Created channel %-14s  id=%s\n", channel.Name, channel.ID)
	} else {
		fmt.Printf("  Channel %-14s already exists  id=%s\n", channel.Name, channel.ID)
	}

	// Add all users to channel (SaveMember is already idempotent via ON CONFLICT).
	for i, uid := range userIDs {
		_, err := store.Channel().SaveMember(&model.ChannelMember{
			ChannelID: channel.ID,
			UserID:    uid,
		})
		if err != nil {
			log.Fatalf("Failed to add user %s to channel: %v", users[i].Username, err)
		}
	}
	fmt.Printf("  Ensured %d members in channel\n", len(userIDs))

	// Print cheat sheet.
	fmt.Println()
	fmt.Println("=== UAT Cheat Sheet ===")
	fmt.Println()
	fmt.Println("Users:")
	fmt.Printf("  %-10s %-18s %s\n", "Username", "Display Name", "X-User-Id header")
	fmt.Printf("  %-10s %-18s %s\n", "--------", "------------", "----------------")
	for _, u := range users {
		fmt.Printf("  %-10s %-18s %s\n", u.Username, u.DisplayName, u.KratosID)
	}
	fmt.Println()
	fmt.Printf("Team:    %s (id=%s)\n", team.Name, team.ID)
	fmt.Printf("Channel: %s (id=%s)\n", channel.Name, channel.ID)
	fmt.Println()
	fmt.Println("Example curl commands:")
	fmt.Println()
	fmt.Println("  # Ping")
	fmt.Println("  curl http://localhost:8065/api/v1/system/ping")
	fmt.Println()
	fmt.Println("  # Who am I (as Alice)")
	fmt.Printf("  curl -H 'X-User-Id: %s' http://localhost:8065/api/v1/users/me\n", users[0].KratosID)
	fmt.Println()
	fmt.Println("  # Post a message (as Alice)")
	fmt.Printf("  curl -X POST -H 'X-User-Id: %s' \\\n", users[0].KratosID)
	fmt.Printf("    -H 'Content-Type: application/json' \\\n")
	fmt.Printf("    -d '{\"channel_id\":\"%s\",\"message\":\"Hello from UAT!\"}' \\\n", channel.ID)
	fmt.Printf("    http://localhost:8065/api/v1/posts\n")
	fmt.Println()
	fmt.Println("  # Read channel posts")
	fmt.Printf("  curl -H 'X-User-Id: %s' http://localhost:8065/api/v1/channels/%s/posts\n", users[0].KratosID, channel.ID)
	fmt.Println()
	fmt.Println("  # Search users")
	fmt.Printf("  curl -H 'X-User-Id: %s' 'http://localhost:8065/api/v1/users?term=bob'\n", users[0].KratosID)
	fmt.Println()
	fmt.Println("  # Create a DM channel (Alice → Bob)")
	fmt.Printf("  curl -X POST -H 'X-User-Id: %s' \\\n", users[0].KratosID)
	fmt.Printf("    -H 'Content-Type: application/json' \\\n")
	fmt.Printf("    -d '[\"%s\",\"%s\"]' \\\n", userIDs[0], userIDs[1])
	fmt.Printf("    http://localhost:8065/api/v1/channels/direct\n")
	fmt.Println()
	fmt.Println("========================")
}
