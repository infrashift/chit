package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/blob"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/pkg/imagecheck"
)

// Attachments (P-CHIT-5, HALO PLAN.md 3.6).
//
// A person uploads an image to a channel they are a member of. Chit checks
// and re-encodes it (package imagecheck), stores the bytes in the object
// store under "<channel_id>/<file_id>", and returns the file. They then
// attach it to one post of theirs in that channel by naming it in
// file_ids. Before that, only they can read it; after, every member of
// the channel can. Deleting the post deletes its files. The sweep
// (SweepFiles) deletes uploads that were never attached, and removes the
// objects of deleted files, each after Config.FileOrphanGrace.
//
// Identical bytes uploaded twice to one channel share one object: the
// second file gets its own row (its own ID, uploader and alt text) and the
// first one's object. Two channels never share an object.

// FileUpload is one uploaded image, as the client sent it.
type FileUpload struct {
	ChannelID    string
	Name         string
	DeclaredType string
	AltText      string
	Data         []byte
}

// Stable error IDs for upload refusals, so a client can tell them apart.
const (
	FileErrDisabled        = "file.disabled"
	FileErrTooLarge        = "file.too_large"
	FileErrUnsupportedType = "file.unsupported_type"
	FileErrSVG             = "file.svg"
	FileErrTypeMismatch    = "file.type_mismatch"
	FileErrPolyglot        = "file.polyglot"
	FileErrMalformed       = "file.malformed"
	FileErrTooManyPixels   = "file.too_many_pixels"
	FileErrQuota           = "file.quota_exceeded"
)

func fileError(id, msg string, status int) *model.AppError {
	return &model.AppError{ID: id, Where: "App.File", Message: msg, StatusCode: status}
}

// requireAttachments is a 503 when no object store is configured.
func (a *App) requireAttachments() error {
	if a.Blobs == nil {
		return fileError(FileErrDisabled, "attachments are not enabled on this server", http.StatusServiceUnavailable)
	}
	return nil
}

// imageRefusal maps an imagecheck refusal to its status and ID.
func imageRefusal(err error) error {
	cases := []struct {
		reason error
		id     string
		status int
	}{
		{imagecheck.ErrTooLarge, FileErrTooLarge, http.StatusRequestEntityTooLarge},
		{imagecheck.ErrSVG, FileErrSVG, http.StatusUnsupportedMediaType},
		{imagecheck.ErrUnsupportedType, FileErrUnsupportedType, http.StatusUnsupportedMediaType},
		{imagecheck.ErrTypeMismatch, FileErrTypeMismatch, http.StatusUnsupportedMediaType},
		{imagecheck.ErrPolyglot, FileErrPolyglot, http.StatusUnprocessableEntity},
		{imagecheck.ErrTooManyPixels, FileErrTooManyPixels, http.StatusUnprocessableEntity},
		{imagecheck.ErrMalformed, FileErrMalformed, http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		if errors.Is(err, c.reason) {
			return fileError(c.id, err.Error(), c.status)
		}
	}
	return err
}

// UploadFile checks, stores and records one image in up.ChannelID for
// uploader. actor is the allowlisted client uploading on the uploader's
// behalf, or nil.
func (a *App) UploadFile(ctx context.Context, up *FileUpload, uploader, actor *model.User) (*model.File, error) {
	assert.That(up != nil && uploader != nil, "App.UploadFile: nil upload or uploader")
	if err := a.checkUploader(ctx, up, uploader, actor); err != nil {
		return nil, err
	}

	img, err := imagecheck.Check(up.DeclaredType, up.Data, imagecheck.DefaultLimits())
	if err != nil {
		return nil, imageRefusal(err)
	}
	assert.That(img != nil && img.SHA256 != "", "App.UploadFile: checked image has no digest")
	if quotaErr := a.checkQuota(ctx, uploader.ID, len(img.Data)); quotaErr != nil {
		return nil, quotaErr
	}

	f := newFile(up, uploader, actor, img)
	stored, err := a.putObject(ctx, f, img)
	if err != nil {
		return nil, err
	}
	saved, err := a.Store.File().Save(ctx, f)
	if err != nil {
		if stored {
			// No row names the object, so the sweep would never find it.
			if delErr := a.Blobs.Delete(context.WithoutCancel(ctx), f.S3Key); delErr != nil {
				slog.Warn("file: could not remove the object of a failed upload", "key", f.S3Key, "error", delErr)
			}
		}
		return nil, err
	}
	assert.That(saved != nil, "App.UploadFile: nil file without an error")
	a.AuditOnBehalfOf(actor, "file_uploaded", uploader.ID, "file_id", saved.ID, "channel_id", saved.ChannelID)
	return saved, nil
}

// checkUploader refuses an upload before its bytes are looked at: with
// attachments off, from a client that may not act on behalf of a user, to a
// channel that does not exist or the uploader is not a member of, or with
// alt text that is too long.
func (a *App) checkUploader(ctx context.Context, up *FileUpload, uploader, actor *model.User) error {
	assert.That(a.Store != nil, "App.checkUploader: nil store")
	assert.That(up != nil && uploader != nil, "App.checkUploader: nil upload or uploader")
	if err := a.requireAttachments(); err != nil {
		return err
	}
	if actor != nil && !a.OnBehalfOfAllowed(actor) {
		return model.NewForbiddenError("App.UploadFile", "this client may not act on behalf of a user")
	}
	if _, err := a.Store.Channel().Get(ctx, up.ChannelID); err != nil {
		return err
	}
	if err := a.requireChannelMember(ctx, up.ChannelID, uploader.ID); err != nil {
		return err
	}
	if err := model.ValidateAltText(up.AltText); err != nil {
		return err
	}
	return nil
}

// checkQuota refuses an image of size bytes that would take uploaderID past
// Config.FileQuotaBytes.
func (a *App) checkQuota(ctx context.Context, uploaderID string, size int) error {
	assert.That(a.Store != nil && a.Config != nil, "App.checkQuota: nil store or config")
	assert.That(uploaderID != "" && size > 0, "App.checkQuota: no uploader or an empty image")
	used, err := a.Store.File().UsageBytes(ctx, uploaderID)
	if err != nil {
		return err
	}
	if quota := a.Config.FileQuotaBytes; used+int64(size) > quota {
		return fileError(FileErrQuota, fmt.Sprintf(
			"storage quota exceeded: %d of %d bytes used, and this image is %d", used, quota, size),
			http.StatusRequestEntityTooLarge)
	}
	return nil
}

// newFile is the record of the checked image img, before it has an object.
func newFile(up *FileUpload, uploader, actor *model.User, img *imagecheck.Image) *model.File {
	assert.That(up != nil && uploader != nil && img != nil, "newFile: nil upload, uploader or image")
	f := &model.File{
		ID:          model.NewID(),
		ChannelID:   up.ChannelID,
		UploaderID:  uploader.ID,
		Name:        model.CleanFileName(up.Name, img.Extension()),
		SHA256:      img.SHA256,
		ContentType: img.ContentType,
		SizeBytes:   int64(len(img.Data)),
		Width:       img.Width,
		Height:      img.Height,
		AltText:     up.AltText,
		CreateAt:    model.GetMillis(),
	}
	if actor != nil {
		f.ViaActorID = actor.ID
	}
	assert.That(f.ID != "" && f.CreateAt > 0, "newFile: no id or creation time")
	return f
}

// putObject gives f its object. Identical bytes are deduplicated within
// f's channel only: a file with the same digest there lends f its object.
// Otherwise the bytes are stored under a key of f's own, and stored is true.
func (a *App) putObject(ctx context.Context, f *model.File, img *imagecheck.Image) (stored bool, err error) {
	assert.That(a.Blobs != nil && a.Store != nil, "App.putObject: attachments are off")
	assert.That(f != nil && img != nil && f.S3Key == "", "App.putObject: no file or image, or an object key already")
	existing, lookupErr := a.Store.File().GetBySHA256(ctx, f.ChannelID, img.SHA256)
	switch {
	case lookupErr == nil:
		f.S3Key = existing.S3Key
		return false, nil
	case isNotFound(lookupErr):
		f.S3Key = model.FileObjectKey(f.ChannelID, f.ID)
		if putErr := a.Blobs.Put(ctx, f.S3Key, img.ContentType, img.Data); putErr != nil {
			return false, fmt.Errorf("store file: %w", putErr)
		}
		return true, nil
	default:
		return false, lookupErr
	}
}

// GetFile returns a file the reader may read: a member of its channel, and
// before it is attached to a post, only its uploader. An unknown file is a
// 404; a file the reader may not read is a 403, as for posts.
func (a *App) GetFile(ctx context.Context, id string, reader *model.User) (*model.File, error) {
	assert.That(reader != nil, "App.GetFile: nil reader")
	assert.That(a.Store != nil, "App.GetFile: nil store")
	if err := a.requireAttachments(); err != nil {
		return nil, err
	}
	f, err := a.Store.File().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	assert.That(f != nil, "App.GetFile: nil file without an error")
	if err := a.requireChannelMember(ctx, f.ChannelID, reader.ID); err != nil {
		return nil, err
	}
	if f.PostID == "" && f.UploaderID != reader.ID {
		return nil, model.NewForbiddenError("App.GetFile", "the file is not attached to a post yet; only its uploader may read it")
	}
	return f, nil
}

// OpenFile is GetFile with the bytes. The caller closes the reader.
func (a *App) OpenFile(ctx context.Context, id string, reader *model.User) (*model.File, io.ReadCloser, int64, error) {
	f, err := a.GetFile(ctx, id, reader)
	if err != nil {
		return nil, nil, 0, err
	}
	assert.That(a.Blobs != nil, "App.OpenFile: attachments are off after GetFile")
	rc, size, err := a.Blobs.Get(ctx, f.S3Key)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			slog.Error("file: the object of a live file is missing", "file_id", f.ID, "key", f.S3Key)
			return nil, nil, 0, model.NewNotFoundError("App.OpenFile", id)
		}
		return nil, nil, 0, err
	}
	assert.That(rc != nil, "App.OpenFile: nil reader without an error")
	return f, rc, size, nil
}

// checkAttachable returns an error unless post may carry post.FileIDs: at
// most imagecheck.MaxImagesPerMessage distinct files, each live, in the
// post's channel, uploaded by the post's author, and attached nowhere yet.
// Store().Post().Save checks the same again, atomically.
func (a *App) checkAttachable(ctx context.Context, post *model.Post) error {
	assert.That(post != nil, "App.checkAttachable: nil post")
	if len(post.FileIDs) == 0 {
		return nil
	}
	const where = "App.CreatePost"
	if err := a.requireAttachments(); err != nil {
		return err
	}
	if len(post.FileIDs) > imagecheck.MaxImagesPerMessage {
		return model.NewBadRequestError(where, fmt.Sprintf("a post has at most %d file_ids", imagecheck.MaxImagesPerMessage))
	}
	seen := map[string]bool{}
	for _, id := range post.FileIDs {
		if seen[id] {
			return model.NewBadRequestError(where, "file_ids names a file twice")
		}
		seen[id] = true
	}
	assert.That(len(seen) == len(post.FileIDs), "App.checkAttachable: a duplicate id got through")
	files, err := a.Store.File().GetByIDs(ctx, post.FileIDs)
	if err != nil {
		return err
	}
	byID := make(map[string]*model.File, len(files))
	for _, f := range files {
		byID[f.ID] = f
	}
	for i, id := range post.FileIDs {
		f, ok := byID[id]
		switch {
		case !ok:
			return model.NewBadRequestError(where, fmt.Sprintf("file_ids[%d] names no file", i))
		case f.ChannelID != post.ChannelID:
			return model.NewBadRequestError(where, fmt.Sprintf("file_ids[%d] was uploaded to another channel", i))
		case f.UploaderID != post.UserID:
			return model.NewBadRequestError(where, fmt.Sprintf("file_ids[%d] was uploaded by somebody else", i))
		case f.PostID != "":
			return model.NewConflictError(where, fmt.Sprintf("file_ids[%d] is already attached to a post", i))
		}
	}
	return nil
}

// SearchFiles finds files by name or alt text in the reader's channels, or
// in channelID only. Nothing derived from the image is searched.
func (a *App) SearchFiles(ctx context.Context, reader *model.User, terms, channelID string, page, perPage int) ([]*model.File, error) {
	assert.That(reader != nil && a.Store != nil, "App.SearchFiles: nil reader or store")
	assert.That(page >= 0 && perPage > 0, "App.SearchFiles: pagination not clamped")
	if err := a.requireAttachments(); err != nil {
		return nil, err
	}
	terms = strings.TrimSpace(terms)
	if len(terms) > maxSearchTermsLen {
		return nil, model.NewBadRequestError("App.SearchFiles", fmt.Sprintf("terms is longer than %d bytes", maxSearchTermsLen))
	}
	var channelIDs []string
	if channelID != "" {
		if err := a.requireChannelMember(ctx, channelID, reader.ID); err != nil {
			return nil, err
		}
		channelIDs = []string{channelID}
	} else {
		ids, err := a.Store.Channel().GetChannelIDsForUser(ctx, reader.ID)
		if err != nil {
			return nil, err
		}
		channelIDs = ids
	}
	return a.Store.File().Search(ctx, &model.FileSearch{
		Terms: terms, ChannelIDs: channelIDs, ReaderID: reader.ID, Page: page, PerPage: perPage,
	})
}

// maxSearchTermsLen bounds a file search's terms.
const maxSearchTermsLen = 200

// SweepFiles runs one bounded pass of the attachment sweep:
//
//  1. Live files older than Config.FileOrphanGrace that no post carries are
//     deleted (an upload whose post never came).
//  2. Files deleted more than Config.FileOrphanGrace ago are purged: their
//     object is removed unless a live file still uses it (a deduplicated
//     copy), and then their row.
//
// Each step handles at most Config.FileSweepBatch files. An object that
// cannot be removed keeps its row, so the next pass tries again.
func (a *App) SweepFiles(ctx context.Context) (expired, purged int, err error) {
	if a.Blobs == nil {
		return 0, 0, nil
	}
	assert.That(a.Store != nil && a.Config != nil, "App.SweepFiles: nil store or config")
	now := model.GetMillis()
	cutoff := now - a.Config.FileOrphanGrace.Milliseconds()
	batch := a.Config.FileSweepBatch
	assert.That(batch > 0, "App.SweepFiles: batch must be positive")

	expired, err = a.Store.File().ExpireUnattached(ctx, cutoff, now, batch)
	if err != nil {
		return 0, 0, err
	}
	deleted, err := a.Store.File().Deleted(ctx, cutoff, batch)
	if err != nil {
		return expired, 0, err
	}
	for _, f := range deleted {
		inUse, err := a.Store.File().ObjectInUse(ctx, f.S3Key)
		if err != nil {
			return expired, purged, err
		}
		if !inUse {
			if err := a.Blobs.Delete(ctx, f.S3Key); err != nil {
				slog.Warn("file sweep: could not remove an object; will retry", "key", f.S3Key, "error", err)
				continue
			}
		}
		if err := a.Store.File().Purge(ctx, f.ID); err != nil {
			return expired, purged, err
		}
		purged++
	}
	assert.That(purged <= len(deleted), "App.SweepFiles: purged more files than were deleted")
	return expired, purged, nil
}
