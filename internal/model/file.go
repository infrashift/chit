package model

import (
	"net/http"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// File is an image attached, or about to be attached, to a post (P-CHIT-5,
// HALO PLAN.md 3.6). The bytes are in the object store under S3Key; the
// row holds what Chit checked when it was uploaded.
type File struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	// UploaderID is the person the upload was authorized as: with
	// X-On-Behalf-Of, the principal. Only they may attach the file, and its
	// bytes count against their quota.
	UploaderID string `json:"uploader_id"`
	// ViaActorID is the allowlisted client that uploaded it on the
	// uploader's behalf, or "".
	ViaActorID  string `json:"via_actor_id,omitempty"`
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	AltText     string `json:"alt_text"`
	// PostID is the post the file is attached to, or "" before it is.
	PostID   string `json:"post_id,omitempty"`
	CreateAt int64  `json:"create_at"`
	// S3Key is the object's key: "<channel_id>/<file_id>" of the upload that
	// stored the bytes, which is another file's ID when this one was
	// deduplicated. Never sent to a client.
	S3Key    string `json:"-"`
	DeleteAt int64  `json:"-"`
}

// File limits.
const (
	FileMaxAltTextRunes = 1000
	FileMaxNameBytes    = 255
	// FileMaxIDsPerPost is imagecheck.MaxImagesPerMessage; the model does
	// not import the image package.
	FileMaxIDsPerPost = 4
)

// FileObjectKey is the object key of a file stored by the upload fileID.
func FileObjectKey(channelID, fileID string) string {
	return channelID + "/" + fileID
}

// ValidateAltText checks alt text: valid UTF-8, at most
// FileMaxAltTextRunes characters, and no control characters other than
// newline and tab.
func ValidateAltText(s string) *AppError {
	bad := func(msg string) *AppError {
		return NewAppError("File.AltText", msg, "", http.StatusBadRequest)
	}
	if !utf8.ValidString(s) {
		return bad("alt_text is not valid UTF-8")
	}
	if utf8.RuneCountInString(s) > FileMaxAltTextRunes {
		return bad("alt_text is longer than 1000 characters")
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return bad("alt_text contains a control character")
		}
	}
	return nil
}

// CleanFileName makes an uploaded file name safe to store and to send back
// in Content-Disposition: the last path element, without control
// characters, quotes or backslashes, at most FileMaxNameBytes bytes, and
// with ext (".png" or ".jpg", the type Chit stored) as its extension.
func CleanFileName(name, ext string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) || r == '"' || r == '/' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, path.Ext(name))
	if name == "" || name == "." || name == ".." {
		name = "image"
	}
	for len(name)+len(ext) > FileMaxNameBytes {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name + ext
}

// FileSearch finds files by name and alt text, in ChannelIDs only. An empty
// ChannelIDs matches nothing. A file not yet attached to a post is found
// only by its uploader, ReaderID.
type FileSearch struct {
	Terms      string
	ChannelIDs []string
	ReaderID   string
	Page       int
	PerPage    int
}
