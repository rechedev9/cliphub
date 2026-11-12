package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/clipmind"
)

type inbox struct {
	mu   sync.Mutex
	body map[string]string
}

func (b *inbox) Send(_ context.Context, to, subject, body string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.body == nil {
		b.body = map[string]string{}
	}
	b.body[to+"|"+subject] = body
	return nil
}

var linkToken = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

func (b *inbox) confirmToken(t *testing.T, to string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		for k, body := range b.body {
			if strings.HasPrefix(k, to+"|") && strings.Contains(k, "Confirma") {
				b.mu.Unlock()
				return linkToken.FindStringSubmatch(body)[1]
			}
		}
		b.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no confirmation email")
	return ""
}

func postJSON(t *testing.T, url, bearer string, body, out any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s = %d %s", url, resp.StatusCode, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
}

type headerTransport struct{ token string }

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+h.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestRemoteModeServesTheWorkspaceToAnAssistantKey(t *testing.T) {
	mail := &inbox{}
	hub, err := clipmind.New(clipmind.Config{DataDir: t.TempDir(), PublicURL: "https://clipmind.test", Mailer: mail, PBKDF2Iterations: 1000})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	hubSrv := httptest.NewServer(hub.Handler())
	t.Cleanup(hubSrv.Close)

	postJSON(t, hubSrv.URL+"/auth/signup", "", map[string]string{"email": "ana@example.com", "password": "correct horse battery"}, nil)
	postJSON(t, hubSrv.URL+"/auth/confirm", "", map[string]string{"token": mail.confirmToken(t, "ana@example.com")}, nil)
	var login struct {
		Access string `json:"access_token"`
	}
	postJSON(t, hubSrv.URL+"/auth/login", "", map[string]string{"email": "ana@example.com", "password": "correct horse battery"}, &login)
	postJSON(t, hubSrv.URL+"/api/library/teams", login.Access, map[string]string{"name": "Los Rivales"}, nil)
	var key struct {
		Secret string `json:"secret"`
	}
	postJSON(t, hubSrv.URL+"/auth/tokens", login.Access, map[string]string{"name": "Claude"}, &key)

	ctx := context.Background()
	handler, err := remoteHandler(ctx, hubSrv.URL)
	if err != nil {
		t.Fatalf("remoteHandler: %v", err)
	}
	mcpSrv := httptest.NewServer(handler)
	t.Cleanup(mcpSrv.Close)

	// No key, or a wrong one, never reaches a tool.
	for _, bearer := range []string{"", "cmk_wrong"} {
		req, _ := http.NewRequest(http.MethodPost, mcpSrv.URL+"/mcp", strings.NewReader(`{}`))
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("MCP request with key %q = %d, want 401", bearer, resp.StatusCode)
		}
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: mcpSrv.URL + "/mcp", HTTPClient: &http.Client{Transport: headerTransport{token: key.Secret}}, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	names := map[string]bool{}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names[tool.Name] = true
	}
	for _, want := range []string{"whoami", "list_teams", "list_matches", "scout_team", "find_rounds"} {
		if !names[want] {
			t.Errorf("remote mode lacks %s", want)
		}
	}
	for _, gone := range []string{"import_demo", "create_job", "generate_short", "render_full_demo", "open_in_studio", "studio_status", "clipmind_sync", "steam_matches", "list_storage_files"} {
		if names[gone] {
			t.Errorf("remote mode still serves %s", gone)
		}
	}
	for name := range names {
		if name != "whoami" && !remoteTools[name] {
			t.Errorf("tool %s is served but not in remoteTools", name)
		}
	}

	who, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "whoami"})
	if err != nil || who.IsError {
		t.Fatalf("whoami: %v %+v", err, who)
	}
	if text := who.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "ana@example.com") || !strings.Contains(text, "Mi espacio") {
		t.Fatalf("whoami = %s", text)
	}
	teams, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_teams"})
	if err != nil || teams.IsError {
		t.Fatalf("list_teams: %v %+v", err, teams)
	}
	if text := teams.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "Los Rivales") {
		t.Fatalf("list_teams = %s", text)
	}
}
