package main

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v4"

	bthttp "github.com/belak/btta/internal/http"
	"github.com/belak/btta/internal/storage"
	"github.com/belak/x/httpx"
	"github.com/belak/x/slogx"
)

func newServeCmd() *ff.Command {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	bind := fs.String("bind", ":8080", "address to listen on")
	dbPath := fs.String("db", "btta.db", "path to SQLite database")
	mediaDir := fs.String("media-dir", "media", "directory to store uploaded media")
	logFormat := slogx.Format(slogx.FormatPretty)
	logLevel := slogx.Level(slogx.LevelInfo)
	fs.TextVar(&logFormat, "log-format", slogx.FormatPretty, "log format (json, pretty, text)")
	fs.TextVar(&logLevel, "log-level", slogx.LevelInfo, "log level (debug, info, warn, error)")

	return &ff.Command{
		Name:      "serve",
		Usage:     "btta serve [FLAGS]",
		ShortHelp: "start the HTTP server",
		Flags:     ff.NewFlagSetFrom("serve", fs),
		Exec: func(ctx context.Context, args []string) error {
			logger := slogx.New(logFormat, logLevel)

			db, err := storage.Open(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			server := bthttp.NewServer(db, *mediaDir, logger)

			ctx, cancel := httpx.WithSignalShutdown(ctx, logger)
			defer cancel()

			logger.Info("starting server",
				slogx.String("bind", *bind),
				slogx.String("db", *dbPath),
				slogx.String("media_dir", *mediaDir),
			)

			return httpx.ListenAndServe(ctx, *bind, server, logger)
		},
	}
}
