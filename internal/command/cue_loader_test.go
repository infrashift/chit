package command

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCUETestData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	schemaDir := filepath.Join(dir, "schema")
	rolesDir := filepath.Join(dir, "roles")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rolesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	commandsCUE := `package auth

#Command: {
	id:          string
	slug:        string
	description: string
	category:    "chat" | "admin" | "agent"
}

registry: [string]: #Command

registry: {
	help: {
		id:          "help"
		slug:        "help"
		description: "List available commands"
		category:    "chat"
	}
	kick: {
		id:          "kick"
		slug:        "kick"
		description: "Remove a user"
		category:    "admin"
	}
}
`
	rolesCUE := `package auth

#Role: {
	name:             string
	allowed_commands: [...string]
}
`
	adminCUE := `package auth

roles: admin: {
	name: "admin"
	allowed_commands: ["help", "kick"]
}
`
	userCUE := `package auth

roles: user: {
	name: "user"
	allowed_commands: ["help"]
}
`

	if err := os.WriteFile(filepath.Join(schemaDir, "commands.cue"), []byte(commandsCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "roles.cue"), []byte(rolesCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rolesDir, "admin.cue"), []byte(adminCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rolesDir, "user.cue"), []byte(userCUE), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestLoadCUE(t *testing.T) {
	dir := writeCUETestData(t)

	cfg, err := LoadCUE(dir)
	if err != nil {
		t.Fatalf("LoadCUE: %v", err)
	}

	if len(cfg.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cfg.Commands))
	}

	found := map[string]bool{}
	for _, c := range cfg.Commands {
		found[c.Slug] = true
		if c.ID == "" {
			t.Errorf("command %s has empty ID", c.Slug)
		}
	}
	if !found["help"] || !found["kick"] {
		t.Error("expected help and kick commands")
	}

	if len(cfg.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(cfg.Roles))
	}

	roleMap := map[string]CUERole{}
	for _, r := range cfg.Roles {
		roleMap[r.Name] = r
	}
	admin, ok := roleMap["admin"]
	if !ok {
		t.Fatal("admin role missing")
	}
	if len(admin.AllowedCommands) != 2 {
		t.Errorf("admin should have 2 commands, got %d", len(admin.AllowedCommands))
	}

	user, ok := roleMap["user"]
	if !ok {
		t.Fatal("user role missing")
	}
	if len(user.AllowedCommands) != 1 {
		t.Errorf("user should have 1 command, got %d", len(user.AllowedCommands))
	}
}

func TestLoadCUEInvalidDir(t *testing.T) {
	_, err := LoadCUE("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error for invalid directory")
	}
}
