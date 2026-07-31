package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/tickcut/internal/artifacts"
	"github.com/rechedev9/tickcut/internal/job"
	"github.com/rechedev9/tickcut/internal/radarmap"
	"github.com/rechedev9/tickcut/internal/storage"
	"github.com/rechedev9/tickcut/internal/tactical"
	"github.com/rechedev9/tickcut/internal/tacticalplan"
	"github.com/rechedev9/tickcut/internal/tasks"
)

// tacticalFixture builds a two-round document plus the position blob it
// indexes, so the round endpoint can be checked against known ticks.
func tacticalFixture(t *testing.T, id uuid.UUID) (tacticalplan.Document, tacticalplan.Blob) {
	t.Helper()
	rounds := []tacticalplan.RoundFrames{
		{Round: 1, Frames: []tacticalplan.Frame{
			{Tick: 100, Samples: []tacticalplan.Sample{{Slot: 0, X: 10, Y: 20, Z: 30, Health: 100, Flags: tacticalplan.FlagAlive}}},
			{Tick: 116, Samples: []tacticalplan.Sample{{Slot: 0, X: 40, Y: 20, Z: 30, Health: 90, Flags: tacticalplan.FlagAlive}}},
		}},
		{Round: 2, Frames: []tacticalplan.Frame{
			{Tick: 500, Samples: []tacticalplan.Sample{{Slot: 1, X: -50, Y: 60, Z: 30, Health: 100, Flags: tacticalplan.FlagAlive | tacticalplan.FlagSideT}}},
		}},
	}
	blob, err := tacticalplan.EncodePositions(rounds, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions error = %v", err)
	}
	doc := tacticalplan.NewDocument()
	doc.JobID = id
	doc.Demo = tacticalplan.Demo{Map: "de_mirage", Tickrate: 64, MaxRounds: 24}
	doc.Teams = []tacticalplan.Team{{Key: "team-a", Name: "A", StartSide: tacticalplan.SideCT}}
	doc.Players = []tacticalplan.Player{{Slot: 0, SteamID64: "1", Name: "one", TeamKey: "team-a", StartSide: tacticalplan.SideCT}}
	doc.Rounds = []tacticalplan.Round{
		{
			Number: 1, TickStart: 90, TickFreezeEnd: 100, TickEnd: 200, Half: 1,
			Winner:  tacticalplan.SideCT,
			Economy: tacticalplan.Economy{CTBuy: tacticalplan.BuyPistol, TBuy: tacticalplan.BuyPistol},
			Class:   tacticalplan.Class{TSide: tacticalplan.TDefault, CTSide: tacticalplan.CTHold, Site: tacticalplan.SiteA},
			Players: []tacticalplan.PlayerRound{{Slot: 0, Side: tacticalplan.SideCT, Kills: 1, Survived: true}},
		},
		{
			Number: 2, TickStart: 480, TickFreezeEnd: 500, TickEnd: 700, Half: 1,
			Winner:  tacticalplan.SideT,
			Economy: tacticalplan.Economy{CTBuy: tacticalplan.BuyFull, TBuy: tacticalplan.BuyEco},
			Class:   tacticalplan.Class{TSide: tacticalplan.TEcoRush, CTSide: tacticalplan.CTHold, Site: tacticalplan.SiteB},
			Players: []tacticalplan.PlayerRound{{Slot: 0, Side: tacticalplan.SideCT, Deaths: 1}},
		},
	}
	calibration, _ := radarmap.Lookup("de_mirage")
	doc.Geometry = tacticalplan.MapGeometry{Map: "de_mirage", Source: tacticalplan.GeometrySourceOccupancy, Calibration: calibration}
	doc.Positions = blob.Descriptor
	return doc, blob
}

// seedTactical writes the fixture artifacts through the given storage.
func seedTactical(t *testing.T, store storage.Storage, id uuid.UUID) tacticalplan.Document {
	t.Helper()
	doc, blob := tacticalFixture(t, id)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.TacticalIndexKey(id), bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.TacticalPositionsKey(id), bytes.NewReader(blob.Data)); err != nil {
		t.Fatal(err)
	}
	return doc
}

func tacticalTestJob(repo *fakeRepo) uuid.UUID {
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/" + id.String() + ".dem"}
	return id
}

func TestStartTacticalAnalysisEnqueuesRetryableDefaultLaneTask(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(repo, store, queue)
	id := tacticalTestJob(repo)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/tactical", nil))

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	var status artifacts.TacticalStatus
	if err := json.Unmarshal(rw.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.State != artifacts.TacticalStateQueued {
		t.Errorf("state = %q, want %q", status.State, artifacts.TacticalStateQueued)
	}
	if status.SchemaVersion != tacticalplan.SchemaVersion {
		t.Errorf("schema_version = %q, want %q", status.SchemaVersion, tacticalplan.SchemaVersion)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeAnalyzeTactical {
		t.Fatalf("enqueued = %+v, want one %s task", queue.enqueued, tasks.TypeAnalyzeTactical)
	}
	// The scan is idempotent and pure CPU: it must stay retryable, and it must
	// not be pinned to the serial capture lane that exists for cs2.exe.
	for _, opt := range queue.options[0] {
		if opt.Type() == asynq.MaxRetryOpt {
			t.Errorf("option %v set, want the default retry policy", opt)
		}
		if opt.Type() == asynq.QueueOpt {
			t.Errorf("option %v set, want the default queue lane", opt)
		}
	}
	if _, ok := store.puts[artifacts.TacticalStatusKey(id)]; !ok {
		t.Error("queued status artifact missing")
	}
}

func TestStartTacticalAnalysisRejectsOutOfRangeSampleRate(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)
	id := tacticalTestJob(repo)

	body := bytes.NewBufferString(`{"sample_hz":1000}`)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/tactical", body))

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Errorf("enqueued %d tasks, want none", len(queue.enqueued))
	}
}

func TestStartTacticalAnalysisRejectsNonCanonicalSampleRate(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)
	id := tacticalTestJob(repo)

	body := bytes.NewBufferString(`{"sample_hz":20}`)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/tactical", body))

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Errorf("enqueued %d tasks, want none", len(queue.enqueued))
	}
}

func TestStartTacticalAnalysisRejectsTrailingJSON(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)
	id := tacticalTestJob(repo)

	body := bytes.NewBufferString(`{"sample_hz":1}{"sample_hz":20}`)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/tactical", body))

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Errorf("enqueued %d tasks, want none", len(queue.enqueued))
	}
}

func TestGetTacticalStatusReportsNoneBeforeAnalysis(t *testing.T) {
	repo := newFakeRepo()
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	id := tacticalTestJob(repo)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/status", nil))

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var status artifacts.TacticalStatus
	if err := json.Unmarshal(rw.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.State != artifacts.TacticalStateNone {
		t.Errorf("state = %q, want %q", status.State, artifacts.TacticalStateNone)
	}
}

func TestGetTacticalStatusDefaultsLegacySampleRateWithoutRewriting(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)
	key := artifacts.TacticalStatusKey(id)
	legacy := []byte(`{"state":"ready","generated_at":"2026-07-29T00:00:00Z","schema_version":"` +
		tacticalplan.SchemaVersion + `"}` + "\n")
	store.puts[key] = append([]byte(nil), legacy...)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/status", nil))

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var status artifacts.TacticalStatus
	if err := json.Unmarshal(rw.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.SampleHZ != tactical.DefaultSampleHZ {
		t.Errorf("sample_hz = %v, want legacy default %v", status.SampleHZ, float64(tactical.DefaultSampleHZ))
	}
	if !bytes.Equal(store.puts[key], legacy) {
		t.Fatalf("stored status = %s, want the legacy artifact left unchanged", store.puts[key])
	}
}

func TestGetTacticalDocumentIsConflictUntilReady(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical", nil))
	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}

	seedTactical(t, store, id)
	rw = httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var doc tacticalplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != tacticalplan.SchemaVersion {
		t.Errorf("schema_version = %q, want %q", doc.SchemaVersion, tacticalplan.SchemaVersion)
	}
	if len(doc.Rounds) != 2 {
		t.Errorf("rounds = %d, want 2", len(doc.Rounds))
	}
}

func TestGetTacticalRoundDecodesOnlyThatRound(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)
	seedTactical(t, store, id)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/rounds/2", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp tacticalRoundResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Round.Number != 2 {
		t.Errorf("round = %d, want 2", resp.Round.Number)
	}
	if len(resp.Frames) != 1 {
		t.Fatalf("frames = %d, want only round 2's single frame", len(resp.Frames))
	}
	if resp.Frames[0].Tick != 500 {
		t.Errorf("frame tick = %d, want 500", resp.Frames[0].Tick)
	}
	if len(resp.Frames[0].Samples) != 1 || resp.Frames[0].Samples[0].Slot != 1 {
		t.Fatalf("samples = %+v, want the slot 1 sample", resp.Frames[0].Samples)
	}
	if got := resp.Frames[0].Samples[0].X; got < -50.5 || got > -49.5 {
		t.Errorf("sample X = %v, want ~-50", got)
	}
}

// A storage backend whose reader cannot seek must still answer with the round's
// frames instead of failing or returning the whole blob.
func TestGetTacticalRoundDecodesWithoutSeekableStorage(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)
	seedTactical(t, store, id)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/rounds/1", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp tacticalRoundResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Frames) != 2 {
		t.Fatalf("frames = %d, want round 1's two frames", len(resp.Frames))
	}
	if resp.Frames[0].Tick != 100 || resp.Frames[1].Tick != 116 {
		t.Errorf("ticks = %d,%d, want 100,116", resp.Frames[0].Tick, resp.Frames[1].Tick)
	}
}

func TestGetTacticalRoundValidatesTheRoundSegment(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)
	seedTactical(t, store, id)

	cases := map[string]int{
		"abc":       http.StatusBadRequest,
		"0":         http.StatusBadRequest,
		"-1":        http.StatusBadRequest,
		"..%2Fplan": http.StatusBadRequest,
		"99":        http.StatusNotFound,
	}
	for segment, want := range cases {
		t.Run(segment, func(t *testing.T) {
			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/rounds/"+segment, nil))
			if rw.Code != want {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, want, rw.Body.String())
			}
		})
	}
}

func TestGetTacticalPositionsHonoursRangeRequests(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)
	seedTactical(t, store, id)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/positions", nil)
	req.Header.Set("Range", "bytes=0-5")
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, req)

	if rw.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206; body=%q", rw.Code, rw.Body.String())
	}
	if got := rw.Body.String(); got != "ZVPOS1" {
		t.Errorf("body = %q, want the blob magic %q", got, "ZVPOS1")
	}
}

func TestGetTacticalAggregateFiltersAndRejectsBadFilters(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	h := NewHandlers(repo, store, &fakeQueue{})
	id := tacticalTestJob(repo)
	seedTactical(t, store, id)

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/aggregate?buy=eco&side=T", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var tendencies tacticalplan.Tendencies
	if err := json.Unmarshal(rw.Body.Bytes(), &tendencies); err != nil {
		t.Fatal(err)
	}
	if tendencies.RoundCount != 1 {
		t.Errorf("round_count = %d, want the single T eco round", tendencies.RoundCount)
	}
	if tendencies.Perspective != tacticalplan.SideT {
		t.Errorf("perspective = %q, want T", tendencies.Perspective)
	}

	rw = httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/aggregate?buy=nonsense", nil))
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
}

func TestGetMapRadarValidatesAndLooksUpTheMap(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	r := Routes(h)

	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/maps/de_mirage/radar", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var calibration radarmap.Calibration
	if err := json.Unmarshal(rw.Body.Bytes(), &calibration); err != nil {
		t.Fatal(err)
	}
	if calibration.Map != "de_mirage" || !calibration.Valid() {
		t.Fatalf("calibration = %+v, want a valid de_mirage transform", calibration)
	}
	if calibration.Source != radarmap.SourceOverview {
		t.Errorf("source = %q, want %q", calibration.Source, radarmap.SourceOverview)
	}

	cases := map[string]int{
		"de_workshop_unknown": http.StatusNotFound,
		"DE_MIRAGE":           http.StatusBadRequest,
		"de-mirage":           http.StatusBadRequest,
		"..%2F..%2Fetc":       http.StatusBadRequest,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/maps/"+name+"/radar", nil))
			if rw.Code != want {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, want, rw.Body.String())
			}
		})
	}
}
