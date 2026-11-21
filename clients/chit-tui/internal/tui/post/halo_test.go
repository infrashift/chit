package post_test

import (
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/post"
)

func agentPost() *model.Post {
	return &model.Post{
		ID: "p1", UserID: "person", Content: "here is the chart",
		FileIDs: []string{"f1aaaaaaaaaa", "f2bbbbbbbbbb"},
		Props: map[string]any{
			model.PropOnBehalfOf: map[string]any{"actor_id": "halod"},
			model.PropProvenance: map[string]any{"model": "m"},
		},
	}
}

func TestPost_HALOBadgesAndFiles(t *testing.T) {
	dir := post.NewDirectory()
	dir.Names = map[string]string{"person": "erin", "halod": "halod"}
	dir.Kinds = map[string]string{"person": model.ActorTypeAgent}
	dir.Files = map[string]*model.File{"f1aaaaaaaaaa": {ID: "f1aaaaaaaaaa", Name: "chart.png", ContentType: "image/png", Width: 4, Height: 3, SizeBytes: 2048, AltText: "a chart"}}
	p := agentPost()
	v := testutil.StripANSI(post.New(p, dir.Username(p.UserID), testutil.Styles(), 100, nil, "", 0, nil).With(dir.Extra(p)).View())
	for _, want := range []string{
		"erin [agent]", "[via @halod]", "[provenance]", "here is the chart",
		`[file] chart.png  image/png  4×3  2.0 KB  "a chart"`, "[file f2bbbbbb]",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("View lacks %q:\n%s", want, v)
		}
	}
}

// A post with files and no text has no empty body line.
func TestPost_AttachmentOnly(t *testing.T) {
	p := &model.Post{ID: "p1", UserID: "u1", FileIDs: []string{"f1"}}
	lines := strings.Split(testutil.StripANSI(post.New(p, "alice", testutil.Styles(), 80, nil, "", 0, nil).View()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "[file f1]") {
		t.Errorf("lines = %q, want the header then the file", lines)
	}
	plain := &model.Post{ID: "p2", UserID: "u1"}
	if n := len(strings.Split(post.New(plain, "alice", testutil.Styles(), 80, nil, "", 0, nil).View(), "\n")); n != 2 {
		t.Errorf("an empty post without files has %d lines, want 2", n)
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KB", 200 << 10: "200 KB", 3 << 20: "3.0 MB", 12 << 20: "12 MB"}
	for n, want := range cases {
		if got := post.HumanSize(n); got != want {
			t.Errorf("HumanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDirectory_ShowsChangesWithWhatThePostShows(t *testing.T) {
	p := agentPost()
	a := post.NewDirectory()
	b := post.NewDirectory()
	if a.Shows(p) != b.Shows(p) {
		t.Fatal("two empty directories differ")
	}
	b.Names = map[string]string{"halod": "halo-agent"}
	if a.Shows(p) == b.Shows(p) {
		t.Error("the name of the actor changed and Shows did not")
	}
	c := post.NewDirectory()
	c.Files = map[string]*model.File{"f2bbbbbbbbbb": {ID: "f2bbbbbbbbbb"}}
	if a.Shows(p) == c.Shows(p) {
		t.Error("a file record arrived and Shows did not change")
	}
	d := post.NewDirectory()
	d.Kinds = map[string]string{"person": model.ActorTypeBot}
	if a.Shows(p) == d.Shows(p) {
		t.Error("the kind of the author changed and Shows did not")
	}
	if a.Username("0123456789") != "01234567" {
		t.Errorf("Username of an unknown ID = %q", a.Username("0123456789"))
	}
	if e := a.Extra(&model.Post{UserID: "x"}); e.Files != nil || e.Via != "" {
		t.Errorf("Extra of a plain post = %+v", e)
	}
}
