package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/storage"
	"github.com/belak/btta/internal/thumbnail"
	"github.com/belak/x/slogx"
)

func newRegenerateThumbnailsCmd() *ff.Command {
	fs := flag.NewFlagSet("regenerate-thumbnails", flag.ContinueOnError)
	dbPath := fs.String("db", "btta.db", "path to SQLite database")
	mediaDir := fs.String("media-dir", "media", "path to media directory")
	force := fs.Bool("force", false, "regenerate even if thumbnail already exists")

	return &ff.Command{
		Name:      "regenerate-thumbnails",
		Usage:     "btta regenerate-thumbnails [flags]",
		ShortHelp: "generate cached thumbnails for all scores (use --force to rebuild existing)",
		Flags:     ff.NewFlagSetFrom("regenerate-thumbnails", fs),
		Exec: func(ctx context.Context, args []string) error {
			logger := slogx.FromContext(ctx)

			database, err := storage.Open(*dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			queries := db.New(database)
			scores, err := queries.ListScores(ctx)
			if err != nil {
				return fmt.Errorf("list scores: %w", err)
			}

			var generated, failed int
			for _, s := range scores {
				if s.GameBanner == "" {
					continue
				}
				srcPath := filepath.Join(*mediaDir, s.GameBanner)
				dstPath := thumbnail.Path(*mediaDir, s.GameBanner)

				if *force {
					err = thumbnail.Generate(srcPath, dstPath)
				} else {
					err = thumbnail.Ensure(srcPath, dstPath)
				}

				if err != nil {
					logger.Warn("thumbnail failed", slog.String("file", s.GameBanner), slogx.Err(err))
					failed++
					continue
				}
				generated++
			}

			if !*force {
				// Ensure skips existing thumbnails silently; we can't distinguish
				// generated vs already-cached without stat-ing before the call.
				logger.Info("done", slog.Int("generated", generated), slog.Int("failed", failed))
			} else {
				logger.Info("done", slog.Int("regenerated", generated), slog.Int("failed", failed))
			}
			return nil
		},
	}
}
