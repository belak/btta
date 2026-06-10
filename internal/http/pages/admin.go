package pages

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"time"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/thumbnail"
	"github.com/belak/x/httpx"
	"github.com/belak/x/pass"
	"github.com/belak/x/ratelimit"
	"github.com/belak/x/slogx"
)

const sessionUserKey = "user_id"
const sessionPendingUserKey = "pending_user_id"

// loginRateLimit is the number of login attempts allowed per minute per
// client IP before requests are throttled.
const loginRateLimit = 10


// maxUploadBytes caps the size of an uploaded media file.
const maxUploadBytes = 10 << 20 // 10 MiB

// allowedImageTypes maps a sniffed content type to the extension used when
// storing the upload. Anything not in this map is rejected.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
}

type AdminHandlers struct {
	queries      *db.Queries
	mediaDir     string
	sessions     *scs.SessionManager
	loginLimiter *ratelimit.RateLimiter
	ips          *httpx.IPResolver
	pass         *pass.Context
}

func NewAdminHandlers(database *sql.DB, mediaDir string, sessions *scs.SessionManager) *AdminHandlers {
	return &AdminHandlers{
		queries:      db.New(database),
		mediaDir:     mediaDir,
		sessions:     sessions,
		loginLimiter: ratelimit.NewRateLimiter(loginRateLimit, time.Minute),
		ips:          httpx.NewIPResolver(nil),
		pass:         pass.NewDefaultContext(),
	}
}

func (h *AdminHandlers) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// RequireAuth is middleware that redirects to /admin/login if no session exists.
func (h *AdminHandlers) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.sessions.GetInt64(r.Context(), sessionUserKey) == 0 {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AdminHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, LoginPage(""))
}

func (h *AdminHandlers) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	logger := slogx.FromContext(r.Context())
	ip := h.ips.ClientIP(r)

	// Throttle login attempts per client IP to blunt brute-force attacks.
	rateKey := "login:" + ip
	if !h.loginLimiter.Allow(rateKey) {
		logger.Warn("login rate limited", slogx.String("ip", ip))
		h.render(w, r, LoginPage("Too many login attempts. Please wait and try again."))
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.queries.GetUserByUsername(r.Context(), username)
	if err != nil {
		// Run a dummy verification so the response time doesn't reveal
		// whether the username exists.
		h.pass.DummyVerify(password)
		logger.Warn("login failed", slogx.String("username", username), slogx.String("ip", ip))
		h.render(w, r, LoginPage("Invalid username or password."))
		return
	}
	if h.pass.Verify(user.PasswordHash, password) != nil {
		logger.Warn("login failed", slogx.String("username", username), slogx.String("ip", ip))
		h.render(w, r, LoginPage("Invalid username or password."))
		return
	}

	// Successful auth — clear the throttle so a legitimate user isn't
	// penalized by earlier failed attempts.
	h.loginLimiter.Reset(rateKey)
	logger.Info("login succeeded", slogx.String("username", username), slogx.String("ip", ip))

	// Transparently rehash on login when the stored hash uses an outdated
	// scheme (e.g. bcrypt → argon2id) or outdated parameters.
	if h.pass.NeedsUpdate(user.PasswordHash) {
		if newHash, err := h.pass.Hash(password); err == nil {
			_ = h.queries.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
				ID:           user.ID,
				PasswordHash: newHash,
			})
		}
	}

	// Rotate the session token on login to prevent session fixation.
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		h.render(w, r, LoginPage("Failed to create session."))
		return
	}

	if user.ForcePasswordReset {
		h.sessions.Put(r.Context(), sessionPendingUserKey, user.ID)
		http.Redirect(w, r, "/admin/password", http.StatusFound)
		return
	}

	h.sessions.Put(r.Context(), sessionUserKey, user.ID)
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

func (h *AdminHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Destroy(r.Context())
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

// getPasswordResetUser returns the user for a password change request.
// It checks the full session first, then the pending-reset session.
// The bool return is true when the request comes from a pending-reset session.
func (h *AdminHandlers) getPasswordResetUser(r *http.Request) (db.User, bool, error) {
	if id := h.sessions.GetInt64(r.Context(), sessionUserKey); id != 0 {
		user, err := h.queries.GetUserByID(r.Context(), id)
		return user, false, err
	}
	if id := h.sessions.GetInt64(r.Context(), sessionPendingUserKey); id != 0 {
		user, err := h.queries.GetUserByID(r.Context(), id)
		return user, true, err
	}
	return db.User{}, false, fmt.Errorf("no authenticated user")
}

func (h *AdminHandlers) ChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	_, forced, err := h.getPasswordResetUser(r)
	if err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	h.render(w, r, ChangePasswordPage("", false, forced))
}

func (h *AdminHandlers) ChangePasswordSubmit(w http.ResponseWriter, r *http.Request) {
	user, forced, err := h.getPasswordResetUser(r)
	if err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	current := r.FormValue("current")
	newPw := r.FormValue("new")
	confirm := r.FormValue("confirm")

	if h.pass.Verify(user.PasswordHash, current) != nil {
		h.render(w, r, ChangePasswordPage("Current password is incorrect.", false, forced))
		return
	}
	if len(newPw) == 0 {
		h.render(w, r, ChangePasswordPage("New password must not be empty.", false, forced))
		return
	}
	if newPw != confirm {
		h.render(w, r, ChangePasswordPage("New passwords do not match.", false, forced))
		return
	}

	hash, err := h.pass.Hash(newPw)
	if err != nil {
		h.render(w, r, ChangePasswordPage("Failed to hash password.", false, forced))
		return
	}

	if err := h.queries.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: hash,
	}); err != nil {
		h.render(w, r, ChangePasswordPage("Failed to update password.", false, forced))
		return
	}

	if forced {
		// Rotate the token as the pending-reset session is promoted to a
		// full authenticated session.
		if err := h.sessions.RenewToken(r.Context()); err != nil {
			h.render(w, r, ChangePasswordPage("Failed to create session.", false, forced))
			return
		}
		h.sessions.Remove(r.Context(), sessionPendingUserKey)
		h.sessions.Put(r.Context(), sessionUserKey, user.ID)
		http.Redirect(w, r, "/admin/", http.StatusFound)
		return
	}

	h.render(w, r, ChangePasswordPage("", true, false))
}

// --- View model helpers ---

func (h *AdminHandlers) scoreView(s db.Score) ScoreView {
	bannerURL := ""
	thumbURL := ""
	if s.GameBanner != "" {
		bannerURL = "/media/" + s.GameBanner
		srcPath := filepath.Join(h.mediaDir, s.GameBanner)
		dstPath := thumbnail.Path(h.mediaDir, s.GameBanner)
		if err := thumbnail.Ensure(srcPath, dstPath); err == nil {
			thumbURL = "/media/thumbnails/" + s.GameBanner + ".jpg"
		}
	}
	return ScoreView{Score: s, BannerURL: bannerURL, ThumbnailURL: thumbURL}
}

func (h *AdminHandlers) imageView(img db.Image) ImageView {
	imageURL := ""
	if img.Image != "" {
		imageURL = "/media/" + img.Image
	}
	return ImageView{Image: img, URL: imageURL}
}

// --- Scores ---

func (h *AdminHandlers) ScoreList(w http.ResponseWriter, r *http.Request) {
	scores, err := h.queries.ListScores(r.Context())
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	views := make([]ScoreView, len(scores))
	for i, s := range scores {
		views[i] = h.scoreView(s)
	}
	h.render(w, r, ScoreListPage(views))
}

func (h *AdminHandlers) ScoreNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ScoreFormPage("New Score", nil, ""))
}

func (h *AdminHandlers) ScoreCreate(w http.ResponseWriter, r *http.Request) {
	if err := limitUpload(w, r); err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, err.Error()))
		return
	}

	gameName := r.FormValue("game_name")
	playerName := r.FormValue("player_name")
	playerScore, err := strconv.ParseInt(r.FormValue("player_score"), 10, 64)
	if err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, "Score must be a number."))
		return
	}

	// Validate the upload before creating the row so a bad file doesn't leave
	// a stranded score. The filename needs the new ID, so the file is written
	// after the insert.
	f, ext, err := openImageUpload(r, "game_banner")
	if err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, fmt.Sprintf("Upload error: %v", err)))
		return
	}
	if f != nil {
		defer f.Close()
	}

	score, err := h.queries.CreateScore(r.Context(), db.CreateScoreParams{
		GameName:    gameName,
		PlayerName:  playerName,
		PlayerScore: playerScore,
	})
	if err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, "Failed to save score."))
		return
	}

	if f != nil {
		banner := mediaFilename("score", score.ID, ext)
		if err := h.writeUpload(f, banner); err != nil {
			_ = h.queries.DeleteScore(r.Context(), score.ID)
			h.render(w, r, ScoreFormPage("New Score", nil, fmt.Sprintf("Upload error: %v", err)))
			return
		}
		if err := h.queries.SetScoreBanner(r.Context(), db.SetScoreBannerParams{
			ID:         score.ID,
			GameBanner: banner,
		}); err != nil {
			h.removeMedia(r, banner, false)
			_ = h.queries.DeleteScore(r.Context(), score.ID)
			h.render(w, r, ScoreFormPage("New Score", nil, "Failed to save score."))
			return
		}
		h.ensureThumbnail(r, banner)
	}

	http.Redirect(w, r, "/admin/scores/", http.StatusFound)
}

func (h *AdminHandlers) ScoreEdit(w http.ResponseWriter, r *http.Request) {
	score, err := h.getScore(w, r)
	if err != nil {
		return
	}
	v := h.scoreView(score)
	h.render(w, r, ScoreFormPage("Edit Score", &v, ""))
}

func (h *AdminHandlers) ScoreUpdate(w http.ResponseWriter, r *http.Request) {
	score, err := h.getScore(w, r)
	if err != nil {
		return
	}
	v := h.scoreView(score)

	if err := limitUpload(w, r); err != nil {
		h.render(w, r, ScoreFormPage("Edit Score", &v, err.Error()))
		return
	}

	gameName := r.FormValue("game_name")
	playerName := r.FormValue("player_name")
	playerScore, err := strconv.ParseInt(r.FormValue("player_score"), 10, 64)
	if err != nil {
		h.render(w, r, ScoreFormPage("Edit Score", &v, "Score must be a number."))
		return
	}

	bannerFilename := score.GameBanner
	f, ext, err := openImageUpload(r, "game_banner")
	if err != nil {
		h.render(w, r, ScoreFormPage("Edit Score", &v, fmt.Sprintf("Upload error: %v", err)))
		return
	}
	if f != nil {
		defer f.Close()
		bannerFilename = mediaFilename("score", score.ID, ext)
		if err := h.writeUpload(f, bannerFilename); err != nil {
			h.render(w, r, ScoreFormPage("Edit Score", &v, fmt.Sprintf("Upload error: %v", err)))
			return
		}
	}

	if _, err := h.queries.UpdateScore(r.Context(), db.UpdateScoreParams{
		ID:          score.ID,
		GameBanner:  bannerFilename,
		GameName:    gameName,
		PlayerName:  playerName,
		PlayerScore: playerScore,
	}); err != nil {
		if f != nil {
			h.removeMedia(r, bannerFilename, false) // drop the file we just wrote
		}
		h.render(w, r, ScoreFormPage("Edit Score", &v, "Failed to update score."))
		return
	}

	// Remove the previous banner (and its thumbnail) if it was replaced, and
	// generate the thumbnail for the new one.
	if bannerFilename != score.GameBanner {
		h.removeMedia(r, score.GameBanner, true)
	}
	if f != nil {
		h.ensureThumbnail(r, bannerFilename)
	}

	http.Redirect(w, r, "/admin/scores/", http.StatusFound)
}

func (h *AdminHandlers) ScoreDelete(w http.ResponseWriter, r *http.Request) {
	score, err := h.getScore(w, r)
	if err != nil {
		return
	}
	if err := h.queries.DeleteScore(r.Context(), score.ID); err != nil {
		http.Error(w, "failed to delete score", http.StatusInternalServerError)
		return
	}
	h.removeMedia(r, score.GameBanner, true)
	http.Redirect(w, r, "/admin/scores/", http.StatusFound)
}

func (h *AdminHandlers) getScore(w http.ResponseWriter, r *http.Request) (db.Score, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return db.Score{}, err
	}
	score, err := h.queries.GetScore(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return db.Score{}, err
	}
	return score, nil
}

// --- Images ---

func (h *AdminHandlers) ImageList(w http.ResponseWriter, r *http.Request) {
	images, err := h.queries.ListImages(r.Context())
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	views := make([]ImageView, len(images))
	for i, img := range images {
		views[i] = h.imageView(img)
	}
	h.render(w, r, ImageListPage(views))
}

func (h *AdminHandlers) ImageNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ImageFormPage("New Image", nil, ""))
}

func (h *AdminHandlers) ImageCreate(w http.ResponseWriter, r *http.Request) {
	if err := limitUpload(w, r); err != nil {
		h.render(w, r, ImageFormPage("New Image", nil, err.Error()))
		return
	}

	name := r.FormValue("name")
	enabled := r.FormValue("enabled") == "1"

	// Validate the upload before creating the row; the filename needs the new
	// ID, so the file is written after the insert.
	f, ext, err := openImageUpload(r, "image")
	if err != nil {
		h.render(w, r, ImageFormPage("New Image", nil, fmt.Sprintf("Upload error: %v", err)))
		return
	}
	if f != nil {
		defer f.Close()
	}

	img, err := h.queries.CreateImage(r.Context(), db.CreateImageParams{
		Name:    name,
		Enabled: enabled,
	})
	if err != nil {
		h.render(w, r, ImageFormPage("New Image", nil, "Failed to save image."))
		return
	}

	if f != nil {
		filename := mediaFilename("image", img.ID, ext)
		if err := h.writeUpload(f, filename); err != nil {
			_ = h.queries.DeleteImage(r.Context(), img.ID)
			h.render(w, r, ImageFormPage("New Image", nil, fmt.Sprintf("Upload error: %v", err)))
			return
		}
		if err := h.queries.SetImageFile(r.Context(), db.SetImageFileParams{
			ID:    img.ID,
			Image: filename,
		}); err != nil {
			h.removeMedia(r, filename, false)
			_ = h.queries.DeleteImage(r.Context(), img.ID)
			h.render(w, r, ImageFormPage("New Image", nil, "Failed to save image."))
			return
		}
	}

	http.Redirect(w, r, "/admin/images/", http.StatusFound)
}

func (h *AdminHandlers) ImageEdit(w http.ResponseWriter, r *http.Request) {
	img, err := h.getImage(w, r)
	if err != nil {
		return
	}
	v := h.imageView(img)
	h.render(w, r, ImageFormPage("Edit Image", &v, ""))
}

func (h *AdminHandlers) ImageUpdate(w http.ResponseWriter, r *http.Request) {
	img, err := h.getImage(w, r)
	if err != nil {
		return
	}
	v := h.imageView(img)

	if err := limitUpload(w, r); err != nil {
		h.render(w, r, ImageFormPage("Edit Image", &v, err.Error()))
		return
	}

	name := r.FormValue("name")
	enabled := r.FormValue("enabled") == "1"

	imageFilename := img.Image
	f, ext, err := openImageUpload(r, "image")
	if err != nil {
		h.render(w, r, ImageFormPage("Edit Image", &v, fmt.Sprintf("Upload error: %v", err)))
		return
	}
	if f != nil {
		defer f.Close()
		imageFilename = mediaFilename("image", img.ID, ext)
		if err := h.writeUpload(f, imageFilename); err != nil {
			h.render(w, r, ImageFormPage("Edit Image", &v, fmt.Sprintf("Upload error: %v", err)))
			return
		}
	}

	if _, err := h.queries.UpdateImage(r.Context(), db.UpdateImageParams{
		ID:      img.ID,
		Name:    name,
		Image:   imageFilename,
		Enabled: enabled,
	}); err != nil {
		if f != nil {
			h.removeMedia(r, imageFilename, false) // drop the file we just wrote
		}
		h.render(w, r, ImageFormPage("Edit Image", &v, "Failed to update image."))
		return
	}

	// Remove the previous image if it was replaced.
	if imageFilename != img.Image {
		h.removeMedia(r, img.Image, false)
	}

	http.Redirect(w, r, "/admin/images/", http.StatusFound)
}

func (h *AdminHandlers) ImageDelete(w http.ResponseWriter, r *http.Request) {
	img, err := h.getImage(w, r)
	if err != nil {
		return
	}
	if err := h.queries.DeleteImage(r.Context(), img.ID); err != nil {
		http.Error(w, "failed to delete image", http.StatusInternalServerError)
		return
	}
	h.removeMedia(r, img.Image, false)
	http.Redirect(w, r, "/admin/images/", http.StatusFound)
}

func (h *AdminHandlers) getImage(w http.ResponseWriter, r *http.Request) (db.Image, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return db.Image{}, err
	}
	img, err := h.queries.GetImage(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return db.Image{}, err
	}
	return img, nil
}

// removeMedia deletes a media file and, optionally, its cached thumbnail.
// It is best-effort: a missing file is ignored and any other error is logged
// rather than returned, since the database is the source of truth.
func (h *AdminHandlers) removeMedia(r *http.Request, filename string, withThumbnail bool) {
	if filename == "" {
		return
	}
	paths := []string{filepath.Join(h.mediaDir, filename)}
	if withThumbnail {
		paths = append(paths, thumbnail.Path(h.mediaDir, filename))
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			slogx.FromContext(r.Context()).Warn("failed to remove media file",
				slogx.String("path", p), slogx.Err(err))
		}
	}
}

// ensureThumbnail generates the cached thumbnail for a freshly uploaded
// banner. It is best-effort: a failure is logged, and the read path falls back
// to the full banner, so a missing thumbnail never breaks display.
func (h *AdminHandlers) ensureThumbnail(r *http.Request, banner string) {
	if banner == "" {
		return
	}
	src := filepath.Join(h.mediaDir, banner)
	dst := thumbnail.Path(h.mediaDir, banner)
	if err := thumbnail.Generate(src, dst); err != nil {
		slogx.FromContext(r.Context()).Warn("failed to generate thumbnail",
			slogx.String("banner", banner), slogx.Err(err))
	}
}

// limitUpload caps the request body at maxUploadBytes and parses the
// multipart form. It must be called before any FormValue/FormFile access so
// the size limit applies to the whole body. The returned error is suitable
// for display to the user.
func limitUpload(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return fmt.Errorf("upload too large (max %d MB)", maxUploadBytes>>20)
		}
		return fmt.Errorf("invalid upload")
	}
	return nil
}

// openImageUpload returns the uploaded file for field along with the
// extension derived from its sniffed content type. It returns (nil, "", nil)
// if no file was provided, and an error if the file is not a PNG or JPEG.
// The caller is responsible for closing the returned file.
func openImageUpload(r *http.Request, field string) (multipart.File, string, error) {
	f, _, err := r.FormFile(field)
	if err != nil {
		// No file uploaded — not an error.
		return nil, "", nil
	}

	// Sniff the content type from the first 512 bytes and require an allowed
	// image type, then rewind so the whole file can be written.
	head := make([]byte, 512)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		f.Close()
		return nil, "", fmt.Errorf("read upload: %w", err)
	}
	ext, ok := allowedImageTypes[http.DetectContentType(head[:n])]
	if !ok {
		f.Close()
		return nil, "", fmt.Errorf("unsupported file type (must be PNG or JPEG)")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, "", fmt.Errorf("rewind upload: %w", err)
	}
	return f, ext, nil
}

// writeUpload writes the contents of f to mediaDir under filename.
func (h *AdminHandlers) writeUpload(f io.Reader, filename string) error {
	if err := os.MkdirAll(h.mediaDir, 0o755); err != nil {
		return fmt.Errorf("create media dir: %w", err)
	}

	dst := filepath.Join(h.mediaDir, filename)
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, f); err != nil {
		os.Remove(dst)
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// mediaFilename builds the stored filename for an upload: a kind/id prefix for
// correlation plus a random suffix so the name is unguessable and changes on
// every replacement (which keeps cached thumbnails from going stale).
func mediaFilename(kind string, id int64, ext string) string {
	return fmt.Sprintf("%s-%d-%s%s", kind, id, randomHex(8), ext)
}
