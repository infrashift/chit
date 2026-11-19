package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/blob"
	"github.com/infrashift/chit/internal/model"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// AuthExtract already refuses a client that is not on the allowlist; the
// app refuses it too, so no other caller of UploadFile can skip the rule.
func TestUploadFile_RefusesAClientNotOnTheAllowlist(t *testing.T) {
	f := newFixture(t)
	blobs := blob.NewMemory()
	f.app.Blobs = blobs
	person := f.user("person")
	f.store.Channels.SeedMember(&model.ChannelMember{ChannelID: f.channel.ID, UserID: person.ID})
	bot := f.user("bot")
	bot.ActorType, bot.OAuthClientID = model.ActorTypeBot, "not-allowlisted"
	f.app.Config.OnBehalfOfClients = []string{"halod"}

	up := &FileUpload{ChannelID: f.channel.ID, Name: "a.png", DeclaredType: "image/png", Data: tinyPNG(t)}
	_, err := f.app.UploadFile(t.Context(), up, person, bot)
	if !isStatus(err, http.StatusForbidden) {
		t.Fatalf("got %v, want 403", err)
	}
	if len(blobs.Keys()) != 0 {
		t.Fatal("a refused upload was stored")
	}
	// As the person, with no client, it is fine.
	if _, err := f.app.UploadFile(t.Context(), up, person, nil); err != nil {
		t.Fatal(err)
	}
}

func isStatus(err error, status int) bool {
	var appErr *model.AppError
	return errors.As(err, &appErr) && appErr.StatusCode == status
}

// The app refuses a fifth attachment before it looks any file up.
func TestCreatePost_AtMostFourFiles(t *testing.T) {
	f := newFixture(t)
	f.app.Blobs = blob.NewMemory()
	person := f.user("person")
	f.store.Channels.SeedMember(&model.ChannelMember{ChannelID: f.channel.ID, UserID: person.ID})
	ids := []string{model.NewID(), model.NewID(), model.NewID(), model.NewID(), model.NewID()}
	_, err := f.app.CreatePost(t.Context(), &model.Post{ChannelID: f.channel.ID, UserID: person.ID, Content: "x", FileIDs: ids})
	if !isStatus(err, http.StatusBadRequest) || !strings.Contains(err.Error(), "at most 4") {
		t.Fatalf("got %v, want 400 for more than 4", err)
	}
}
