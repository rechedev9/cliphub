package clipmind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/store"
)

// captureMailer keeps the emails so a test can follow their links.
type captureMailer struct {
	mu   sync.Mutex
	mail []string
}

func (m *captureMailer) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mail = append(m.mail, to+"\n"+subject+"\n"+body)
	return nil
}

var tokenInMail = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// tokenFor waits for the dispatcher to deliver an email to the address whose
// subject contains the text, and returns the token in its link.
func (m *captureMailer) tokenFor(t *testing.T, to, subject string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		for i := len(m.mail) - 1; i >= 0; i-- {
			parts := strings.SplitN(m.mail[i], "\n", 3)
			if parts[0] == to && strings.Contains(parts[1], subject) {
				if match := tokenInMail.FindStringSubmatch(parts[2]); match != nil {
					m.mu.Unlock()
					return match[1]
				}
			}
		}
		m.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %q email with a token for %s", subject, to)
	return ""
}

type testServer struct {
	t      *testing.T
	srv    *Server
	mailer *captureMailer
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	mailer := &captureMailer{}
	srv, err := New(Config{DataDir: t.TempDir(), PublicURL: "https://clipmind.test", Mailer: mailer, PBKDF2Iterations: 1000, CommunityURL: httpapi.CommunityOff})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return &testServer{t: t, srv: srv, mailer: mailer}
}

func (ts *testServer) call(method, path, token string, body any) (int, []byte) {
	ts.t.Helper()
	var raw []byte
	switch b := body.(type) {
	case nil:
	case []byte:
		raw = b
	default:
		raw, _ = json.Marshal(b)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.RemoteAddr = "203.0.113.7:4000"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		if _, isBytes := body.([]byte); !isBytes {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	rw := httptest.NewRecorder()
	ts.srv.Handler().ServeHTTP(rw, req)
	return rw.Code, rw.Body.Bytes()
}

func (ts *testServer) want(status int, method, path, token string, body any) []byte {
	ts.t.Helper()
	code, out := ts.call(method, path, token, body)
	if code != status {
		ts.t.Fatalf("%s %s = %d, want %d; body=%s", method, path, code, status, out)
	}
	return out
}

type session struct {
	Access    string `json:"access_token"`
	Refresh   string `json:"refresh_token"`
	Workspace string `json:"workspace_id"`
	UserID    string `json:"user_id"`
}

// signUp creates a confirmed account and logs it in.
func (ts *testServer) signUp(email string) session {
	ts.t.Helper()
	ts.want(http.StatusAccepted, http.MethodPost, "/auth/signup", "", map[string]string{"email": email, "password": "correct horse battery"})
	token := ts.mailer.tokenFor(ts.t, email, "Confirma")
	ts.want(http.StatusOK, http.MethodPost, "/auth/confirm", "", map[string]string{"token": token, "password": "correct horse battery"})
	var s session
	out := ts.want(http.StatusOK, http.MethodPost, "/auth/login", "", map[string]string{"email": email, "password": "correct horse battery", "device": "test"})
	if err := json.Unmarshal(out, &s); err != nil {
		ts.t.Fatal(err)
	}
	return s
}

func TestAccountRoutesRequireAuthAndNoDataWithoutIt(t *testing.T) {
	ts := newTestServer(t)
	for _, path := range []string{"/auth/me", "/workspaces", "/api/library/matches", "/api/sync/changes?after=0"} {
		ts.want(http.StatusUnauthorized, http.MethodGet, path, "", nil)
	}
	ts.want(http.StatusUnauthorized, http.MethodGet, "/api/library/matches", "garbage.token", nil)
	ts.want(http.StatusUnauthorized, http.MethodPost, "/auth/login", "", map[string]string{"email": "x@example.com", "password": "whatever it is"})
}

func TestWorkspacesAreIsolated(t *testing.T) {
	ts := newTestServer(t)
	ana := ts.signUp("ana@example.com")
	bob := ts.signUp("bob@example.com")

	team := map[string]string{"name": "Los Rivales"}
	ts.want(http.StatusCreated, http.MethodPost, "/api/library/teams", ana.Access, team)
	var anaTeams struct {
		Teams []struct{ Name string } `json:"teams"`
	}
	if err := json.Unmarshal(ts.want(http.StatusOK, http.MethodGet, "/api/library/teams", ana.Access, nil), &anaTeams); err != nil || len(anaTeams.Teams) != 1 {
		t.Fatalf("ana sees %+v (%v), want her team", anaTeams, err)
	}
	var bobTeams struct {
		Teams []struct{ Name string } `json:"teams"`
	}
	if err := json.Unmarshal(ts.want(http.StatusOK, http.MethodGet, "/api/library/teams", bob.Access, nil), &bobTeams); err != nil || len(bobTeams.Teams) != 0 {
		t.Fatalf("bob sees %+v (%v), want an empty library", bobTeams, err)
	}
	if ana.Workspace == bob.Workspace {
		t.Fatal("two accounts share a workspace")
	}
}

func TestInviteAndMembersShareTheWorkspaceLibrary(t *testing.T) {
	ts := newTestServer(t)
	ana := ts.signUp("ana@example.com")
	bob := ts.signUp("bob@example.com")

	var team struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ts.want(http.StatusCreated, http.MethodPost, "/workspaces", ana.Access, map[string]string{"name": "Equipo A"}), &team); err != nil {
		t.Fatal(err)
	}
	ts.want(http.StatusNotFound, http.MethodGet, "/workspaces/"+team.ID+"/members", bob.Access, nil)
	ts.want(http.StatusCreated, http.MethodPost, "/workspaces/"+team.ID+"/invitations", ana.Access, map[string]string{"email": "bob@example.com"})
	invite := ts.mailer.tokenFor(t, "bob@example.com", "invitado")
	ts.want(http.StatusOK, http.MethodPost, "/invitations/accept", bob.Access, map[string]string{"token": invite})

	// Both work in the team workspace once they switch to it.
	var anaTeamSession, bobTeamSession session
	_ = json.Unmarshal(ts.want(http.StatusOK, http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": ana.Refresh, "workspace_id": team.ID}), &anaTeamSession)
	_ = json.Unmarshal(ts.want(http.StatusOK, http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": bob.Refresh, "workspace_id": team.ID}), &bobTeamSession)
	ts.want(http.StatusCreated, http.MethodPost, "/api/library/teams", anaTeamSession.Access, map[string]string{"name": "Compartido"})
	if out := ts.want(http.StatusOK, http.MethodGet, "/api/library/teams", bobTeamSession.Access, nil); !bytes.Contains(out, []byte("Compartido")) {
		t.Fatalf("bob does not see the shared team: %s", out)
	}

	// Only the owner expels; the member loses the workspace at once.
	ts.want(http.StatusForbidden, http.MethodDelete, "/workspaces/"+team.ID+"/members/"+ana.UserID, bobTeamSession.Access, nil)
	ts.want(http.StatusOK, http.MethodDelete, "/workspaces/"+team.ID+"/members/"+bob.UserID, anaTeamSession.Access, nil)
	ts.want(http.StatusConflict, http.MethodGet, "/api/library/teams", bobTeamSession.Access, nil)
}

func TestWebEditionClosesWhatNeedsTheDemoOrTheMachine(t *testing.T) {
	ts := newTestServer(t)
	ana := ts.signUp("ana@example.com")
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/library/demos"},
		{http.MethodPost, "/api/library/demos/00000000-0000-0000-0000-000000000000/analyze"},
		{http.MethodPost, "/api/library/matches/00000000-0000-0000-0000-000000000000/voice"},
		{http.MethodGet, "/api/library/matches/00000000-0000-0000-0000-000000000000/voice/tracks/0/audio"},
		{http.MethodPost, "/api/jobs"},
		{http.MethodGet, "/api/storage/files"},
		{http.MethodPost, "/api/steam/import"},
		{http.MethodPost, "/api/editor/assets"},
		{http.MethodGet, "/api/capabilities"},
		{http.MethodGet, "/metrics"},
	} {
		code, _ := ts.call(c.method, c.path, ana.Access, nil)
		if code != http.StatusNotFound && code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 404 or 405", c.method, c.path, code)
		}
	}
}

func TestWorkspaceRoutesAreExactlyTheOnesTheOrchestratorServes(t *testing.T) {
	ts := newTenantsForRoutes(t)
	inner := map[string]bool{}
	err := chi.Walk(ts, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		inner[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, rule := range workspaceRoutes() {
		key := rule.method + " " + rule.pattern
		if seen[key] {
			t.Errorf("%s is listed twice", key)
		}
		seen[key] = true
		if !inner[key] {
			t.Errorf("%s is in workspaceRoutes but the orchestrator has no such route", key)
		}
		if rule.perm == "" {
			t.Errorf("%s has no permission", key)
		}
	}
	for _, rule := range neverExposed {
		if seen[rule.method+" "+rule.pattern] {
			t.Errorf("%s %s must never be exposed", rule.method, rule.pattern)
		}
	}
}

// newTenantsForRoutes builds the same handlers a workspace gets, to walk their routes.
func newTenantsForRoutes(t *testing.T) chi.Routes {
	t.Helper()
	repos := store.NewMemory()
	h := httpapi.NewHandlers(repos.Jobs, nil, noQueue{},
		httpapi.WithLibrary(repos.Library), httpapi.WithLibraryUploads(), httpapi.WithSyncHub(store.NewSyncLog(nil)))
	return httpapi.Routes(h)
}

func TestPublicRoutesAreRateLimited(t *testing.T) {
	ts := newTestServer(t)
	var last int
	for i := 0; i < 12; i++ {
		last, _ = ts.call(http.MethodPost, "/auth/login", "", map[string]string{"email": "x@example.com", "password": "whatever it is"})
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("12th login in a minute = %d, want 429", last)
	}
}

func TestUnconfirmedAccountCannotLogInAndErrorsLeakNothing(t *testing.T) {
	ts := newTestServer(t)
	ts.want(http.StatusAccepted, http.MethodPost, "/auth/signup", "", map[string]string{"email": "ana@example.com", "password": "correct horse battery"})
	code, out := ts.call(http.MethodPost, "/auth/login", "", map[string]string{"email": "ana@example.com", "password": "correct horse battery"})
	if code != http.StatusForbidden || !bytes.Contains(out, []byte("email_not_confirmed")) {
		t.Fatalf("login before confirming = %d %s", code, out)
	}
	_, unknown := ts.call(http.MethodPost, "/auth/login", "", map[string]string{"email": "nobody@example.com", "password": "correct horse battery"})
	_, wrong := ts.call(http.MethodPost, "/auth/login", "", map[string]string{"email": "ana@example.com", "password": "not the password!!"})
	if !bytes.Equal(unknown, wrong) {
		t.Fatalf("login errors differ: unknown=%s wrong=%s", unknown, wrong)
	}
}

// neverExposed names orchestrator routes the web server must not serve under
// any permission, because they read the demo file or the host. A test checks
// that none of them is in workspaceRoutes.
var neverExposed = []routeRule{
	{http.MethodPost, "/api/library/demos", permWrite},
	{http.MethodPost, "/api/library/demos/{id}/analyze", permWrite},
	{http.MethodPost, "/api/library/demos/{id}/jobs", permWrite},
	{http.MethodPost, "/api/library/jobs", permWrite},
	{http.MethodPost, "/api/library/matches/{id}/replay-video", permWrite},
	{http.MethodPost, "/api/library/matches/{id}/tactical/rounds/{round}/bot-replay", permWrite},
	{http.MethodPost, "/api/library/matches/{id}/voice", permWrite},
	{http.MethodPost, "/api/library/matches/{id}/voice/demo", permWrite},
	{http.MethodPost, "/api/library/matches/{id}/voice/transcribe", permWrite},
	{http.MethodGet, "/api/library/matches/{id}/voice/tracks/{track}/audio", permRead},
	{http.MethodGet, "/api/library/matches/{id}/voice/demo/{steamid}/audio", permRead},
	{http.MethodGet, "/api/library/replay-videos/{video}", permRead},
	{http.MethodPost, "/api/library/voice/plugin/setup", permWrite},
	{http.MethodPost, "/api/library/voice/transcriber/install", permWrite},
	{http.MethodPost, "/api/library/voice/recordings/{id}/attach", permWrite},
	{http.MethodPost, "/api/jobs", permWrite},
	{http.MethodPost, "/api/steam/import", permWrite},
	{http.MethodGet, "/api/storage/files", permRead},
}

func TestAssistantTokenReadsTheLibraryButCannotManageTheAccount(t *testing.T) {
	ts := newTestServer(t)
	ana := ts.signUp("ana@example.com")
	ts.want(http.StatusCreated, http.MethodPost, "/api/library/teams", ana.Access, map[string]string{"name": "Rival"})

	var created struct {
		Secret string `json:"secret"`
		Token  struct {
			ID string `json:"id"`
		} `json:"token"`
	}
	if err := json.Unmarshal(ts.want(http.StatusCreated, http.MethodPost, "/auth/tokens", ana.Access, map[string]string{"name": "Claude"}), &created); err != nil {
		t.Fatal(err)
	}
	if out := ts.want(http.StatusOK, http.MethodGet, "/api/library/teams", created.Secret, nil); !bytes.Contains(out, []byte("Rival")) {
		t.Fatalf("the assistant does not see the library: %s", out)
	}
	ts.want(http.StatusOK, http.MethodGet, "/auth/me", created.Secret, nil)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/auth/tokens"}, {http.MethodPost, "/auth/tokens"}, {http.MethodGet, "/auth/sessions"},
		{http.MethodGet, "/auth/export"}, {http.MethodGet, "/workspaces"},
		{http.MethodPost, "/workspaces/" + ana.Workspace + "/invitations"},
	} {
		ts.want(http.StatusForbidden, c.method, c.path, created.Secret, map[string]string{"name": "x", "email": "x@example.com"})
	}

	// The MCP adapter sends the key in the orchestrator's header.
	req := httptest.NewRequest(http.MethodGet, "/api/library/teams", nil)
	req.Header.Set("X-ClipHub-Token", created.Secret)
	rw := httptest.NewRecorder()
	ts.srv.Handler().ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("X-ClipHub-Token = %d", rw.Code)
	}

	ts.want(http.StatusOK, http.MethodDelete, "/auth/tokens/"+created.Token.ID, ana.Access, nil)
	ts.want(http.StatusUnauthorized, http.MethodGet, "/api/library/teams", created.Secret, nil)
}
