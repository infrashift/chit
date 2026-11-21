package model

import "github.com/infrashift/chit/clients/chit-tui/internal/assert"

// Post props that the server owns and checks.
const (
	// PropOnBehalfOf is set by the server on a post that an allowlisted
	// client made for a person: {"actor_id": <the client's user ID>}. The
	// post's UserID is the person.
	PropOnBehalfOf = "on_behalf_of"
	// PropProvenance records what produced an agent's message.
	PropProvenance = "provenance"
)

// MaxPostFiles is the most files one post can carry, as the server allows.
const MaxPostFiles = 4

// Post mirrors the server's Post model.
type Post struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id"`
	UserID    string         `json:"user_id"`
	RootID    string         `json:"root_id,omitempty"`
	Content   string         `json:"content"`
	Type      string         `json:"type,omitempty"`
	Props     map[string]any `json:"props,omitempty"`
	Hashtags  string         `json:"hashtags,omitempty"`
	FileIDs   []string       `json:"file_ids,omitempty"`
	IsPinned  bool           `json:"is_pinned"`
	EditAt    int64          `json:"edit_at"`
	CreateAt  int64          `json:"create_at"`
	UpdateAt  int64          `json:"update_at"`
	DeleteAt  int64          `json:"delete_at"`
}

// Provenance is what produced an agent's message, as props.provenance holds
// it. Each field is optional.
type Provenance struct {
	Model           string
	ToolIDs         []string
	OKFNamespaceIDs []string
	OKFBlobSHA256   string
	AttachmentIDs   []string
}

// OnBehalfOf returns the user ID of the client that made the post for its
// author, or "" when the author made it.
func (p *Post) OnBehalfOf() string {
	assert.That(p != nil, "Post.OnBehalfOf: nil post")
	obj, ok := p.Props[PropOnBehalfOf].(map[string]any)
	if !ok {
		return ""
	}
	actor, ok := obj["actor_id"].(string)
	if !ok {
		return ""
	}
	assert.That(actor == "" || actor != p.ID, "Post.OnBehalfOf: the actor is the post itself")
	return actor
}

// Provenance returns the post's provenance, and false when it has none or
// the value is not an object.
func (p *Post) Provenance() (*Provenance, bool) {
	assert.That(p != nil, "Post.Provenance: nil post")
	obj, ok := p.Props[PropProvenance].(map[string]any)
	if !ok {
		return nil, false
	}
	pv := &Provenance{
		Model:           propString(obj, "model"),
		ToolIDs:         propStrings(obj, "tool_ids"),
		OKFNamespaceIDs: propStrings(obj, "okf_namespace_ids"),
		OKFBlobSHA256:   propString(obj, "okf_blob_sha256"),
		AttachmentIDs:   propStrings(obj, "attachment_ids"),
	}
	assert.That(pv != nil, "Post.Provenance: nil result")
	return pv, true
}

// propString returns obj[key] when it is a string, else "".
func propString(obj map[string]any, key string) string {
	assert.That(key != "", "propString: empty key")
	s, ok := obj[key].(string)
	if !ok {
		return ""
	}
	return s
}

// propStrings returns the strings in the list obj[key]. It skips each item
// that is not a string, and it returns nil when the value is not a list.
func propStrings(obj map[string]any, key string) []string {
	assert.That(key != "", "propStrings: empty key")
	list, ok := obj[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	assert.That(len(out) <= len(list), "propStrings: more strings than items")
	return out
}

// PostList mirrors the server's PostList model.
type PostList struct {
	Order []*Post `json:"order"`
}
