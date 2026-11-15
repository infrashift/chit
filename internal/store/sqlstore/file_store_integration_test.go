//go:build integration

package sqlstore

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func saveFile(t *testing.T, ss *SqlStore, channelID, uploaderID, sha, name, alt string, createAt int64) *model.File {
	t.Helper()
	id := model.NewID()
	f := &model.File{
		ID: id, ChannelID: channelID, UploaderID: uploaderID, Name: name, SHA256: sha,
		ContentType: "image/png", SizeBytes: 100, Width: 2, Height: 3, AltText: alt,
		S3Key: model.FileObjectKey(channelID, id), CreateAt: createAt,
	}
	if _, err := ss.File().Save(t.Context(), f); err != nil {
		t.Fatalf("save file: %v", err)
	}
	return f
}

func sha(c byte) string { return strings.Repeat(string(c), 64) }

func wantStatus(t *testing.T, err error, status int) {
	t.Helper()
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != status {
		t.Fatalf("got %v, want HTTP %d", err, status)
	}
}

func TestFileStoreIntegration_SaveGetAttachDelete(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("other"))
	if err != nil {
		t.Fatal(err)
	}
	a := saveFile(t, ss, channel.ID, user.ID, sha('a'), "a.png", "first", 1000)
	b := saveFile(t, ss, channel.ID, user.ID, sha('b'), "b.png", "second", 1001)
	theirs := saveFile(t, ss, channel.ID, other.ID, sha('c'), "c.png", "", 1002)

	got, err := ss.File().Get(t.Context(), a.ID)
	if err != nil || got.Name != "a.png" || got.AltText != "first" || got.PostID != "" || got.S3Key != a.S3Key || got.ViaActorID != "" {
		t.Fatalf("Get: %+v %v", got, err)
	}
	if _, err = ss.File().Get(t.Context(), "not-a-uuid"); err == nil {
		t.Fatal("a malformed id was found")
	}

	// Attach in the order given; somebody else's file fails the whole post.
	_, err = ss.Post().Save(t.Context(), &model.Post{ChannelID: channel.ID, UserID: user.ID, FileIDs: []string{b.ID, theirs.ID}})
	wantStatus(t, err, http.StatusConflict)
	post, err := ss.Post().Save(t.Context(), &model.Post{ChannelID: channel.ID, UserID: user.ID, FileIDs: []string{b.ID, a.ID}})
	if err != nil {
		t.Fatalf("save post with files: %v", err)
	}
	read, err := ss.Post().Get(t.Context(), post.ID)
	if err != nil || !slices.Equal(read.FileIDs, []string{b.ID, a.ID}) {
		t.Fatalf("post file_ids %v, %v", read.FileIDs, err)
	}
	list, _ := ss.Post().GetPostsForChannel(t.Context(), channel.ID, model.GetPostsOptions{PerPage: 10})
	if len(list.Order) != 1 || len(list.Order[0].FileIDs) != 2 {
		t.Fatalf("channel posts: %+v", list.Order)
	}
	if got, _ := ss.File().Get(t.Context(), a.ID); got.PostID != post.ID {
		t.Fatalf("PostID %q", got.PostID)
	}
	// A file attaches once.
	_, err = ss.Post().Save(t.Context(), &model.Post{ChannelID: channel.ID, UserID: user.ID, Content: "x", FileIDs: []string{a.ID}})
	wantStatus(t, err, http.StatusConflict)
	// A post without files reads back with none.
	plain := savePost(t, ss, channel.ID, user.ID, "", "plain", 5000)
	if p, _ := ss.Post().Get(t.Context(), plain.ID); p.FileIDs != nil {
		t.Fatalf("plain post file_ids %v", p.FileIDs)
	}

	// Deleting the post deletes and unlinks its files.
	if err = ss.Post().Delete(t.Context(), post.ID, 7000); err != nil {
		t.Fatal(err)
	}
	for _, f := range []*model.File{a, b} {
		if _, getErr := ss.File().Get(t.Context(), f.ID); getErr == nil {
			t.Fatalf("file %s of a deleted post is still live", f.Name)
		}
	}
	deleted, err := ss.File().Deleted(t.Context(), 7001, 10)
	if err != nil || len(deleted) != 2 || deleted[0].PostID != "" {
		t.Fatalf("Deleted: %+v %v", deleted, err)
	}
}

func TestFileStoreIntegration_DedupLookupIsPerChannel(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.Channel().Save(t.Context(), &model.Channel{TeamID: channel.TeamID, Name: "other",
		DisplayName: "Other", Type: model.ChannelOpen, CreatorID: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	first := saveFile(t, ss, channel.ID, user.ID, sha('d'), "d.png", "", 1000)
	saveFile(t, ss, channel.ID, user.ID, sha('d'), "d2.png", "", 2000)

	got, err := ss.File().GetBySHA256(t.Context(), channel.ID, sha('d'))
	if err != nil || got.ID != first.ID {
		t.Fatalf("GetBySHA256 = %+v %v, want the oldest", got, err)
	}
	_, err = ss.File().GetBySHA256(t.Context(), other.ID, sha('d'))
	wantStatus(t, err, http.StatusNotFound)
}

func TestFileStoreIntegration_UsageAndSearch(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("other"))
	if err != nil {
		t.Fatal(err)
	}
	wb := saveFile(t, ss, channel.ID, user.ID, sha('e'), "Whiteboard.png", "", 1000)
	alt := saveFile(t, ss, channel.ID, user.ID, sha('f'), "x.png", "the 100%_whiteboard", 2000)
	theirs := saveFile(t, ss, channel.ID, other.ID, sha('g'), "whiteboard3.png", "", 3000)

	if n, err := ss.File().UsageBytes(t.Context(), user.ID); err != nil || n != 200 {
		t.Fatalf("usage %d %v", n, err)
	}
	search := func(terms, reader string) []string {
		t.Helper()
		files, err := ss.File().Search(t.Context(), &model.FileSearch{Terms: terms, ChannelIDs: []string{channel.ID}, ReaderID: reader, PerPage: 10})
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, f := range files {
			ids = append(ids, f.ID)
		}
		return ids
	}
	// Newest first; theirs is unattached, so only its uploader finds it.
	if got := search("WHITEBOARD", user.ID); !slices.Equal(got, []string{alt.ID, wb.ID}) {
		t.Fatalf("search: %v", got)
	}
	if got := search("whiteboard", other.ID); !slices.Equal(got, []string{theirs.ID}) {
		t.Fatalf("search as other: %v", got)
	}
	// % and _ are literal.
	if got := search("100%_", user.ID); !slices.Equal(got, []string{alt.ID}) {
		t.Fatalf("literal wildcard: %v", got)
	}
	if got := search("1000", user.ID); len(got) != 0 {
		t.Fatalf("%% matched as a wildcard: %v", got)
	}
	// Attached, every reader finds it.
	if _, err := ss.Post().Save(t.Context(), &model.Post{ChannelID: channel.ID, UserID: other.ID, FileIDs: []string{theirs.ID}}); err != nil {
		t.Fatal(err)
	}
	if got := search("whiteboard3", user.ID); !slices.Equal(got, []string{theirs.ID}) {
		t.Fatalf("attached: %v", got)
	}
}

func TestFileStoreIntegration_Sweep(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	old := saveFile(t, ss, channel.ID, user.ID, sha('h'), "old.png", "", 1000)
	attached := saveFile(t, ss, channel.ID, user.ID, sha('i'), "att.png", "", 1000)
	fresh := saveFile(t, ss, channel.ID, user.ID, sha('j'), "new.png", "", 9000)
	// A deduplicated copy that shares old's object.
	copyOf := &model.File{ID: model.NewID(), ChannelID: channel.ID, UploaderID: user.ID, Name: "copy.png", SHA256: sha('h'),
		ContentType: "image/png", SizeBytes: 100, Width: 2, Height: 3, S3Key: old.S3Key, CreateAt: 9000}
	if _, err := ss.File().Save(t.Context(), copyOf); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.Post().Save(t.Context(), &model.Post{ChannelID: channel.ID, UserID: user.ID, FileIDs: []string{attached.ID}}); err != nil {
		t.Fatal(err)
	}

	n, err := ss.File().ExpireUnattached(t.Context(), 5000, 6000, 10)
	if err != nil || n != 1 {
		t.Fatalf("expired %d, %v; want only the old unattached one", n, err)
	}
	if _, err = ss.File().Get(t.Context(), fresh.ID); err != nil {
		t.Fatal("a fresh upload was expired")
	}
	deleted, err := ss.File().Deleted(t.Context(), 6001, 10)
	if err != nil || len(deleted) != 1 || deleted[0].ID != old.ID {
		t.Fatalf("Deleted %+v %v", deleted, err)
	}
	if inUse, err := ss.File().ObjectInUse(t.Context(), old.S3Key); err != nil || !inUse {
		t.Fatalf("the copy's object is not in use: %v %v", inUse, err)
	}
	if err := ss.File().Purge(t.Context(), old.ID); err != nil {
		t.Fatal(err)
	}
	if deleted, _ := ss.File().Deleted(t.Context(), 6001, 10); len(deleted) != 0 {
		t.Fatalf("purged row still listed: %+v", deleted)
	}
	// Purge leaves a live file alone.
	if err := ss.File().Purge(t.Context(), fresh.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.File().Get(t.Context(), fresh.ID); err != nil {
		t.Fatal("Purge removed a live file")
	}
}
