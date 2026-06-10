package pages

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/storage"
)

// newTestAdmin builds an AdminHandlers backed by a temp DB, plus an http
// handler exposing login and an auth-probe route through scs middleware.
func newTestAdmin(t *testing.T) (*AdminHandlers, http.Handler) {
	t.Helper()
	database, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	assert.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	sessions := scs.New()
	h := NewAdminHandlers(database, t.TempDir(), sessions)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/login", h.LoginSubmit)
	mux.HandleFunc("GET /probe", func(w http.ResponseWriter, r *http.Request) {
		if h.sessions.GetInt64(r.Context(), sessionUserKey) != 0 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
	return h, sessions.LoadAndSave(mux)
}

func createUser(t *testing.T, h *AdminHandlers, username, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	assert.NoError(t, err)
	_, err = h.queries.CreateUser(context.Background(), db.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
	})
	assert.NoError(t, err)
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	assert.NoError(t, err)
	return &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func login(t *testing.T, c *http.Client, base, user, pass string) *http.Response {
	t.Helper()
	resp, err := c.PostForm(base+"/admin/login", url.Values{"username": {user}, "password": {pass}})
	assert.NoError(t, err)
	return resp
}

func sessionToken(t *testing.T, c *http.Client, base string) string {
	t.Helper()
	u, err := url.Parse(base)
	assert.NoError(t, err)
	cookies := c.Jar.Cookies(u)
	if len(cookies) == 0 {
		return ""
	}
	return cookies[0].Value
}

func TestLogin(t *testing.T) {
	h, handler := newTestAdmin(t)
	createUser(t, h, "alice", "secret")
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tests := []struct {
		name       string
		user, pass string
		wantStatus int
		wantAuthed bool
	}{
		{"valid credentials", "alice", "secret", http.StatusFound, true},
		{"wrong password", "alice", "wrong", http.StatusOK, false},
		{"unknown user", "ghost", "secret", http.StatusOK, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t)
			resp := login(t, c, srv.URL, tt.user, tt.pass)
			resp.Body.Close()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			probe, err := c.Get(srv.URL + "/probe")
			assert.NoError(t, err)
			probe.Body.Close()
			wantProbe := http.StatusUnauthorized
			if tt.wantAuthed {
				wantProbe = http.StatusOK
			}
			assert.Equal(t, wantProbe, probe.StatusCode)
		})
	}
}

func TestLoginRotatesSessionToken(t *testing.T) {
	h, handler := newTestAdmin(t)
	createUser(t, h, "alice", "secret")
	srv := httptest.NewServer(handler)
	defer srv.Close()
	c := newClient(t)

	login(t, c, srv.URL, "alice", "secret").Body.Close()
	tok1 := sessionToken(t, c, srv.URL)
	assert.NotZero(t, tok1)

	// A second login must rotate the token (session-fixation defense).
	login(t, c, srv.URL, "alice", "secret").Body.Close()
	tok2 := sessionToken(t, c, srv.URL)
	assert.NotEqual(t, tok1, tok2)
}

func TestLoginRateLimited(t *testing.T) {
	h, handler := newTestAdmin(t)
	createUser(t, h, "alice", "secret")
	srv := httptest.NewServer(handler)
	defer srv.Close()

	var body string
	for range loginRateLimit + 2 {
		c := newClient(t) // fresh session each time; same (loopback) client IP
		resp := login(t, c, srv.URL, "alice", "wrong")
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		body = string(b)
	}
	assert.Contains(t, body, "Too many login attempts")
}

// --- uploads ---

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	assert.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	return buf.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	assert.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil))
	return buf.Bytes()
}

func multipartRequest(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	assert.NoError(t, err)
	_, err = fw.Write(content)
	assert.NoError(t, err)
	assert.NoError(t, mw.Close())

	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestOpenImageUpload(t *testing.T) {
	tests := []struct {
		name     string
		field    string // form field the file is uploaded under
		content  []byte
		wantErr  bool
		wantFile bool
		wantExt  string
	}{
		// Filenames are deliberately misleading to prove the type is sniffed.
		{name: "png accepted", field: "image", content: pngBytes(t), wantFile: true, wantExt: ".png"},
		{name: "jpeg accepted", field: "image", content: jpegBytes(t), wantFile: true, wantExt: ".jpg"},
		{name: "non-image rejected", field: "image", content: []byte("definitely not an image"), wantErr: true},
		{name: "missing field is a no-op", field: "other", content: pngBytes(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := multipartRequest(t, tt.field, "upload.bin", tt.content)
			f, ext, err := openImageUpload(req, "image")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantFile {
				assert.NotZero(t, f)
				f.Close()
			} else {
				assert.Zero(t, f)
			}
			assert.Equal(t, tt.wantExt, ext)
		})
	}
}

func TestMediaFilename(t *testing.T) {
	name := mediaFilename("score", 42, ".png")
	assert.HasPrefix(t, name, "score-42-")
	assert.HasSuffix(t, name, ".png")
	// The random suffix makes each call unique.
	assert.NotEqual(t, name, mediaFilename("score", 42, ".png"))
}

func TestRemoveMedia(t *testing.T) {
	h, _ := newTestAdmin(t)
	banner := "score-1-abc.png"
	assert.NoError(t, os.WriteFile(filepath.Join(h.mediaDir, banner), []byte("x"), 0o644))
	thumbs := filepath.Join(h.mediaDir, "thumbnails")
	assert.NoError(t, os.MkdirAll(thumbs, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(thumbs, banner+".jpg"), []byte("x"), 0o644))

	h.removeMedia(httptest.NewRequest("GET", "/", nil), banner, true)

	_, err := os.Stat(filepath.Join(h.mediaDir, banner))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(thumbs, banner+".jpg"))
	assert.True(t, os.IsNotExist(err))
}
