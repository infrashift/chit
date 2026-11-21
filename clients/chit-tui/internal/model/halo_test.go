package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// A post as the server sends it for an agent that posted for a person, with
// provenance and two files.
const haloPostJSON = `{
  "id": "p1", "channel_id": "c1", "user_id": "person", "content": "",
  "file_ids": ["f1", "f2"],
  "props": {
    "on_behalf_of": {"actor_id": "agent"},
    "provenance": {
      "model": "claude-opus-5-5",
      "tool_ids": ["t1", "t2"],
      "okf_namespace_ids": ["n1"],
      "okf_blob_sha256": "abc",
      "attachment_ids": ["f1", 7]
    }
  }
}`

func TestPost_DecodesTheHALOFields(t *testing.T) {
	var p model.Post
	if err := json.Unmarshal([]byte(haloPostJSON), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.FileIDs) != 2 || p.FileIDs[1] != "f2" {
		t.Errorf("FileIDs = %v, want [f1 f2]", p.FileIDs)
	}
	if got := p.OnBehalfOf(); got != "agent" {
		t.Errorf("OnBehalfOf = %q, want agent", got)
	}
	pv, ok := p.Provenance()
	if !ok {
		t.Fatal("Provenance: not found")
	}
	if pv.Model != "claude-opus-5-5" || len(pv.ToolIDs) != 2 || len(pv.OKFNamespaceIDs) != 1 || pv.OKFBlobSHA256 != "abc" {
		t.Errorf("Provenance = %+v", pv)
	}
	// A value of the wrong type in a list is skipped, not fatal.
	if len(pv.AttachmentIDs) != 1 || pv.AttachmentIDs[0] != "f1" {
		t.Errorf("AttachmentIDs = %v, want [f1]", pv.AttachmentIDs)
	}
}

func TestPost_NoHALOProps(t *testing.T) {
	cases := map[string]map[string]any{
		"no props":           nil,
		"actor not a string": {"on_behalf_of": map[string]any{"actor_id": 1}, "provenance": "x"},
		"not objects":        {"on_behalf_of": "agent", "provenance": []any{}},
	}
	for name, props := range cases {
		t.Run(name, func(t *testing.T) {
			p := model.Post{ID: "p", Props: props}
			if got := p.OnBehalfOf(); got != "" {
				t.Errorf("OnBehalfOf = %q, want empty", got)
			}
			if pv, ok := p.Provenance(); ok || pv != nil {
				t.Errorf("Provenance = %+v, %v; want none", pv, ok)
			}
		})
	}
	p := model.Post{Props: map[string]any{"provenance": map[string]any{"tool_ids": "t1"}}}
	pv, ok := p.Provenance()
	if !ok || pv.ToolIDs != nil {
		t.Errorf("a tool_ids that is not a list: got %+v, %v", pv, ok)
	}
}

func TestPost_FileIDsOmittedWhenEmpty(t *testing.T) {
	data, err := json.Marshal(model.Post{ChannelID: "c1", Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "file_ids") {
		t.Errorf("a post with no files sent file_ids: %s", data)
	}
}

func TestUser_IsMachine(t *testing.T) {
	for _, tc := range []struct {
		actor string
		want  bool
	}{{model.ActorTypeAgent, true}, {model.ActorTypeBot, true}, {model.ActorTypeUser, false}, {"", false}} {
		u := &model.User{ActorType: tc.actor}
		if got := u.IsMachine(); got != tc.want {
			t.Errorf("IsMachine(%q) = %v, want %v", tc.actor, got, tc.want)
		}
	}
	var nilUser *model.User
	if nilUser.IsMachine() {
		t.Error("a nil user is a machine")
	}
	var u model.User
	if err := json.Unmarshal([]byte(`{"id":"a","actor_type":"agent"}`), &u); err != nil || !u.IsMachine() {
		t.Errorf("actor_type did not decode: %+v, %v", u, err)
	}
}

func TestFile_Decode(t *testing.T) {
	var f model.File
	in := `{"id":"f1","channel_id":"c1","uploader_id":"u1","via_actor_id":"a1","name":"x.png","sha256":"s","content_type":"image/png","size_bytes":120,"width":4,"height":3,"alt_text":"alt","post_id":"p1","create_at":5}`
	if err := json.Unmarshal([]byte(in), &f); err != nil {
		t.Fatal(err)
	}
	want := model.File{ID: "f1", ChannelID: "c1", UploaderID: "u1", ViaActorID: "a1", Name: "x.png", SHA256: "s", ContentType: "image/png", SizeBytes: 120, Width: 4, Height: 3, AltText: "alt", PostID: "p1", CreateAt: 5}
	if f != want {
		t.Errorf("File = %+v, want %+v", f, want)
	}
}

func TestValidTagName(t *testing.T) {
	good := []string{"docs", "Résumé", "release 1.2", "a_b-c", "日本語", "  trimmed  ", "ｒésumé", strings.Repeat("x", 50)}
	for _, n := range good {
		if err := model.ValidTagName(n); err != nil {
			t.Errorf("ValidTagName(%q) = %v, want nil", n, err)
		}
	}
	bad := map[string]string{
		"":                      "1 to 50",
		"   ":                   "1 to 50",
		strings.Repeat("x", 51): "1 to 50",
		"a/b":                   "only letters",
		"emoji😀":                "only letters",
		"#hash":                 "only letters",
		"profile/x":             "reserved",
		"PROFILE/x":             "reserved",
	}
	for n, want := range bad {
		err := model.ValidTagName(n)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidTagName(%q) = %v, want an error with %q", n, err, want)
		}
	}
}

func TestNormalizeTagName(t *testing.T) {
	if got := model.NormalizeTagName("  ｆｕｌｌ  "); got != "full" {
		t.Errorf("NormalizeTagName = %q, want full", got)
	}
}

func TestTag_DecodesTimestampsAndTaggedChannel(t *testing.T) {
	var tc model.TaggedChannel
	in := `{"channel":{"id":"c1","name":"town"},"tags":[{"id":"t1","name":"x","scope":"channel","channel_id":"c1","create_at":1,"update_at":2}]}`
	if err := json.Unmarshal([]byte(in), &tc); err != nil {
		t.Fatal(err)
	}
	if tc.Channel.ID != "c1" || len(tc.Tags) != 1 || tc.Tags[0].CreateAt != 1 || tc.Tags[0].UpdateAt != 2 {
		t.Errorf("TaggedChannel = %+v", tc)
	}
}
