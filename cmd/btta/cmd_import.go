package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/belak/btta/internal/storage"
)

type importScore struct {
	GameBanner  string    `json:"game_banner"`
	GameName    string    `json:"game_name"`
	PlayerName  string    `json:"player_name"`
	PlayerScore int64     `json:"player_score"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
}

type importImage struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Enabled bool   `json:"enabled"`
}

func newImportCmd() *ff.Command {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	dbPath := fs.String("db", "btta.db", "path to SQLite database")
	mediaDir := fs.String("media-dir", "media", "directory to store downloaded media")
	from := fs.String("from", "", "base URL of the source instance (required)")

	return &ff.Command{
		Name:      "import",
		Usage:     "btta import --from <url> [FLAGS]",
		ShortHelp: "import scores and images from another instance",
		Flags:     ff.NewFlagSetFrom("import", fs),
		Exec: func(ctx context.Context, args []string) error {
			if *from == "" {
				return fmt.Errorf("--from is required")
			}

			base := *from
			database, err := storage.Open(*dbPath)
			if err != nil {
				return err
			}
			defer database.Close()

			if err := importScores(ctx, database, base, *mediaDir); err != nil {
				return fmt.Errorf("import scores: %w", err)
			}
			if err := importImages(ctx, database, base, *mediaDir); err != nil {
				return fmt.Errorf("import images: %w", err)
			}

			return nil
		},
	}
}

func importScores(ctx context.Context, database *sql.DB, base, mediaDir string) error {
	scores, err := fetchJSON[[]importScore](ctx, base+"/api/scores/")
	if err != nil {
		return err
	}

	for _, s := range scores {
		filename, err := downloadMedia(s.GameBanner, mediaDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: download %s: %v\n", s.GameBanner, err)
		}

		_, err = database.ExecContext(ctx, `
			INSERT INTO scores (game_banner, game_name, player_name, player_score, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			filename, s.GameName, s.PlayerName, s.PlayerScore,
			s.Created.UTC().Format(time.RFC3339Nano),
			s.Modified.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("insert score %q: %w", s.GameName, err)
		}
		fmt.Printf("score: %s — %s (%d)\n", s.GameName, s.PlayerName, s.PlayerScore)
	}

	fmt.Printf("imported %d scores\n", len(scores))
	return nil
}

func importImages(ctx context.Context, database *sql.DB, base, mediaDir string) error {
	images, err := fetchJSON[[]importImage](ctx, base+"/api/images/")
	if err != nil {
		return err
	}

	for _, img := range images {
		filename, err := downloadMedia(img.Image, mediaDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: download %s: %v\n", img.Image, err)
		}

		_, err = database.ExecContext(ctx, `
			INSERT INTO images (name, image, enabled)
			VALUES (?, ?, ?)`,
			img.Name, filename, img.Enabled,
		)
		if err != nil {
			return fmt.Errorf("insert image %q: %w", img.Name, err)
		}
		fmt.Printf("image: %s\n", img.Name)
	}

	fmt.Printf("imported %d images\n", len(images))
	return nil
}

// fetchJSON GETs url and decodes the JSON response body into T.
func fetchJSON[T any](ctx context.Context, url string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return v, nil
}

// downloadMedia downloads a file from srcURL into mediaDir, using the
// filename from the URL path. Returns the filename (not the full path).
// If srcURL is empty, returns an empty string without error.
// If the file already exists, skips the download.
func downloadMedia(srcURL, mediaDir string) (string, error) {
	if srcURL == "" {
		return "", nil
	}

	u, err := url.Parse(srcURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	filename := path.Base(u.Path)
	if filename == "." || filename == "/" {
		return "", fmt.Errorf("could not determine filename from %s", srcURL)
	}

	dst := filepath.Join(mediaDir, filename)

	if _, err := os.Stat(dst); err == nil {
		return filename, nil // already exists
	}

	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	resp, err := http.Get(srcURL) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("write file: %w", err)
	}

	return filename, nil
}
