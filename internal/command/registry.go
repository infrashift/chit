package command

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
		r.bySlug[c.Slug] = c
	}
	return r
}

// Lookup returns the command with the given slug, or false if not found.
func (r *Registry) Lookup(slug string) (*Command, bool) {
	c, ok := r.bySlug[slug]
	return c, ok
}

// All returns every registered command.
func (r *Registry) All() []*Command {
	out := make([]*Command, len(r.all))
	copy(out, r.all)
	return out
}
