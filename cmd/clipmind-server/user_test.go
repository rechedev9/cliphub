package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/accounts"
	"github.com/rechedev9/cliphub/internal/clipmind"
)

func testOperator(t *testing.T, stdin string) (operator, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return operator{dataDir: t.TempDir(), iterations: 1000, stdin: strings.NewReader(stdin), stdout: &out, stderr: &errOut}, &out, &errOut
}

func (o operator) with(stdin string) operator {
	o.stdin = strings.NewReader(stdin)
	return o
}

func TestOperatorSetsAPasswordItNeverPrints(t *testing.T) {
	op, out, _ := testOperator(t, "first password\n")
	if err := op.runUser([]string{"add", "-username", "korvax", "-name", "Korvax"}); err != nil {
		t.Fatal(err)
	}
	if err := op.with("second password\r\n").runUser([]string{"passwd", "-username", "KORVAX"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "password\n") || strings.Contains(out.String(), "second") {
		t.Fatalf("output %q shows the password", out.String())
	}
	svc, err := op.catalog()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()
	if _, err := svc.Login(ctx, accounts.LoginInput{Login: "korvax", Password: "first password"}); !errors.Is(err, accounts.ErrInvalidCredentials) {
		t.Fatalf("login with the old password = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Login(ctx, accounts.LoginInput{Login: "korvax", Password: "second password"}); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
	for _, args := range [][]string{{"passwd", "-username", "nadie"}, {"passwd"}, {"rename", "-username", "korvax"}} {
		if err := op.with("third password\n").runUser(args); err == nil {
			t.Errorf("user %v succeeded", args)
		}
	}
	if err := op.with("short\n").runUser([]string{"passwd", "-username", "korvax"}); !errors.Is(err, accounts.ErrInvalidInput) {
		t.Fatalf("a short password = %v, want ErrInvalidInput", err)
	}
}

func TestOperatorTokenReachesTheAccountWorkspaceUntilRevoked(t *testing.T) {
	op, out, errOut := testOperator(t, "first password\n")
	if err := op.runUser([]string{"add", "-username", "korvax"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := op.runToken([]string{"add", "-username", "korvax", "-name", "carga de demos"}); err != nil {
		t.Fatal(err)
	}
	secret := strings.TrimSpace(out.String())
	if !strings.HasPrefix(secret, "cmk_") || strings.Contains(secret, "\n") || strings.Contains(errOut.String(), secret) {
		t.Fatalf("stdout %q, stderr %q: want the secret alone on stdout", out.String(), errOut.String())
	}
	id := strings.Fields(errOut.String())[1]

	svc, err := clipmind.OpenCatalog(clipmind.Config{DataDir: op.dataDir, PBKDF2Iterations: 1000})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p, err := svc.Authenticate(ctx, secret)
	user, _ := svc.UserByLogin(ctx, "korvax")
	spaces, _ := svc.ListWorkspaces(ctx, user.ID)
	_ = svc.Close()
	if err != nil || !p.APIToken || p.UserID != user.ID || len(spaces) != 1 || p.WorkspaceID != spaces[0].ID {
		t.Fatalf("Authenticate = %+v, %v; want the account's workspace", p, err)
	}

	if err := op.runToken([]string{"rm", "-username", "korvax"}); err == nil {
		t.Fatal("token rm without -id succeeded")
	}
	if err := op.runToken([]string{"rm", "-username", "korvax", "-id", id}); err != nil {
		t.Fatal(err)
	}
	svc, err = clipmind.OpenCatalog(clipmind.Config{DataDir: op.dataDir, PBKDF2Iterations: 1000})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Close() }()
	if _, err := svc.Authenticate(ctx, secret); !errors.Is(err, accounts.ErrTokenInvalid) {
		t.Fatalf("a revoked token = %v, want ErrTokenInvalid", err)
	}
}
