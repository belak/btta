package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/storage"
)

func newUsersCmd() *ff.Command {
	return &ff.Command{
		Name:      "users",
		Usage:     "btta users <subcommand>",
		ShortHelp: "manage admin users",
		Subcommands: []*ff.Command{
			newUsersCreateCmd(),
			newUsersForcePasswordResetCmd(),
		},
	}
}

func newUsersCreateCmd() *ff.Command {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	dbPath := fs.String("db", "btta.db", "path to SQLite database")
	passwordFlag := fs.String("password", "", "password (omit to prompt interactively)")

	return &ff.Command{
		Name:      "create",
		Usage:     "btta users create [FLAGS] <username>",
		ShortHelp: "create an admin user",
		Flags:     ff.NewFlagSetFrom("create", fs),
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: btta users create <username>")
			}
			username := args[0]

			var password []byte
			if *passwordFlag != "" {
				password = []byte(*passwordFlag)
			} else {
				fmt.Fprintf(os.Stderr, "Password: ")
				var err error
				password, err = term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
			}
			if len(password) == 0 {
				return fmt.Errorf("password must not be empty")
			}

			hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}

			database, err := storage.Open(*dbPath)
			if err != nil {
				return err
			}
			defer database.Close()

			queries := db.New(database)
			user, err := queries.CreateUser(ctx, db.CreateUserParams{
				Username:     username,
				PasswordHash: string(hash),
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}

			fmt.Printf("Created user %q (id=%d)\n", user.Username, user.ID)
			return nil
		},
	}
}

func newUsersForcePasswordResetCmd() *ff.Command {
	fs := flag.NewFlagSet("force-password-reset", flag.ContinueOnError)
	dbPath := fs.String("db", "btta.db", "path to SQLite database")

	return &ff.Command{
		Name:      "force-password-reset",
		Usage:     "btta users force-password-reset [FLAGS] <username>",
		ShortHelp: "require a user to reset their password on next login",
		Flags:     ff.NewFlagSetFrom("force-password-reset", fs),
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: btta users force-password-reset <username>")
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
