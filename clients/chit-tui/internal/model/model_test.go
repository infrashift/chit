package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

func TestUser_JSONRoundTrip(t *testing.T) {
	u := model.User{
		ID:          "01234567-89ab-7def-0123-456789abcdef",
		KratosID:    "kratos-id",
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       "system_user",
		CreateAt:    1700000000000,
		UpdateAt:    1700000000000,
		DeleteAt:    0,
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var got model.User
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != u {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, u)
	}
}

func TestTeam_JSONRoundTrip(t *testing.T) {
	tm := model.Team{
		ID:          "01234567-89ab-7def-0123-456789abcdef",
		Name:        "eng",
		DisplayName: "Engineering",
		Description: "Engineering team",
		Type:        model.TeamOpen,
		CreatorID:   "creator-id",
		CreateAt:    1700000000000,
		UpdateAt:    1700000000000,
	}
	data, err := json.Marshal(tm)
	if err != nil {
		t.Fatal(err)
	}
	var got model.Team
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != tm {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, tm)
	}
}

func TestChannel_JSONRoundTrip(t *testing.T) {
	ch := model.Channel{
		ID:            "ch-id",
		TeamID:        "team-id",
		CreatorID:     "creator-id",
		Name:          "general",
		DisplayName:   "General",
		Header:        "Welcome",
		Purpose:       "General discussion",
		Type:          model.ChannelOpen,
		TotalMsgCount: 42,
		LastPostAt:    1700000000000,
		CreateAt:      1700000000000,
		UpdateAt:      1700000000000,
	}
	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatal(err)
	}
	var got model.Channel
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != ch {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, ch)
	}
}

func TestChannel_OmitEmptyTeamID(t *testing.T) {
	ch := model.Channel{ID: "ch-id", Type: model.ChannelDirect}
	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatal(err)
	}
	raw := make(map[string]any)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["team_id"]; ok {
		t.Error("expected team_id to be omitted for direct channel")
	}
}

func TestPost_JSONRoundTrip(t *testing.T) {
	p := model.Post{
		ID:        "post-id",
		ChannelID: "ch-id",
		UserID:    "user-id",
		Content:   "Hello **world**",
		IsPinned:  true,
		CreateAt:  1700000000000,
		UpdateAt:  1700000000000,
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got model.Post
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID || got.Content != p.Content || got.IsPinned != p.IsPinned {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, p)
	}
}

func TestPost_OmitEmptyOptionals(t *testing.T) {
	p := model.Post{ID: "post-id", ChannelID: "ch-id", UserID: "user-id", Content: "hi"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	raw := make(map[string]any)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"root_id", "type", "props", "hashtags"} {
		if _, ok := raw[field]; ok {
			t.Errorf("expected %s to be omitted", field)
		}
	}
}

func TestThread_JSONRoundTrip(t *testing.T) {
	th := model.Thread{
		PostID:       "post-id",
		ChannelID:    "ch-id",
		ReplyCount:   5,
		LastReplyAt:  1700000000000,
		Participants: []string{"user-1", "user-2"},
	}
	data, err := json.Marshal(th)
	if err != nil {
		t.Fatal(err)
	}
	var got model.Thread
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.PostID != th.PostID || got.ReplyCount != th.ReplyCount || len(got.Participants) != 2 {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, th)
	}
}

func TestWebSocketEvent_JSONRoundTrip(t *testing.T) {
	evt := model.WebSocketEvent{
		Event: model.WebSocketEventPosted,
		Data:  map[string]any{"post": "data"},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: "ch-id",
		},
		Sequence: 42,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var got model.WebSocketEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Event != evt.Event || got.Sequence != evt.Sequence {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, evt)
	}
	if got.Broadcast.ChannelID != "ch-id" {
		t.Errorf("broadcast channel_id mismatch: got %s", got.Broadcast.ChannelID)
	}
}

func TestAppError_JSONRoundTrip(t *testing.T) {
	ae := model.AppError{
		ID:         "test.error",
		Message:    "something went wrong",
		StatusCode: 400,
	}
	data, err := json.Marshal(ae)
	if err != nil {
		t.Fatal(err)
	}
	var got model.AppError
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != ae {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, ae)
	}
}

func TestAppError_OmitDetailedError(t *testing.T) {
	ae := model.AppError{ID: "test", Message: "err", StatusCode: 500}
	data, err := json.Marshal(ae)
	if err != nil {
		t.Fatal(err)
	}
	raw := make(map[string]any)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["detailed_error"]; ok {
		t.Error("expected detailed_error to be omitted when empty")
	}
}

func TestMillisToTime(t *testing.T) {
	millis := int64(1700000000000)
	got := model.MillisToTime(millis)
	want := time.UnixMilli(millis)
	if !got.Equal(want) {
		t.Errorf("MillisToTime(%d) = %v, want %v", millis, got, want)
	}
}

func TestCommand_JSONRoundTrip(t *testing.T) {
	cmd := model.Command{
		ID:          "cmd-id",
		Slug:        "remind",
		Description: "Set a reminder",
		Category:    "workflow",
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	var got model.Command
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != cmd {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, cmd)
	}
}

func TestTag_JSONRoundTrip(t *testing.T) {
	tag := model.Tag{ID: "tag-id", Name: "urgent"}
	data, err := json.Marshal(tag)
	if err != nil {
		t.Fatal(err)
	}
	var got model.Tag
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != tag {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, tag)
	}
}

func TestPostList_JSONRoundTrip(t *testing.T) {
	pl := model.PostList{
		Order: []*model.Post{
			{ID: "p1", Content: "first"},
			{ID: "p2", Content: "second"},
		},
	}
	data, err := json.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	var got model.PostList
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Order) != 2 || got.Order[0].ID != "p1" || got.Order[1].ID != "p2" {
		t.Errorf("round trip mismatch: got %d posts", len(got.Order))
	}
}

func TestThreadResponse_JSONRoundTrip(t *testing.T) {
	tr := model.ThreadResponse{
		Thread: &model.Thread{PostID: "root", ChannelID: "ch", ReplyCount: 1},
		Posts:  []*model.Post{{ID: "root", Content: "root post"}, {ID: "reply", Content: "reply", RootID: "root"}},
	}
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var got model.ThreadResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Thread.PostID != "root" || len(got.Posts) != 2 {
		t.Errorf("round trip mismatch")
	}
}
