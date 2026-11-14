package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/crueber/loom/internal/store"
)

func newSQLiteMux(t *testing.T) *http.ServeMux {
	t.Helper()
	st, err := store.OpenSQLite(filepath.Join(t.TempDir(), "img.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h := &Handler{Store: st}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

// Upload an 800x600 PNG, expect stored URLs, then fetch the thumbnail
// and confirm it is a small JPEG (long edge <= 480).
func TestImageUploadAndThumb(t *testing.T) {
	mux := newSQLiteMux(t)

	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	// A flat image compresses to a PNG smaller than any JPEG thumbnail, so the
	// byte-size check below only holds for real content. Fill with deterministic
	// noise (alpha opaque) so the source is a realistic photo-sized file.
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = uint8(rnd.Intn(256))
		img.Pix[i+1] = uint8(rnd.Intn(256))
		img.Pix[i+2] = uint8(rnd.Intn(256))
		img.Pix[i+3] = 255
	}
	var pbuf bytes.Buffer
	if err := png.Encode(&pbuf, img); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(pbuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/images", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var up struct {
		URL      string `json:"url"`
		ThumbURL string `json:"thumb_url"`
		Width    int    `json:"width"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &up); err != nil {
		t.Fatal(err)
	}
	if up.Width != 800 || up.URL == "" || up.ThumbURL == "" {
		t.Fatalf("unexpected upload response: %+v", up)
	}

	treq := httptest.NewRequest("GET", up.ThumbURL, nil)
	trec := httptest.NewRecorder()
	mux.ServeHTTP(trec, treq)
	if trec.Code != 200 {
		t.Fatalf("thumb: %d", trec.Code)
	}
	if ct := trec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("thumb content-type = %q", ct)
	}
	if trec.Header().Get("Cache-Control") == "" {
		t.Fatal("thumb missing Cache-Control")
	}
	thumb, _, err := image.Decode(bytes.NewReader(trec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if dx := thumb.Bounds().Dx(); dx != 480 {
		t.Fatalf("thumb long edge = %d, want 480", dx)
	}
	if len(trec.Body.Bytes()) >= pbuf.Len() {
		t.Fatal("thumbnail should be smaller than the 800x600 source PNG")
	}

	// Non-image upload must be rejected.
	var bad bytes.Buffer
	bw := multipart.NewWriter(&bad)
	fw, _ = bw.CreateFormFile("file", "evil.txt")
	_, _ = fw.Write([]byte("not an image"))
	_ = bw.Close()
	breq := httptest.NewRequest("POST", "/api/images", &bad)
	breq.Header.Set("Content-Type", bw.FormDataContentType())
	brec := httptest.NewRecorder()
	mux.ServeHTTP(brec, breq)
	if brec.Code != 400 {
		t.Fatalf("bad upload: got %d, want 400", brec.Code)
	}
}
