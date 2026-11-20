package command

import (
	"github.com/infrashift/chit/internal/assert"
)

// Command represents a slash command definition.
type Command struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// Registry holds known slash commands and supports lookup by slug.
type Registry struct {
	bySlug map[string]*Command
	all    []*Command
}

// NewRegistry creates a Registry from a slice of commands.
func NewRegistry(commands []*Command) *Registry {
	r := &Registry{
		bySlug: make(map[string]*Command, len(commands)),
		all:    make([]*Command, len(commands)),
	}
	copy(r.all, commands)
	for _, c := range commands {
		assert.That(c != nil, "NewRegistry: nil command")
		r.bySlug[c.Slug] = c
	}
	assert.That(len(r.all) == len(commands), "NewRegistry: copy lost commands")
	assert.That(len(r.bySlug) <= len(r.all), "NewRegistry: more slugs than commands")
	return r
}

// Lookup returns the command with the given slug, or false if not found.
func (r *Registry) Lookup(slug string) (*Command, bool) {
	assert.That(r != nil && r.bySlug != nil, "Registry.Lookup: registry not built by NewRegistry")
	c, ok := r.bySlug[slug]
	assert.That(!ok || c.Slug == slug, "Registry.Lookup: command filed under another slug")
	return c, ok
}

// All returns every registered command.
func (r *Registry) All() []*Command {
	assert.That(r != nil, "Registry.All: nil registry")
	out := make([]*Command, len(r.all))
	n := copy(out, r.all)
	assert.That(n == len(r.all), "Registry.All: copy lost commands")
	return out
}
