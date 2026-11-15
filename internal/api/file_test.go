package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/blob"
	"github.com/infrashift/chit/internal/model"
)

// Attachments (P-CHIT-5), through the whole router: AuthExtract, the
// OpenAPI validator (which reads the multipart body too) and the handlers.

const otherChannelID = "019421a0-0000-7000-8000-000000000021"

type fileEnv struct {
	*bridgeEnv
	blobs *blob.Memory
}

func newFileEnv(t *testing.T) *fileEnv {
	t.Helper()
	e := newBridgeEnv(t)
	blobs := blob.NewMemory()
	e.a.Blobs = blobs
	// extraUser (alice, kratos-002) is a member too; thirdUser is not.
	e.ms.Channels.SeedMember(&model.ChannelMember{ChannelID: testChannelID, UserID: extraUserID, CreateAt: 1000})
	// A second channel that testUser is in.
	e.ms.Channels.Seed(&model.Channel{ID: otherChannelID, TeamID: testTeamID, CreatorID: testUserID,
		Name: "random", DisplayName: "Random", Type: "O", CreateAt: 1000, UpdateAt: 1000})
	e.ms.Channels.SeedMember(&model.ChannelMember{ChannelID: otherChannelID, UserID: testUserID, CreateAt: 1000})
	return &fileEnv{bridgeEnv: e, blobs: blobs}
}

const (
	me    = "user:" + testKratosID // testUser
	alice = "user:kratos-002"      // extraUser, a member
	carol = "user:kratos-003"      // thirdUser, not a member
	rogue = "client:" + otherClientID
)

// ─── Fixtures ────────────────────────────────────────────────────

func testPNG(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 6, 4))
	for i := 0; i < 24; i++ {
		img.Set(i%6, i/6, c)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func chunk(typ string, body []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	out = append(out, typ...)
	out = append(out, body...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(typ), body...)))
}

// htmlPolyglotPNG is a valid PNG with an HTML page in a tEXt chunk.
func htmlPolyglotPNG(t *testing.T) []byte {
	src := testPNG(t, color.White)
	at := 8 + 12 + 13 // after IHDR
	out := append([]byte{}, src[:at]...)
	out = append(out, chunk("tEXt", []byte("Comment\x00<html><script>alert(1)</script></html>"))...)
	return append(out, src[at:]...)
}

// hugeHeaderPNG claims 10000×10000 in 57 bytes.
func hugeHeaderPNG() []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, 10000)
	ihdr = binary.BigEndian.AppendUint32(ihdr, 10000)
	ihdr = append(ihdr, 8, 2, 0, 0, 0)
	out := []byte("\x89PNG\r\n\x1a\n")
	out = append(out, chunk("IHDR", ihdr)...)
	out = append(out, chunk("IDAT", []byte{0x78, 0x9c, 0x03, 0x00})...)
	return append(out, chunk("IEND", nil)...)
}

const gpsMarker = "GPS 51.5007N 0.1246W"

// jpegWithEXIFGPS has an APP1 Exif segment with a GPS IFD.
func jpegWithEXIFGPS(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	tiff := le.AppendUint32([]byte("II*\x00"), 8)
	tiff = le.AppendUint16(tiff, 1)
	tiff = le.AppendUint16(tiff, 0x8825)
	tiff = le.AppendUint16(tiff, 4)
	tiff = le.AppendUint32(tiff, 1)
	tiff = le.AppendUint32(tiff, 26)
	tiff = le.AppendUint32(tiff, 0)
	tiff = le.AppendUint16(tiff, 1)
	tiff = le.AppendUint16(tiff, 0x0001)
	tiff = le.AppendUint16(tiff, 2)
	tiff = le.AppendUint32(tiff, 2)
	tiff = append(tiff, 'N', 0, 0, 0)
	tiff = le.AppendUint32(tiff, 0)
	tiff = append(tiff, gpsMarker...)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := binary.BigEndian.AppendUint16([]byte{0xFF, 0xE1}, uint16(len(payload)+2))
	src := b.Bytes()
	out := append(append([]byte{}, src[:2]...), append(seg, payload...)...)
	return append(out, src[2:]...)
}

// ─── Requests ────────────────────────────────────────────────────

type part struct {
	field, filename, ctype string
	data                   []byte
}

func filePart(name, ctype string, data []byte) part { return part{"file", name, ctype, data} }
func altPart(text string) part                      { return part{"alt_text", "", "", []byte(text)} }

func (e *fileEnv) upload(t *testing.T, channelID, who string, headers map[string]string, parts ...part) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		disp := fmt.Sprintf(`form-data; name=%q`, p.field)
		if p.filename != "" {
			disp += fmt.Sprintf(`; filename=%q`, p.filename)
		}
		h.Set("Content-Disposition", disp)
		if p.ctype != "" {
			h.Set("Content-Type", p.ctype)
		}
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(p.data)
	}
	_ = mw.Close()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/channels/"+channelID+"/files", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	setWho(r, who)
	for k, v := range headers {
		r.Header[k] = []string{v}
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, r)
	return w
}

func setWho(r *http.Request, who string) {
	switch kind, id, _ := strings.Cut(who, ":"); kind {
	case "client":
		r.Header.Set("X-Client-Id", id)
	case "user":
		r.Header.Set("X-User-Id", id)
	}
}

func (e *fileEnv) mustUpload(t *testing.T, channelID, who string, headers map[string]string, data []byte, alt string) *model.File {
	t.Helper()
	w := e.upload(t, channelID, who, headers, filePart("photo.png", "image/png", data), altPart(alt))
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var f model.File
	decodeJSON(t, w.Body, &f)
	return &f
}

func (e *fileEnv) get(t *testing.T, path, who string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, path, who, "", headers)
}

func (e *fileEnv) attach(t *testing.T, who string, headers map[string]string, content string, fileIDs ...string) *httptest.ResponseRecorder {
	t.Helper()
	ids, _ := json.Marshal(fileIDs)
	body := fmt.Sprintf(`{"channel_id":%q,"content":%q,"file_ids":%s}`, testChannelID, content, ids)
	return e.do(t, http.MethodPost, "/api/v1/posts", who, body, headers)
}

func errorID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e model.AppError
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("not an AppError: %s", w.Body.String())
	}
	return e.ID
}

func requireFileHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	h := w.Header()
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("HTTP %d: X-Content-Type-Options = %q", w.Code, h.Get("X-Content-Type-Options"))
	}
	if h.Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("HTTP %d: Content-Security-Policy = %q", w.Code, h.Get("Content-Security-Policy"))
	}
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") {
		t.Errorf("HTTP %d: Content-Disposition = %q", w.Code, h.Get("Content-Disposition"))
	}
}

// ─── The flow ────────────────────────────────────────────────────

func TestFiles_UploadAttachDownload(t *testing.T) {
	e := newFileEnv(t)
	f := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "a black rectangle")
	if f.UploaderID != testUserID || f.ViaActorID != "" || f.ChannelID != testChannelID ||
		f.ContentType != "image/png" || f.Width != 6 || f.Height != 4 || f.AltText != "a black rectangle" ||
		f.Name != "photo.png" || f.PostID != "" || len(f.SHA256) != 64 {
		t.Fatalf("file = %+v", f)
	}
	if keys := e.blobs.Keys(); len(keys) != 1 || keys[0] != testChannelID+"/"+f.ID {
		t.Fatalf("objects %v, want <channel>/<file>", keys)
	}

	// Before it is attached, only the uploader reads it.
	if w := e.get(t, "/api/v1/files/"+f.ID, alice, nil); w.Code != http.StatusForbidden {
		t.Fatalf("another member before attaching: %d, want 403", w.Code)
	}
	w := e.get(t, "/api/v1/files/"+f.ID, me, nil)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("uploader: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	stored, _ := e.blobs.Object(testChannelID + "/" + f.ID)
	if !bytes.Equal(w.Body.Bytes(), stored) {
		t.Fatal("the download is not the stored object")
	}

	// A post of the image alone.
	w = e.attach(t, me, nil, "", f.ID)
	if w.Code != http.StatusCreated {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}
	var post model.Post
	decodeJSON(t, w.Body, &post)
	if len(post.FileIDs) != 1 || post.FileIDs[0] != f.ID {
		t.Fatalf("post file_ids = %v", post.FileIDs)
	}
	got, err := e.ms.Posts.Get(t.Context(), post.ID)
	if err != nil || len(got.FileIDs) != 1 {
		t.Fatalf("stored post: %+v %v", got, err)
	}

	// Now every member reads it, and its info names the post.
	if w = e.get(t, "/api/v1/files/"+f.ID, alice, nil); w.Code != http.StatusOK {
		t.Fatalf("another member after attaching: %d", w.Code)
	}
	w = e.get(t, "/api/v1/files/"+f.ID+"/info", alice, nil)
	var info model.File
	decodeJSON(t, w.Body, &info)
	if w.Code != http.StatusOK || info.PostID != post.ID {
		t.Fatalf("info: %d %+v", w.Code, info)
	}
	if strings.Contains(w.Body.String(), "s3") {
		t.Errorf("info exposes the object key: %s", w.Body.String())
	}
}

func TestFiles_NonMemberCannotDownload(t *testing.T) {
	e := newFileEnv(t)
	f := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "")
	if w := e.attach(t, me, nil, "look", f.ID); w.Code != http.StatusCreated {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/v1/files/" + f.ID, "/api/v1/files/" + f.ID + "/info"} {
		w := e.get(t, path, carol, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s as a non-member: %d, want 403 (Chit answers 403 to a non-member, as for posts)", path, w.Code)
		}
		requireFileHeaders(t, w)
	}
	if w := e.get(t, "/api/v1/files/"+model.NewID(), me, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown file: %d, want 404", w.Code)
	}
}

func TestFiles_HeadersOnEveryResponse(t *testing.T) {
	e := newFileEnv(t)
	f := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "")
	cases := map[string]*httptest.ResponseRecorder{
		"upload 201": e.upload(t, testChannelID, me, nil, filePart("a.png", "image/png", testPNG(t, color.White))),
		"upload 415": e.upload(t, testChannelID, me, nil, filePart("a.svg", "image/svg+xml", []byte("<svg/>"))),
		"upload 401": e.upload(t, testChannelID, "", nil, filePart("a.png", "image/png", testPNG(t, color.White))),
		"upload 403": e.upload(t, testChannelID, rogue, obo(testUserID), filePart("a.png", "image/png", testPNG(t, color.White))),
		"get 200":    e.get(t, "/api/v1/files/"+f.ID, me, nil),
		"get 404":    e.get(t, "/api/v1/files/"+model.NewID(), me, nil),
		"get 401":    e.get(t, "/api/v1/files/"+f.ID, "", nil),
		"info 200":   e.get(t, "/api/v1/files/"+f.ID+"/info", me, nil),
		"search 200": e.do(t, http.MethodPost, "/api/v1/files/search", me, `{"terms":"photo"}`, nil),
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			if want := name[len(name)-3:]; fmt.Sprint(w.Code) != want {
				t.Fatalf("got %d, want %s: %s", w.Code, want, w.Body.String())
			}
			requireFileHeaders(t, w)
		})
	}
	// The download names the file.
	if cd := cases["get 200"].Header().Get("Content-Disposition"); cd != `attachment; filename=photo.png` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	// And other routes are untouched.
	if w := e.get(t, "/api/v1/channels/"+testChannelID+"/posts", me, nil); w.Header().Get("Content-Security-Policy") != "" {
		t.Errorf("a post route got the file headers")
	}
}

// ─── The validation pipeline, through the API ────────────────────

func TestFiles_UploadRefusals(t *testing.T) {
	e := newFileEnv(t)
	pngData := testPNG(t, color.Black)
	cases := []struct {
		name    string
		channel string
		who     string
		parts   []part
		status  int
		id      string
	}{
		{"SVG", testChannelID, me, []part{filePart("x.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))}, 415, "file.svg"},
		{"SVG declared as PNG", testChannelID, me, []part{filePart("x.png", "image/png", []byte(`<svg onload="alert(1)"/>`))}, 415, "file.svg"},
		{"PNG/HTML polyglot", testChannelID, me, []part{filePart("x.png", "image/png", htmlPolyglotPNG(t))}, 422, "file.polyglot"},
		{"PNG with HTML appended", testChannelID, me, []part{filePart("x.png", "image/png", append(testPNG(t, color.White), "<html>"...))}, 422, "file.polyglot"},
		{"declared JPEG, sniffed PNG", testChannelID, me, []part{filePart("x.jpg", "image/jpeg", pngData)}, 415, "file.type_mismatch"},
		{"HTML declared as PNG", testChannelID, me, []part{filePart("x.png", "image/png", []byte("<html><body>hi</body></html>"))}, 415, "file.type_mismatch"},
		{"BMP", testChannelID, me, []part{filePart("x.bmp", "image/bmp", pngData)}, 415, "file.unsupported_type"},
		{"100 MP in the header", testChannelID, me, []part{filePart("x.png", "image/png", hugeHeaderPNG())}, 422, "file.too_many_pixels"},
		{"over 10 MiB", testChannelID, me, []part{filePart("x.png", "image/png", make([]byte, 10<<20+1))}, 413, "file.too_large"},
		{"not a member", testChannelID, carol, []part{filePart("x.png", "image/png", pngData)}, 403, ""},
		{"unknown channel", model.NewID(), me, []part{filePart("x.png", "image/png", pngData)}, 404, ""},
		{"no file part", testChannelID, me, []part{altPart("nothing")}, 400, ""},
		{"two file parts", testChannelID, me, []part{filePart("x.png", "image/png", pngData), filePart("y.png", "image/png", pngData)}, 400, ""},
		{"alt text too long", testChannelID, me, []part{filePart("x.png", "image/png", pngData), altPart(strings.Repeat("a", 1001))}, 400, ""},
		{"alt text with a control character", testChannelID, me, []part{filePart("x.png", "image/png", pngData), altPart("a\x00b")}, 400, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := e.upload(t, tc.channel, tc.who, nil, tc.parts...)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.id != "" && errorID(t, w) != tc.id {
				t.Fatalf("error id %q, want %q", errorID(t, w), tc.id)
			}
			requireFileHeaders(t, w)
		})
	}
	if keys := e.blobs.Keys(); len(keys) != 0 {
		t.Fatalf("refused uploads stored objects: %v", keys)
	}
}

func TestFiles_EXIFGPSIsGoneAfterUpload(t *testing.T) {
	e := newFileEnv(t)
	in := jpegWithEXIFGPS(t)
	w := e.upload(t, testChannelID, me, nil, filePart("IMG_0001.JPG", "image/jpeg", in))
	if w.Code != http.StatusCreated {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var f model.File
	decodeJSON(t, w.Body, &f)
	if f.ContentType != "image/jpeg" || f.Name != "IMG_0001.jpg" {
		t.Fatalf("file %+v", f)
	}
	got := e.get(t, "/api/v1/files/"+f.ID, me, nil).Body.Bytes()
	if bytes.Contains(got, []byte("Exif")) || bytes.Contains(got, []byte(gpsMarker)) {
		t.Fatal("the stored image still carries EXIF GPS")
	}
}

func TestFiles_Quota(t *testing.T) {
	e := newFileEnv(t)
	first := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "")
	e.a.Config.FileQuotaBytes = first.SizeBytes + 10
	w := e.upload(t, testChannelID, me, nil, filePart("b.png", "image/png", testPNG(t, color.White)))
	if w.Code != http.StatusRequestEntityTooLarge || errorID(t, w) != "file.quota_exceeded" {
		t.Fatalf("over quota: %d %s", w.Code, w.Body.String())
	}
	// The quota is per person: alice still has room.
	e.mustUpload(t, testChannelID, alice, nil, testPNG(t, color.Black), "")
}

func TestFiles_DisabledWithoutAnObjectStore(t *testing.T) {
	e := newFileEnv(t)
	f := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "")
	e.a.Blobs = nil
	if w := e.upload(t, testChannelID, me, nil, filePart("a.png", "image/png", testPNG(t, color.White))); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("upload: %d", w.Code)
	}
	if w := e.get(t, "/api/v1/files/"+f.ID, me, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("download: %d", w.Code)
	}
	if w := e.attach(t, me, nil, "x", f.ID); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("attach: %d", w.Code)
	}
}

// ─── Deduplication ───────────────────────────────────────────────

func TestFiles_DedupWithinAChannelOnly(t *testing.T) {
	e := newFileEnv(t)
	data := testPNG(t, color.Black)
	a1 := e.mustUpload(t, testChannelID, me, nil, data, "one")
	a2 := e.mustUpload(t, testChannelID, alice, nil, data, "two")
	if a1.ID == a2.ID || a1.SHA256 != a2.SHA256 {
		t.Fatalf("two uploads: %s %s", a1.ID, a2.ID)
	}
	if keys := e.blobs.Keys(); len(keys) != 1 {
		t.Fatalf("the same bytes twice in one channel stored %d objects, want 1: %v", len(keys), keys)
	}
	if a2.UploaderID != extraUserID || a2.AltText != "two" {
		t.Fatalf("the second file lost its own metadata: %+v", a2)
	}

	b := e.mustUpload(t, otherChannelID, me, nil, data, "elsewhere")
	keys := e.blobs.Keys()
	if len(keys) != 2 || !contains(keys, otherChannelID+"/"+b.ID) {
		t.Fatalf("the same bytes in another channel did not get their own object: %v", keys)
	}
	stored, _ := e.ms.Files.Get(t.Context(), b.ID)
	if !strings.HasPrefix(stored.S3Key, otherChannelID+"/") {
		t.Fatalf("a file of %s uses the object %s", otherChannelID, stored.S3Key)
	}
}

// ─── Attaching ───────────────────────────────────────────────────

func TestFiles_AttachRules(t *testing.T) {
	e := newFileEnv(t)
	mine := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "")
	alices := e.mustUpload(t, testChannelID, alice, nil, testPNG(t, color.White), "")
	elsewhere := e.mustUpload(t, otherChannelID, me, nil, testPNG(t, color.Gray{Y: 7}), "")

	five := make([]string, 0, 5)
	for i := range 5 {
		five = append(five, e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Gray{Y: uint8(100 + i)}), "").ID)
	}

	cases := []struct {
		name   string
		ids    []string
		status int
	}{
		{"somebody else's file", []string{alices.ID}, 400},
		{"a file of another channel", []string{elsewhere.ID}, 400},
		{"an unknown file", []string{model.NewID()}, 400},
		{"the same file twice", []string{mine.ID, mine.ID}, 400},
		{"more than four", five, 400},
		{"not a UUID", []string{"nope"}, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := e.attach(t, me, nil, "x", tc.ids...); w.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}

	if w := e.attach(t, me, nil, "four is fine", five[:4]...); w.Code != http.StatusCreated {
		t.Fatalf("four files: %d %s", w.Code, w.Body.String())
	}
	if w := e.attach(t, me, nil, "first", mine.ID); w.Code != http.StatusCreated {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}
	if w := e.attach(t, me, nil, "again", mine.ID); w.Code != http.StatusConflict {
		t.Fatalf("a file already attached: %d, want 409", w.Code)
	}
	// A post with neither text nor files is still refused.
	if w := e.do(t, http.MethodPost, "/api/v1/posts", me, `{"channel_id":"`+testChannelID+`","content":""}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("empty post: %d", w.Code)
	}
}

// ─── Deleting and sweeping ───────────────────────────────────────

func TestFiles_DeletingThePostDeletesItsFiles(t *testing.T) {
	e := newFileEnv(t)
	e.a.Config.FileOrphanGrace = time.Millisecond
	f := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "")
	w := e.attach(t, me, nil, "bye", f.ID)
	var post model.Post
	decodeJSON(t, w.Body, &post)

	e.mustOK(t, http.MethodDelete, "/api/v1/posts/"+post.ID, me, "")
	if w := e.get(t, "/api/v1/files/"+f.ID, me, nil); w.Code != http.StatusNotFound {
		t.Fatalf("a deleted post's file: %d, want 404", w.Code)
	}
	if len(e.blobs.Keys()) != 1 {
		t.Fatal("the object went before the grace period")
	}
	time.Sleep(5 * time.Millisecond)
	if _, purged, err := e.a.SweepFiles(t.Context()); err != nil || purged != 1 {
		t.Fatalf("sweep: purged %d, %v", purged, err)
	}
	if keys := e.blobs.Keys(); len(keys) != 0 {
		t.Fatalf("objects after the sweep: %v", keys)
	}
}

func TestFiles_SweepRemovesUnattachedUploadsButKeepsSharedObjects(t *testing.T) {
	e := newFileEnv(t)
	e.a.Config.FileOrphanGrace = time.Millisecond
	data := testPNG(t, color.Black)
	orphan := e.mustUpload(t, testChannelID, me, nil, data, "")
	shared := e.mustUpload(t, testChannelID, alice, nil, data, "")
	lonely := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.White), "")
	if w := e.attach(t, alice, nil, "kept", shared.ID); w.Code != http.StatusCreated {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}
	time.Sleep(5 * time.Millisecond)

	expired, _, err := e.a.SweepFiles(t.Context())
	if err != nil || expired != 2 {
		t.Fatalf("first pass expired %d (want the 2 unattached), %v", expired, err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, purged, err := e.a.SweepFiles(t.Context()); err != nil || purged != 2 {
		t.Fatalf("second pass purged %d, want 2: %v", purged, err)
	}
	keys := e.blobs.Keys()
	// orphan's object is shared's too, and shared is live: it stays.
	if len(keys) != 1 || keys[0] != testChannelID+"/"+orphan.ID {
		t.Fatalf("objects after the sweep: %v (lonely was %s)", keys, lonely.ID)
	}
	if w := e.get(t, "/api/v1/files/"+shared.ID, me, nil); w.Code != http.StatusOK {
		t.Fatalf("the attached copy: %d", w.Code)
	}
}

func TestFiles_SweepIsBounded(t *testing.T) {
	e := newFileEnv(t)
	e.a.Config.FileOrphanGrace = time.Millisecond
	e.a.Config.FileSweepBatch = 2
	for i := range 5 {
		e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Gray{Y: uint8(i)}), "")
	}
	time.Sleep(5 * time.Millisecond)
	if expired, _, _ := e.a.SweepFiles(t.Context()); expired != 2 {
		t.Fatalf("expired %d in one pass, want the batch of 2", expired)
	}
}

// ─── Search ──────────────────────────────────────────────────────

func TestFiles_SearchByNameAndAltTextInTheReadersChannels(t *testing.T) {
	e := newFileEnv(t)
	byAlt := e.mustUpload(t, testChannelID, me, nil, testPNG(t, color.Black), "the Whiteboard after standup")
	w := e.upload(t, testChannelID, me, nil, filePart("whiteboard-2.png", "image/png", testPNG(t, color.White)))
	var byName model.File
	decodeJSON(t, w.Body, &byName)
	elsewhere := e.mustUpload(t, otherChannelID, me, nil, testPNG(t, color.Gray{Y: 9}), "whiteboard in random")
	alicesUnattached := e.mustUpload(t, testChannelID, alice, nil, testPNG(t, color.Gray{Y: 10}), "alice's whiteboard")
	for _, id := range []string{byAlt.ID, byName.ID} {
		if w := e.attach(t, me, nil, "x", id); w.Code != http.StatusCreated {
			t.Fatal(w.Body.String())
		}
	}

	search := func(who, body string) []string {
		t.Helper()
		w := e.do(t, http.MethodPost, "/api/v1/files/search", who, body, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("search %s as %s: %d %s", body, who, w.Code, w.Body.String())
		}
		var files []model.File
		decodeJSON(t, w.Body, &files)
		ids := []string{}
		for _, f := range files {
			ids = append(ids, f.ID)
		}
		return ids
	}

	// testUser: both attached ones and the one in random; not alice's
	// unattached upload.
	got := search(me, `{"terms":"WHITEBOARD"}`)
	if len(got) != 3 || !contains(got, byAlt.ID) || !contains(got, byName.ID) || !contains(got, elsewhere.ID) {
		t.Fatalf("me: %v", got)
	}
	// alice is not in random, and sees her own unattached upload.
	got = search(alice, `{"terms":"whiteboard"}`)
	if len(got) != 3 || contains(got, elsewhere.ID) || !contains(got, alicesUnattached.ID) {
		t.Fatalf("alice: %v", got)
	}
	// One channel.
	if got = search(me, `{"terms":"whiteboard","channel_id":"`+otherChannelID+`"}`); len(got) != 1 || got[0] != elsewhere.ID {
		t.Fatalf("one channel: %v", got)
	}
	// carol is in neither.
	if got = search(carol, `{"terms":"whiteboard"}`); len(got) != 0 {
		t.Fatalf("carol: %v", got)
	}
	if w := e.do(t, http.MethodPost, "/api/v1/files/search", carol, `{"terms":"x","channel_id":"`+testChannelID+`"}`, nil); w.Code != http.StatusForbidden {
		t.Fatalf("a channel carol is not in: %d", w.Code)
	}
	// The image bytes are not searched: "IHDR" is in every PNG.
	if got = search(me, `{"terms":"IHDR"}`); len(got) != 0 {
		t.Fatalf("searched the bytes: %v", got)
	}
}

// ─── On behalf of ────────────────────────────────────────────────

func TestFiles_OnBehalfOf(t *testing.T) {
	e := newFileEnv(t)
	w := e.upload(t, testChannelID, halod, obo(testUserID), filePart("gen.png", "image/png", testPNG(t, color.Black)), altPart("generated"))
	if w.Code != http.StatusCreated {
		t.Fatalf("upload on behalf: %d %s", w.Code, w.Body.String())
	}
	var f model.File
	decodeJSON(t, w.Body, &f)
	if f.UploaderID != testUserID || f.ViaActorID != haloAgentID {
		t.Fatalf("uploader %s via %s, want the person via halod", f.UploaderID, f.ViaActorID)
	}
	data, err := os.ReadFile(e.audit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"action":"file_uploaded"`) || !strings.Contains(string(data), f.ID) {
		t.Fatalf("no file_uploaded audit record: %s", data)
	}

	// halod reads it as the person (it is not attached yet, so as itself it
	// may not), then posts it as the person.
	if w := e.get(t, "/api/v1/files/"+f.ID, halod, nil); w.Code != http.StatusForbidden {
		t.Fatalf("halod as itself, before attaching: %d, want 403", w.Code)
	}
	if w := e.get(t, "/api/v1/files/"+f.ID, halod, obo(testUserID)); w.Code != http.StatusOK {
		t.Fatalf("halod on behalf of the uploader: %d %s", w.Code, w.Body.String())
	}
	if w := e.attach(t, halod, obo(testUserID), "here it is", f.ID); w.Code != http.StatusCreated {
		t.Fatalf("attach on behalf: %d %s", w.Code, w.Body.String())
	}
	// Downloads on behalf of a person who is not a member are refused,
	// although halod itself is one.
	if w := e.get(t, "/api/v1/files/"+f.ID, halod, obo(thirdUserID)); w.Code != http.StatusForbidden {
		t.Fatalf("on behalf of a non-member: %d, want 403", w.Code)
	}
}

func TestFiles_OnBehalfOfRefusals(t *testing.T) {
	e := newFileEnv(t)
	pngData := testPNG(t, color.Black)
	cases := []struct {
		name, who, principal string
		want                 int
	}{
		{"a client not on the allowlist", rogue, testUserID, http.StatusForbidden},
		{"a person's session", me, extraUserID, http.StatusForbidden},
		{"a principal who is not a member", halod, thirdUserID, http.StatusForbidden},
		{"an unknown principal", halod, unknownPersonID, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := e.upload(t, testChannelID, tc.who, obo(tc.principal), filePart("x.png", "image/png", pngData))
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
	if keys := e.blobs.Keys(); len(keys) != 0 {
		t.Fatalf("a refused upload stored %v", keys)
	}
	// Download by a client not on the list.
	f := e.mustUpload(t, testChannelID, me, nil, pngData, "")
	if w := e.get(t, "/api/v1/files/"+f.ID, rogue, obo(testUserID)); w.Code != http.StatusForbidden {
		t.Fatalf("download by a client not on the list: %d", w.Code)
	}
	// /info is not an on-behalf-of route.
	if w := e.get(t, "/api/v1/files/"+f.ID+"/info", halod, obo(testUserID)); w.Code != http.StatusBadRequest {
		t.Fatalf("info with the header: %d, want 400", w.Code)
	}
}
