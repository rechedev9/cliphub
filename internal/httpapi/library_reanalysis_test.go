package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestResumeLibraryScansRescansAnalysesFromAnOlderScanner(t *testing.T) {
	env := newLibraryTestEnv(t)
	ctx := context.Background()
	current := library.Demo{SHA256: "c", FileName: "c.dem"}
	old := library.Demo{SHA256: "o", FileName: "o.dem"}
	missing := library.Demo{SHA256: "m", FileName: "m.dem"}
	for _, d := range []*library.Demo{&current, &old, &missing} {
		if _, err := env.repo.CreateDemo(ctx, d); err != nil {
			t.Fatal(err)
		}
		if err := env.repo.SetDemoState(ctx, d.ID, library.ParseReady, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	currentDoc := fmt.Sprintf(`{"schema_version":"1.0","analysis_version":%d}`, tacticalplan.AnalysisVersion)
	_ = env.storage.Put(artifacts.LibraryTacticalIndexKey(current.ID), bytes.NewReader([]byte(currentDoc)))
	_ = env.storage.Put(artifacts.LibraryTacticalIndexKey(old.ID), bytes.NewReader([]byte(`{"schema_version":"1.0"}`)))

	h := NewHandlers(newFakeRepo(), env.storage, env.queue, WithLibrary(env.repo))
	resumed, err := h.ResumeLibraryScans(ctx)
	if err != nil || resumed != 2 {
		t.Fatalf("ResumeLibraryScans = %d, %v; want the old and the missing analysis", resumed, err)
	}
	for _, tc := range []struct {
		demo library.Demo
		want library.ParseState
	}{{current, library.ParseReady}, {old, library.ParseQueued}, {missing, library.ParseQueued}} {
		if d, _ := env.repo.GetDemo(ctx, tc.demo.ID); d.State != tc.want {
			t.Errorf("demo %s state = %s, want %s", tc.demo.FileName, d.State, tc.want)
		}
	}
}
