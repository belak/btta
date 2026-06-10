package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/belak/x/slogx"
	"github.com/belak/x/versionx"

	"github.com/belak/btta/internal/buildinfo"
)

func main() {
	root := &ff.Command{
		Name:  "btta",
		Usage: "btta <subcommand>",
		Subcommands: []*ff.Command{
			newServeCmd(),
			newImportCmd(),
			newUsersCmd(),
			newThumbnailsCmd(),
			{
				Name:      "version",
				Usage:     "btta version",
				ShortHelp: "print version and exit",
				Exec: func(ctx context.Context, args []string) error {
					fmt.Printf("btta %s (go %s)\n", versionx.Get(buildinfo.Version), versionx.GoVersion())
					return nil
				},
			},
		},
	}

	if err := root.ParseAndRun(context.Background(), os.Args[1:],
		ff.WithEnvVarPrefix("BTTA"),
	); err != nil {
		if errors.Is(err, ff.ErrHelp) || errors.Is(err, ff.ErrNoExec) {
			fmt.Fprintln(os.Stderr, ffhelp.Command(root))
			os.Exit(0)
		}
		slogx.FromContext(context.Background()).Error("fatal", slogx.Err(err))
		os.Exit(1)
	}
}
