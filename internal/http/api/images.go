package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/belak/btta/internal/db"
	"github.com/belak/x/httpx"
)

type ImageHandlers struct {
	queries *db.Queries
}

func NewImageHandlers(database *sql.DB) *ImageHandlers {
	return &ImageHandlers{
		queries: db.New(database),
	}
}

type imageResponse struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Enabled bool   `json:"enabled"`
}

func (h *ImageHandlers) toResponse(img db.Image) imageResponse {
	imageURL := ""
	if img.Image != "" {
		imageURL = "/media/" + img.Image
	}
	return imageResponse{
		ID:      img.ID,
		Name:    img.Name,
		Image:   imageURL,
		Enabled: img.Enabled,
	}
}

func (h *ImageHandlers) List(w http.ResponseWriter, r *http.Request) {
	images, err := h.queries.ListEnabledImages(r.Context())
	if err != nil {
		httpx.RespondError(w, http.StatusInternalServerError, "failed to list images", "SERVER_ERROR")
		return
	}

	resp := make([]imageResponse, len(images))
	for i, img := range images {
		resp[i] = h.toResponse(img)
	}

	httpx.RespondJSON(w, http.StatusOK, resp)
}

func (h *ImageHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.RespondError(w, http.StatusNotFound, "not found", "NOT_FOUND")
		return
	}

	img, err := h.queries.GetImage(r.Context(), id)
	if err != nil {
		httpx.RespondError(w, http.StatusNotFound, "not found", "NOT_FOUND")
		return
	}

	// Public API only returns enabled images.
	if !img.Enabled {
		httpx.RespondError(w, http.StatusNotFound, "not found", "NOT_FOUND")
		return
	}

	httpx.RespondJSON(w, http.StatusOK, h.toResponse(img))
}
