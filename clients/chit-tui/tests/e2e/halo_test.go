//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// These tests cover the HALO contract against a real chitd: actor types,
// provenance, on-behalf-of posts, files, scoped tags, channel tags,
// stream_status, 409 and 429. Each WebSocket test checks the payload
// shape that the TUI handlers read.

func unique(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func mustMe(t *testing.T, c api.ChitClient) *model.User {
	t.Helper()
	u, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	return u
}

// connectWS connects a WebSocket client for kratosID and closes it at the
// end of the test.
func connectWS(t *testing.T, kratosID string) ws.WSClient {
	t.Helper()
	c := newTestWSClient(kratosID, 256)
	if err := c.Connect(); err != nil {
		t.Fatalf("WS Connect: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Logf("WS Close: %v", err)
		}
	})
	return c
}

// waitEvent returns the first event named name for which match is true.
func waitEvent(t *testing.T, c ws.WSClient, name string, match func(model.WebSocketEvent) bool) model.WebSocketEvent {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case evt := <-c.Events():
			if evt.Event == name && match(evt) {
				return evt
			}
		case <-timeout:
			t.Fatalf("timeout: no %s event", name)
			return model.WebSocketEvent{}
		}
	}
}

func dataString(evt model.WebSocketEvent, key string) string {
	s, ok := evt.Data[key].(string)
	if !ok {
		return ""
	}
	return s
}

// rawDo sends one JSON request with the given headers, outside the TUI
// client, for what the TUI never sends itself.
func rawDo(t *testing.T, method, path string, headers map[string]string, body any) (int, []byte) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, serverURL()+"/api/v1"+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("close body: %v", err)
		}
	}()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, out
}

// pngBytes returns a small PNG image.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for x := range 4 {
		img.Set(x, 1, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestHALO_ActorTypes(t *testing.T) {
	alice := newTestClient(aliceKratosID)
	if me := mustMe(t, alice); me.ActorType != model.ActorTypeUser || me.IsMachine() {
		t.Errorf("alice: actor_type = %q", me.ActorType)
	}
	agent := mustMe(t, newTestClient(agentKratosID))
	users, err := alice.GetUsersByIDs(context.Background(), []string{agent.ID})
	if err != nil || len(users) != 1 {
		t.Fatalf("GetUsersByIDs = %v, %v", users, err)
	}
	if users[0].ActorType != model.ActorTypeAgent || !users[0].IsMachine() {
		t.Errorf("chit-agent: actor_type = %q, want agent", users[0].ActorType)
	}
}

func TestHALO_AgentPostWithProvenance(t *testing.T) {
	ctx := context.Background()
	agent := newTestClient(agentKratosID)
	channelID := mustGetTownSquareID(t, newTestClient(aliceKratosID))
	toolID := "0193b2a4-0000-7000-8000-000000000001"
	created, err := agent.CreatePost(ctx, &model.Post{
		ChannelID: channelID,
		Content:   unique("agent reply"),
		Props: map[string]any{model.PropProvenance: map[string]any{
			"model": "claude-opus-5-5", "tool_ids": []string{toolID},
		}},
	})
	if err != nil {
		t.Fatalf("CreatePost (agent): %v", err)
	}
	got, err := newTestClient(aliceKratosID).GetPost(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	pv, ok := got.Provenance()
	if !ok || pv.Model != "claude-opus-5-5" || len(pv.ToolIDs) != 1 || pv.ToolIDs[0] != toolID {
		t.Errorf("Provenance = %+v, %v", pv, ok)
	}
}

func TestHALO_PersonCannotSetProvenance(t *testing.T) {
	channelID := mustGetTownSquareID(t, newTestClient(aliceKratosID))
	_, err := newTestClient(bobKratosID).CreatePost(context.Background(), &model.Post{
		ChannelID: channelID, Content: "x",
		Props: map[string]any{model.PropProvenance: map[string]any{"model": "m"}},
	})
	if api.StatusOf(err) != http.StatusForbidden {
		t.Errorf("a person set provenance: err = %v, want a 403", err)
	}
}

func TestHALO_OnBehalfOfPost(t *testing.T) {
	alice := newTestClient(aliceKratosID)
	me := mustMe(t, alice)
	channelID := mustGetTownSquareID(t, alice)
	status, body := rawDo(t, http.MethodPost, "/posts", map[string]string{
		"X-Client-Id": halodClientID, "X-On-Behalf-Of": me.ID,
	}, map[string]string{"channel_id": channelID, "content": unique("for alice")})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("on-behalf-of post: %d %s", status, body)
	}
	var created model.Post
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	got, err := alice.GetPost(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if got.UserID != me.ID {
		t.Errorf("UserID = %q, want alice %q", got.UserID, me.ID)
	}
	actorID := got.OnBehalfOf()
	if actorID == "" || actorID == me.ID {
		t.Fatalf("OnBehalfOf = %q, want the halod user", actorID)
	}
	actors, err := alice.GetUsersByIDs(context.Background(), []string{actorID})
	if err != nil || len(actors) != 1 || actors[0].ActorType != model.ActorTypeAgent {
		t.Errorf("the actor = %v, %v; want an agent", actors, err)
	}
}

func TestHALO_FileUploadPostAndDownload(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	bob := newTestClient(bobKratosID)
	channelID := mustGetTownSquareID(t, alice)

	f, err := alice.UploadFile(ctx, channelID, "red.png", "image/png", "a red line", bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if f.ID == "" || f.ContentType != "image/png" || f.Width != 4 || f.Height != 3 || f.AltText != "a red line" || f.PostID != "" {
		t.Errorf("File = %+v", f)
	}
	// An attachment-only post: the content may be empty.
	post, err := alice.CreatePost(ctx, &model.Post{ChannelID: channelID, FileIDs: []string{f.ID}})
	if err != nil {
		t.Fatalf("CreatePost with a file: %v", err)
	}
	got, err := bob.GetPost(ctx, post.ID)
	if err != nil || len(got.FileIDs) != 1 || got.FileIDs[0] != f.ID || got.Content != "" {
		t.Fatalf("GetPost = %+v, %v", got, err)
	}
	info, err := bob.GetFileInfo(ctx, f.ID)
	if err != nil || info.PostID != post.ID {
		t.Fatalf("GetFileInfo = %+v, %v", info, err)
	}
	var buf bytes.Buffer
	n, err := bob.DownloadFile(ctx, f.ID, &buf, model.MaxFileBytes)
	if err != nil || n != info.SizeBytes {
		t.Fatalf("DownloadFile = %d, %v; want %d bytes", n, err, info.SizeBytes)
	}
	sum := sha256.Sum256(buf.Bytes())
	if hex.EncodeToString(sum[:]) != info.SHA256 {
		t.Error("the downloaded bytes do not match the sha256 of the file record")
	}
}

func TestHALO_FileRefusals(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	channelID := mustGetTownSquareID(t, alice)
	_, err := alice.UploadFile(ctx, channelID, "x.svg", "image/svg+xml", "", strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if msg := api.UserMessage(err); err == nil || !strings.Contains(msg, "only PNG") {
		t.Errorf("an SVG upload: %v (%q)", err, msg)
	}
	// Before it is attached, only the uploader can read a file.
	f, err := alice.UploadFile(ctx, channelID, "p.png", "image/png", "", bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if _, err := newTestClient(bobKratosID).GetFileInfo(ctx, f.ID); api.StatusOf(err) != http.StatusForbidden {
		t.Errorf("bob read an unattached file: %v", err)
	}
}

func TestHALO_TagRenameAndDeleteEvents(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	bob := newTestClient(bobKratosID)
	channelID := mustGetTownSquareID(t, alice)
	bobWS := connectWS(t, bobKratosID)

	tag, err := alice.CreateTag(ctx, unique("e2e"), model.TagScopeChannel, channelID)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if tag.Scope != model.TagScopeChannel || tag.ChannelID != channelID || tag.OwnerID == "" || tag.CreateAt == 0 {
		t.Errorf("Tag = %+v", tag)
	}
	visible, err := bob.GetTags(ctx, channelID)
	if err != nil || !hasTag(visible, tag.ID) {
		t.Errorf("bob cannot see the channel tag: %v, %v", visible, err)
	}

	newName := unique("renamed")
	renamed, err := alice.RenameTag(ctx, tag.ID, newName)
	if err != nil || renamed.Name != newName {
		t.Fatalf("RenameTag = %+v, %v", renamed, err)
	}
	evt := waitEvent(t, bobWS, model.WebSocketEventTagUpdated, func(e model.WebSocketEvent) bool {
		obj, ok := e.Data["tag"].(map[string]any)
		return ok && obj["id"] == tag.ID
	})
	if obj, ok := evt.Data["tag"].(map[string]any); !ok || obj["name"] != newName {
		t.Errorf("tag_updated data = %v", evt.Data)
	}
	if _, err := bob.RenameTag(ctx, tag.ID, unique("bob")); err == nil {
		t.Error("bob renamed alice's tag")
	}

	if err := alice.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	waitEvent(t, bobWS, model.WebSocketEventTagDeleted, func(e model.WebSocketEvent) bool {
		return dataString(e, "tag_id") == tag.ID
	})
}

func TestHALO_TagRenameConflict(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	name := unique("taken")
	if _, err := alice.CreateTag(ctx, name, model.TagScopePrivate, ""); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	other, err := alice.CreateTag(ctx, unique("other"), model.TagScopePrivate, "")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	_, err = alice.RenameTag(ctx, other.ID, strings.ToUpper(name))
	if api.StatusOf(err) != http.StatusConflict {
		t.Errorf("rename to a taken name: err = %v, want a 409", err)
	}
}

func TestHALO_PrivateTagStaysPrivate(t *testing.T) {
	ctx := context.Background()
	tag, err := newTestClient(aliceKratosID).CreateTag(ctx, unique("mine"), model.TagScopePrivate, "")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	all, err := newTestClient(bobKratosID).GetAllTags(ctx)
	if err != nil || hasTag(all, tag.ID) {
		t.Errorf("bob sees alice's private tag: %v", err)
	}
}

func TestHALO_ChannelTags(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	channelID := mustGetTownSquareID(t, alice)
	bobWS := connectWS(t, bobKratosID)

	tag, err := alice.CreateTag(ctx, unique("chan"), model.TagScopeChannel, channelID)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if err := alice.AddChannelTag(ctx, channelID, tag.ID); err != nil {
		t.Fatalf("AddChannelTag: %v", err)
	}
	evt := waitEvent(t, bobWS, model.WebSocketEventChannelTagsUpdated, func(e model.WebSocketEvent) bool {
		return dataString(e, "channel_id") == channelID
	})
	var tags []*model.Tag
	if raw, err := json.Marshal(evt.Data["tags"]); err != nil || json.Unmarshal(raw, &tags) != nil || !hasTag(tags, tag.ID) {
		t.Errorf("channel_tags_updated tags = %v", evt.Data["tags"])
	}
	ctags, err := newTestClient(bobKratosID).GetChannelTags(ctx, channelID)
	if err != nil || !hasTag(ctags, tag.ID) {
		t.Errorf("GetChannelTags = %v, %v", ctags, err)
	}
	found, err := alice.FindChannelsByTags(ctx, []string{tag.ID}, "")
	if err != nil || len(found) != 1 || found[0].Channel.ID != channelID {
		t.Errorf("FindChannelsByTags = %v, %v", found, err)
	}
	if err := alice.RemoveChannelTag(ctx, channelID, tag.ID); err != nil {
		t.Fatalf("RemoveChannelTag: %v", err)
	}
	if ctags, err := alice.GetChannelTags(ctx, channelID); err != nil || hasTag(ctags, tag.ID) {
		t.Errorf("after the remove: %v, %v", ctags, err)
	}
}

func hasTag(tags []*model.Tag, id string) bool {
	for _, t := range tags {
		if t.ID == id {
			return true
		}
	}
	return false
}

func TestHALO_StreamStatusEvent(t *testing.T) {
	alice := newTestClient(aliceKratosID)
	channelID := mustGetTownSquareID(t, alice)
	agent := mustMe(t, newTestClient(agentKratosID))
	aliceWS := connectWS(t, aliceKratosID)

	streamID := unique("s")
	for _, st := range []string{model.StreamStarted, model.StreamFinished} {
		status, body := rawDo(t, http.MethodPost, "/channels/"+channelID+"/stream_status",
			map[string]string{"X-User-Id": agentKratosID},
			map[string]string{"status": st, "stream_id": streamID})
		if status != http.StatusOK {
			t.Fatalf("stream_status %s: %d %s", st, status, body)
		}
		evt := waitEvent(t, aliceWS, model.WebSocketEventStreamStatus, func(e model.WebSocketEvent) bool {
			return dataString(e, "stream_id") == streamID && dataString(e, "status") == st
		})
		if dataString(evt, "channel_id") != channelID || dataString(evt, "user_id") != agent.ID {
			t.Errorf("stream_status data = %v", evt.Data)
		}
	}
}

func TestHALO_DuplicateChannelName(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	teams, err := alice.GetMyTeams(ctx)
	if err != nil || len(teams) == 0 {
		t.Fatalf("GetMyTeams = %v, %v", teams, err)
	}
	name := unique("dup")
	ch := &model.Channel{TeamID: teams[0].ID, Name: name, DisplayName: name, Type: "O"}
	if _, err := alice.CreateChannel(ctx, ch); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	_, err = alice.CreateChannel(ctx, ch)
	if api.StatusOf(err) != http.StatusConflict || !strings.Contains(api.UserMessage(err), "that name is in use") {
		t.Errorf("a taken name: %v (%q)", err, api.UserMessage(err))
	}
}

// TestHALO_RateLimit uses diana alone, so the empty bucket does not slow
// the other tests. It waits for the bucket to fill again at the end.
func TestHALO_RateLimit(t *testing.T) {
	diana := newTestClient(dianaKratosID)
	ctx := context.Background()
	var limited error
	for range 2000 {
		if _, err := diana.GetMe(ctx); err != nil {
			limited = err
			break
		}
	}
	wait, ok := api.RetryAfterOf(limited)
	if !ok {
		t.Fatalf("2000 requests gave no 429: last error %v", limited)
	}
	if wait <= 0 {
		t.Errorf("the 429 had no Retry-After: %v", limited)
	}
	if !strings.Contains(api.UserMessage(limited), "too many requests") {
		t.Errorf("UserMessage = %q", api.UserMessage(limited))
	}
	time.Sleep(wait + 5*time.Second)
}
