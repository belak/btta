package pages

import "github.com/belak/btta/internal/db"

type ScoreView struct {
	db.Score
	BannerURL    string
	ThumbnailURL string
}

type ImageView struct {
	db.Image
	URL string
}
