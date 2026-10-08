package mcp

import (
	"encoding/json"
	"slices"

	"github.com/infrashift/chit/internal/model"
)

// postView is what an agent gets for a post: what was said, by whom, where
// and when. Everything a tool returns is read into the model's context, so
// the rest of model.Post stays out — props in particular, which on agent
// replies carry session IDs, hop counts, usage figures and mention lists that
// cost tokens and tell the reader nothing it can act on. The JSON names match
// model.Post, so a view decodes as a post.
type postView struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id,omitempty"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	Type      string `json:"type,omitempty"`
	IsPinned  bool   `json:"is_pinned,omitempty"`
	Edited    bool   `json:"edited,omitempty"`
	CreateAt  int64  `json:"create_at"`
}

func viewPost(p *model.Post) *postView {
	if p == nil {
		return nil
	}
	return &postView{
		ID:        p.ID,
		ChannelID: p.ChannelID,
		RootID:    p.RootID,
		UserID:    p.UserID,
		Content:   p.Content,
		Type:      p.Type,
		IsPinned:  p.IsPinned,
		Edited:    p.EditAt > 0,
		CreateAt:  p.CreateAt,
	}
}

// postListView keeps model.PostList's shape: {"order": [...]}.
type postListView struct {
	Order []*postView `json:"order"`
}

func viewPosts(posts []*model.Post) *postListView {
	out := &postListView{Order: make([]*postView, 0, len(posts))}
	for _, p := range posts {
		out.Order = append(out.Order, viewPost(p))
	}
	return out
}

func viewPostList(list *model.PostList) *postListView {
	if list == nil {
		return viewPosts(nil)
	}
	return viewPosts(list.Order)
}

// defaultEventTypes are the events get_new_events returns when the caller
// names none: the ones that change what was said. Typing, read state,
// membership and the rest are discarded unless asked for.
var defaultEventTypes = []string{
	model.WebSocketEventPosted,
	model.WebSocketEventPostEdited,
	model.WebSocketEventPostDeleted,
}

// viewEvents keeps the events of the wanted types, slimming the post that
// posted and post_edited events carry whole.
func viewEvents(events []*model.WebSocketEvent, types []string) []*model.WebSocketEvent {
	if len(types) == 0 {
		types = defaultEventTypes
	}
	var out []*model.WebSocketEvent
	for _, e := range events {
		if !slices.Contains(types, e.Event) {
			continue
		}
		if e.Event == model.WebSocketEventPosted || e.Event == model.WebSocketEventPostEdited {
			e = &model.WebSocketEvent{Event: e.Event, Data: slimPostData(e.Data), Sequence: e.Sequence}
		}
		out = append(out, e)
	}
	return out
}

// slimPostData re-encodes a post carried as event data through postView.
// Data that is not a post is returned as it came.
func slimPostData(data map[string]any) map[string]any {
	raw, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var post model.Post
	if json.Unmarshal(raw, &post) != nil || post.ID == "" {
		return data
	}
	raw, err = json.Marshal(viewPost(&post))
	if err != nil {
		return data
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return data
	}
	return out
}
