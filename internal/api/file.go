package api

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/pkg/imagecheck"
)

// Attachments (P-CHIT-5): POST /channels/{id}/files, GET /files/{id},
// GET /files/{id}/info and POST /files/search.

// maxUploadBodyBytes bounds an upload request: one image of at most
// imagecheck.MaxImageBytes, its alt text and the multipart framing. It is
// applied before the OpenAPI validator reads the body.
const maxUploadBodyBytes = imagecheck.MaxImageBytes + 64<<10

// maxAltTextBytes is the most bytes FileMaxAltTextRunes characters take.
const maxAltTextBytes = 4 * model.FileMaxAltTextRunes

var (
	fileRoutePath   = regexp.MustCompile(`^/api/v1/(files(/.*)?|channels/[^/]+/files)$`)
	uploadRoutePath = regexp.MustCompile(`^/api/v1/channels/[^/]+/files$`)
)

// FileResponseHeaders puts the attachment headers on every response of the
// file routes, refusals included: nosniff, so a browser never reads the
// bytes as anything but their declared type; a sandbox CSP, so a page that
// somehow renders them has no origin and runs no script; and attachment
// disposition, so following a link downloads rather than renders. It also
// bounds the upload body before anything reads it. It runs ahead of
// AuthExtract, so its refusals carry the headers too.
func FileResponseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !fileRoutePath.MatchString(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "sandbox")
		h.Set("Content-Disposition", "attachment")
		if r.Method == http.MethodPost && uploadRoutePath.MatchString(r.URL.Path) {
			r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func uploadFile(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const where = "uploadFile"
		user := ContextGetUser(r)
		up, err := readUpload(r)
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				WriteError(w, &model.AppError{ID: app.FileErrTooLarge, Where: where, StatusCode: http.StatusRequestEntityTooLarge,
					Message: fmt.Sprintf("the upload is larger than %d bytes", imagecheck.MaxImageBytes)})
				return
			}
			WriteAppError(w, where, err)
			return
		}
		up.ChannelID = chi.URLParam(r, "id")
		saved, err := a.UploadFile(r.Context(), up, user, ContextGetActor(r))
		if err != nil {
			WriteAppError(w, where, err)
			return
		}
		WriteJSON(w, http.StatusCreated, saved)
	}
}

// readUpload reads the multipart form: one "file" part (its Content-Type is
// the declared type, its filename the name) and an optional "alt_text".
func readUpload(r *http.Request) (*app.FileUpload, error) {
	const where = "uploadFile"
	bad := func(msg string) error { return model.NewBadRequestError(where, msg) }
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, bad("expected a multipart/form-data body")
	}
	up := &app.FileUpload{}
	var gotFile, gotAlt bool
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				return nil, err
			}
			return nil, bad("malformed multipart body")
		}
		switch part.FormName() {
		case "file":
			if gotFile {
				return nil, bad("only one file per upload")
			}
			gotFile = true
			up.Name = part.FileName()
			up.DeclaredType = part.Header.Get("Content-Type")
			data, err := io.ReadAll(io.LimitReader(part, imagecheck.MaxImageBytes+1))
			if err != nil {
				return nil, err
			}
			if len(data) > imagecheck.MaxImageBytes {
				return nil, &http.MaxBytesError{Limit: imagecheck.MaxImageBytes}
			}
			up.Data = data
		case "alt_text":
			if gotAlt {
				return nil, bad("alt_text is given twice")
			}
			gotAlt = true
			b, err := io.ReadAll(io.LimitReader(part, maxAltTextBytes+1))
			if err != nil {
				return nil, err
			}
			if len(b) > maxAltTextBytes {
				return nil, bad("alt_text is longer than 1000 characters")
			}
			up.AltText = string(b)
		default:
			return nil, bad(fmt.Sprintf("unexpected form field %q", part.FormName()))
		}
		_ = part.Close()
	}
	if !gotFile {
		return nil, bad("the file part is missing")
	}
	return up, nil
}

// downloadFile serves a file's bytes, to a member of its channel. halod
// calls it on behalf of the person it serves the image to.
func downloadFile(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, rc, size, err := a.OpenFile(r.Context(), chi.URLParam(r, "id"), ContextGetUser(r))
		if err != nil {
			WriteAppError(w, "downloadFile", err)
			return
		}
		defer func() { _ = rc.Close() }()
		h := w.Header()
		h.Set("Content-Type", f.ContentType)
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
		// Not cached by anything shared, and read again each time: a member
		// who leaves the channel must stop seeing it.
		h.Set("Cache-Control", "private, no-store")
		h.Set("ETag", `"`+f.SHA256+`"`)
		if size > 0 {
			h.Set("Content-Length", strconv.FormatInt(size, 10))
		}
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, rc); err != nil {
			slog.Debug("download file: write body", "file_id", f.ID, "error", err)
		}
	}
}

func getFileInfo(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := a.GetFile(r.Context(), chi.URLParam(r, "id"), ContextGetUser(r))
		if err != nil {
			WriteAppError(w, "getFileInfo", err)
			return
		}
		WriteJSON(w, http.StatusOK, f)
	}
}

// searchFiles finds files by file name and alt text, in the caller's
// channels (or one, with channel_id).
func searchFiles(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Terms     string `json:"terms"`
			ChannelID string `json:"channel_id"`
			Page      int    `json:"page"`
			PerPage   int    `json:"per_page"`
		}
		if !decodeBody(w, r, &body, "searchFiles") {
			return
		}
		if body.PerPage == 0 {
			body.PerPage = 60
		}
		page, perPage := clampPagination(body.Page, body.PerPage)
		files, err := a.SearchFiles(r.Context(), ContextGetUser(r), body.Terms, body.ChannelID, page, perPage)
		if err != nil {
			WriteAppError(w, "searchFiles", err)
			return
		}
		writeList(w, files)
	}
}
