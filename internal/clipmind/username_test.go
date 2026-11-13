package clipmind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/accounts"
	"github.com/rechedev9/cliphub/internal/httpapi"
)

// An account made with "clipmind-server user add" signs in with its username,
// and a wrong password answers exactly like an unknown username.
func TestOperatorAccountSignsInWithItsUsername(t *testing.T) {
	dir := t.TempDir()
	catalog, err := OpenCatalog(Config{DataDir: dir, PBKDF2Iterations: 1000})
	if err != nil {
		t.Fatalf("OpenCatalog: %v", err)
	}
	if _, err := catalog.CreateUser(context.Background(), accounts.NewUserInput{Username: "solo", Password: "ocho8888"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := catalog.Close(); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{DataDir: dir, PublicURL: "https://clipmind.test", Mailer: &captureMailer{}, PBKDF2Iterations: 1000, CommunityURL: httpapi.CommunityOff})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	ts := &testServer{t: t, srv: srv}

	var s session
	if err := json.Unmarshal(ts.want(http.StatusOK, http.MethodPost, "/auth/login", "", map[string]string{"login": "Solo", "password": "ocho8888"}), &s); err != nil {
		t.Fatal(err)
	}
	var me struct {
		User accounts.User `json:"user"`
	}
	if err := json.Unmarshal(ts.want(http.StatusOK, http.MethodGet, "/auth/me", s.Access, nil), &me); err != nil {
		t.Fatal(err)
	}
	if me.User.Username != "solo" || me.User.Email != "" {
		t.Fatalf("/auth/me user = %+v", me.User)
	}
	_, unknown := ts.call(http.MethodPost, "/auth/login", "", map[string]string{"login": "nadie", "password": "ocho8888"})
	code, wrong := ts.call(http.MethodPost, "/auth/login", "", map[string]string{"login": "solo", "password": "otra cosa"})
	if code != http.StatusUnauthorized || !bytes.Equal(unknown, wrong) || !bytes.Contains(wrong, []byte("invalid_credentials")) {
		t.Fatalf("login errors: unknown=%s wrong=%d %s", unknown, code, wrong)
	}
	ts.want(http.StatusOK, http.MethodPost, "/auth/logout", s.Access, nil)
	ts.want(http.StatusUnauthorized, http.MethodGet, "/auth/me", s.Access, nil)
}

func TestSignupRefusesATakenUsername(t *testing.T) {
	ts := newTestServer(t)
	ts.signUp("ana@example.com")
	out := ts.want(http.StatusConflict, http.MethodPost, "/auth/signup", "", map[string]string{"email": "bob@example.com", "username": "ANA", "password": "correct horse battery"})
	if !bytes.Contains(out, []byte("username_taken")) {
		t.Fatalf("signup with a taken username = %s", out)
	}
}
