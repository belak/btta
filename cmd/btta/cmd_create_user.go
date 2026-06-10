package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/belak/x/pass"
	"github.com/peterbourgon/ff/v4"
	"golang.org/x/term"

	"github.com/belak/btta/internal/db"
	"github.com/belak/btta/internal/storage"
)

func newCreateUserCmd() *ff.Command {
	fs := flag.NewFlagSet("create-user", flag.ContinueOnError)
	dbPath := fs.String("db", "btta.db", "path to SQLite database")
	passwordFlag := fs.String("password", "", "password (omit to prompt interactively)")

	return &ff.Command{
		Name:      "create-user",
		Usage:     "btta create-user [FLAGS] <username>",
		ShortHelp: "create an admin user",
		Flags:     ff.NewFlagSetFrom("create-user", fs),
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: btta create-user <username>")
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

			hash, err := pass.NewDefaultContext().Hash(string(password))
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
				PasswordHash: hash,
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}

			fmt.Printf("Created user %q (id=%d)\n", user.Username, user.ID)
			return nil
		},
	}
}
