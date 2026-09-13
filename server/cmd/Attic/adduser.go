package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/config"
	"github.com/perleybrook/attic/server/internal/store"
	"github.com/perleybrook/attic/server/migrations"
)

// runAddUser creates an account.
//
// Attic has no open registration: an admin runs this against the database, on
// the box. That is the whole account-creation story, and it is deliberate —
// a self-hosted server reachable only over a tailnet has no business exposing
// a signup form.
func runAddUser(args []string) error {
	fs := flag.NewFlagSet("adduser", flag.ExitOnError)
	role := fs.String("role", "", "account role: admin, member or kid (default: admin for the first account, otherwise member)")
	passwordStdin := fs.Bool("password-stdin", false, "read the password from stdin instead of prompting")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: attic adduser [flags] <username>\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("adduser: exactly one username is required")
	}
	username := strings.TrimSpace(fs.Arg(0))
	if username == "" {
		return errors.New("adduser: username must not be empty")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := cfg.Logger()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The CLI is often the very first thing run against a new deployment, so
	// it applies migrations too rather than demanding the server be started
	// first.
	if err := migrations.Up(cfg.DatabaseURL, log); err != nil {
		return err
	}

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	existing, err := db.CountUsers(ctx)
	if err != nil {
		return err
	}

	resolvedRole, err := resolveRole(*role, existing == 0)
	if err != nil {
		return err
	}

	password, err := readPassword(*passwordStdin)
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return errors.New("adduser: password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	user, err := db.CreateUser(ctx, username, hash, resolvedRole)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return fmt.Errorf("adduser: the username %q is already taken", username)
		}
		return err
	}

	fmt.Printf("Created %s (%s), id %s\n", user.Username, user.Role, user.ID)
	return nil
}

func resolveRole(requested string, isFirstUser bool) (auth.Role, error) {
	if requested == "" {
		// Somebody has to be able to trigger a scan, so the first account is
		// an admin.
		if isFirstUser {
			return auth.RoleAdmin, nil
		}
		return auth.RoleMember, nil
	}

	role := auth.Role(strings.ToLower(strings.TrimSpace(requested)))
	if !role.Valid() {
		return "", fmt.Errorf("adduser: %q is not a role (admin, member or kid)", requested)
	}
	return role, nil
}

// readPassword prompts twice with echo off, or reads one line from stdin when
// scripted.
func readPassword(fromStdin bool) (string, error) {
	if fromStdin {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("adduser: read password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("adduser: stdin is not a terminal; use --password-stdin")
	}

	fmt.Print("Password: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("adduser: read password: %w", err)
	}

	fmt.Print("Repeat password: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("adduser: read password: %w", err)
	}

	if string(first) != string(second) {
		return "", errors.New("adduser: the passwords do not match")
	}
	return string(first), nil
}
