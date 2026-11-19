// Package imagecheck decides whether an uploaded image may be stored, and
// re-encodes the ones that may.
//
// It has no Chit dependencies, so HALO can apply the same rules to the
// images that its models and Tools produce (HALO PLAN.md 3.6.3):
//
//  1. Only PNG, JPEG, WebP and GIF are accepted. A GIF keeps only its first
//     frame. SVG is refused.
//  2. The declared type must be the type the magic bytes give. A file with
//     data after the end of the image, or with markup in its metadata (a
//     polyglot), is refused.
//  3. image.DecodeConfig runs before the full decode, and an image of more
//     than MaxPixels pixels is refused without being decoded.
//  4. The image is decoded and encoded again in pure Go: JPEG as JPEG, and
//     PNG, GIF and WebP as PNG. The output carries no EXIF, no text chunks
//     and no trailing bytes.
//  5. The input and the output must each be MaxBytes or smaller.
//
// MaxImagesPerMessage and storage quotas are the caller's to apply.
package imagecheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // registers GIF with image.DecodeConfig and image.Decode
	"image/jpeg"
	"image/png"
	"mime"
	"strings"

	// Registers WebP with image.DecodeConfig and image.Decode.
	_ "golang.org/x/image/webp"

	"github.com/infrashift/chit/pkg/imagecheck/internal/assert"
)

// Limits from HALO PLAN.md 3.6.
const (
	// MaxImageBytes bounds the uploaded file and the re-encoded one.
	MaxImageBytes = 10 << 20
	// MaxImagePixels bounds width × height, checked before decoding.
	MaxImagePixels = 40_000_000
	// MaxImagesPerMessage bounds the attachments of one message.
	MaxImagesPerMessage = 4
)

// The accepted types.
const (
	TypePNG  = "image/png"
	TypeJPEG = "image/jpeg"
	TypeWebP = "image/webp"
	TypeGIF  = "image/gif"
	typeSVG  = "image/svg+xml"
)

// JPEGQuality is the quality of a re-encoded JPEG.
const JPEGQuality = 90

// The reasons for a refusal. Check wraps one of them, so a caller can tell
// them apart with errors.Is.
var (
	ErrUnsupportedType = errors.New("unsupported image type")
	ErrSVG             = errors.New("SVG images are not accepted")
	ErrTypeMismatch    = errors.New("the declared type is not the type of the content")
	ErrPolyglot        = errors.New("the image carries other content")
	ErrTooLarge        = errors.New("the image is too large")
	ErrTooManyPixels   = errors.New("the image has too many pixels")
	ErrMalformed       = errors.New("the image is malformed")
)

// Limits bounds one image.
type Limits struct {
	MaxBytes  int64
	MaxPixels int64
}

// DefaultLimits returns MaxImageBytes and MaxImagePixels.
func DefaultLimits() Limits {
	return Limits{MaxBytes: MaxImageBytes, MaxPixels: MaxImagePixels}
}

// Image is an accepted image, re-encoded.
type Image struct {
	// Data is the re-encoded image.
	Data []byte
	// ContentType is the type of Data: TypePNG or TypeJPEG.
	ContentType string
	// SourceType is the type of the input.
	SourceType string
	Width      int
	Height     int
	// SHA256 is the lowercase hex SHA-256 of Data.
	SHA256 string
}

// Extension returns the file extension of the image's type, with the dot.
func (img *Image) Extension() string {
	assert.That(img != nil, "Image.Extension: nil image")
	if img.ContentType == TypeJPEG {
		return ".jpg"
	}
	return ".png"
}

func refuse(reason error, format string, args ...any) error {
	assert.That(reason != nil, "refuse: nil reason")
	assert.That(format != "", "refuse: empty format")
	err := fmt.Errorf("%w: %s", reason, fmt.Sprintf(format, args...))
	assert.That(errors.Is(err, reason), "refuse: the error does not wrap the reason")
	return err
}

// Check validates data, which the uploader declared as declaredType (a
// Content-Type value; parameters are ignored), and returns it re-encoded. A
// refusal wraps one of the Err values.
func Check(declaredType string, data []byte, lim Limits) (*Image, error) {
	assert.That(lim.MaxBytes > 0, "Check: MaxBytes must be positive")
	assert.That(lim.MaxPixels > 0, "Check: MaxPixels must be positive")
	if int64(len(data)) > lim.MaxBytes {
		return nil, refuse(ErrTooLarge, "%d bytes, more than %d", len(data), lim.MaxBytes)
	}
	sniffed, err := checkType(declaredType, data)
	if err != nil {
		return nil, err
	}
	if hdrErr := checkHeader(sniffed, data, lim); hdrErr != nil {
		return nil, hdrErr
	}

	// 4. Decode (a GIF's first frame) and encode again.
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, refuse(ErrMalformed, "%v", err)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || int64(b.Dx())*int64(b.Dy()) > lim.MaxPixels {
		return nil, refuse(ErrTooManyPixels, "the decoded image is %d×%d", b.Dx(), b.Dy())
	}
	out, outType, err := reencode(img, sniffed)
	if err != nil {
		return nil, err
	}

	// 5. The re-encoded image is what is stored.
	if int64(len(out)) > lim.MaxBytes {
		return nil, refuse(ErrTooLarge, "%d bytes after re-encoding as %s, more than %d", len(out), outType, lim.MaxBytes)
	}
	sum := sha256.Sum256(out)
	res := &Image{
		Data:        out,
		ContentType: outType,
		SourceType:  sniffed,
		Width:       b.Dx(),
		Height:      b.Dy(),
		SHA256:      hex.EncodeToString(sum[:]),
	}
	assert.That(len(res.SHA256) == 2*sha256.Size, "Check: the digest is not hex SHA-256")
	assert.That(res.Width > 0 && res.Height > 0, "Check: an accepted image has no pixels")
	assert.That(len(res.Data) > 0, "Check: an accepted image has no bytes")
	return res, nil
}

// checkType applies steps 1 and 2 of Check: the allowlist, then the
// declared type against the sniffed one and the structure. It returns the
// sniffed type, which is then also the declared one.
func checkType(declaredType string, data []byte) (string, error) {
	declared := normalizeType(declaredType)
	sniffed := Sniff(data)

	// 1. The allowlist. SVG gets its own reason, by name or by content.
	if declared == typeSVG || (sniffed == "" && looksLikeSVG(data)) {
		return "", ErrSVG
	}
	if !accepted(declared) {
		return "", refuse(ErrUnsupportedType, "%q is not one of %s, %s, %s or %s",
			declared, TypePNG, TypeJPEG, TypeWebP, TypeGIF)
	}

	// 2. Declared against sniffed, then the structure: trailing data and
	// markup in metadata make a polyglot.
	if sniffed != declared {
		if sniffed == "" {
			return "", refuse(ErrTypeMismatch, "declared %s, but the content is not an accepted image", declared)
		}
		return "", refuse(ErrTypeMismatch, "declared %s, but the content is %s", declared, sniffed)
	}
	assert.That(sniffed == declared, "checkType: the declared type is not the sniffed one")
	assert.That(accepted(sniffed), "checkType: the sniffed type is not accepted")
	if err := checkStructure(sniffed, data); err != nil {
		return "", err
	}
	return sniffed, nil
}

// checkHeader applies step 3 of Check: the size from the header, before any
// pixel is decoded.
func checkHeader(sniffed string, data []byte, lim Limits) error {
	assert.That(accepted(sniffed), "checkHeader: the sniffed type is not accepted")
	assert.That(lim.MaxPixels > 0, "checkHeader: MaxPixels must be positive")
	assert.That(Sniff(data) == sniffed, "checkHeader: sniffed is not the type of data")
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return refuse(ErrMalformed, "%v", err)
	}
	if "image/"+format != sniffed {
		return refuse(ErrTypeMismatch, "the decoder reads %s, the magic bytes say %s", format, sniffed)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return refuse(ErrMalformed, "%d×%d pixels", cfg.Width, cfg.Height)
	}
	if pixels := int64(cfg.Width) * int64(cfg.Height); pixels > lim.MaxPixels {
		return refuse(ErrTooManyPixels, "%d×%d is %d pixels, more than %d", cfg.Width, cfg.Height, pixels, lim.MaxPixels)
	}
	return nil
}

// reencode encodes img again: a JPEG as JPEG, any other type as PNG. It
// returns the bytes and their type.
func reencode(img image.Image, sniffed string) (out []byte, outType string, err error) {
	assert.That(img != nil, "reencode: nil image")
	assert.That(accepted(sniffed), "reencode: the sniffed type is not accepted")
	var buf bytes.Buffer
	outType = TypePNG
	if sniffed == TypeJPEG {
		outType = TypeJPEG
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality})
	} else {
		err = png.Encode(&buf, img)
	}
	if err != nil {
		return nil, "", refuse(ErrMalformed, "re-encode: %v", err)
	}
	assert.That(outType == TypePNG || outType == TypeJPEG, "reencode: the output type is not PNG or JPEG")
	assert.That((outType == TypeJPEG) == (sniffed == TypeJPEG), "reencode: only a JPEG stays a JPEG")
	assert.That(buf.Len() > 0, "reencode: the encoder wrote nothing")
	return buf.Bytes(), outType, nil
}

func accepted(t string) bool {
	switch t {
	case TypePNG, TypeJPEG, TypeWebP, TypeGIF:
		return true
	}
	return false
}

// normalizeType lowercases a Content-Type and drops its parameters. A value
// that does not parse is returned lowercased, and then matches nothing.
func normalizeType(t string) string {
	mt, _, err := mime.ParseMediaType(t)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(t))
	}
	assert.That(!strings.Contains(mt, ";"), "normalizeType: the media type keeps a parameter")
	assert.That(mt != "" && mt == strings.ToLower(mt), "normalizeType: the media type is empty or not lowercase")
	return mt
}

// Sniff returns the accepted type that data's magic bytes give, or "".
func Sniff(data []byte) string {
	typ := ""
	switch {
	case bytes.HasPrefix(data, pngSignature()):
		typ = TypePNG
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		typ = TypeJPEG
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		typ = TypeGIF
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		typ = TypeWebP
	}
	assert.That(typ == "" || accepted(typ), "Sniff: the type is not accepted")
	return typ
}

// pngSignature returns the eight bytes that start every PNG file.
func pngSignature() []byte {
	sig := []byte("\x89PNG\r\n\x1a\n")
	assert.That(len(sig) == 8, "pngSignature: not eight bytes")
	return sig
}

// looksLikeSVG reports whether data starts like an SVG document: an XML
// declaration or an <svg element, after any byte order mark and whitespace.
func looksLikeSVG(data []byte) bool {
	head := data[:min(len(data), 1024)]
	assert.That(len(head) <= 1024, "looksLikeSVG: the head is longer than 1024 bytes")
	head = bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	head = bytes.ToLower(bytes.TrimLeft(head, " \t\r\n"))
	return bytes.HasPrefix(head, []byte("<svg")) ||
		(bytes.HasPrefix(head, []byte("<?xml")) || bytes.HasPrefix(head, []byte("<!doctype svg"))) &&
			bytes.Contains(head, []byte("<svg"))
}
