package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/storage"
)

func newForcePasswordResetCmd() *ff.Command {
	fs := flag.NewFlagSet("force-password-reset", flag.ContinueOnError)
	dbPath := fs.String("db", "btta.db", "path to SQLite database")

	return &ff.Command{
		Name:      "force-password-reset",
		Usage:     "btta force-password-reset [FLAGS] <username>",
		ShortHelp: "require a user to reset their password on next login",
		Flags:     ff.NewFlagSetFrom("force-password-reset", fs),
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: btta force-password-reset <username>")
			}
			username := args[0]

			database, err := storage.Open(*dbPath)
			if err != nil {
				return err
			}
			defer database.Close()

			queries := db.New(database)
			user, err := queries.GetUserByUsername(ctx, username)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("user %q not found", username)
			} else if err != nil {
				return fmt.Errorf("get user %q: %w", username, err)
			}

			if err := queries.SetUserForcePasswordReset(ctx, user.ID); err != nil {
				return fmt.Errorf("set force password reset: %w", err)
			}

			fmt.Printf("User %q will be required to reset their password on next login.\n", username)
			return nil
		},
	}
}
