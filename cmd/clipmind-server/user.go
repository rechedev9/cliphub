package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/rechedev9/cliphub/internal/accounts"
	"github.com/rechedev9/cliphub/internal/clipmind"
)

const (
	userUsage = `usage: clipmind-server user add -username NAME [-email ADDRESS] [-name "SHOWN NAME"] < password-file
       clipmind-server user passwd -username NAME < password-file`
	tokenUsage = `usage: clipmind-server token add -username NAME -name LABEL [-workspace ID]
       clipmind-server token rm -username NAME -id TOKEN_ID`
)

// operator is one run of an operator command over CLIPMIND_DATA_DIR.
type operator struct {
	dataDir        string
	iterations     int
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (o operator) catalog() (*accounts.Service, error) {
	return clipmind.OpenCatalog(clipmind.Config{DataDir: o.dataDir, PBKDF2Iterations: o.iterations})
}

// readPassword takes the first line of standard input, never an argument, so it
// stays out of the shell history and the process list.
func (o operator) readPassword() (string, error) {
	line, err := bufio.NewReader(o.stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the password from standard input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// runUser is "clipmind-server user add|passwd".
func (o operator) runUser(args []string) error {
	if len(args) == 0 {
		return errors.New(userUsage)
	}
	flags := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
	username := flags.String("username", "", "login name: 3 to 32 letters, digits, dots, dashes or underscores")
	email, name := new(string), new(string)
	switch args[0] {
	case "add":
		email = flags.String("email", "", "optional email; without one the account cannot reset its password by email")
		name = flags.String("name", "", "optional name the team sees")
	case "passwd":
	default:
		return errors.New(userUsage)
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *username == "" || flags.NArg() > 0 {
		return errors.New(userUsage)
	}
	password, err := o.readPassword()
	if err != nil {
		return err
	}
	svc, err := o.catalog()
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()
	if args[0] == "add" {
		user, err := svc.CreateUser(ctx, accounts.NewUserInput{Username: *username, Email: *email, Password: password, DisplayName: *name})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(o.stdout, "created user %s (%s)\n", user.Username, user.ID)
		return err
	}
	user, err := svc.UserByLogin(ctx, *username)
	if err != nil {
		return fmt.Errorf("user %q: %w", *username, err)
	}
	if err := svc.SetPassword(ctx, user.ID, password); err != nil {
		return err
	}
	_, err = fmt.Fprintf(o.stdout, "changed the password of %s and signed out its sessions\n", user.Username)
	return err
}

// runToken is "clipmind-server token add|rm": an assistant key for one
// workspace, as the account's own settings page makes it. add prints the secret
// alone on standard output, once, and its id on standard error.
func (o operator) runToken(args []string) error {
	if len(args) == 0 || (args[0] != "add" && args[0] != "rm") {
		return errors.New(tokenUsage)
	}
	flags := flag.NewFlagSet("token "+args[0], flag.ContinueOnError)
	username := flags.String("username", "", "the account the token acts as")
	label := flags.String("name", "", "what the token is for (add)")
	workspace := flags.String("workspace", "", "workspace id (add); default: the account's only workspace")
	id := flags.String("id", "", "token id (rm)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *username == "" || flags.NArg() > 0 || (args[0] == "rm" && *id == "") {
		return errors.New(tokenUsage)
	}
	svc, err := o.catalog()
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()
	user, err := svc.UserByLogin(ctx, *username)
	if err != nil {
		return fmt.Errorf("user %q: %w", *username, err)
	}
	if args[0] == "rm" {
		return svc.RevokeAPIToken(ctx, user.ID, *id)
	}
	workspaceID, err := pickWorkspace(ctx, svc, user.ID, *workspace)
	if err != nil {
		return err
	}
	secret, token, err := svc.CreateAPIToken(ctx, user.ID, workspaceID, *label)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(o.stdout, secret); err != nil {
		return err
	}
	_, err = fmt.Fprintf(o.stderr, "token %s for workspace %s; revoke it with: clipmind-server token rm -username %s -id %s\n",
		token.ID, workspaceID, user.Username, token.ID)
	return err
}

func pickWorkspace(ctx context.Context, svc *accounts.Service, userID, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	spaces, err := svc.ListWorkspaces(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(spaces) != 1 {
		return "", fmt.Errorf("the account has %d workspaces; pass -workspace", len(spaces))
	}
	return spaces[0].ID, nil
}
