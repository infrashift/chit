package imagecheck

import (
	"bytes"
	"encoding/binary"

	"github.com/infrashift/chit/pkg/imagecheck/internal/assert"
)

// The walkers below read container structure from untrusted bytes. Every
// loop is bounded by len(data): each pass moves pos forward by at least one
// byte, and the assertions state the position invariants the bounds checks
// establish. No assertion looks at the value of an image byte.

// checkStructure walks the container format of data, which Sniff says is
// typ. It refuses (ErrPolyglot) any byte after the end of the image, and
// any metadata that holds markup a browser or another parser would act on:
// an HTML page in a PNG text chunk or a JPEG comment, say. Pixel data is not
// scanned, since compressed bytes can spell anything by chance.
func checkStructure(typ string, data []byte) error {
	assert.That(accepted(typ), "checkStructure: the type is not accepted")
	assert.That(Sniff(data) == typ, "checkStructure: typ is not the type of data")
	var (
		meta [][]byte
		end  int
		ok   bool
	)
	switch typ {
	case TypePNG:
		meta, end, ok = walkPNG(data)
	case TypeJPEG:
		meta, end, ok = walkJPEG(data)
	case TypeGIF:
		meta, end, ok = walkGIF(data)
	case TypeWebP:
		meta, end, ok = walkWebP(data)
	}
	if !ok {
		return refuse(ErrMalformed, "the %s structure does not parse", typ)
	}
	assert.That(end > 0 && end <= len(data), "checkStructure: the end of the image is outside the data")
	if end < len(data) {
		return refuse(ErrPolyglot, "%d bytes after the end of the image", len(data)-end)
	}
	for _, m := range meta {
		assert.That(len(m) <= len(data), "checkStructure: metadata is longer than the image")
		if marker := markupIn(m); marker != "" {
			return refuse(ErrPolyglot, "metadata contains %q", marker)
		}
	}
	return nil
}

// markupMarkers returns the strings, lowercased, that make metadata a
// polyglot. They are all longer than a few bytes, and only metadata is
// scanned, so a real photo does not hit one by chance. XMP is XML, so
// "<?xml" is not one.
func markupMarkers() []string {
	return []string{
		"<html", "<head", "<body", "<script", "<iframe", "<object", "<embed",
		"<svg", "<!doctype", "<meta", "<link", "<style", "<form", "<img",
		"javascript:", "<?php",
	}
}

func markupIn(b []byte) string {
	lower := bytes.ToLower(b)
	for _, m := range markupMarkers() {
		assert.That(m != "" && bytes.Equal([]byte(m), bytes.ToLower([]byte(m))), "markupIn: a marker is empty or not lowercase")
		if bytes.Contains(lower, []byte(m)) {
			return m
		}
	}
	return ""
}

// walkPNG returns the data of every chunk but IDAT, and the offset after
// IEND.
func walkPNG(data []byte) (meta [][]byte, end int, ok bool) {
	pos := len(pngSignature())
	for pos+12 <= len(data) {
		start := pos
		n := int(binary.BigEndian.Uint32(data[pos:]))
		typ := string(data[pos+4 : pos+8])
		if n < 0 || n > len(data)-pos-12 {
			return nil, 0, false
		}
		body := data[pos+8 : pos+8+n]
		assert.That(len(body) == n, "walkPNG: the body does not match the length")
		pos += 12 + n
		assert.That(pos > start && pos <= len(data), "walkPNG: a chunk does not end inside the data")
		assert.That(12*len(meta) <= pos-len(pngSignature()), "walkPNG: more chunks than the bytes read can hold")
		switch typ {
		case "IEND":
			return meta, pos, true
		case "IDAT":
		default:
			meta = append(meta, body)
		}
	}
	return nil, 0, false
}

// walkJPEG returns the APPn and COM segments, and the offset after EOI.
func walkJPEG(data []byte) (meta [][]byte, end int, ok bool) {
	pos := 2 // SOI
	for pos < len(data) {
		marker, next, found := jpegMarker(data, pos)
		if !found {
			return nil, 0, false
		}
		assert.That(next > pos && next <= len(data), "walkJPEG: a marker does not end inside the data")
		pos = next
		if marker == 0xD9 { // EOI
			return meta, pos, true
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) { // no length
			continue
		}
		seg, after, found := jpegSegment(data, pos)
		if !found {
			return nil, 0, false
		}
		if (marker >= 0xE0 && marker <= 0xEF) || marker == 0xFE {
			meta = append(meta, seg)
		}
		assert.That(after >= pos+2, "walkJPEG: a segment is shorter than its length field")
		pos = after
		assert.That(4*len(meta) <= pos, "walkJPEG: more segments than the bytes read can hold")
		if marker == 0xDA { // SOS: entropy-coded data up to the next marker
			pos = skipEntropy(data, pos)
		}
		assert.That(pos <= len(data), "walkJPEG: the position is past the data")
	}
	return nil, 0, false
}

// jpegMarker reads the marker at pos, which must be 0xFF, after any fill
// bytes. It returns the marker byte and the offset after it.
func jpegMarker(data []byte, pos int) (marker byte, next int, ok bool) {
	assert.That(pos >= 0 && pos < len(data), "jpegMarker: the position is outside the data")
	if data[pos] != 0xFF {
		return 0, 0, false
	}
	start := pos
	for pos < len(data) && data[pos] == 0xFF { // fill bytes
		pos++
	}
	if pos >= len(data) {
		return 0, 0, false
	}
	assert.That(pos > start, "jpegMarker: no 0xFF was read")
	return data[pos], pos + 1, true
}

// jpegSegment reads the length of the segment at pos and returns its body
// and the offset after it.
func jpegSegment(data []byte, pos int) (seg []byte, next int, ok bool) {
	assert.That(pos >= 0 && pos <= len(data), "jpegSegment: the position is outside the data")
	if pos+2 > len(data) {
		return nil, 0, false
	}
	n := int(binary.BigEndian.Uint16(data[pos:]))
	if n < 2 || pos+n > len(data) {
		return nil, 0, false
	}
	seg = data[pos+2 : pos+n]
	assert.That(len(seg) == n-2, "jpegSegment: the body does not match the length")
	return seg, pos + n, true
}

// skipEntropy skips entropy-coded data from pos up to the next marker. It
// returns the offset of that marker's 0xFF, or len(data) when the data ends
// first (walkJPEG then refuses it).
func skipEntropy(data []byte, pos int) int {
	assert.That(pos >= 0 && pos <= len(data), "skipEntropy: the position is outside the data")
	for pos < len(data) {
		if data[pos] != 0xFF {
			pos++
			continue
		}
		if pos+1 >= len(data) {
			return len(data)
		}
		next := data[pos+1]
		switch {
		case next == 0x00 || (next >= 0xD0 && next <= 0xD7):
			pos += 2
		case next == 0xFF:
			pos++
		default:
			assert.That(pos+1 < len(data), "skipEntropy: the marker byte is outside the data")
			return pos
		}
	}
	assert.That(pos == len(data), "skipEntropy: the loop ended inside the data")
	return pos
}

// walkGIF returns the comment, plain text and application extensions, and
// the offset after the trailer.
func walkGIF(data []byte) (meta [][]byte, end int, ok bool) {
	if len(data) < 13 {
		return nil, 0, false
	}
	pos := 13
	if data[10]&0x80 != 0 {
		pos += 3 << (int(data[10]&0x07) + 1)
	}
	for pos < len(data) {
		start := pos
		b := data[pos]
		pos++
		switch b {
		case 0x3B: // trailer
			return meta, pos, true
		case 0x21: // extension
			body, label, next, found := gifExtension(data, pos)
			if !found {
				return nil, 0, false
			}
			if label == 0xFE || label == 0xFF || label == 0x01 {
				meta = append(meta, body)
			}
			pos = next
		case 0x2C: // image descriptor
			if pos, ok = gifImage(data, pos); !ok {
				return nil, 0, false
			}
		default:
			return nil, 0, false
		}
		assert.That(pos > start && pos <= len(data), "walkGIF: a block does not end inside the data")
		assert.That(3*len(meta) <= pos, "walkGIF: more extensions than the bytes read can hold")
	}
	return nil, 0, false
}

// gifExtension reads the extension whose label is at pos. It returns the
// contents of its data sub-blocks, the label and the offset after it.
func gifExtension(data []byte, pos int) (body []byte, label byte, next int, ok bool) {
	assert.That(pos >= 0 && pos <= len(data), "gifExtension: the position is outside the data")
	if pos >= len(data) {
		return nil, 0, 0, false
	}
	label = data[pos]
	body, next, ok = gifSubBlocks(data, pos+1)
	if !ok {
		return nil, 0, 0, false
	}
	assert.That(next > pos, "gifExtension: no progress")
	assert.That(len(body) < next-pos, "gifExtension: more contents than bytes read")
	return body, label, next, true
}

// gifImage skips the image descriptor whose fields start at pos, its local
// color table and its image data, and returns the offset after them.
func gifImage(data []byte, pos int) (next int, ok bool) {
	assert.That(pos >= 0 && pos <= len(data), "gifImage: the position is outside the data")
	if pos+9 > len(data) {
		return 0, false
	}
	packed := data[pos+8]
	pos += 9
	if packed&0x80 != 0 {
		pos += 3 << (int(packed&0x07) + 1)
	}
	pos++ // LZW minimum code size
	if pos > len(data) {
		return 0, false
	}
	_, next, ok = gifSubBlocks(data, pos)
	if !ok {
		return 0, false
	}
	assert.That(next > pos, "gifImage: no progress")
	return next, true
}

// gifSubBlocks skips a run of data sub-blocks from pos. It returns their
// contents and the offset after the terminating empty block.
func gifSubBlocks(data []byte, pos int) (out []byte, next int, ok bool) {
	assert.That(pos >= 0, "gifSubBlocks: negative position")
	start := pos
	for pos < len(data) {
		n := int(data[pos])
		pos++
		if n == 0 {
			assert.That(pos <= len(data), "gifSubBlocks: the position is past the data")
			assert.That(len(out) < pos-start, "gifSubBlocks: more contents than bytes read")
			return out, pos, true
		}
		if pos+n > len(data) {
			return nil, 0, false
		}
		out = append(out, data[pos:pos+n]...)
		pos += n
	}
	return nil, 0, false
}

// webpImageChunk reports whether a WebP chunk holds image data; any other
// chunk (EXIF, XMP, ICCP, an unknown one) is metadata.
func webpImageChunk(fourcc []byte) bool {
	assert.That(len(fourcc) == 4, "webpImageChunk: a FourCC is four bytes")
	switch string(fourcc) {
	case "VP8\x20", "VP8L", "VP8X", "ALPH", "ANIM", "ANMF":
		return true
	}
	return false
}

// walkWebP returns the metadata chunks, and the offset after the RIFF
// container.
func walkWebP(data []byte) (meta [][]byte, end int, ok bool) {
	if len(data) < 12 {
		return nil, 0, false
	}
	size := int64(binary.LittleEndian.Uint32(data[4:]))
	if size < 4 || 8+size > int64(len(data)) {
		return nil, 0, false
	}
	end = 8 + int(size)
	assert.That(end >= 12 && end <= len(data), "walkWebP: the container does not end inside the data")
	pos := 12
	for pos < end {
		start := pos
		if pos+8 > end {
			return nil, 0, false
		}
		fourcc := data[pos : pos+4]
		n := int64(binary.LittleEndian.Uint32(data[pos+4:]))
		if int64(pos)+8+n > int64(end) {
			return nil, 0, false
		}
		body := data[pos+8 : pos+8+int(n)]
		assert.That(int64(len(body)) == n, "walkWebP: the body does not match the length")
		pos += 8 + int(n) + int(n&1)
		assert.That(pos > start && pos <= end+1, "walkWebP: a chunk does not end inside the container")
		if !webpImageChunk(fourcc) {
			meta = append(meta, body)
		}
		assert.That(8*len(meta) <= pos-12, "walkWebP: more chunks than the bytes read can hold")
	}
	return meta, end, true
}
