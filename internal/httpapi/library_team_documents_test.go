package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type documentUpload struct {
	teamID, title, fileName string
	content                 []byte
}

func (e libraryTestEnv) uploadDocument(u documentUpload) (int, string) {
	e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if u.teamID != "" {
		_ = mw.WriteField("team_id", u.teamID)
	}
	if u.title != "" {
		_ = mw.WriteField("title", u.title)
	}
	if u.content != nil {
		part, err := mw.CreateFormFile("file", u.fileName)
		if err != nil {
			e.t.Fatal(err)
		}
		_, _ = part.Write(u.content)
	}
	_ = mw.Close()
	rw := e.do(http.MethodPost, "/api/library/document-uploads", body.Bytes(), mw.FormDataContentType())
	return rw.Code, rw.Body.String()
}

func TestTeamDocumentUploadListReadAndDelete(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("a", "SEGA-dll", "Los Galacticos")
	sega := teamOfMatch(t, m, "ct-start").TeamID.String()

	code, body := env.uploadDocument(documentUpload{teamID: sega, fileName: `C:\informes\sega-cache-pistolas.html`, content: []byte("<title>x</title><p>Pistolas</p>")})
	if code != http.StatusCreated {
		t.Fatalf("upload = %d %s, want 201", code, body)
	}
	var list struct {
		Documents []library.TeamDocument `json:"documents"`
	}
	env.json(http.MethodGet, "/api/library/documents?team_id="+sega, nil, http.StatusOK, &list)
	if len(list.Documents) != 1 {
		t.Fatalf("documents = %+v, want the upload", list.Documents)
	}
	doc := list.Documents[0]
	if doc.Title != "sega-cache-pistolas" || doc.FileName != "sega-cache-pistolas.html" || doc.TeamName != "SEGA-dll" || doc.ContentType != "text/html" {
		t.Fatalf("document = %+v, want the title from the base file name and the team", doc)
	}
	if strings.Contains(body, "Pistolas") {
		t.Fatalf("upload response carries the content: %s", body)
	}

	rw := env.do(http.MethodGet, "/api/library/documents/"+doc.ID.String()+"/content", nil, "")
	if rw.Code != http.StatusOK {
		t.Fatalf("content = %d", rw.Code)
	}
	h := rw.Header()
	if !strings.HasPrefix(h.Get("Content-Security-Policy"), "sandbox allow-scripts ") || strings.Contains(h.Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatalf("CSP = %q, want a sandbox without allow-same-origin", h.Get("Content-Security-Policy"))
	}
	if h.Get("Content-Type") != "text/html; charset=utf-8" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("headers = %v", h)
	}
	if got := rw.Body.String(); !strings.HasPrefix(got, "<!doctype html>") || !strings.HasSuffix(got, "<p>Pistolas</p>") {
		t.Fatalf("content = %q, want the fragment behind a doctype", got)
	}

	env.json(http.MethodDelete, "/api/library/documents/"+doc.ID.String(), nil, http.StatusNoContent, nil)
	env.json(http.MethodGet, "/api/library/documents/"+doc.ID.String(), nil, http.StatusNotFound, nil)
}

func TestTeamDocumentUploadRefusesWhatIsNotAnHTMLDocument(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("a", "SEGA-dll", "Los Galacticos")
	sega := teamOfMatch(t, m, "ct-start").TeamID.String()
	big := append([]byte("<p>"), bytes.Repeat([]byte("a"), library.MaxTeamDocumentBytes)...)

	for _, tc := range []struct {
		name string
		up   documentUpload
		want int
	}{
		{"no file", documentUpload{teamID: sega}, http.StatusBadRequest},
		{"bad team id", documentUpload{teamID: "x", fileName: "a.html", content: []byte("<p>a</p>")}, http.StatusBadRequest},
		{"unknown team", documentUpload{teamID: uuid.NewString(), fileName: "a.html", content: []byte("<p>a</p>")}, http.StatusNotFound},
		{"a PDF", documentUpload{teamID: sega, fileName: "a.pdf", content: []byte("%PDF-1.7")}, http.StatusBadRequest},
		{"too large", documentUpload{teamID: sega, fileName: "a.html", content: big}, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code, body := env.uploadDocument(tc.up); code != tc.want {
				t.Fatalf("upload = %d %s, want %d", code, body, tc.want)
			}
		})
	}
	var list struct {
		Documents []library.TeamDocument `json:"documents"`
	}
	env.json(http.MethodGet, "/api/library/documents", nil, http.StatusOK, &list)
	if len(list.Documents) != 0 {
		t.Fatalf("documents = %+v, want none stored", list.Documents)
	}
}

func TestTeamDocumentKeepsADeclaredDoctype(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("a", "SEGA-dll", "Los Galacticos")
	html := "\xEF\xBB\xBF<!-- informe -->\n<!DOCTYPE html><html><body>BO3</body></html>"
	code, _ := env.uploadDocument(documentUpload{teamID: teamOfMatch(t, m, "ct-start").TeamID.String(), title: "Vetos BO3", fileName: "v.html", content: []byte(html)})
	if code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	docs, _ := env.repo.ListTeamDocuments(t.Context(), library.TeamDocumentFilter{})
	if len(docs) != 1 || docs[0].Title != "Vetos BO3" {
		t.Fatalf("documents = %+v, want the given title", docs)
	}
	rw := env.do(http.MethodGet, "/api/library/documents/"+docs[0].ID.String()+"/content", nil, "")
	if rw.Body.String() != html {
		t.Fatalf("content = %q, want the stored bytes untouched", rw.Body.String())
	}
}

// The web app serves and frames a document under the same policy, so a change here must reach both copies.
func TestTeamDocumentPolicyMatchesTheWebApp(t *testing.T) {
	for _, path := range []string{"../../web/lib/api/team-documents.ts", "../../web/next.config.mjs"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), `"`+TeamDocumentPolicy+`"`) {
			t.Errorf("%s does not hold TeamDocumentPolicy word for word", path)
		}
	}
}
