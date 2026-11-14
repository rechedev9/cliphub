package clipmind

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// uploadNaming posts an upload that names only the workspace it was queued in.
func (ts *testServer) uploadNaming(token, workspace string) (int, []byte) {
	ts.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("workspace_id", workspace); err != nil {
		ts.t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		ts.t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/library/uploads", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rw := httptest.NewRecorder()
	ts.srv.Handler().ServeHTTP(rw, req)
	return rw.Code, rw.Body.Bytes()
}

// A bulk import queued in one workspace never lands in the one the session switched to.
func TestUploadQueuedInAnotherWorkspaceIsRefused(t *testing.T) {
	ts := newTestServer(t)
	ana := ts.signUp("ana@example.com")
	var team struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ts.want(http.StatusCreated, http.MethodPost, "/workspaces", ana.Access, map[string]string{"name": "Equipo A"}), &team); err != nil {
		t.Fatal(err)
	}
	var inTeam session
	out := ts.want(http.StatusOK, http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": ana.Refresh, "workspace_id": team.ID})
	if err := json.Unmarshal(out, &inTeam); err != nil || inTeam.Workspace != team.ID {
		t.Fatalf("refresh into the team = %+v (%v)", inTeam, err)
	}

	code, out := ts.uploadNaming(inTeam.Access, ana.Workspace)
	if code != http.StatusConflict || !bytes.Contains(out, []byte(`"code":"workspace_changed"`)) {
		t.Fatalf("upload queued in the personal workspace = %d %s, want 409 workspace_changed", code, out)
	}
	// The session's own workspace passes the check; this empty upload then fails on its missing parts.
	code, out = ts.uploadNaming(inTeam.Access, team.ID)
	if code != http.StatusBadRequest || !bytes.Contains(out, []byte("missing document part")) {
		t.Fatalf("upload queued in the team = %d %s, want 400 for the missing document", code, out)
	}
}
