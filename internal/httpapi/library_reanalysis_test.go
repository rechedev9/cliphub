package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/store"
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
	_ = env.storage.Put(artifacts.LibraryTacticalIndexKey(old.ID), bytes.NewReader([]byte(`{"schema_version":"1.0","generated_at":"2026-01-01T00:00:00Z","rounds":[]}`)))

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

// A demo an older Studio could not read is scanned again once; a damaged demo,
// or a second start of the same Studio, leaves it failed.
func TestResumeLibraryScansRetriesAnIncompatibleDemoOnce(t *testing.T) {
	env := newLibraryTestEnv(t)
	ctx := context.Background()
	incompatible := library.Demo{SHA256: "i", FileName: "i.dem"}
	truncated := library.Demo{SHA256: "t", FileName: "t.dem"}
	for _, d := range []*library.Demo{&incompatible, &truncated} {
		if _, err := env.repo.CreateDemo(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	failIncompatible := func() {
		t.Helper()
		if err := env.repo.SetDemoState(ctx, incompatible.ID, library.ParseFailed, obs.ClassDemoIncompatible, "parsing demo: demo_incompatible: nextByte"); err != nil {
			t.Fatal(err)
		}
	}
	failIncompatible()
	if err := env.repo.SetDemoState(ctx, truncated.ID, library.ParseFailed, obs.ClassTruncatedDemo, "truncated_demo"); err != nil {
		t.Fatal(err)
	}

	h := NewHandlers(newFakeRepo(), env.storage, env.queue, WithLibrary(env.repo))
	if resumed, err := h.ResumeLibraryScans(ctx); err != nil || resumed != 1 || len(env.queue.enqueued) != 1 {
		t.Fatalf("first start: ResumeLibraryScans = %d, %v, enqueued %d; want the incompatible demo only", resumed, err, len(env.queue.enqueued))
	}
	if d, _ := env.repo.GetDemo(ctx, incompatible.ID); d.State != library.ParseQueued {
		t.Fatalf("incompatible demo state = %s, want queued", d.State)
	}
	if d, _ := env.repo.GetDemo(ctx, truncated.ID); d.State != library.ParseFailed {
		t.Fatalf("truncated demo state = %s, want failed", d.State)
	}

	// The retry fails again on this build: the next start must not loop on it.
	failIncompatible()
	if resumed, err := h.ResumeLibraryScans(ctx); err != nil || resumed != 0 {
		t.Fatalf("second start: ResumeLibraryScans = %d, %v; want nothing on the same build", resumed, err)
	}
}

// A retry the queue refuses keeps the demo failed with its reason and does not spend this build's retry.
func TestResumeLibraryScansKeepsTheRetryOfARefusedFailedDemo(t *testing.T) {
	env := newLibraryTestEnv(t)
	ctx := context.Background()
	d := library.Demo{SHA256: "i", FileName: "i.dem"}
	if _, err := env.repo.CreateDemo(ctx, &d); err != nil {
		t.Fatal(err)
	}
	const reason = "parsing demo: demo_incompatible: nextByte"
	if err := env.repo.SetDemoState(ctx, d.ID, library.ParseFailed, obs.ClassDemoIncompatible, reason); err != nil {
		t.Fatal(err)
	}

	env.queue.err = errors.New("queue full")
	if resumed, err := env.h.ResumeLibraryScans(ctx); err == nil || resumed != 0 {
		t.Fatalf("refused start: ResumeLibraryScans = %d, %v; want the queue error", resumed, err)
	}
	if got, _ := env.repo.GetDemo(ctx, d.ID); got.State != library.ParseFailed || got.ErrorCode != obs.ClassDemoIncompatible || got.Error != reason {
		t.Fatalf("refused demo = %+v, want it failed with its reason", got)
	}
	env.queue.err = nil
	if resumed, err := env.h.ResumeLibraryScans(ctx); err != nil || resumed != 1 {
		t.Fatalf("next start: ResumeLibraryScans = %d, %v; want the refused retry queued", resumed, err)
	}
}

// queueRefusingRepo fails every write that would queue a demo while refuse is set.
type queueRefusingRepo struct {
	*store.MemoryLibraryRepository
	refuse bool
}

func (r *queueRefusingRepo) SetDemoState(ctx context.Context, id uuid.UUID, state library.ParseState, errorCode, message string) error {
	if r.refuse && state == library.ParseQueued {
		return errors.New("database is locked")
	}
	return r.MemoryLibraryRepository.SetDemoState(ctx, id, state, errorCode, message)
}

// A claimed retry whose queued state could not be written is not spent: the next start queues it.
func TestResumeLibraryScansKeepsTheRetryWhenQueueingTheDemoFails(t *testing.T) {
	env := newLibraryTestEnv(t)
	ctx := context.Background()
	repo := &queueRefusingRepo{MemoryLibraryRepository: env.repo}
	d := library.Demo{SHA256: "i", FileName: "i.dem"}
	if _, err := repo.CreateDemo(ctx, &d); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDemoState(ctx, d.ID, library.ParseFailed, obs.ClassDemoIncompatible, "parsing demo: demo_incompatible: nextByte"); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), env.storage, env.queue, WithLibrary(repo))

	repo.refuse = true
	if resumed, err := h.ResumeLibraryScans(ctx); err == nil || resumed != 0 || len(env.queue.enqueued) != 0 {
		t.Fatalf("refused start: ResumeLibraryScans = %d, %v, enqueued %d; want the state error and nothing queued", resumed, err, len(env.queue.enqueued))
	}
	repo.refuse = false
	if resumed, err := h.ResumeLibraryScans(ctx); err != nil || resumed != 1 {
		t.Fatalf("next start: ResumeLibraryScans = %d, %v; want the retry queued", resumed, err)
	}
	if got, _ := repo.GetDemo(ctx, d.ID); got.State != library.ParseQueued {
		t.Fatalf("demo state = %s, want queued", got.State)
	}
}

// The stale check stops after the leading keys, so it depends on the order a
// real Document marshals them in.
func TestLeadingAnalysisVersionReadsAMarshalledDocument(t *testing.T) {
	raw, err := json.Marshal(tacticalplan.NewDocument())
	if err != nil {
		t.Fatal(err)
	}
	version, ok := leadingAnalysisVersion(json.NewDecoder(bytes.NewReader(raw)))
	if !ok || version != tacticalplan.AnalysisVersion {
		t.Fatalf("leadingAnalysisVersion = %d, %v; want %d from %s", version, ok, tacticalplan.AnalysisVersion, raw)
	}
	if _, ok := leadingAnalysisVersion(json.NewDecoder(strings.NewReader("not json"))); ok {
		t.Fatal("leadingAnalysisVersion accepted a corrupt document")
	}
}
