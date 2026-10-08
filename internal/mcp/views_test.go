package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/model"
)

// agentReply is a post as chit-claude writes it: a paragraph of answer plus
// the props a bridge attaches.
func agentReply() *model.Post {
	return &model.Post{
		ID: "019421a0-0000-7000-8000-0000000000a1", ChannelID: "019421a0-0000-7000-8000-0000000000c1",
		RootID: "019421a0-0000-7000-8000-0000000000a0", UserID: "019421a0-0000-7000-8000-0000000000u1",
		Content: "The migration has three phases: dual-write, backfill, cut-over. " +
			"Dual-write needs the new table and a feature flag; backfill runs as a batch job; " +
			"cut-over flips reads once the backfill's row counts match.",
		Props: map[string]any{
			"claude_session_id": "6c6145db-bf08-4143-a1a0-5b897d097a47",
			"claude_agent_hops": 0,
			"mentions":          []any{"019421a0-0000-7000-8000-0000000000u2"},
			"claude_usage": map[string]any{
				"input_tokens": 9, "output_tokens": 412, "cache_read_input_tokens": 13898,
				"cache_creation_input_tokens": 7530, "total_cost_usd": 0.0166, "num_turns": 1,
			},
		},
		Hashtags: "#migration", CreateAt: 1791400000000, UpdateAt: 1791400000000,
	}
}

func TestViewPost_KeepsWhatWasSaid(t *testing.T) {
	full := agentReply()
	full.EditAt = 1791400001000
	raw, _ := json.Marshal(viewPost(full))

	var back model.Post
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != full.ID || back.RootID != full.RootID || back.UserID != full.UserID ||
		back.Content != full.Content || back.CreateAt != full.CreateAt {
		t.Errorf("view lost a field the reader needs: %s", raw)
	}
	for _, gone := range []string{"props", "claude_session_id", "hashtags", "update_at", "delete_at"} {
		if strings.Contains(string(raw), gone) {
			t.Errorf("view should not carry %q: %s", gone, raw)
		}
	}
	if !strings.Contains(string(raw), `"edited":true`) {
		t.Errorf("an edited post should say so: %s", raw)
	}

	// What it saves on a typical agent reply, measured rather than assumed.
	fullRaw, _ := json.Marshal(full)
	t.Logf("agent reply: full %d bytes, view %d bytes (%.1fx)",
		len(fullRaw), len(raw), float64(len(fullRaw))/float64(len(raw)))
	if len(raw) >= len(fullRaw) {
		t.Errorf("view (%d bytes) is no smaller than the post (%d)", len(raw), len(fullRaw))
	}
}

func TestViewEvents(t *testing.T) {
	data, _ := json.Marshal(agentReply())
	var postData map[string]any
	_ = json.Unmarshal(data, &postData)
	events := []*model.WebSocketEvent{
		{Event: model.WebSocketEventTyping, Data: map[string]any{"user_id": "u"}, Sequence: 1},
		{Event: model.WebSocketEventPosted, Data: postData, Sequence: 2},
		{Event: model.WebSocketEventChannelUpdated, Data: map[string]any{"channel_id": "c"}, Sequence: 3},
		{Event: model.WebSocketEventPostDeleted, Data: map[string]any{"post_id": "p"}, Sequence: 4},
	}

	got := viewEvents(events, nil)
	if len(got) != 2 || got[0].Event != model.WebSocketEventPosted || got[1].Event != model.WebSocketEventPostDeleted {
		t.Fatalf("default types should keep posted and post_deleted, got %+v", got)
	}
	if _, ok := got[0].Data["props"]; ok || got[0].Data["content"] == nil {
		t.Errorf("a posted event's post should be slimmed, not emptied: %v", got[0].Data)
	}
	if got[0].Sequence != 2 {
		t.Errorf("sequence must survive slimming, got %d", got[0].Sequence)
	}
	if _, ok := events[1].Data["props"]; !ok {
		t.Error("slimming must not modify the buffered event")
	}

	got = viewEvents(events, []string{model.WebSocketEventTyping})
	if len(got) != 1 || got[0].Event != model.WebSocketEventTyping {
		t.Errorf("named types replace the default, got %+v", got)
	}
}

func TestMCP_Prompt_SummarizeChannelCapsPosts(t *testing.T) {
	ctx, cs, _, cleanup := setupTestMCP(t)
	defer cleanup()

	_, err := cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name:      "summarize_channel",
		Arguments: map[string]string{"channel_id": channelID, "num_posts": "5000"},
	})
	if err == nil || !strings.Contains(err.Error(), "from 1 to 200") {
		t.Errorf("want num_posts refused above 200, got %v", err)
	}
}
