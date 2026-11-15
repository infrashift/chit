package model

// Tag scopes, as the server names them.
const (
	TagScopePrivate = "private"
	TagScopeChannel = "channel"
)

// Tag mirrors the server's Tag model. A channel tag belongs to ChannelID and
// is shared by its members; a private tag is seen by its owner only.
type Tag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scope     string `json:"scope,omitempty"`
	OwnerID   string `json:"owner_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
}

// Private reports whether only its owner sees the tag.
func (t *Tag) Private() bool { return t.Scope == TagScopePrivate }

// UsableIn reports whether the tag can be applied in channelID: a channel
// tag in its own channel, a private tag anywhere its owner can read.
func (t *Tag) UsableIn(channelID string) bool {
	return t.Scope != TagScopeChannel || t.ChannelID == channelID
}
