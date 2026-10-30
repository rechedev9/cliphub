package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func uploadJobDemo(t *testing.T, router http.Handler, data []byte) uuid.UUID {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("demo", "scrim-mirage.dem")
	_, _ = part.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, req)
	if rw.Code != http.StatusCreated {
		t.Fatalf("POST /api/jobs = %d; body=%s", rw.Code, rw.Body.String())
	}
	var created struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func TestCrearJobsJoinTheLibrary(t *testing.T) {
	jobs := newFakeRepo()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	queue := &fakeQueue{}
	env := libraryTestEnv{t: t, repo: repo, storage: files, queue: queue, router: Routes(NewHandlers(jobs, files, queue, WithLibrary(repo)))}
	data := append(append([]byte{}, demoMagic...), []byte("crear demo")...)

	jobID := uploadJobDemo(t, env.router, data)
	var demo library.Demo
	env.json(http.MethodGet, "/api/library/jobs/"+jobID.String(), nil, http.StatusOK, &demo)
	if demo.State != library.ParseQueued || demo.Path != artifacts.LibraryDemoKey(demo.ID) || len(demo.JobIDs) != 1 || demo.JobIDs[0] != jobID {
		t.Fatalf("library demo of the job = %+v", demo)
	}
	if !bytes.Equal(files.puts[demo.Path], data) {
		t.Fatalf("library copy holds %q, want the uploaded demo", files.puts[demo.Path])
	}
	if n := len(queue.enqueued); n != 2 || queue.enqueued[0].Type() != tasks.TypeScanRoster || queue.enqueued[1].Type() != tasks.TypeAnalyzeLibraryDemo {
		t.Fatalf("enqueued %d tasks, want the roster scan before the library scan", n)
	}

	// A second Crear job over the library demo links to the same demo.
	var made struct {
		JobID uuid.UUID `json:"job_id"`
	}
	env.json(http.MethodPost, "/api/library/demos/"+demo.ID.String()+"/jobs", nil, http.StatusCreated, &made)
	env.json(http.MethodGet, "/api/library/demos/"+demo.ID.String(), nil, http.StatusOK, &demo)
	if len(demo.JobIDs) != 2 || !containsJob(demo.JobIDs, made.JobID) {
		t.Fatalf("job ids = %v, want both jobs", demo.JobIDs)
	}
	// The analysis area asks for one player's POV: the job parses at once, and
	// asking again before it records returns the same job.
	var pov struct {
		JobID  uuid.UUID `json:"job_id"`
		Reused bool      `json:"reused"`
	}
	target := map[string]string{"target_steamid": "76561198000000009"}
	env.json(http.MethodPost, "/api/library/demos/"+demo.ID.String()+"/jobs", target, http.StatusCreated, &pov)
	if last := queue.enqueued[len(queue.enqueued)-1]; last.Type() != tasks.TypeParseDemo || pov.Reused {
		t.Fatalf("target job enqueued %s (reused %v), want a parse", last.Type(), pov.Reused)
	}
	if j, _ := jobs.Get(context.Background(), pov.JobID); j.TargetSteamID != "76561198000000009" || !bytes.Equal(files.puts[j.DemoPath], data) {
		t.Fatalf("target job = %+v", j)
	}
	first := pov.JobID
	env.json(http.MethodPost, "/api/library/demos/"+demo.ID.String()+"/jobs", target, http.StatusOK, &pov)
	if pov.JobID != first || !pov.Reused {
		t.Fatalf("second ask = %+v, want job %s reused", pov, first)
	}
	if rw := env.do(http.MethodPost, "/api/library/demos/"+demo.ID.String()+"/jobs", []byte(`{"target_steamid":"x"}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("bad target = %d, want 400", rw.Code)
	}

	var listed struct {
		Demos []library.Demo `json:"demos"`
	}
	env.json(http.MethodGet, "/api/library/demos", nil, http.StatusOK, &listed)
	if len(listed.Demos) != 1 {
		t.Fatalf("library holds %d demos, want 1", len(listed.Demos))
	}

	// A job made before the library existed waits until the user adds it.
	ctx := context.Background()
	old := job.Job{ID: uuid.New(), Status: job.StatusDone, DemoPath: "demos/old.dem", DemoSHA256: "old-hash", DemoFileName: "old.dem"}
	_ = jobs.Create(ctx, &old)
	_ = files.Put(old.DemoPath, bytes.NewReader([]byte("old demo")))
	gone := job.Job{ID: uuid.New(), Status: job.StatusDone, DemoPath: "demos/gone.dem", DemoSHA256: "gone-hash"}
	_ = jobs.Create(ctx, &gone)
	var unlinked struct {
		IDs []uuid.UUID `json:"unlinked_job_ids"`
	}
	env.json(http.MethodGet, "/api/library/jobs", nil, http.StatusOK, &unlinked)
	if len(unlinked.IDs) != 1 || unlinked.IDs[0] != old.ID {
		t.Fatalf("unlinked jobs = %v, want only %s", unlinked.IDs, old.ID)
	}
	env.json(http.MethodPost, "/api/library/jobs", nil, http.StatusOK, &listed)
	if len(listed.Demos) != 1 || listed.Demos[0].SHA256 != "old-hash" {
		t.Fatalf("linked demos = %+v", listed.Demos)
	}
	env.json(http.MethodGet, "/api/library/jobs", nil, http.StatusOK, &unlinked)
	if len(unlinked.IDs) != 0 {
		t.Fatalf("unlinked jobs after linking = %v", unlinked.IDs)
	}

	if rw := env.do(http.MethodPost, "/api/library/jobs/"+gone.ID.String(), nil, ""); rw.Code != http.StatusConflict {
		t.Fatalf("link of a job without its demo = %d, want 409", rw.Code)
	}
	if rw := env.do(http.MethodPost, "/api/library/jobs/"+uuid.NewString(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("link of a missing job = %d, want 404", rw.Code)
	}
	if rw := env.do(http.MethodGet, "/api/library/jobs/"+gone.ID.String(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("demo of an unlinked job = %d, want 404", rw.Code)
	}
}

func containsJob(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestLibraryPlayerRoleReadsTheMatchDocuments(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("role", "Vitality", "MOUZ")
	var role library.PlayerRole
	env.json(http.MethodGet, "/api/library/players/76561198000000001/role", nil, http.StatusOK, &role)
	if role.MatchCount != 1 || role.Role != library.RoleUnknown || role.Thresholds.MinRounds != library.RoleMinRounds {
		t.Fatalf("role = %+v, want one match read and no role under %d rounds", role, library.RoleMinRounds)
	}
	for _, e := range role.AWPKills.Examples {
		if e.MatchID != m.ID {
			t.Fatalf("example %+v does not name match %s", e, m.ID)
		}
	}
	if rw := env.do(http.MethodGet, "/api/library/players/76561198000000999/role", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("role of an unknown player = %d, want 404", rw.Code)
	}
}
