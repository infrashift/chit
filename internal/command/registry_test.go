package command

import "testing"

func TestRegistryLookup(t *testing.T) {
	cmds := []*Command{
		{ID: "cmd-help", Slug: "help", Description: "List available commands", Category: "chat"},
		{ID: "cmd-kick", Slug: "kick", Description: "Remove a user", Category: "admin"},
	}
	reg := NewRegistry(cmds)

	t.Run("hit", func(t *testing.T) {
		c, ok := reg.Lookup("help")
		if !ok {
			t.Fatal("expected to find 'help'")
		}
		if c.ID != "cmd-help" {
			t.Fatalf("unexpected id: %s", c.ID)
		}
	})

	t.Run("miss", func(t *testing.T) {
		_, ok := reg.Lookup("nonexistent")
		if ok {
			t.Fatal("expected miss for 'nonexistent'")
		}
	})

	t.Run("all returns copy", func(t *testing.T) {
		all := reg.All()
		if len(all) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(all))
		}
		// Mutating the returned slice should not affect the registry.
		all[0] = nil
		c, ok := reg.Lookup("help")
		if !ok || c == nil {
			t.Fatal("registry was mutated through All()")
		}
	})
}
