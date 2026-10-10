package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/cloudbridge"
	"github.com/rechedev9/cliphub/internal/cloudclient"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const cloudTestToken = "feedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedface"

// The cloud worker drives the pipeline through these very routes, so its
// loopback client is tested against the real handlers, not a copy of them.
func TestCloudWorkerLoopbackPipelineAgainstTheRealRoutes(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 1640},
		{ID: "seg-002", TickStart: 9000, TickEnd: 9800},
	}
	parsed := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan, CloudRequestID: "cloud-parsed"}
	const hookCrash = `zv-recorder.exe failed: exit status 6: HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`
	crashed := job.Job{ID: uuid.New(), Status: job.StatusFailed, FailureReason: hookCrash, Rules: rules.Default(), CloudRequestID: "cloud-crashed"}
	manual := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default()}
	for _, j := range []job.Job{parsed, crashed, manual} {
		repo.jobs[j.ID] = j
	}
	h := NewHandlers(repo, store, queue,
		WithMutationToken(cloudTestToken),
		WithRequireReadAuth(true),
		WithCapabilities(Capabilities{RecordEnabled: true}),
	)
	server := httptest.NewServer(Routes(h))
	t.Cleanup(server.Close)
	pipeline := cloudbridge.NewLoopbackPipeline(strings.TrimPrefix(server.URL, "http://"), cloudTestToken)
	ctx := context.Background()

	view, found, err := pipeline.JobView(ctx, parsed.ID.String())
	if err != nil || !found || view.Status != "parsed" {
		t.Fatalf("JobView(parsed) = %+v, found=%v, err=%v", view, found, err)
	}
	// The worker pauses itself on this code, so it must survive the trip.
	view, found, err = pipeline.JobView(ctx, crashed.ID.String())
	if err != nil || !found || view.Status != "failed" || view.FailureCode != "capture_incompatible" || view.FailureReason != hookCrash {
		t.Fatalf("JobView(crashed) = %+v, found=%v, err=%v, want failed with code capture_incompatible and the untouched reason", view, found, err)
	}
	if _, found, err := pipeline.JobView(ctx, uuid.NewString()); err != nil || found {
		t.Fatalf("JobView(unknown) found=%v err=%v, want not found without an error", found, err)
	}

	gotPlan, err := pipeline.KillPlan(ctx, parsed.ID.String())
	if err != nil || gotPlan.Demo.Tickrate != 64 || len(gotPlan.Segments) != 2 || gotPlan.Segments[1].TickEnd != 9800 {
		t.Fatalf("KillPlan = %+v, err=%v", gotPlan, err)
	}

	if _, found, err := pipeline.VariantView(ctx, parsed.ID.String(), "viral-60-clean"); err != nil || found {
		t.Fatalf("VariantView before any render found=%v err=%v, want not found", found, err)
	}

	rejected, err := pipeline.Generate(ctx, parsed.ID.String(), json.RawMessage(`{"preset":"no-such-preset","segment_ids":["seg-002"]}`))
	if err != nil || rejected.Status != http.StatusBadRequest || !strings.Contains(rejected.Message, "no-such-preset") {
		t.Fatalf("Generate(unknown preset) = %+v, err=%v, want a 400 that names the preset", rejected, err)
	}
	rejected, err = pipeline.Generate(ctx, parsed.ID.String(), json.RawMessage(`{"preset":"viral-60-clean","segment_ids":["seg-404"]}`))
	if err != nil || rejected.Status != http.StatusBadRequest {
		t.Fatalf("Generate(unknown segment) = %+v, err=%v, want a 400", rejected, err)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("rejected generates enqueued %d tasks, want 0", len(queue.enqueued))
	}

	accepted, err := pipeline.Generate(ctx, parsed.ID.String(), json.RawMessage(`{"preset":"viral-60-clean","segment_ids":["seg-002"]}`))
	if err != nil || accepted.Status != http.StatusAccepted || accepted.Variant != "viral-60-clean" {
		t.Fatalf("Generate = %+v, err=%v, want 202 for variant viral-60-clean", accepted, err)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRecordDemo {
		t.Fatalf("enqueued = %d tasks, want the one capture", len(queue.enqueued))
	}
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil || len(payload.SegmentIDs) != 1 || payload.SegmentIDs[0] != "seg-002" {
		t.Fatalf("capture payload = %+v, err=%v, want only seg-002", payload, err)
	}

	refs, err := pipeline.CloudJobs(ctx)
	if err != nil || len(refs) != 2 {
		t.Fatalf("CloudJobs = %+v, err=%v, want the two cloud jobs and not the manual one", refs, err)
	}
	for _, ref := range refs {
		if ref.ID == manual.ID.String() || ref.CloudRequestID == "" {
			t.Fatalf("CloudJobs returned %+v", ref)
		}
	}

	// A job with a generate run in flight cannot be deleted yet.
	if err := pipeline.DeleteJob(ctx, parsed.ID.String()); !errors.Is(err, cloudbridge.ErrLocalJobBusy) {
		t.Fatalf("DeleteJob(generating) error = %v, want ErrLocalJobBusy", err)
	}
	if err := pipeline.DeleteJob(ctx, crashed.ID.String()); err != nil {
		t.Fatalf("DeleteJob(failed) error = %v", err)
	}
	if _, still := repo.jobs[crashed.ID]; still {
		t.Fatal("the deleted job is still in the repository")
	}
	if err := pipeline.DeleteJob(ctx, crashed.ID.String()); err != nil {
		t.Fatalf("DeleteJob of a job that is already gone error = %v, want nil", err)
	}

	stranger := cloudbridge.NewLoopbackPipeline(strings.TrimPrefix(server.URL, "http://"), strings.Repeat("0", 64))
	if _, _, err := stranger.JobView(ctx, parsed.ID.String()); err == nil {
		t.Fatal("JobView with the wrong session token succeeded")
	}
}

func TestAdmitCloudDemoParsesForTheSpecTargetAndRules(t *testing.T) {
	demo := func() *strings.Reader { return strings.NewReader(string(demoMagic) + "the rest of the demo") }
	awpOnly := rules.Default()
	awpOnly.Weapons, awpOnly.MinKillsInWindow = []string{"awp"}, 2

	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(repo, store, queue)
	created, err := h.AdmitCloudDemo(context.Background(), CloudDemoAdmission{
		Demo:           demo(),
		FileName:       "cloud-1234-Luis-match.dem",
		CloudRequestID: "cloud-request-1",
		TargetSteamID:  "76561198000000001",
		Rules:          awpOnly,
	})
	if err != nil {
		t.Fatalf("AdmitCloudDemo error = %v", err)
	}
	stored := repo.jobs[created.ID]
	if stored.TargetSteamID != "76561198000000001" || stored.CloudRequestID != "cloud-request-1" || stored.DemoSHA256 == "" {
		t.Fatalf("stored job = %+v, want the target, the cloud request id and a demo hash", stored)
	}
	if len(stored.Rules.Weapons) != 1 || stored.Rules.Weapons[0] != "awp" || stored.Rules.MinKillsInWindow != 2 {
		t.Fatalf("stored rules = %+v, want the rules of the spec", stored.Rules)
	}
	// With a target the job parses straight away; nobody picks one by hand.
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeParseDemo {
		t.Fatalf("enqueued = %v, want one parse task", queue.enqueued)
	}

	rejections := []struct {
		name         string
		in           CloudDemoAdmission
		wantRejected bool
	}{
		{name: "not a demo", in: CloudDemoAdmission{Demo: strings.NewReader("<html>not a demo</html>"), TargetSteamID: "76561198000000001", Rules: awpOnly}, wantRejected: true},
		{name: "a target that is not a SteamID", in: CloudDemoAdmission{Demo: demo(), TargetSteamID: "not-a-steamid", Rules: awpOnly}},
		{name: "rules the parser would reject", in: CloudDemoAdmission{Demo: demo(), TargetSteamID: "76561198000000001", Rules: rules.Rules{}}},
	}
	for _, tc := range rejections {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStorage()
			queue := &fakeQueue{}
			h := NewHandlers(newFakeRepo(), store, queue)
			tc.in.CloudRequestID = "cloud-request-2"
			_, err := h.AdmitCloudDemo(context.Background(), tc.in)
			if err == nil {
				t.Fatal("AdmitCloudDemo error = nil, want a rejection")
			}
			if got := errors.Is(err, ErrCloudDemoRejected); got != tc.wantRejected {
				t.Fatalf("errors.Is(err, ErrCloudDemoRejected) = %v, want %v (err: %v)", got, tc.wantRejected, err)
			}
			if len(store.puts) != 0 || len(queue.enqueued) != 0 {
				t.Fatalf("a rejected admission stored %d files and queued %d tasks, want none", len(store.puts), len(queue.enqueued))
			}
		})
	}
}

// fakeCloud is a CloudService that records what the routes pass to it.
type fakeCloud struct {
	account    cloudclient.Account
	jobs       []cloudclient.LocalJob
	err        error
	submitted  []cloudclient.SubmitInput
	canceled   []string
	removed    []string
	unlinked   int
	videoPaths map[string]string
}

func (f *fakeCloud) Account(context.Context) cloudclient.Account { return f.account }

func (f *fakeCloud) StartLink(context.Context) (cloudclient.Account, error) {
	return f.account, f.err
}

func (f *fakeCloud) Unlink(context.Context) error {
	f.unlinked++
	return f.err
}

func (f *fakeCloud) Submit(_ context.Context, in cloudclient.SubmitInput) (cloudclient.LocalJob, error) {
	if f.err != nil {
		return cloudclient.LocalJob{}, f.err
	}
	f.submitted = append(f.submitted, in)
	return f.jobs[0], nil
}

func (f *fakeCloud) Jobs() []cloudclient.LocalJob { return f.jobs }

func (f *fakeCloud) Cancel(_ context.Context, id string) (cloudclient.LocalJob, error) {
	if f.err != nil {
		return cloudclient.LocalJob{}, f.err
	}
	f.canceled = append(f.canceled, id)
	return f.jobs[0], nil
}

func (f *fakeCloud) Remove(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.removed = append(f.removed, id)
	return nil
}

func (f *fakeCloud) VideoPath(id, name string) (string, error) {
	path, ok := f.videoPaths[id+"/"+name]
	if !ok {
		return "", &cloudclient.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Ese vídeo todavía no está en este equipo."}
	}
	return path, nil
}

func cloudRequest(t *testing.T, h *Handlers, method, target, body string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("X-ClipHub-Token", cloudTestToken)
	for name, values := range header {
		req.Header[name] = values
	}
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, req)
	return rw
}

func decodeErrorCode(t *testing.T, rw *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %s", rw.Body.String())
	}
	return body.Code
}

func TestCloudRoutesAnswerNotConfiguredWhenTheCloudIsOff(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithMutationToken(cloudTestToken), WithRequireReadAuth(true), WithCloud(nil))
	routes := []struct{ method, target string }{
		{http.MethodGet, "/api/cloud/account"},
		{http.MethodPost, "/api/cloud/link"},
		{http.MethodDelete, "/api/cloud/link"},
		{http.MethodPost, "/api/cloud/jobs"},
		{http.MethodGet, "/api/cloud/jobs"},
		{http.MethodPost, "/api/cloud/jobs/job-1/cancel"},
		{http.MethodDelete, "/api/cloud/jobs/job-1"},
		{http.MethodGet, "/api/cloud/jobs/job-1/videos/seg-001.mp4"},
	}
	for _, route := range routes {
		rw := cloudRequest(t, h, route.method, route.target, `{}`, nil)
		if rw.Code != http.StatusServiceUnavailable || decodeErrorCode(t, rw) != "not_configured" {
			t.Errorf("%s %s = %d %s, want 503 not_configured", route.method, route.target, rw.Code, rw.Body.String())
		}
	}
}

func TestCloudRoutesNeedTheSessionToken(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithMutationToken(cloudTestToken), WithRequireReadAuth(true), WithCloud(&fakeCloud{}))
	for _, route := range []struct{ method, target string }{
		{http.MethodGet, "/api/cloud/account"},
		{http.MethodGet, "/api/cloud/jobs"},
		{http.MethodPost, "/api/cloud/jobs"},
		{http.MethodDelete, "/api/cloud/link"},
	} {
		req := httptest.NewRequest(route.method, route.target, strings.NewReader(`{}`))
		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, req)
		if rw.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without the session token = %d, want 401", route.method, route.target, rw.Code)
		}
	}
}

func TestCloudRoutesSpeakTheLocalContract(t *testing.T) {
	access := "allowed"
	percent := 62
	stage := "capturing"
	stalled := cloudclient.StalledPortal
	cloud := &fakeCloud{
		account: cloudclient.Account{
			PortalURL: "https://cliphub.gravityroom.app",
			Linked:    true,
			User:      &cloudclient.StoredUser{Name: "Luis", Email: "luis@example.com"},
			Access:    &access,
			Limits:    &cloudclient.AccountLimits{MaxActive: 3, DailySeconds: 5400},
			Usage:     &cloudclient.AccountUsage{Active: 1, SecondsLast24h: 1200, SecondsCommitted: 480},
			Kinds:     []string{"short"},
			Queue:     &cloudclient.AccountQueue{State: "online", Queued: 4, WaitSeconds: map[string]*int{"short": &percent}},
		},
		jobs: []cloudclient.LocalJob{{
			ID: "cloud-1", LocalJobID: "local-1", Kind: "short", Title: "R3 4k", Status: "running", Stage: &stage, Percent: &percent,
			Stalled:   &stalled,
			CreatedAt: "2026-10-09T10:00:00Z",
			Videos:    []cloudclient.LocalVideo{{Name: "seg-001.mp4", Variant: "viral-60-clean", SizeBytes: 48211332}},
		}},
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithMutationToken(cloudTestToken), WithRequireReadAuth(true), WithCloud(cloud))

	account := cloudRequest(t, h, http.MethodGet, "/api/cloud/account", "", nil)
	const wantAccount = `{"portal_url":"https://cliphub.gravityroom.app","linked":true,"link":null,"user":{"name":"Luis","email":"luis@example.com"},"access":"allowed","limits":{"max_active":3,"daily_seconds":5400},"usage":{"active":1,"seconds_last_24h":1200,"seconds_committed":480},"kinds":["short"],"queue":{"state":"online","queued":4,"wait_seconds":{"short":62}},"error":null}`
	if account.Code != http.StatusOK || strings.TrimSpace(account.Body.String()) != wantAccount {
		t.Fatalf("GET account = %d %s\nwant %s", account.Code, account.Body.String(), wantAccount)
	}

	if link := cloudRequest(t, h, http.MethodPost, "/api/cloud/link", `{}`, nil); link.Code != http.StatusAccepted || !strings.Contains(link.Body.String(), `"linked":true`) {
		t.Fatalf("POST link = %d %s, want 202 with the account document", link.Code, link.Body.String())
	}

	list := cloudRequest(t, h, http.MethodGet, "/api/cloud/jobs", "", nil)
	const wantJobs = `{"jobs":[{"id":"cloud-1","local_job_id":"local-1","kind":"short","title":"R3 4k","status":"running","stage":"capturing","percent":62,"queue":null,"failure":null,"cancel_requested":false,"stalled":"portal_unreachable","created_at":"2026-10-09T10:00:00Z","finished_at":null,"videos":[{"name":"seg-001.mp4","variant":"viral-60-clean","size_bytes":48211332,"ready":false}]}]}`
	if list.Code != http.StatusOK || strings.TrimSpace(list.Body.String()) != wantJobs {
		t.Fatalf("GET jobs = %d %s\nwant %s", list.Code, list.Body.String(), wantJobs)
	}

	const submit = `{"job_id":"local-1","kind":"short","title":"R3 4k","preset":"viral-60-clean","music":{"key":"track-1"},"edit":{"intro":true},"segment_ids":["seg-001"]}`
	created := cloudRequest(t, h, http.MethodPost, "/api/cloud/jobs", submit, nil)
	if created.Code != http.StatusAccepted || !strings.Contains(created.Body.String(), `"id":"cloud-1"`) {
		t.Fatalf("POST jobs = %d %s, want 202 with the job", created.Code, created.Body.String())
	}
	if len(cloud.submitted) != 1 {
		t.Fatalf("submits = %d, want 1", len(cloud.submitted))
	}
	got := cloud.submitted[0]
	if got.JobID != "local-1" || got.Preset != "viral-60-clean" || string(got.Music) != `{"key":"track-1"}` || string(got.Edit) != `{"intro":true}` || len(got.SegmentIDs) != 1 || got.SegmentIDs[0] != "seg-001" {
		t.Fatalf("submit input = %+v, want the body passed through untouched", got)
	}
	if bad := cloudRequest(t, h, http.MethodPost, "/api/cloud/jobs", `{"job_id":`, nil); bad.Code != http.StatusBadRequest {
		t.Fatalf("POST jobs with broken JSON = %d, want 400", bad.Code)
	}

	if canceled := cloudRequest(t, h, http.MethodPost, "/api/cloud/jobs/cloud-1/cancel", "", nil); canceled.Code != http.StatusAccepted || len(cloud.canceled) != 1 || cloud.canceled[0] != "cloud-1" {
		t.Fatalf("POST cancel = %d, canceled %v", canceled.Code, cloud.canceled)
	}
	if removed := cloudRequest(t, h, http.MethodDelete, "/api/cloud/jobs/cloud-1", "", nil); removed.Code != http.StatusNoContent || len(cloud.removed) != 1 {
		t.Fatalf("DELETE job = %d, removed %v", removed.Code, cloud.removed)
	}
	if unlinked := cloudRequest(t, h, http.MethodDelete, "/api/cloud/link", "", nil); unlinked.Code != http.StatusNoContent || cloud.unlinked != 1 {
		t.Fatalf("DELETE link = %d, unlinked %d", unlinked.Code, cloud.unlinked)
	}
}

func TestCloudRoutesForwardTheClientErrorWithItsStatusAndCode(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "not linked", err: &cloudclient.Error{Status: http.StatusConflict, Code: "not_linked", Message: "Conecta tu cuenta."}, wantStatus: http.StatusConflict, wantCode: "not_linked"},
		{name: "a portal limit", err: &cloudclient.Error{Status: http.StatusTooManyRequests, Code: "limit_daily", Message: "Sin minutos."}, wantStatus: http.StatusTooManyRequests, wantCode: "limit_daily"},
		{name: "a Studio the cloud machine does not match", err: &cloudclient.Error{Status: http.StatusConflict, Code: "studio_version_mismatch", Message: "studio_version_mismatch"}, wantStatus: http.StatusConflict, wantCode: "studio_version_mismatch"},
		{name: "the user's demo storage is full", err: &cloudclient.Error{Status: http.StatusTooManyRequests, Code: "limit_storage", Message: "limit_storage"}, wantStatus: http.StatusTooManyRequests, wantCode: "limit_storage"},
		{name: "portal unreachable", err: &cloudclient.Error{Status: http.StatusServiceUnavailable, Code: "portal_unreachable", Message: "Sin conexión."}, wantStatus: http.StatusServiceUnavailable, wantCode: "portal_unreachable"},
		{name: "anything else stays internal", err: errors.New("open /home/luis/secret: access denied"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cloud := &fakeCloud{err: tc.err}
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithMutationToken(cloudTestToken), WithRequireReadAuth(true), WithCloud(cloud))
			for _, route := range []struct{ method, target string }{
				{http.MethodPost, "/api/cloud/jobs"},
				{http.MethodPost, "/api/cloud/jobs/cloud-1/cancel"},
				{http.MethodDelete, "/api/cloud/jobs/cloud-1"},
				{http.MethodPost, "/api/cloud/link"},
			} {
				rw := cloudRequest(t, h, route.method, route.target, `{}`, nil)
				if rw.Code != tc.wantStatus || decodeErrorCode(t, rw) != tc.wantCode {
					t.Fatalf("%s %s = %d %s, want %d %s", route.method, route.target, rw.Code, rw.Body.String(), tc.wantStatus, tc.wantCode)
				}
				if strings.Contains(rw.Body.String(), "secret") {
					t.Fatalf("%s %s leaked an internal error: %s", route.method, route.target, rw.Body.String())
				}
			}
		})
	}
}

func TestCloudVideoIsServedFromDiskWithRangeSupport(t *testing.T) {
	video := []byte("0123456789abcdefghij")
	path := filepath.Join(t.TempDir(), "seg-001.mp4")
	if err := os.WriteFile(path, video, 0o600); err != nil {
		t.Fatal(err)
	}
	cloud := &fakeCloud{videoPaths: map[string]string{"cloud-1/seg-001.mp4": path}}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithMutationToken(cloudTestToken), WithRequireReadAuth(true), WithCloud(cloud))

	whole := cloudRequest(t, h, http.MethodGet, "/api/cloud/jobs/cloud-1/videos/seg-001.mp4", "", nil)
	if whole.Code != http.StatusOK || whole.Body.String() != string(video) || whole.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("GET video = %d, %d bytes, type %q", whole.Code, whole.Body.Len(), whole.Header().Get("Content-Type"))
	}
	// The player seeks with Range requests.
	part := cloudRequest(t, h, http.MethodGet, "/api/cloud/jobs/cloud-1/videos/seg-001.mp4", "", http.Header{"Range": {"bytes=10-14"}})
	if part.Code != http.StatusPartialContent || part.Body.String() != "abcde" {
		t.Fatalf("GET video with Range = %d %q, want 206 with bytes 10 to 14", part.Code, part.Body.String())
	}
	// Only names the client knows are served; nothing is joined into a path.
	for _, name := range []string{"seg-002.mp4", "..%2F..%2Fclient.json"} {
		if missing := cloudRequest(t, h, http.MethodGet, "/api/cloud/jobs/cloud-1/videos/"+name, "", nil); missing.Code != http.StatusNotFound {
			t.Fatalf("GET video %q = %d, want 404", name, missing.Code)
		}
	}
}
