package imagecheck

import (
	"bytes"
	"encoding/binary"
)

// checkStructure walks the container format of data, which Sniff says is
// typ. It refuses (ErrPolyglot) any byte after the end of the image, and
// any metadata that holds markup a browser or another parser would act on:
// an HTML page in a PNG text chunk or a JPEG comment, say. Pixel data is not
// scanned, since compressed bytes can spell anything by chance.
func checkStructure(typ string, data []byte) error {
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
	if end < len(data) {
		return refuse(ErrPolyglot, "%d bytes after the end of the image", len(data)-end)
	}
	for _, m := range meta {
		if marker := markupIn(m); marker != "" {
			return refuse(ErrPolyglot, "metadata contains %q", marker)
		}
	}
	return nil
}

// markupMarkers are the strings, lowercased, that make metadata a polyglot.
// They are all longer than a few bytes, and only metadata is scanned, so a
// real photo does not hit one by chance. XMP is XML, so "<?xml" is not one.
var markupMarkers = []string{
	"<html", "<head", "<body", "<script", "<iframe", "<object", "<embed",
	"<svg", "<!doctype", "<meta", "<link", "<style", "<form", "<img",
	"javascript:", "<?php",
}

func markupIn(b []byte) string {
	lower := bytes.ToLower(b)
	for _, m := range markupMarkers {
		if bytes.Contains(lower, []byte(m)) {
			return m
		}
	}
	return ""
}

// walkPNG returns the data of every chunk but IDAT, and the offset after
// IEND.
func walkPNG(data []byte) (meta [][]byte, end int, ok bool) {
	pos := len(pngSignature)
	for pos+12 <= len(data) {
		n := int(binary.BigEndian.Uint32(data[pos:]))
		typ := string(data[pos+4 : pos+8])
		if n < 0 || n > len(data)-pos-12 {
			return nil, 0, false
		}
		body := data[pos+8 : pos+8+n]
		pos += 12 + n
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
	for {
		if pos >= len(data) || data[pos] != 0xFF {
			return nil, 0, false
		}
		for pos < len(data) && data[pos] == 0xFF { // fill bytes
			pos++
		}
		if pos >= len(data) {
			return nil, 0, false
		}
		marker := data[pos]
		pos++
		switch {
		case marker == 0xD9: // EOI
			return meta, pos, true
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7): // no length
			continue
		}
		if pos+2 > len(data) {
			return nil, 0, false
		}
		n := int(binary.BigEndian.Uint16(data[pos:]))
		if n < 2 || pos+n > len(data) {
			return nil, 0, false
		}
		if (marker >= 0xE0 && marker <= 0xEF) || marker == 0xFE {
			meta = append(meta, data[pos+2:pos+n])
		}
		pos += n
		if marker == 0xDA { // SOS: entropy-coded data up to the next marker
			for pos < len(data) {
				if data[pos] != 0xFF {
					pos++
					continue
				}
				if pos+1 >= len(data) {
					return nil, 0, false
				}
				next := data[pos+1]
				if next == 0x00 || (next >= 0xD0 && next <= 0xD7) {
					pos += 2
					continue
				}
				if next == 0xFF {
					pos++
					continue
				}
				break
			}
		}
	}
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
	// subBlocks skips a run of data sub-blocks and returns their contents.
	subBlocks := func() ([]byte, bool) {
		var out []byte
		for pos < len(data) {
			n := int(data[pos])
			pos++
			if n == 0 {
				return out, true
			}
			if pos+n > len(data) {
				return nil, false
			}
			out = append(out, data[pos:pos+n]...)
			pos += n
		}
		return nil, false
	}
	for pos < len(data) {
		b := data[pos]
		pos++
		switch b {
		case 0x3B: // trailer
			return meta, pos, true
		case 0x21: // extension
			if pos >= len(data) {
				return nil, 0, false
			}
			label := data[pos]
			pos++
			body, ok := subBlocks()
			if !ok {
				return nil, 0, false
			}
			if label == 0xFE || label == 0xFF || label == 0x01 {
				meta = append(meta, body)
			}
		case 0x2C: // image descriptor
			if pos+9 > len(data) {
				return nil, 0, false
			}
			packed := data[pos+8]
			pos += 9
			if packed&0x80 != 0 {
				pos += 3 << (int(packed&0x07) + 1)
			}
			pos++ // LZW minimum code size
			if pos > len(data) {
				return nil, 0, false
			}
			if _, ok := subBlocks(); !ok {
				return nil, 0, false
			}
		default:
			return nil, 0, false
		}
	}
	return nil, 0, false
}

// webpImageChunk reports whether a WebP chunk holds image data; any other
// chunk (EXIF, XMP, ICCP, an unknown one) is metadata.
func webpImageChunk(fourcc []byte) bool {
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
	pos := 12
	for pos < end {
		if pos+8 > end {
			return nil, 0, false
		}
		fourcc := data[pos : pos+4]
		n := int64(binary.LittleEndian.Uint32(data[pos+4:]))
		if int64(pos)+8+n > int64(end) {
			return nil, 0, false
		}
		body := data[pos+8 : pos+8+int(n)]
		pos += 8 + int(n) + int(n&1)
		if !webpImageChunk(fourcc) {
			meta = append(meta, body)
		}
	}
	return meta, end, true
}
