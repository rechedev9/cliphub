package accounts

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestLoginTakesTheUsernameOrTheEmail(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	if err := s.Signup(ctx, SignupInput{Email: "ana@example.com", Username: " Ana_1 ", Password: "correct horse battery"}); err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if _, err := s.Login(ctx, LoginInput{Login: "ana_1", Password: "correct horse battery"}); !errors.Is(err, ErrEmailNotConfirmed) {
		t.Fatalf("username login before confirming = %v, want ErrEmailNotConfirmed", err)
	}
	if err := s.ConfirmEmail(ctx, lastToken(t, s, "ana@example.com"), "correct horse battery"); err != nil {
		t.Fatalf("ConfirmEmail: %v", err)
	}
	for _, login := range []string{"ana_1", "ANA_1", " ana_1 ", "ana@example.com"} {
		tokens, err := s.Login(ctx, LoginInput{Login: login, Password: "correct horse battery"})
		if err != nil {
			t.Fatalf("Login(%q): %v", login, err)
		}
		u, err := s.GetUser(ctx, tokens.UserID)
		if err != nil || u.Username != "ana_1" || u.Email != "ana@example.com" {
			t.Fatalf("GetUser after Login(%q) = %+v, %v", login, u, err)
		}
	}
	for _, in := range []LoginInput{
		{Login: "ana_1", Password: "wrong password!"},
		{Login: "nadie", Password: "correct horse battery"},
		{Login: "", Password: "correct horse battery"},
	} {
		if _, err := s.Login(ctx, in); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login(%+v) = %v, want ErrInvalidCredentials", in, err)
		}
	}
}

func TestUsernameBelongsToOneAccount(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	confirmedUser(t, s, "ana@example.com")
	if err := s.Signup(ctx, SignupInput{Email: "bob@example.com", Username: "ANA", Password: "correct horse battery"}); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("Signup with a taken username = %v, want ErrUsernameTaken", err)
	}
	if _, err := s.CreateUser(ctx, NewUserInput{Username: "ana", Password: "correct horse battery"}); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("CreateUser with a taken username = %v, want ErrUsernameTaken", err)
	}
	if _, err := s.CreateUser(ctx, NewUserInput{Username: "otra", Email: "ANA@example.com", Password: "correct horse battery"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateUser with a taken email = %v, want ErrConflict", err)
	}
	// Signing up again before confirming may change the username of the same account.
	if err := s.Signup(ctx, SignupInput{Email: "carol@example.com", Username: "carol", Password: "correct horse battery"}); err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if err := s.Signup(ctx, SignupInput{Email: "carol@example.com", Username: "carol", Password: "correct horse battery"}); err != nil {
		t.Fatalf("second Signup with its own username: %v", err)
	}
}

func TestCreateUserWithoutEmail(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, NewUserInput{Username: "solo", Password: "siete77"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateUser with 7 characters = %v, want ErrInvalidInput", err)
	}
	solo, err := s.CreateUser(ctx, NewUserInput{Username: "Solo", Password: "ocho8888", DisplayName: "Solo"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// A second account without email proves email is optional, not unique-empty.
	if _, err := s.CreateUser(ctx, NewUserInput{Username: "otro", Password: "ocho8888"}); err != nil {
		t.Fatalf("second CreateUser without email: %v", err)
	}
	tokens, err := s.Login(ctx, LoginInput{Login: "solo", Password: "ocho8888"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.UserID != solo.ID || tokens.WorkspaceID == "" {
		t.Fatalf("Login = %+v, want the user and its own workspace", tokens)
	}
	members, err := s.Members(ctx, tokens.WorkspaceID)
	if err != nil || len(members) != 1 || members[0].Username != "solo" || members[0].Email != "" {
		t.Fatalf("Members = %+v, %v", members, err)
	}
	if export, err := s.ExportUser(ctx, solo.ID); err != nil || export.User.Username != "solo" {
		t.Fatalf("ExportUser = %+v, %v", export, err)
	}
	if err := s.ForgotPassword(ctx, "solo"); err != nil {
		t.Fatalf("ForgotPassword with a username: %v", err)
	}
	if _, err := s.DeleteUser(ctx, solo.ID, "ocho8888"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.Login(ctx, LoginInput{Login: "solo", Password: "ocho8888"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login after delete = %v, want ErrInvalidCredentials", err)
	}
}

// A catalog made before usernames required an email; opening it keeps every
// account and lets new ones go without email.
func TestOpenMigratesACatalogWithoutUsernames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := (&Service{iterations: 1000}).hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE users (
			id                 TEXT PRIMARY KEY,
			email              TEXT NOT NULL UNIQUE COLLATE NOCASE,
			password_hash      TEXT NOT NULL,
			display_name       TEXT NOT NULL DEFAULT '',
			email_confirmed_at INTEGER,
			failed_logins      INTEGER NOT NULL DEFAULT 0,
			locked_until       INTEGER NOT NULL DEFAULT 0,
			created_at         INTEGER NOT NULL
		)`,
		`INSERT INTO users (id, email, password_hash, display_name, email_confirmed_at, created_at) VALUES ('u1', 'ana@example.com', '` + hash + `', 'Ana', 1, 1)`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("old schema: %v", err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	opts := Options{SigningKey: []byte("0123456789abcdef0123456789abcdef"), PBKDF2Iterations: 1000}
	for range 2 {
		s, err := Open(path, opts)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		ctx := context.Background()
		if _, err := s.Login(ctx, LoginInput{Login: "ana@example.com", Password: "correct horse battery"}); err != nil {
			t.Fatalf("Login of a migrated account: %v", err)
		}
		if _, err := s.CreateWorkspace(ctx, "u1", "Equipo"); err != nil {
			t.Fatalf("CreateWorkspace for a migrated account: %v", err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(path, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, NewUserInput{Username: "nuevo", Password: "ocho8888"}); err != nil {
		t.Fatalf("CreateUser without email after migrating: %v", err)
	}
	if deleted, err := s.DeleteUser(ctx, "u1", "correct horse battery"); err != nil || len(deleted) != 2 {
		t.Fatalf("DeleteUser of a migrated account = %v, %v; want its 2 workspaces", deleted, err)
	}
}
