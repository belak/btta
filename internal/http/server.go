package http

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"

	"github.com/belak/btta/internal/http/api"
	"github.com/belak/btta/internal/http/frontend"
	"github.com/belak/btta/internal/http/pages"
	"github.com/belak/btta/internal/http/static"
	"github.com/belak/x/fsx"
	"github.com/belak/x/httpx"
)

type Server struct {
	router   *httpx.Router
	sessions *scs.SessionManager
	mediaDir string
}

func NewServer(database *sql.DB, mediaDir string, logger *slog.Logger) *Server {
	sessions := scs.New()
	sessions.Store = sqlite3store.NewWithCleanupInterval(database, 30*time.Minute)
	sessions.Cookie.Name = "btta_session"
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	// Require HTTPS for the session cookie. Browsers treat loopback
	// (localhost / 127.0.0.1) as secure, so this still works for local dev;
	// production is expected to run behind a TLS-terminating proxy.
	sessions.Cookie.Secure = true
	sessions.Lifetime = 7 * 24 * time.Hour

	s := &Server{
		router:   httpx.NewRouter(),
		sessions: sessions,
		mediaDir: mediaDir,
	}

	// CrossOriginProtection rejects non-safe cross-origin browser requests
	// (CSRF) via Sec-Fetch-Site / Origin checks. Safe methods (GET/HEAD/
	// OPTIONS) and non-browser requests without those headers are allowed, so
	// the public API and the import CLI are unaffected.
	csrf := http.NewCrossOriginProtection()

	s.router.Use(
		sessions.LoadAndSave,
		httpx.WithRequestID,
		httpx.WithRenderInfo,
		httpx.Logging(logger),
		httpx.Recovery(logger),
		httpx.SecurityHeaders,
		corsMiddleware,
		csrf.Handler,
	)

	s.setupRoutes(database)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupRoutes(database *sql.DB) {
	scores := api.NewScoreHandlers(database, s.mediaDir)
	images := api.NewImageHandlers(database)
	admin := pages.NewAdminHandlers(database, s.mediaDir, s.sessions)

	// Public API
	s.router.Handle("GET /api/scores/", scores.List)
	s.router.Handle("GET /api/scores/{id}/", scores.Get)
	s.router.Handle("GET /api/images/", images.List)
	s.router.Handle("GET /api/images/{id}/", images.Get)

	// Static assets (embedded)
	staticFS := fsx.NoListFS{FS: http.FS(static.FS)}
	s.router.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(staticFS)).ServeHTTP)

	// Media files (originals + thumbnails). NoListFS disables directory
	// listings so the media directory's contents can't be enumerated. Media
	// filenames carry a random suffix that changes on every replacement, so a
	// given URL's content never changes — safe to cache immutably.
	mediaFS := fsx.NoListFS{FS: http.Dir(s.mediaDir)}
	mediaHandler := http.StripPrefix("/media/", http.FileServer(mediaFS))
	s.router.Handle("GET /media/", cacheControl("public, max-age=31536000, immutable", mediaHandler).ServeHTTP)

	// Auth
	s.router.Handle("GET /admin/login", admin.LoginPage)
	s.router.Handle("POST /admin/login", admin.LoginSubmit)
	s.router.Handle("POST /admin/logout", admin.Logout)

	// Password change — accessible to both full sessions and pending-reset sessions
	s.router.Handle("GET /admin/password", admin.ChangePasswordPage)
	s.router.Handle("POST /admin/password", admin.ChangePasswordSubmit)

	// Admin (authenticated)
	s.router.Group(func(r *httpx.Router) {
		r.Use(admin.RequireAuth)

		r.Handle("GET /admin/", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/admin/scores/", http.StatusFound)
		})

		r.Handle("GET /admin/scores/", admin.ScoreList)
		r.Handle("GET /admin/scores/new", admin.ScoreNew)
		r.Handle("POST /admin/scores/new", admin.ScoreCreate)
		r.Handle("GET /admin/scores/{id}/edit", admin.ScoreEdit)
		r.Handle("POST /admin/scores/{id}/edit", admin.ScoreUpdate)
		r.Handle("POST /admin/scores/{id}/delete", admin.ScoreDelete)

		r.Handle("GET /admin/images/", admin.ImageList)
		r.Handle("GET /admin/images/new", admin.ImageNew)
		r.Handle("POST /admin/images/new", admin.ImageCreate)
		r.Handle("GET /admin/images/{id}/edit", admin.ImageEdit)
		r.Handle("POST /admin/images/{id}/edit", admin.ImageUpdate)
		r.Handle("POST /admin/images/{id}/delete", admin.ImageDelete)
	})

	// Frontend SPA (catch-all — must be last)
	frontendFS, err := fs.Sub(frontend.FS, "dist")
	if err != nil {
		panic(err)
	}
	s.router.Handle("GET /", http.FileServer(http.FS(frontendFS)).ServeHTTP)
}

// cacheControl wraps h, setting the given Cache-Control header on responses.
func cacheControl(value string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/media/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
