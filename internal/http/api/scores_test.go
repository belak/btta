package api

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/belak/btta/internal/db"
)

// TestScoreListThumbnailURL covers the relative-URL contract and the
// thumbnail-or-banner fallback (#9): the banner is always relative, and the
// thumbnail URL is only advertised when the cached file actually exists.
func TestScoreListThumbnailURL(t *testing.T) {
	tests := []struct {
		name       string
		writeThumb bool
		wantThumb  string
	}{
		{"falls back to banner when thumbnail missing", false, "/media/score-1-abc.png"},
		{"uses thumbnail when present", true, "/media/thumbnails/score-1-abc.png.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := testDB(t)
			mediaDir := t.TempDir()
			_, err := db.New(database).CreateScore(context.Background(), db.CreateScoreParams{
				GameBanner: "score-1-abc.png", GameName: "Galaga", PlayerName: "AAA", PlayerScore: 100,
			})
			assert.NoError(t, err)

			if tt.writeThumb {
				thumbs := filepath.Join(mediaDir, "thumbnails")
				assert.NoError(t, os.MkdirAll(thumbs, 0o755))
				assert.NoError(t, os.WriteFile(filepath.Join(thumbs, "score-1-abc.png.jpg"), []byte("x"), 0o644))
			}

			h := NewScoreHandlers(database, mediaDir)
			w := httptest.NewRecorder()
			h.List(w, httptest.NewRequest("GET", "/api/scores/", nil))
			assert.Equal(t, 200, w.Code)

			var got []scoreResponse
			decodeJSON(t, w, &got)
			assert.Equal(t, 1, len(got))
			assert.Equal(t, "/media/score-1-abc.png", got[0].GameBanner)
			assert.Equal(t, tt.wantThumb, got[0].GameBannerThumbnail)
		})
	}
}

func TestScoreGetBadID(t *testing.T) {
	database := testDB(t)
	h := NewScoreHandlers(database, t.TempDir())

	tests := []struct {
		name string
		id   string
	}{
		{"nonexistent", "999"},
		{"non-numeric", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/scores/"+tt.id+"/", nil)
			req.SetPathValue("id", tt.id)
			h.Get(w, req)
			assert.Equal(t, 404, w.Code)
		})
	}
}
