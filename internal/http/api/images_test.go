package api

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/belak/btta/internal/db"
)

func TestImageListReturnsOnlyEnabled(t *testing.T) {
	database := testDB(t)
	q := db.New(database)
	_, err := q.CreateImage(context.Background(), db.CreateImageParams{Name: "on", Image: "a.png", Enabled: true})
	assert.NoError(t, err)
	_, err = q.CreateImage(context.Background(), db.CreateImageParams{Name: "off", Image: "b.png", Enabled: false})
	assert.NoError(t, err)

	h := NewImageHandlers(database)
	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/api/images/", nil))
	assert.Equal(t, 200, w.Code)

	var got []imageResponse
	decodeJSON(t, w, &got)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "on", got[0].Name)
	assert.Equal(t, "/media/a.png", got[0].Image)
}

// TestImageGet checks the public endpoint only exposes enabled images.
func TestImageGet(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		wantStatus int
	}{
		{"enabled image is served", true, 200},
		{"disabled image is hidden", false, 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := testDB(t)
			img, err := db.New(database).CreateImage(context.Background(), db.CreateImageParams{
				Name: "img", Image: "a.png", Enabled: tt.enabled,
			})
			assert.NoError(t, err)

			h := NewImageHandlers(database)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/images/", nil)
			req.SetPathValue("id", strconv.FormatInt(img.ID, 10))
			h.Get(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantStatus == 200 {
				var got imageResponse
				decodeJSON(t, w, &got)
				assert.Equal(t, "/media/a.png", got.Image)
			}
		})
	}
}
