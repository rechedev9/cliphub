package library

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewTeamDocumentCleansTheUpload(t *testing.T) {
	team := uuid.New()
	doc, err := NewTeamDocument(NewTeamDocumentInput{TeamID: team, Title: "  \tAntistrat\x00 SEGA ", FileName: "../../x/sega.html", Content: []byte("<p>a</p>")})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Antistrat SEGA" || doc.FileName != "sega.html" || doc.SizeBytes != 8 || len(doc.SHA256) != 64 || doc.TeamID != team {
		t.Fatalf("document = %+v", doc)
	}
	for name, in := range map[string]NewTeamDocumentInput{
		"empty":          {TeamID: team, FileName: "a.html"},
		"not HTML":       {TeamID: team, FileName: "a.txt", Content: []byte("hola")},
		"not UTF-8":      {TeamID: team, FileName: "a.html", Content: []byte("<p>\xff</p>")},
		"no title":       {TeamID: team, Content: []byte("<p>a</p>")},
		"long title":     {TeamID: team, Title: strings.Repeat("a", MaxTeamDocumentTitleRunes+1), Content: []byte("<p>a</p>")},
		"too large":      {TeamID: team, FileName: "a.html", Content: append([]byte("<p>"), make([]byte, MaxTeamDocumentBytes)...)},
		"a lone bracket": {TeamID: team, FileName: "a.html", Content: []byte("< 3 rondas")},
	} {
		if _, err := NewTeamDocument(in); err == nil {
			t.Errorf("%s: NewTeamDocument accepted it", name)
		}
	}
}

func TestHasDoctypeLooksPastCommentsAndTheByteOrderMark(t *testing.T) {
	for content, want := range map[string]bool{
		"<!doctype html><p>a</p>":               true,
		"\xEF\xBB\xBF\n  <!DOCTYPE html>":       true,
		"<!-- a --><!-- b -->\n<!DocType html>": true,
		"<p>a</p>":                              false,
		"<!-- no end":                           false,
		"<html><body>":                          false,
	} {
		if got := HasDoctype([]byte(content)); got != want {
			t.Errorf("HasDoctype(%q) = %v, want %v", content, got, want)
		}
	}
}
