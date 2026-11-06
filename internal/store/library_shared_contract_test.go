package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibrarySharedDemoContract(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			f := libraryFixture{t: t, ctx: ctx, repo: repo}
			d := library.Demo{SHA256: "shared-sha", FileName: "match.dem", Source: library.SourceShared, SourceManual: true, State: library.ParseParsing}
			if created, err := repo.CreateDemo(ctx, &d); err != nil || !created {
				t.Fatalf("CreateDemo = %v, %v", created, err)
			}
			m := f.analyze(d, "de_mirage", side{clan: "Alpha", score: 13}, side{clan: "Beta", score: 5})
			if m.Source != library.SourceShared {
				t.Fatalf("a scan changed the source of a shared match to %q", m.Source)
			}

			attached, err := repo.AttachDemoFile(ctx, d.ID, library.DemoFile{Path: "library/x/demo.dem", FileName: "real.dem", SizeBytes: 42, Source: library.SourceFaceit})
			if err != nil {
				t.Fatalf("AttachDemoFile: %v", err)
			}
			if attached.Path != "library/x/demo.dem" || attached.FileName != "real.dem" || attached.SizeBytes != 42 ||
				attached.Source != library.SourceFaceit || attached.SourceManual {
				t.Fatalf("attached demo = %+v", attached)
			}
			if attached.MatchID == nil || *attached.MatchID != m.ID {
				t.Fatalf("attaching the file lost the match: %+v", attached.MatchID)
			}
			again, err := repo.AttachDemoFile(ctx, d.ID, library.DemoFile{Path: "p", FileName: "f.dem", Source: library.SourceShared})
			if err != nil || again.Source != library.SourceUnknown {
				t.Fatalf("a file never keeps the shared source: %q, %v", again.Source, err)
			}
			if _, err := repo.AttachDemoFile(ctx, uuid.New(), library.DemoFile{}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("AttachDemoFile(missing) = %v, want ErrNotFound", err)
			}
		})
	}
}
