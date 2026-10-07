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
	ActorType   string
}

var users = []seedUser{
	{"alice", "Alice Anderson", "alice@example.com", "a11ce000-0000-4000-a000-000000000001", "system_admin", model.ActorTypeUser},
	{"bob", "Bob Baker", "bob@example.com", "b0b00000-0000-4000-a000-000000000002", "system_user", model.ActorTypeUser},
	{"chad", "Chad Cooper", "chad@example.com", "c4ad0000-0000-4000-a000-000000000003", "system_user", model.ActorTypeUser},
	{"diana", "Diana Drake", "diana@example.com", "d1a4a000-0000-4000-a000-000000000004", "system_user", model.ActorTypeUser},
	{"eve", "Eve Ellis", "eve@example.com", "e0e00000-0000-4000-a000-000000000005", "system_user", model.ActorTypeUser},
	// The MCP agent is an equal actor: a normal user row with actor_type=agent,
	// added to the same team and channel. Point CHIT_MCP_AGENT_KRATOS_ID at its
	// Kratos ID.
	{"chit-agent", "Chit Agent", "agent@example.com", "a9e47000-0000-4000-a000-000000000006", "system_user", model.ActorTypeAgent},
}

func isNotFound(err error) bool {
	var appErr *model.AppError
	return errors.As(err, &appErr) && appErr.StatusCode == 404
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// run does the seeding; it is split from main so the deferred store.Close
// runs on every exit path, including failures.
func run(ctx context.Context) error {
	dbURL := os.Getenv("CHIT_DATABASE_URL")
	if dbURL == "" {
		return errors.New("CHIT_DATABASE_URL is not set")
	}

	store, err := sqlstore.New(ctx, dbURL, 5, 2)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer store.Close()

	// Create users (idempotent — skip if already exists).
	userIDs := make([]string, len(users))
	for i, u := range users {
		existing, lookupErr := store.User().GetByKratosID(ctx, u.KratosID)
		if lookupErr == nil {
			userIDs[i] = existing.ID
			fmt.Printf("  User %-8s already exists  id=%s\n", u.Username, existing.ID)
			continue
		}
		if !isNotFound(lookupErr) {
			return fmt.Errorf("check user %s: %w", u.Username, lookupErr)
		}

		// The Kratos ID may have moved: seed-kratos creates real identities
		// and rebinds these users to them, so the hardcoded ID above stops
		// matching. Fall back to the username, which is the stable identity
		// here and is what the unique constraint is on — without this, a
		// second `make uat-up` after seeding Kratos fails on a duplicate.
		byName, nameErr := store.User().GetByUsername(ctx, u.Username)
		if nameErr == nil {
			userIDs[i] = byName.ID
			fmt.Printf("  User %-8s already exists  id=%s (kratos_id=%s)\n",
				u.Username, byName.ID, byName.KratosID)
			continue
		}
		if !isNotFound(nameErr) {
			return fmt.Errorf("check user %s by username: %w", u.Username, nameErr)
		}

		saved, saveErr := store.User().Save(ctx, &model.User{
			KratosID:    u.KratosID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Roles:       u.Roles,
			ActorType:   u.ActorType,
		})
		if saveErr != nil {
			return fmt.Errorf("create user %s: %w", u.Username, saveErr)
		}
		userIDs[i] = saved.ID
		fmt.Printf("  Created user %-8s  id=%s\n", u.Username, saved.ID)
	}

	// Create team (idempotent).
	var team *model.Team
	team, err = store.Team().GetByName(ctx, "uat-team")
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("check team: %w", err)
	}
	if team == nil {
		team, err = store.Team().Save(ctx, &model.Team{
			Name:        "uat-team",
			DisplayName: "UAT Team",
			Type:        model.TeamOpen,
			CreatorID:   userIDs[0],
		})
		if err != nil {
			return fmt.Errorf("create team: %w", err)
		}
		fmt.Printf("  Created team %-8s  id=%s\n", team.Name, team.ID)
	} else {
		fmt.Printf("  Team %-8s already exists  id=%s\n", team.Name, team.ID)
	}

	// Add all users to team (SaveMember is already idempotent via ON CONFLICT).
	for i, uid := range userIDs {
		if _, err = store.Team().SaveMember(ctx, &model.TeamMember{
			TeamID: team.ID,
			UserID: uid,
		}); err != nil {
			return fmt.Errorf("add user %s to team: %w", users[i].Username, err)
		}
	}
	fmt.Printf("  Ensured %d members in team\n", len(userIDs))

	// Create channel (idempotent).
	var channel *model.Channel
	channel, err = store.Channel().GetByName(ctx, team.ID, "town-square")
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("check channel: %w", err)
	}
	if channel == nil {
		channel, err = store.Channel().Save(ctx, &model.Channel{
			TeamID:      team.ID,
			CreatorID:   userIDs[0],
			Name:        "town-square",
			DisplayName: "Town Square",
			Type:        model.ChannelOpen,
		})
		if err != nil {
			return fmt.Errorf("create channel: %w", err)
		}
		fmt.Printf("  Created channel %-14s  id=%s\n", channel.Name, channel.ID)
	} else {
		fmt.Printf("  Channel %-14s already exists  id=%s\n", channel.Name, channel.ID)
	}

	// Add all users to channel (SaveMember is already idempotent via ON CONFLICT).
	for i, uid := range userIDs {
		if _, err = store.Channel().SaveMember(ctx, &model.ChannelMember{
			ChannelID: channel.ID,
			UserID:    uid,
		}); err != nil {
			return fmt.Errorf("add user %s to channel: %w", users[i].Username, err)
		}
	}
	fmt.Printf("  Ensured %d members in channel\n", len(userIDs))

	// Print cheat sheet.
	fmt.Println()
	fmt.Println("=== UAT Cheat Sheet ===")
	fmt.Println()
	fmt.Println("Users:")
	fmt.Printf("  %-10s %-18s %-7s %s\n", "Username", "Display Name", "Actor", "X-User-Id header")
	fmt.Printf("  %-10s %-18s %-7s %s\n", "--------", "------------", "-----", "----------------")
	for _, u := range users {
		fmt.Printf("  %-10s %-18s %-7s %s\n", u.Username, u.DisplayName, u.ActorType, u.KratosID)
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
	fmt.Printf("    -d '{\"channel_id\":\"%s\",\"content\":\"Hello from UAT!\"}' \\\n", channel.ID)
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
	return nil
}
