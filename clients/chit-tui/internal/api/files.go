package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// MaxAltText is the most characters of alt text that the server keeps.
const MaxAltText = 1000

// ErrFileTooLarge tells that a file is larger than model.MaxFileBytes. The
// client finds it before it sends the file.
//
// semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity
var ErrFileTooLarge = errors.New("the file is too large (10 MB at most)")

// GetFileInfo returns the record of one uploaded file.
func (c *httpClient) GetFileInfo(ctx context.Context, fileID string) (*model.File, error) {
	assert.That(ctx != nil, "GetFileInfo: nil context")
	assert.That(fileID != "", "GetFileInfo: empty file ID")
	var f model.File
	if err := c.get(ctx, fmt.Sprintf("/files/%s/info", fileID), &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// UploadFile sends one image to channelID as a multipart form, and returns
// the record that the server made. contentType goes on the file part,
// because the server checks the bytes against it.
func (c *httpClient) UploadFile(ctx context.Context, channelID, name, contentType, altText string, r io.Reader) (*model.File, error) {
	assert.That(ctx != nil, "UploadFile: nil context")
	assert.That(channelID != "" && name != "", "UploadFile: empty channel ID or file name")
	assert.That(r != nil, "UploadFile: nil reader")
	body, formType, err := uploadForm(name, contentType, altText, r)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+fmt.Sprintf("/channels/%s/files", channelID), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", formType)
	var f model.File
	if err := c.do(req, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// uploadForm builds the multipart body of an upload: the file part, then
// alt_text when it is set. It reads at most model.MaxFileBytes+1 bytes of r,
// and it returns ErrFileTooLarge when r holds more.
func uploadForm(name, contentType, altText string, r io.Reader) (*bytes.Buffer, string, error) {
	assert.That(name != "", "uploadForm: empty file name")
	assert.That(contentType != "", "uploadForm: empty content type")
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, name))
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, "", err
	}
	n, err := io.Copy(part, io.LimitReader(r, model.MaxFileBytes+1))
	if err != nil {
		return nil, "", err
	}
	if n > model.MaxFileBytes {
		return nil, "", ErrFileTooLarge
	}
	if altText = strings.TrimSpace(altText); altText != "" {
		if err := w.WriteField("alt_text", altText); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	assert.That(buf.Len() >= int(n), "uploadForm: body shorter than the file")
	return &buf, w.FormDataContentType(), nil
}

// DownloadFile writes the bytes of one file to w. It writes at most limit
// bytes, and it returns ErrFileTooLarge when the file holds more. It
// returns the count of bytes written.
func (c *httpClient) DownloadFile(ctx context.Context, fileID string, w io.Writer, limit int64) (int64, error) {
	assert.That(ctx != nil, "DownloadFile: nil context")
	assert.That(fileID != "" && w != nil, "DownloadFile: empty file ID or nil writer")
	assert.That(limit > 0, "DownloadFile: the limit must be positive")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf("/files/%s", fileID), nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	assert.That(resp != nil && resp.Body != nil, "DownloadFile: no response without an error")
	defer errlog.Close(resp.Body, "file download body")
	if resp.StatusCode >= 400 {
		return 0, parseErrorResponse(resp)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, ErrFileTooLarge
	}
	return n, nil
}
