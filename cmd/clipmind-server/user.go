package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rechedev9/cliphub/internal/accounts"
	"github.com/rechedev9/cliphub/internal/clipmind"
)

const userUsage = `usage: clipmind-server user add -username NAME [-email ADDRESS] [-name "SHOWN NAME"] < password-file`

// runUser is "clipmind-server user add": an active account in CLIPMIND_DATA_DIR
// with its own workspace. The password comes from standard input, never an argument.
func runUser(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New(userUsage)
	}
	flags := flag.NewFlagSet("user add", flag.ContinueOnError)
	username := flags.String("username", "", "login name: 3 to 32 letters, digits, dots, dashes or underscores")
	email := flags.String("email", "", "optional email; without one the account cannot reset its password by email")
	name := flags.String("name", "", "optional name the team sees")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *username == "" || flags.NArg() > 0 {
		return errors.New(userUsage)
	}
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read the password from standard input: %w", err)
	}
	svc, err := clipmind.OpenCatalog(clipmind.Config{DataDir: env("CLIPMIND_DATA_DIR", "clipmind-data")})
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	user, err := svc.CreateUser(context.Background(), accounts.NewUserInput{
		Username: *username, Email: *email, Password: strings.TrimRight(password, "\r\n"), DisplayName: *name,
	})
	if err != nil {
		return err
	}
	fmt.Printf("created user %s (%s)\n", user.Username, user.ID)
	return nil
}
