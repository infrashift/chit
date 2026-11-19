package imagecheck

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"
)

// ─── Fixtures ────────────────────────────────────────────────────

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, solid(4, 3, color.RGBA{R: 200, A: 255})); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, solid(16, 16, color.RGBA{G: 200, A: 255}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// gifTwoFrames is a red frame then a blue one.
func gifTwoFrames(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	frame := func(i uint8) *image.Paletted {
		p := image.NewPaletted(image.Rect(0, 0, 5, 5), pal)
		for j := range p.Pix {
			p.Pix[j] = i
		}
		return p
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{frame(0), frame(1)}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// pngChunk builds one PNG chunk with its CRC.
func pngChunk(typ string, body []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	out = append(out, typ...)
	out = append(out, body...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(typ), body...)))
}

// insertPNGChunk puts a chunk right after IHDR.
func insertPNGChunk(src, chunk []byte) []byte {
	ihdrEnd := len(pngSignature()) + 12 + 13
	out := append([]byte{}, src[:ihdrEnd]...)
	out = append(out, chunk...)
	return append(out, src[ihdrEnd:]...)
}

// exifWithGPS is an APP1 Exif segment whose IFD0 points at a GPS IFD with a
// latitude reference, plus the coordinates as text so a test can look for
// them in the output.
const gpsText = "GPS 51.5007N 0.1246W"

func exifWithGPS() []byte {
	le := binary.LittleEndian
	tiff := []byte("II*\x00")
	tiff = le.AppendUint32(tiff, 8)
	// IFD0 at 8: one entry, GPSInfo (0x8825) → the GPS IFD at 26.
	tiff = le.AppendUint16(tiff, 1)
	tiff = le.AppendUint16(tiff, 0x8825)
	tiff = le.AppendUint16(tiff, 4) // LONG
	tiff = le.AppendUint32(tiff, 1)
	tiff = le.AppendUint32(tiff, 26)
	tiff = le.AppendUint32(tiff, 0)
	// GPS IFD at 26: GPSLatitudeRef "N".
	tiff = le.AppendUint16(tiff, 1)
	tiff = le.AppendUint16(tiff, 0x0001)
	tiff = le.AppendUint16(tiff, 2) // ASCII
	tiff = le.AppendUint32(tiff, 2)
	tiff = append(tiff, 'N', 0, 0, 0)
	tiff = le.AppendUint32(tiff, 0)
	tiff = append(tiff, gpsText...)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1}
	return append(binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2)), payload...)
}

func jpegWithGPS(t *testing.T) []byte {
	t.Helper()
	src := jpegBytes(t)
	out := append([]byte{}, src[:2]...) // SOI
	out = append(out, exifWithGPS()...)
	return append(out, src[2:]...)
}

func wantReason(t *testing.T, err, reason error) {
	t.Helper()
	if !errors.Is(err, reason) {
		t.Fatalf("got %v, want %v", err, reason)
	}
}

// ─── Accepted ────────────────────────────────────────────────────

func TestCheck_AcceptsEachType(t *testing.T) {
	cases := []struct {
		name, declared, wantOut string
		data                    []byte
		w, h                    int
	}{
		{"png", TypePNG, TypePNG, pngBytes(t), 4, 3},
		{"jpeg", TypeJPEG, TypeJPEG, jpegBytes(t), 16, 16},
		{"gif", TypeGIF, TypePNG, gifTwoFrames(t), 5, 5},
		{"webp lossy", TypeWebP, TypePNG, readFixture(t, "blue-purple-pink.lossy.webp"), 0, 0},
		{"webp lossless", TypeWebP, TypePNG, readFixture(t, "gopher-doc.8bpp.lossless.webp"), 0, 0},
		{"declared with parameters and capitals", "Image/PNG; charset=binary", TypePNG, pngBytes(t), 4, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img, err := Check(tc.declared, tc.data, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			wantAccepted(t, img, tc.wantOut, tc.w, tc.h)
		})
	}
}

// wantAccepted checks an accepted image: its output type, its size (unless
// w is 0), its digest, and that the output passes the same checks.
func wantAccepted(t *testing.T, img *Image, wantOut string, w, h int) {
	t.Helper()
	if img.ContentType != wantOut {
		t.Errorf("content type %s, want %s", img.ContentType, wantOut)
	}
	if Sniff(img.Data) != wantOut {
		t.Errorf("output sniffs as %q, want %s", Sniff(img.Data), wantOut)
	}
	if w != 0 && (img.Width != w || img.Height != h) {
		t.Errorf("%d×%d, want %d×%d", img.Width, img.Height, w, h)
	}
	if len(img.SHA256) != 64 {
		t.Errorf("sha256 %q", img.SHA256)
	}
	// The output passes the same checks.
	if _, err := Check(img.ContentType, img.Data, DefaultLimits()); err != nil {
		t.Errorf("the re-encoded image is refused: %v", err)
	}
}

// A GIF keeps only its first frame: red, not blue.
func TestCheck_GIFKeepsTheFirstFrame(t *testing.T) {
	img, err := Check(TypeGIF, gifTwoFrames(t), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	out, err := png.Decode(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatal(err)
	}
	r, _, b, _ := out.At(2, 2).RGBA()
	if r>>8 != 255 || b != 0 {
		t.Fatalf("pixel %v, want the first frame's red", out.At(2, 2))
	}
}

// Re-encoding is deterministic, so the same upload has the same SHA-256,
// which is what deduplication keys on.
func TestCheck_SameInputSameDigest(t *testing.T) {
	a, err := Check(TypeJPEG, jpegBytes(t), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Check(TypeJPEG, jpegBytes(t), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if a.SHA256 != b.SHA256 {
		t.Fatal("two checks of one image gave different digests")
	}
}

// ─── EXIF ────────────────────────────────────────────────────────

func TestCheck_ReencodeDropsEXIFAndGPS(t *testing.T) {
	in := jpegWithGPS(t)
	if !bytes.Contains(in, []byte(gpsText)) || !bytes.Contains(in, []byte("Exif\x00\x00")) {
		t.Fatal("the fixture carries no EXIF GPS")
	}
	img, err := Check(TypeJPEG, in, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(img.Data, []byte("Exif")) || bytes.Contains(img.Data, []byte(gpsText)) {
		t.Fatal("EXIF or GPS survived the re-encode")
	}
	meta, _, ok := walkJPEG(img.Data)
	if !ok {
		t.Fatal("the output does not parse as JPEG")
	}
	if len(meta) != 0 {
		t.Fatalf("the output has %d APPn/COM segments, want none", len(meta))
	}
}

// ─── Refused ─────────────────────────────────────────────────────

func TestCheck_RefusesSVG(t *testing.T) {
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	wantReason(t, firstErr(Check(typeSVG, svg, DefaultLimits())), ErrSVG)
	// Declared as an image it is not.
	wantReason(t, firstErr(Check(TypePNG, svg, DefaultLimits())), ErrSVG)
	wantReason(t, firstErr(Check(TypePNG, []byte("  <svg onload=alert(1)>"), DefaultLimits())), ErrSVG)
	// Declared SVG is refused as SVG whatever the bytes are.
	wantReason(t, firstErr(Check("image/svg+xml; charset=utf-8", pngBytes(t), DefaultLimits())), ErrSVG)
}

func firstErr(_ *Image, err error) error { return err }

func TestCheck_RefusesOtherTypes(t *testing.T) {
	for _, typ := range []string{"image/bmp", "image/tiff", "text/html", "application/octet-stream", ""} {
		wantReason(t, firstErr(Check(typ, pngBytes(t), DefaultLimits())), ErrUnsupportedType)
	}
}

func TestCheck_RefusesDeclaredSniffedMismatch(t *testing.T) {
	wantReason(t, firstErr(Check(TypeJPEG, pngBytes(t), DefaultLimits())), ErrTypeMismatch)
	wantReason(t, firstErr(Check(TypePNG, jpegBytes(t), DefaultLimits())), ErrTypeMismatch)
	wantReason(t, firstErr(Check(TypeGIF, readFixture(t, "blue-purple-pink.lossy.webp"), DefaultLimits())), ErrTypeMismatch)
	wantReason(t, firstErr(Check(TypePNG, []byte("<html><body>hi</body></html>"), DefaultLimits())), ErrTypeMismatch)
}

const htmlPage = "<html><body><script>alert(document.cookie)</script></body></html>"

func TestCheck_RefusesPolyglots(t *testing.T) {
	cases := map[string]struct {
		typ  string
		data []byte
	}{
		"PNG with HTML in a tEXt chunk": {TypePNG, insertPNGChunk(pngBytes(t), pngChunk("tEXt", []byte("Comment\x00"+htmlPage)))},
		"PNG with HTML after IEND":      {TypePNG, append(pngBytes(t), htmlPage...)},
		"PNG with a ZIP after IEND":     {TypePNG, append(pngBytes(t), "PK\x03\x04payload"...)},
		"JPEG with HTML in a comment": {TypeJPEG, func() []byte {
			src := jpegBytes(t)
			com := binary.BigEndian.AppendUint16([]byte{0xFF, 0xFE}, uint16(len(htmlPage)+2))
			return append(append(append([]byte{}, src[:2]...), append(com, htmlPage...)...), src[2:]...)
		}()},
		"JPEG with HTML after EOI":        {TypeJPEG, append(jpegBytes(t), htmlPage...)},
		"GIF with HTML after the trailer": {TypeGIF, append(gifTwoFrames(t), htmlPage...)},
		"WebP with HTML after the RIFF":   {TypeWebP, append(readFixture(t, "gopher-doc.8bpp.lossless.webp"), htmlPage...)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			wantReason(t, firstErr(Check(tc.typ, tc.data, DefaultLimits())), ErrPolyglot)
		})
	}
}

// A GIF comment extension holding HTML.
func TestCheck_RefusesGIFCommentPolyglot(t *testing.T) {
	src := gifTwoFrames(t)
	// Insert a comment extension before the first block, after the header
	// and the global color table, if any.
	at := 13
	if src[10]&0x80 != 0 {
		at += 3 << (int(src[10]&0x07) + 1)
	}
	ext := []byte{0x21, 0xFE, byte(len(htmlPage))}
	ext = append(ext, htmlPage...)
	ext = append(ext, 0)
	data := append(append(append([]byte{}, src[:at]...), ext...), src[at:]...)
	wantReason(t, firstErr(Check(TypeGIF, data, DefaultLimits())), ErrPolyglot)
}

// A tiny file whose IHDR claims 10000×10000 (100 MP). It is refused by
// DecodeConfig's numbers, before any pixel is decoded: the IDAT here would
// not even decode.
func TestCheck_RefusesTooManyPixelsFromTheHeader(t *testing.T) {
	ihdr := binary.BigEndian.AppendUint32(nil, 10000)
	ihdr = binary.BigEndian.AppendUint32(ihdr, 10000)
	ihdr = append(ihdr, 8, 2, 0, 0, 0) // 8-bit RGB
	data := append([]byte{}, pngSignature()...)
	data = append(data, pngChunk("IHDR", ihdr)...)
	data = append(data, pngChunk("IDAT", []byte{0x78, 0x9c, 0x03, 0x00})...)
	data = append(data, pngChunk("IEND", nil)...)
	if len(data) > 100 {
		t.Fatalf("the fixture is %d bytes; it should be tiny", len(data))
	}
	err := firstErr(Check(TypePNG, data, DefaultLimits()))
	wantReason(t, err, ErrTooManyPixels)
	if !strings.Contains(err.Error(), "100000000 pixels") {
		t.Errorf("error %q does not give the pixel count", err)
	}
}

func TestCheck_RefusesTooManyBytes(t *testing.T) {
	data := pngBytes(t)
	wantReason(t, firstErr(Check(TypePNG, data, Limits{MaxBytes: int64(len(data) - 1), MaxPixels: MaxImagePixels})), ErrTooLarge)
}

// The input limit applies on its own: a file padded with a large, harmless
// text chunk is refused although its re-encoded image would be small.
func TestCheck_RefusesAnInputOverTheLimitThatWouldShrink(t *testing.T) {
	data := insertPNGChunk(pngBytes(t), pngChunk("tEXt", append([]byte("Comment\x00"), bytes.Repeat([]byte("a"), 4096)...)))
	lim := Limits{MaxBytes: 1024, MaxPixels: MaxImagePixels}
	if img, err := Check(TypePNG, data, DefaultLimits()); err != nil || len(img.Data) > 1024 {
		t.Fatalf("the fixture: %v", err)
	}
	wantReason(t, firstErr(Check(TypePNG, data, lim)), ErrTooLarge)
}

// The output limit applies too: a WebP re-encoded as PNG can grow.
func TestCheck_RefusesAnOutputOverTheLimit(t *testing.T) {
	data := readFixture(t, "blue-purple-pink.lossy.webp")
	img, err := Check(TypeWebP, data, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Data) <= len(data) {
		t.Skip("the PNG is not larger than the WebP")
	}
	wantReason(t, firstErr(Check(TypeWebP, data, Limits{MaxBytes: int64(len(data)), MaxPixels: MaxImagePixels})), ErrTooLarge)
}

func TestCheck_RefusesMalformed(t *testing.T) {
	data := pngBytes(t)
	wantReason(t, firstErr(Check(TypePNG, data[:len(data)-20], DefaultLimits())), ErrMalformed)
	wantReason(t, firstErr(Check(TypeJPEG, jpegBytes(t)[:40], DefaultLimits())), ErrMalformed)
}

// Metadata that merely looks like XML (XMP) is not markup.
func TestCheck_AcceptsXMPMetadata(t *testing.T) {
	xmp := `<?xpacket begin=""?><x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF><dc:format>image/png</dc:format></rdf:RDF></x:xmpmeta>`
	data := insertPNGChunk(pngBytes(t), pngChunk("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"+xmp)))
	img, err := Check(TypePNG, data, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(img.Data, []byte("xmpmeta")) {
		t.Fatal("XMP survived the re-encode")
	}
}
