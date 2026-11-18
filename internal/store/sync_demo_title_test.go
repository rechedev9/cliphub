package store

import (
	"context"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestSyncLogCarriesADemoTitleEdit(t *testing.T) {
	ctx := context.Background()
	db, log := newTestSyncLog(t)
	repo := NewSQLiteLibraryRepository(db)
	d := library.Demo{SHA256: "titled", FileName: "titled.dem", Path: "C:/demos/titled.dem"}
	if _, err := repo.CreateDemo(ctx, &d); err != nil {
		t.Fatal(err)
	}
	before, err := log.Changes(ctx, 0, syncLogBatchCap)
	if err != nil {
		t.Fatal(err)
	}
	title := "Final ESL"
	if _, err := repo.UpdateDemo(ctx, d.ID, library.DemoPatch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	batch, err := log.Changes(ctx, before.Next, syncLogBatchCap)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range batch.Changes {
		if c.Table == "library_demos" && c.Op == "upsert" {
			if c.Row["title"] != title {
				t.Fatalf("logged title = %#v, want %q", c.Row["title"], title)
			}
			if _, leaked := c.Row["path"]; leaked {
				t.Fatal("the logged demo row carries its local path")
			}
			return
		}
	}
	t.Fatalf("changes after the title edit = %+v, want an upsert of the demo", batch.Changes)
}
