package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryTeamDocumentContract(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("a", 0), "de_cache", side{"SEGA-dll", 13, []string{"1"}}, side{"Los Galacticos", 11, []string{"2"}})
			sega := *teamOf(t, m, "ct-start").TeamID
			rival := *teamOf(t, m, "t-start").TeamID

			html := []byte("<!doctype html><title>Antistrat</title><p>Pistolas</p>")
			if err := repo.CreateTeamDocument(f.ctx, &library.TeamDocument{TeamID: uuid.New(), Title: "x", ContentType: library.TeamDocumentHTML}, html); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("CreateTeamDocument(unknown team) = %v, want ErrNotFound", err)
			}
			first := &library.TeamDocument{TeamID: sega, Title: "Pistolas en Cache", FileName: "cache.html", ContentType: library.TeamDocumentHTML, SHA256: "s1"}
			if err := repo.CreateTeamDocument(f.ctx, first, html); err != nil {
				t.Fatal(err)
			}
			if first.ID == uuid.Nil || first.TeamName != "SEGA-dll" || first.SizeBytes != int64(len(html)) || first.CreatedAt.IsZero() {
				t.Fatalf("created document = %+v, want an id, the team name, the size and a date", first)
			}
			html[0] = 'X'
			if content, err := repo.TeamDocumentContent(f.ctx, first.ID); err != nil || !bytes.HasPrefix(content, []byte("<!doctype")) {
				t.Fatalf("TeamDocumentContent = %q, %v; want the stored bytes, not the caller's slice", content, err)
			}
			time.Sleep(2 * time.Millisecond)
			second := &library.TeamDocument{TeamID: sega, Title: "Vetos", ContentType: library.TeamDocumentHTML, SHA256: "s2"}
			if err := repo.CreateTeamDocument(f.ctx, second, []byte("<p>BO3</p>")); err != nil {
				t.Fatal(err)
			}
			other := &library.TeamDocument{TeamID: rival, Title: "Propio", ContentType: library.TeamDocumentHTML, SHA256: "s3"}
			if err := repo.CreateTeamDocument(f.ctx, other, []byte("<p>nosotros</p>")); err != nil {
				t.Fatal(err)
			}
			list, err := repo.ListTeamDocuments(f.ctx, library.TeamDocumentFilter{TeamID: &sega})
			if err != nil || len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
				t.Fatalf("ListTeamDocuments(team) = %+v, %v; want the team's two, newest first", list, err)
			}
			if all, _ := repo.ListTeamDocuments(f.ctx, library.TeamDocumentFilter{}); len(all) != 3 {
				t.Fatalf("ListTeamDocuments() = %d documents, want 3", len(all))
			}
			if page, _ := repo.ListTeamDocuments(f.ctx, library.TeamDocumentFilter{TeamID: &sega, Limit: 1, Offset: 1}); len(page) != 1 || page[0].ID != first.ID {
				t.Fatalf("ListTeamDocuments(page 2) = %+v", page)
			}

			// A merge moves the documents; deleting the team deletes them with it.
			if _, err := repo.MergeTeams(f.ctx, sega, rival); err != nil {
				t.Fatal(err)
			}
			moved, err := repo.GetTeamDocument(f.ctx, first.ID)
			if err != nil || moved.TeamID != rival || moved.TeamName != "Los Galacticos" {
				t.Fatalf("document after the merge = %+v, %v; want it on the team it was merged into", moved, err)
			}
			if err := repo.DeleteTeamDocument(f.ctx, second.ID); err != nil {
				t.Fatal(err)
			}
			if err := repo.DeleteTeamDocument(f.ctx, second.ID); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("DeleteTeamDocument twice = %v, want ErrNotFound", err)
			}
			if _, err := repo.GetTeamDocument(f.ctx, second.ID); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("GetTeamDocument(deleted) = %v, want ErrNotFound", err)
			}
			if _, err := repo.TeamDocumentContent(f.ctx, second.ID); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("TeamDocumentContent(deleted) = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestTeamDocumentKeepsAnEmptiedTeam(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("a", 0), "de_cache", side{"SEGA-dll", 13, []string{"1"}}, side{"Los Galacticos", 11, []string{"2"}})
			auto := *teamOf(t, m, "ct-start").TeamID
			doc := &library.TeamDocument{TeamID: auto, Title: "Antistrat", ContentType: library.TeamDocumentHTML, SHA256: "s"}
			if err := repo.CreateTeamDocument(f.ctx, doc, []byte("<p>x</p>")); err != nil {
				t.Fatal(err)
			}
			named := &library.Team{Name: "SEGA"}
			if err := repo.CreateTeam(f.ctx, named); err != nil {
				t.Fatal(err)
			}
			// Moving the clan name empties the auto team, which a document still holds.
			if _, err := repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: m.ID, TeamKey: "ct-start", TeamID: &named.ID, RememberAlias: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.GetTeamDocument(f.ctx, doc.ID); err != nil {
				t.Fatalf("document after the alias move = %v, want it kept with its team", err)
			}
		})
	}
}
