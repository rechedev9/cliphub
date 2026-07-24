package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/anticheat"
	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/rules"
	"github.com/rechedev9/fragforge/internal/tasks"
)

// anticheatRouter mounts the three CheaterDetect endpoints on a chi router so
// the {id} and {steamid} params resolve exactly as they do in Routes.
func anticheatRouter(h *Handlers) chi.Router {
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/anticheat", h.StartAnticheat)
	r.Get("/api/jobs/{id}/anticheat", h.GetAnticheat)
	r.Get("/api/jobs/{id}/anticheat/dossier/{steamid}", h.GetAnticheatDossier)
	return r
}

func scannedJob() job.Job {
	return job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default(), DemoSHA256: "abc"}
}

func putAnticheat(t *testing.T, store *fakeStorage, id uuid.UUID, doc anticheat.Document) {
	t.Helper()
	var buf bytes.Buffer
	if err := doc.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.AnticheatKey(id), bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
}

// readyDocument returns a stored analysis holding one screened player.
func readyDocument(id uuid.UUID, steamID string) anticheat.Document {
	report := anticheat.Report{
		SchemaVersion: anticheat.SchemaVersion,
		Baseline:      anticheat.BaselineHeader{ID: "cs2-population-v1", Source: "measured-cs2-population"},
		Match:         anticheat.MatchSummary{Map: "de_mirage", Rounds: 24, TickRate: 64},
		Players: []anticheat.PlayerReport{{
			SteamID64: steamID,
			Name:      "sospechoso",
			Verdict:   anticheat.VerdictAnomalous,
			Score:     72.4,
			GunKills:  25,
		}},
		Limitations: []string{"Este informe es un detector de anomalías estadísticas, no una prueba de trampas."},
	}
	return anticheat.NewRunningDocument(id.String(), time.Now()).Complete(report, time.Now())
}

func TestStartAnticheatQueuesTheAnalysisAndClaimsTheLane(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	j := scannedJob()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/anticheat", nil))

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeAnalyzeAnticheat {
		t.Fatalf("queue = %#v, want one anticheat task", queue.enqueued)
	}
	if _, ok := store.puts[artifacts.AnticheatKey(j.ID)]; !ok {
		t.Fatal("the running document must be stored before the task is queued")
	}
	if !strings.Contains(rw.Body.String(), `"status":"running"`) {
		t.Fatalf("body = %s, want a running status", rw.Body.String())
	}
	if repo.jobs[j.ID].Status != job.StatusScanned {
		t.Fatalf("job status = %s, want the screening to leave the clip pipeline untouched", repo.jobs[j.ID].Status)
	}
}

func TestStartAnticheatRejectsADemoStillBeingIngested(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusScanning, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/anticheat", nil))

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("queue = %#v, want nothing enqueued", queue.enqueued)
	}
}

func TestStartAnticheatIsIdempotentWhileRunning(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, anticheat.NewRunningDocument(j.ID.String(), time.Now()))
	h := NewHandlers(repo, store, queue)

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/anticheat", nil))

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("queue = %#v, want no second parse of the same demo", queue.enqueued)
	}
}

func TestStartAnticheatReleasesTheLaneWhenTheQueueRejects(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: errors.New("queue is full")}
	j := scannedJob()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/anticheat", nil))

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}
	var doc anticheat.Document
	if err := json.Unmarshal(store.puts[artifacts.AnticheatKey(j.ID)], &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Status != anticheat.StatusFailed {
		t.Fatalf("document status = %q, want %q so the UI never polls an unowned run", doc.Status, anticheat.StatusFailed)
	}
}

func TestGetAnticheatReturns409BeforeAnyRun(t *testing.T) {
	repo := newFakeRepo()
	j := scannedJob()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat", nil))

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "not started") {
		t.Fatalf("body = %s, want a not-started reason", rw.Body.String())
	}
}

func TestGetAnticheatReturnsTheStoredReport(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, readyDocument(j.ID, "76561198012345678"))
	h := NewHandlers(repo, store, &fakeQueue{})

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat", nil))

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{`"status":"ready"`, `"sospechoso"`, `"anomalous"`, `"limitations"`} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rw.Body.String())
		}
	}
}

func TestGetAnticheatDossierRendersTheEvidencePack(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, readyDocument(j.ID, "76561198012345678"))
	h := NewHandlers(repo, store, &fakeQueue{})

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(
		http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat/dossier/76561198012345678", nil))

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var dossier anticheat.Dossier
	if err := json.Unmarshal(rw.Body.Bytes(), &dossier); err != nil {
		t.Fatal(err)
	}
	if dossier.ProfileURL != "https://steamcommunity.com/profiles/76561198012345678" {
		t.Fatalf("profile url = %q", dossier.ProfileURL)
	}
	if !strings.Contains(dossier.Policy.Rejected, "no envía denuncias automáticamente") {
		t.Fatalf("policy = %+v, want the refusal to automate reporting", dossier.Policy)
	}
	if len(dossier.Channels) == 0 || !dossier.Channels[0].Effective {
		t.Fatalf("channels = %+v, want the effective channel first", dossier.Channels)
	}
}

func TestGetAnticheatDossierRejectsAnUnknownPlayer(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, readyDocument(j.ID, "76561198012345678"))
	h := NewHandlers(repo, store, &fakeQueue{})

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(
		http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat/dossier/76561198099999999", nil))

	if rw.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rw.Code, rw.Body.String())
	}
}

func TestGetAnticheatDossierRejectsAnUnfinishedAnalysis(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, anticheat.NewRunningDocument(j.ID.String(), time.Now()))
	h := NewHandlers(repo, store, &fakeQueue{})

	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(
		http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat/dossier/76561198012345678", nil))

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
}
