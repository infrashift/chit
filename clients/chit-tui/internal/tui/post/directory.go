package post

import (
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// Directory holds what the panes know about the people and files that
// posts name. The history and the thread pane each keep one.
type Directory struct {
	// Names maps a user ID to a username.
	Names map[string]string
	// Kinds maps a user ID to its actor type, for agents and bots.
	Kinds map[string]string
	// Files maps a file ID to its record.
	Files map[string]*model.File
}

// NewDirectory returns an empty Directory.
func NewDirectory() Directory {
	d := Directory{Names: map[string]string{}, Kinds: map[string]string{}, Files: map[string]*model.File{}}
	assert.That(d.Names != nil && d.Kinds != nil && d.Files != nil, "NewDirectory: nil map")
	return d
}

// Username returns the name of id, or the start of id when the name has
// not loaded.
func (d Directory) Username(id string) string {
	assert.That(d.Names != nil, "Directory.Username: directory not made by NewDirectory")
	if name := d.Names[id]; name != "" {
		return name
	}
	return id[:min(8, len(id))]
}

// Extra returns what p shows from d.
func (d Directory) Extra(p *model.Post) Extra {
	assert.That(p != nil, "Directory.Extra: nil post")
	e := Extra{AuthorKind: d.Kinds[p.UserID]}
	if actor := p.OnBehalfOf(); actor != "" {
		e.Via = d.Username(actor)
	}
	if len(p.FileIDs) > 0 {
		e.Files = make([]*model.File, len(p.FileIDs))
		for i, id := range p.FileIDs {
			e.Files[i] = d.Files[id]
		}
	}
	assert.That(len(e.Files) == len(p.FileIDs), "Directory.Extra: a record slot for each file")
	return e
}

// Shows returns a key of all that p shows from d. When the key of a post
// changes between two directories, the post must be drawn again.
func (d Directory) Shows(p *model.Post) string {
	assert.That(p != nil, "Directory.Shows: nil post")
	parts := []string{d.Username(p.UserID), d.Kinds[p.UserID]}
	if actor := p.OnBehalfOf(); actor != "" {
		parts = append(parts, d.Username(actor))
	}
	for _, id := range p.FileIDs {
		if d.Files[id] != nil {
			parts = append(parts, id)
		}
	}
	assert.That(len(parts) >= 2, "Directory.Shows: parts missing")
	return strings.Join(parts, "\x00")
}
