package api

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/thumbnail"
	"github.com/belak/x/httpx"
)

type ScoreHandlers struct {
	queries  *db.Queries
	mediaDir string
}

func NewScoreHandlers(database *sql.DB, mediaDir string) *ScoreHandlers {
	return &ScoreHandlers{
		queries:  db.New(database),
		mediaDir: mediaDir,
	}
}

type scoreResponse struct {
	ID                  int64     `json:"id"`
	GameBanner          string    `json:"game_banner"`
	GameBannerThumbnail string    `json:"game_banner_thumbnail"`
	GameName            string    `json:"game_name"`
	PlayerName          string    `json:"player_name"`
	PlayerScore         int64     `json:"player_score"`
	Created             time.Time `json:"created"`
	Modified            time.Time `json:"modified"`
}

func (h *ScoreHandlers) toResponse(s db.Score) scoreResponse {
	bannerURL := ""
	thumbURL := ""
	if s.GameBanner != "" {
		bannerURL = "/media/" + s.GameBanner
		// Advertise the thumbnail only if it exists; otherwise fall back to
		// the full banner so the client never gets a 404 image URL.
		thumbURL = bannerURL
		if _, err := os.Stat(thumbnail.Path(h.mediaDir, s.GameBanner)); err == nil {
			thumbURL = "/media/thumbnails/" + s.GameBanner + ".jpg"
		}
	}
	return scoreResponse{
		ID:                  s.ID,
		GameBanner:          bannerURL,
		GameBannerThumbnail: thumbURL,
		GameName:            s.GameName,
		PlayerName:          s.PlayerName,
		PlayerScore:         s.PlayerScore,
		Created:             s.CreatedAt,
		Modified:            s.UpdatedAt,
	}
}

func (h *ScoreHandlers) List(w http.ResponseWriter, r *http.Request) {
	scores, err := h.queries.ListScores(r.Context())
	if err != nil {
		httpx.RespondError(w, http.StatusInternalServerError, "failed to list scores", "SERVER_ERROR")
		return
	}

	resp := make([]scoreResponse, len(scores))
	for i, s := range scores {
		resp[i] = h.toResponse(s)
	}

	httpx.RespondJSON(w, http.StatusOK, resp)
}

func (h *ScoreHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.RespondError(w, http.StatusNotFound, "not found", "NOT_FOUND")
		return
	}

	score, err := h.queries.GetScore(r.Context(), id)
	if err != nil {
		httpx.RespondError(w, http.StatusNotFound, "not found", "NOT_FOUND")
		return
	}

	httpx.RespondJSON(w, http.StatusOK, h.toResponse(score))
}
