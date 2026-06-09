package pages

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/thumbnail"
	"golang.org/x/crypto/bcrypt"
)

const sessionUserKey = "user_id"
const sessionPendingUserKey = "pending_user_id"

type AdminHandlers struct {
	queries  *db.Queries
	mediaDir string
	sessions *scs.SessionManager
	baseURL  func(r *http.Request) string
}

func NewAdminHandlers(database *sql.DB, mediaDir string, sessions *scs.SessionManager, baseURL func(r *http.Request) string) *AdminHandlers {
	return &AdminHandlers{
		queries:  db.New(database),
		mediaDir: mediaDir,
		sessions: sessions,
		baseURL:  baseURL,
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
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.queries.GetUserByUsername(r.Context(), username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		h.render(w, r, LoginPage("Invalid username or password."))
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

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
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

	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		h.render(w, r, ChangePasswordPage("Failed to hash password.", false, forced))
		return
	}

	if err := h.queries.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: string(hash),
	}); err != nil {
		h.render(w, r, ChangePasswordPage("Failed to update password.", false, forced))
		return
	}

	if forced {
		h.sessions.Remove(r.Context(), sessionPendingUserKey)
		h.sessions.Put(r.Context(), sessionUserKey, user.ID)
		http.Redirect(w, r, "/admin/", http.StatusFound)
		return
	}

	h.render(w, r, ChangePasswordPage("", true, false))
}

// --- View model helpers ---

func (h *AdminHandlers) scoreView(r *http.Request, s db.Score) ScoreView {
	bannerURL := ""
	thumbURL := ""
	if s.GameBanner != "" {
		bannerURL = h.baseURL(r) + "/media/" + s.GameBanner
		srcPath := filepath.Join(h.mediaDir, s.GameBanner)
		dstPath := thumbnail.Path(h.mediaDir, s.GameBanner)
		if err := thumbnail.Ensure(srcPath, dstPath); err == nil {
			thumbURL = h.baseURL(r) + "/media/thumbnails/" + s.GameBanner + ".jpg"
		}
	}
	return ScoreView{Score: s, BannerURL: bannerURL, ThumbnailURL: thumbURL}
}

func (h *AdminHandlers) imageView(r *http.Request, img db.Image) ImageView {
	imageURL := ""
	if img.Image != "" {
		imageURL = h.baseURL(r) + "/media/" + img.Image
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
		views[i] = h.scoreView(r, s)
	}
	h.render(w, r, ScoreListPage(views))
}

func (h *AdminHandlers) ScoreNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ScoreFormPage("New Score", nil, ""))
}

func (h *AdminHandlers) ScoreCreate(w http.ResponseWriter, r *http.Request) {
	gameName := r.FormValue("game_name")
	playerName := r.FormValue("player_name")
	playerScore, err := strconv.ParseInt(r.FormValue("player_score"), 10, 64)
	if err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, "Score must be a number."))
		return
	}

	bannerFilename, err := h.saveUpload(r, "game_banner")
	if err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, fmt.Sprintf("Upload error: %v", err)))
		return
	}

	if _, err := h.queries.CreateScore(r.Context(), db.CreateScoreParams{
		GameBanner:  bannerFilename,
		GameName:    gameName,
		PlayerName:  playerName,
		PlayerScore: playerScore,
	}); err != nil {
		h.render(w, r, ScoreFormPage("New Score", nil, "Failed to save score."))
		return
	}

	http.Redirect(w, r, "/admin/scores/", http.StatusFound)
}

func (h *AdminHandlers) ScoreEdit(w http.ResponseWriter, r *http.Request) {
	score, err := h.getScore(w, r)
	if err != nil {
		return
	}
	v := h.scoreView(r, score)
	h.render(w, r, ScoreFormPage("Edit Score", &v, ""))
}

func (h *AdminHandlers) ScoreUpdate(w http.ResponseWriter, r *http.Request) {
	score, err := h.getScore(w, r)
	if err != nil {
		return
	}
	v := h.scoreView(r, score)

	gameName := r.FormValue("game_name")
	playerName := r.FormValue("player_name")
	playerScore, err := strconv.ParseInt(r.FormValue("player_score"), 10, 64)
	if err != nil {
		h.render(w, r, ScoreFormPage("Edit Score", &v, "Score must be a number."))
		return
	}

	bannerFilename := score.GameBanner
	if uploaded, err := h.saveUpload(r, "game_banner"); err != nil {
		h.render(w, r, ScoreFormPage("Edit Score", &v, fmt.Sprintf("Upload error: %v", err)))
		return
	} else if uploaded != "" {
		bannerFilename = uploaded
	}

	if _, err := h.queries.UpdateScore(r.Context(), db.UpdateScoreParams{
		ID:          score.ID,
		GameBanner:  bannerFilename,
		GameName:    gameName,
		PlayerName:  playerName,
		PlayerScore: playerScore,
	}); err != nil {
		h.render(w, r, ScoreFormPage("Edit Score", &v, "Failed to update score."))
		return
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
		views[i] = h.imageView(r, img)
	}
	h.render(w, r, ImageListPage(views))
}

func (h *AdminHandlers) ImageNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ImageFormPage("New Image", nil, ""))
}

func (h *AdminHandlers) ImageCreate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	enabled := r.FormValue("enabled") == "1"

	imageFilename, err := h.saveUpload(r, "image")
	if err != nil {
		h.render(w, r, ImageFormPage("New Image", nil, fmt.Sprintf("Upload error: %v", err)))
		return
	}

	if _, err := h.queries.CreateImage(r.Context(), db.CreateImageParams{
		Name:    name,
		Image:   imageFilename,
		Enabled: enabled,
	}); err != nil {
		h.render(w, r, ImageFormPage("New Image", nil, "Failed to save image."))
		return
	}

	http.Redirect(w, r, "/admin/images/", http.StatusFound)
}

func (h *AdminHandlers) ImageEdit(w http.ResponseWriter, r *http.Request) {
	img, err := h.getImage(w, r)
	if err != nil {
		return
	}
	v := h.imageView(r, img)
	h.render(w, r, ImageFormPage("Edit Image", &v, ""))
}

func (h *AdminHandlers) ImageUpdate(w http.ResponseWriter, r *http.Request) {
	img, err := h.getImage(w, r)
	if err != nil {
		return
	}
	v := h.imageView(r, img)

	name := r.FormValue("name")
	enabled := r.FormValue("enabled") == "1"

	imageFilename := img.Image
	if uploaded, err := h.saveUpload(r, "image"); err != nil {
		h.render(w, r, ImageFormPage("Edit Image", &v, fmt.Sprintf("Upload error: %v", err)))
		return
	} else if uploaded != "" {
		imageFilename = uploaded
	}

	if _, err := h.queries.UpdateImage(r.Context(), db.UpdateImageParams{
		ID:      img.ID,
		Name:    name,
		Image:   imageFilename,
		Enabled: enabled,
	}); err != nil {
		h.render(w, r, ImageFormPage("Edit Image", &v, "Failed to update image."))
		return
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

// saveUpload saves an uploaded file to mediaDir and returns the filename.
// Returns an empty string (no error) if no file was provided.
func (h *AdminHandlers) saveUpload(r *http.Request, field string) (string, error) {
	f, header, err := r.FormFile(field)
	if err != nil {
		// No file uploaded — not an error.
		return "", nil
	}
	defer f.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s%s", randomHex(16), ext)
	dst := filepath.Join(h.mediaDir, filename)

	if err := os.MkdirAll(h.mediaDir, 0o755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, f); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("write file: %w", err)
	}

	return filename, nil
}
