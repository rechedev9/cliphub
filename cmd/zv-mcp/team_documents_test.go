package main

import (
	"net/http"
	"strings"
	"testing"
)

const testDocumentID = "7d1c5a0e-3b2f-4c58-9e61-0a4b8f2d6c13"

func TestTeamDocumentTools(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	doc := `{"id":"` + testDocumentID + `","team_id":"` + testTeamID + `","team_name":"Vitality","title":"Antistrats‮ Mirage","file_name":"Antistrats Mirage.html",` +
		`"content_type":"text/html","size_bytes":42,"updated_at":"2026-11-14T08:00:00Z"}`
	rec.handle(mux, "GET /api/library/documents", `{"documents":[`+doc+`]}`)
	rec.handle(mux, "GET /api/library/documents/{id}", doc)
	rec.handle(mux, "POST /api/library/document-uploads", doc)
	rec.handle(mux, "DELETE /api/library/documents/{id}", "")
	cs := serveTools(t, mux, registerTeamDocumentTools)

	var list listTeamDocumentsOut
	callTool(t, cs, "list_team_documents", map[string]any{"team_id": testTeamID}, &list)
	if got := rec.get("GET /api/library/documents"); !strings.Contains(got, "team_id="+testTeamID) {
		t.Errorf("list query = %s", got)
	}
	want := "/patrones/" + testTeamID + "?documento=" + testDocumentID + "&panel=documentos"
	if len(list.Documents) != 1 || list.Documents[0].Title != "Antistrats Mirage" || list.Documents[0].StudioPath != want || list.Documents[0].Updated != "2026-11-14" {
		t.Fatalf("documents = %+v, want the cleaned title and the viewer path", list.Documents)
	}

	var added teamDocumentOut
	callTool(t, cs, "add_team_document", map[string]any{"team_id": testTeamID, "title": " B: Antistrats Mirage/B ", "html": "<h1>Mirage</h1>"}, &added)
	body := rec.get("POST /api/library/document-uploads")
	for _, part := range []string{`name="team_id"`, `name="title"` + "\r\n\r\nB: Antistrats Mirage/B\r\n", `filename="B- Antistrats Mirage-B.html"`, "<h1>Mirage</h1>"} {
		if !strings.Contains(body, part) {
			t.Errorf("upload = %q, want %q", body, part)
		}
	}
	toolFails(t, cs, "add_team_document", map[string]any{"team_id": testTeamID, "title": "x", "html": "solo texto"}, "HTML page or fragment")
	toolFails(t, cs, "add_team_document", map[string]any{"team_id": testTeamID, "title": " ", "html": "<p>a</p>"}, "title must not be empty")
	toolFails(t, cs, "add_team_document", map[string]any{"team_id": testTeamID, "title": "x", "html": "<p>" + strings.Repeat("a", 5<<20)}, "at most 5 MiB")

	var del deleteOut
	callTool(t, cs, "delete_team_document", map[string]any{"document_id": testDocumentID}, &del)
	if !del.NeedsApproval || !strings.Contains(del.Summary, "«Antistrats Mirage»") || rec.get("DELETE /api/library/documents/{id}") != "" {
		t.Fatalf("delete = %+v", del)
	}
	callTool(t, cs, "delete_team_document", map[string]any{"document_id": testDocumentID, "approve": del.ApprovalCode}, &del)
	if !del.Deleted || rec.get("DELETE /api/library/documents/{id}") == "" {
		t.Errorf("approved delete = %+v", del)
	}
}
