package telemetryalert

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type replayStep struct {
	at      string
	want    []string // rule:priority of the alerts delivered by this run
	contain []string // substrings of those alerts' text
}

// TestIncidentReplay replays the AGENTS.md incidents through the whole
// alerter (fake admin API, alert.db, rules, outbox) and checks what reaches
// the phone on every timer run.
func TestIncidentReplay(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		bootstrap  string
		keepDigest bool
		steps      []replayStep
	}{
		{
			name: "loudnorm TP out of range: one P1, silent repeats, new code on 3.0.0", fixture: "loudnorm.json", bootstrap: "2026-09-06T12:00:00Z",
			steps: []replayStep{
				{at: "2026-09-07T13:11:00Z", want: []string{"new_issue:P1"}, contain: []string{
					"[P1] Nuevo · render:variant · sin código · 2.4.62 · #1 · ×1", "pista: el master AAC agotó sus intentos", "job 10b10001…",
					"https://report.invalid/alerts/issue/"}},
				{at: "2026-09-07T13:25:00Z"},
				{at: "2026-09-07T13:42:00Z"},
				{at: "2026-09-07T14:06:00Z"},
				{at: "2026-09-07T14:12:00Z", want: []string{"repeat:P2/silent"}, contain: []string{"[P2] Repite · render:variant · sin código · 2.4.62 · ×3", "instalaciones: #1 ×3"}},
				{at: "2026-09-07T15:31:00Z"},
				{at: "2026-09-07T20:15:00Z", want: []string{"repeat:P2/silent"}, contain: []string{"×1"}},
				{at: "2026-09-10T16:03:00Z", want: []string{"new_issue:P1"}, contain: []string{
					"[P1] Nuevo · render:variant/audio_master · loudnorm_param_out_of_range · 3.0.0 · #1",
					"pista: bucle de retarget del master, clamp TP [-9,0]", "ver AGENTS.md › Full Demo audio mastering"}},
			},
		},
		{
			name: "HLAE crash after CS2 14182: new issue, then platform break", fixture: "hlae.json", bootstrap: "2026-09-22T00:00:00Z",
			steps: []replayStep{
				{at: "2026-09-22T20:00:00Z"},
				{at: "2026-09-23T13:40:00Z", want: []string{"new_issue:P1"}, contain: []string{
					"[P1] Nuevo · record:demo/hook_start · hlae_hook_incompatible · 4.0.1 · #1 · ×1",
					"pista: exit 6 = HLAE frente a la build de CS2", "cs2_build=14182 hlae=v2.192.2 encoder=h264_nvenc"}},
				{at: "2026-09-23T13:52:00Z", want: []string{"platform_break:P1"}, contain: []string{
					"[P1] Plataforma rota · record:demo · 4.0.1 · #1 · 2/2 fallos, 0 ok",
					"último bueno: 3.0.8 cs2_build=14170 hlae=v2.192.2", "primer fallo: cs2_build=14182 hlae=v2.192.2",
					"ver AGENTS.md › HLAE pin and CS2 updates"}},
			},
		},
		{
			name: "black first-person capture: output quality P1 with the machine context", fixture: "black-capture.json", bootstrap: "2026-09-23T00:00:00Z",
			steps: []replayStep{
				{at: "2026-09-23T17:42:00Z", want: []string{"output_quality:P1"}, contain: []string{
					"[P1] Calidad · gameplay-pov-60 · 5.0.0 · #1", "bitrate 2400 kb/s < 10000 · YAVG 18.5 < 25",
					"cs2_build=14182 hlae=v2.192.2-cliphub.1 encoder=h264_nvenc gpu_vendor=nvidia", "blackdetect/signalstats"}},
			},
		},
		{
			name: "logoff kills are not crashes", fixture: "shutdown-kill.json", bootstrap: "2026-09-23T00:00:00Z",
			steps: []replayStep{{at: "2026-09-23T13:36:00Z"}},
		},
		{
			name: "FACEIT format change reaches the digest; the report pages", fixture: "faceit.json", bootstrap: "2026-09-10T00:00:00Z", keepDigest: true,
			steps: []replayStep{
				{at: "2026-09-10T14:01:00Z", want: []string{"new_issue:P1", "digest:P2/silent"}},
				{at: "2026-09-10T14:31:00Z", want: []string{"platform_break:P1"}, contain: []string{"último bueno: 2.4.60"}},
				{at: "2026-09-10T20:13:00Z", want: []string{"repeat:P2/silent"}},
				{at: "2026-09-11T07:05:00Z", want: []string{"digest:P2/silent"}, contain: []string{
					"render.profile source_kind=faceit ahora overlay_source=demo (3.0.1); último con overlay_source=faceit: 2.4.60 2026-09-05",
					"render:variant 3.0.0: 0 ok / 2 error", "render:variant 3.0.1: 1 ok / 0 error"}},
				{at: "2026-09-11T07:31:00Z", want: []string{"user_report:P1"}, contain: []string{
					"[P1] Reporte · wrong_overlay · 3.0.1 · #1 · render:variant", "job face1001…",
					"render.profile: source_kind=faceit overlay_source=demo hud=circuit",
					"delivery.quality: preset=gameplay-pov-60 bitrate_kbps=39800", "ver AGENTS.md › Full Demo overlay format"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, tt.fixture)
			if result := h.run(tt.bootstrap); !result.Bootstrap || len(h.rec.take()) != 0 {
				t.Fatalf("bootstrap run = %+v", result)
			}
			for _, step := range tt.steps {
				h.run(step.at)
				delivered := h.rec.take()
				if got := summary(delivered, tt.keepDigest); !slices.Equal(got, step.want) {
					t.Fatalf("%s: delivered %v, want %v\n%s", step.at, got, step.want, texts(delivered))
				}
				all := texts(delivered)
				for _, want := range step.contain {
					if !strings.Contains(all, want) {
						t.Errorf("%s: missing %q in\n%s", step.at, want, all)
					}
				}
			}
		})
	}
}

func texts(alerts []Alert) string {
	var parts []string
	for _, a := range alerts {
		parts = append(parts, a.Text())
	}
	return strings.Join(parts, "\n---\n")
}

func countRows(t *testing.T, h *harness, query string) int {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(h.cfg.StateDir, "alert.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestShutdownKillCreatesNoIssue(t *testing.T) {
	h := newHarness(t, "shutdown-kill.json")
	h.run("2026-09-23T00:00:00Z")
	h.run("2026-09-23T13:36:00Z")
	if n := countRows(t, h, "SELECT COUNT(*) FROM issues"); n != 0 {
		t.Fatalf("issues = %d, want 0", n)
	}
}

// Today's clients label the error event with its journal stage/class and the
// attempt.finished log with the operation, and the texts differ. One failed
// job must still page once.
func TestEventAndLogCopiesOfOneJobPageOnce(t *testing.T) {
	h := newHarness(t, "shutdown-kill.json")
	h.run("2026-09-24T09:00:00Z") // bootstrap
	const support, session, job = "CH-1111-2222-3333-4444-5555", "5e551011-0000-4000-8000-0000000000aa", "4ae10001-0000-4000-8000-0000000000aa"
	h.admin.add(fixtureFile{
		Events: []ErrorEvent{{
			ReceivedAt: mustTime(t, "2026-09-24T10:00:05Z"), OccurredAt: mustTime(t, "2026-09-24T10:00:00Z"), SupportCode: support, SessionID: session,
			Release: "5.2.1", Component: "orchestrator", Name: "pipeline.error", Stage: "record", Class: "unknown",
			JobID: job, Message: "HLAE hook crashed with a native error dialog (Error - AfxHookSource2)",
		}},
		Logs: []LogRecord{{
			ReceivedAt: mustTime(t, "2026-09-24T10:00:06Z"), OccurredAt: mustTime(t, "2026-09-24T10:00:01Z"), SupportCode: support, SessionID: session,
			Release: "5.2.1", Source: "orchestrator", Level: "error", Event: "attempt.finished", JobID: job,
			Operation: "record:demo", Outcome: "error", Message: "record:demo exited 6 after 5 s",
		}},
	})
	result := h.run("2026-09-24T10:01:00Z")
	if got := summary(result.Alerts, false); !slices.Equal(got, []string{"new_issue:P1"}) {
		t.Fatalf("alerts = %v, want a single new_issue", got)
	}
}

func TestBootstrapSendsNothingAndMarksKeysKnown(t *testing.T) {
	h := newHarness(t, "loudnorm.json", "hlae.json", "faceit.json", "black-capture.json", "shutdown-kill.json")
	tg := newFakeTelegram(t)
	h.notifier = tg.client()
	result := h.run("2026-09-24T12:00:00Z")
	if !result.Bootstrap || len(result.Alerts) != 0 || result.Delivered != 0 {
		t.Fatalf("bootstrap result = %+v", result)
	}
	if calls := tg.methods(); len(calls) != 0 {
		t.Fatalf("bootstrap called Telegram: %v", calls)
	}
	if n := countRows(t, h, "SELECT COUNT(*) FROM issues"); n != 3 {
		t.Fatalf("issues = %d, want every incident key known", n)
	}
	// A known key seen again is a repeat, never a new-issue page.
	h.admin.add(fixtureFile{Events: []ErrorEvent{{
		ReceivedAt: mustTime(t, "2026-09-24T12:30:00Z"), SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000099",
		Release: "5.2.1", Component: "orchestrator", Name: "pipeline.error", Stage: "worker", Class: "record:demo",
		JobID: "4ae10001-0000-4000-8000-000000000099", Message: "failure_code=hlae_hook_incompatible substage=hook_start; exited 6",
	}}})
	result = h.run("2026-09-24T12:31:00Z")
	if result.Bootstrap || len(result.Alerts) != 0 {
		t.Fatalf("after bootstrap: %v", summary(result.Alerts, true))
	}
	if result = h.run("2026-09-24T13:05:00Z"); !slices.Equal(summary(result.Alerts, false), []string{"repeat:P2/silent"}) {
		t.Fatalf("repeat after bootstrap: %v", summary(result.Alerts, true))
	}
}

func TestOldCollectorWithoutErrorsEndpointKeepsWorking(t *testing.T) {
	h := newHarness(t, "hlae.json")
	h.admin.noErrors = true
	h.run("2026-09-22T00:00:00Z")
	h.run("2026-09-22T20:00:00Z")
	result := h.run("2026-09-23T13:40:00Z")
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %v", result.Failures)
	}
	if got := summary(h.rec.take(), false); !slices.Equal(got, []string{"new_issue:P1"}) {
		t.Fatalf("delivered %v", got)
	}
}

func TestDryRunPrintsAlertsAndKeepsNoState(t *testing.T) {
	h := newHarness(t, "hlae.json")
	var out strings.Builder
	dry := func(o *Options) { o.DryRun, o.Notifier, o.Out = true, nil, &out }
	result := h.run("2026-09-23T13:52:00Z", dry)
	if result.Bootstrap {
		t.Fatal("a dry run must not bootstrap")
	}
	for _, want := range []string{"--- new_issue\n[P1] Nuevo", "--- platform_break\n[P1] Plataforma rota"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry-run output lacks %q:\n%s", want, out.String())
		}
	}
	if n := countRows(t, h, "SELECT COUNT(*) FROM issues"); n != 0 {
		t.Fatalf("dry run kept %d issues", n)
	}
}

func TestOutboxRetriesWhenTelegramIsDown(t *testing.T) {
	h := newHarness(t, "hlae.json")
	h.run("2026-09-22T00:00:00Z")
	h.rec.fail = true
	result := h.run("2026-09-23T13:40:00Z")
	if !slices.Contains(result.Failures, "telegram") {
		t.Fatalf("failures = %v", result.Failures)
	}
	h.rec.fail = false
	h.run("2026-09-23T13:41:00Z")
	if got := summary(h.rec.take(), false); !slices.Equal(got, []string{"new_issue:P1"}) {
		t.Fatalf("after recovery delivered %v", got)
	}
}

func TestRealCrashPagesWithCooldown(t *testing.T) {
	h := newHarness(t)
	h.run("2026-09-20T00:00:00Z")
	crash := func(at string) LogRecord {
		return LogRecord{ReceivedAt: mustTime(t, at), SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000080",
			Release: "5.2.1", Source: "studio", Level: "error", Event: "process.gone", Message: "type=GPU reason=crashed exit=-1073741819"}
	}
	steps := []struct {
		at   string
		want []string
	}{
		{"2026-09-20T10:00:00Z", []string{"new_issue:P1"}},
		{"2026-09-20T11:00:00Z", []string{"crash:P1"}},
		{"2026-09-20T13:00:00Z", nil},
		{"2026-09-20T17:30:00Z", []string{"crash:P1"}},
	}
	for _, step := range steps {
		h.admin.add(fixtureFile{Logs: []LogRecord{crash(step.at)}})
		h.run(mustTime(t, step.at).Add(time.Minute).Format(time.RFC3339))
		if got := summary(h.rec.take(), false); !slices.Equal(got, step.want) {
			t.Fatalf("%s: delivered %v, want %v", step.at, got, step.want)
		}
	}
	// A GPU process that merely exited cleanly is not a crash.
	h.admin.add(fixtureFile{Logs: []LogRecord{{ReceivedAt: mustTime(t, "2026-09-20T18:00:00Z"), SupportCode: "CH-1111-2222-3333-4444-5555",
		SessionID: "5e551011-0000-4000-8000-000000000080", Release: "5.2.1", Source: "studio", Level: "error", Event: "process.gone",
		Message: "type=GPU reason=clean-exit exit=0"}}})
	h.run("2026-09-20T18:01:00Z")
	if got := h.rec.take(); len(got) != 0 {
		t.Fatalf("clean exit paged: %v", summary(got, true))
	}
}

func TestFloodGuardSilencesAfterTenP1s(t *testing.T) {
	h := newHarness(t)
	h.run("2026-09-20T09:00:00Z")
	var events []ErrorEvent
	for i := range 12 {
		events = append(events, ErrorEvent{ReceivedAt: mustTime(t, "2026-09-20T10:00:00Z").Add(time.Duration(i) * time.Second),
			SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000081", Release: "5.2.1",
			Component: "orchestrator", Name: "pipeline.error", Stage: "worker", Class: "render:variant",
			Message: fmt.Sprintf("failure_code=synthetic_%c substage=test; boom", 'a'+i)})
	}
	h.admin.add(fixtureFile{Events: events})
	h.run("2026-09-20T10:01:00Z")
	delivered := h.rec.take()
	var audible, silent, flood int
	for _, a := range delivered {
		switch {
		case a.Rule == RuleFlood:
			flood++
		case a.Silent:
			silent++
		default:
			audible++
		}
	}
	if audible != floodLimit || flood != 1 || silent != 2 {
		t.Fatalf("audible=%d flood=%d silent=%d\n%v", audible, flood, silent, summary(delivered, true))
	}
}

func TestDeliveryLossIsSilentAndDeduped(t *testing.T) {
	h := newHarness(t)
	h.run("2026-09-20T09:00:00Z")
	health := func(at string) LogRecord {
		return LogRecord{ReceivedAt: mustTime(t, at), SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000082",
			Release: "5.2.1", Source: "telemetry", Level: "warn", Event: "delivery.health",
			Message: "pending_bytes=13828 dropped_records=3 rejected_records=0 last_ack=none error=log_upload_deferred"}
	}
	h.admin.add(fixtureFile{Logs: []LogRecord{health("2026-09-20T10:00:00Z"), health("2026-09-20T10:01:00Z")}})
	h.run("2026-09-20T10:02:00Z")
	if got := summary(h.rec.take(), false); !slices.Equal(got, []string{"delivery_loss:P2/silent"}) {
		t.Fatalf("delivered %v", got)
	}
}

type fakeHealthchecks struct {
	mu    sync.Mutex
	pings []string
}

func (f *fakeHealthchecks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.pings = append(f.pings, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(body)))
	f.mu.Unlock()
}

func (f *fakeHealthchecks) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pings) == 0 {
		return ""
	}
	return f.pings[len(f.pings)-1]
}

func TestChannelHealthAndDeadman(t *testing.T) {
	h := newHarness(t)
	checks := &fakeHealthchecks{}
	server := httptest.NewServer(checks)
	t.Cleanup(server.Close)
	h.cfg.DeadmanURL = server.URL + "/ping/check-a"
	ratio := 0.12
	h.admin.health.Rejections = map[string]int64{"events:400:invalid_request": 3, "logs:409:log_identity_conflict": 1}
	h.admin.health.LogsStorageRatio = &ratio

	h.run("2026-09-20T09:00:00Z") // bootstrap and baseline
	h.run("2026-09-20T09:01:00Z")
	if got := h.rec.take(); len(got) != 0 {
		t.Fatalf("baseline paged: %v", summary(got, true))
	}
	if got := checks.last(); got != "GET /ping/check-a" {
		t.Fatalf("green ping = %q", got)
	}

	h.admin.health.Rejections = map[string]int64{"events:400:invalid_request": 5, "logs:409:log_identity_conflict": 4}
	high := 0.83
	h.admin.health.LogsStorageRatio = &high
	h.run("2026-09-20T09:02:00Z")
	got := h.rec.take()
	if !slices.Equal(summary(got, false), []string{"channel_health:P1", "channel_health:P1"}) ||
		!strings.Contains(texts(got), "rechazos events:400:invalid_request · +2 desde la última ejecución") ||
		!strings.Contains(texts(got), "almacenamiento logs · 83% del límite") {
		t.Fatalf("channel health delivered:\n%s", texts(got))
	}

	down := false
	h.admin.health.DBOK = &down
	h.run("2026-09-20T09:03:00Z")
	if got := summary(h.rec.take(), false); !slices.Equal(got, []string{"channel_health:P1"}) {
		t.Fatalf("db down delivered %v", got)
	}
	if got := checks.last(); got != "POST /ping/check-a/fail db_ok=false" {
		t.Fatalf("fail ping = %q", got)
	}

	// A restarted collector resets its counters: current values are new.
	up := true
	h.admin.health.DBOK = &up
	h.admin.health.StartedAt = "2026-09-20T09:03:30Z"
	h.admin.health.Rejections = map[string]int64{"logs:429:rate_limited": 1}
	h.run("2026-09-20T09:04:00Z")
	if got := texts(h.rec.take()); !strings.Contains(got, "rechazos logs:429:rate_limited · +1") {
		t.Fatalf("after restart:\n%s", got)
	}
}

func TestRegressedFiresOncePerNewerRelease(t *testing.T) {
	resolved := &Issue{Key: "k", Status: "resolved", ResolvedIn: "2.4.62", Count: 5}
	base := Occurrence{Key: "k", Code: "audio_master_exhausted", Substage: "audio_master", Labels: Labels{"orchestrator", "pipeline.error", "worker", "render:variant"}, Alias: 1}
	tests := []struct {
		release       string
		prior         *Issue
		wantRule      string
		wantRegressed bool
	}{
		{"3.0.0", resolved, RuleRegressed, true},
		{"2.4.62", resolved, "", false},
		{"2.4.61", resolved, "", false},
		{"3.0.0", nil, RuleNewIssue, false},
		{"3.0.0", &Issue{Key: "k", Status: "ignored"}, "", false},
		{"3.0.0", &Issue{Key: "k", Status: "open"}, "", false},
	}
	for _, tt := range tests {
		o := base
		o.Release = tt.release
		alerts, regressed := evalIssue(o, tt.prior, time.Now())
		var rule string
		if len(alerts) == 1 {
			rule = alerts[0].Rule
		}
		if rule != tt.wantRule || regressed != tt.wantRegressed {
			t.Errorf("release %s prior %+v: rule %q regressed %t", tt.release, tt.prior, rule, regressed)
		}
		if rule == RuleRegressed && alerts[0].DedupKey != "k|3.0.0" {
			t.Errorf("regressed dedup key = %q", alerts[0].DedupKey)
		}
	}
}

func TestPlatformBreakNeedsTwoFailuresAndNoSuccess(t *testing.T) {
	row := func(outcome string) attemptRow { return attemptRow{Outcome: outcome} }
	tests := []struct {
		recent []attemptRow
		want   bool
	}{
		{[]attemptRow{row("error")}, false},
		{[]attemptRow{row("error"), row("error")}, true},
		{[]attemptRow{row("error"), row("cancelled"), row("error")}, true},
		{[]attemptRow{row("error"), row("error"), row("ok")}, false},
		{[]attemptRow{row("ok"), row("error"), row("error")}, false},
	}
	for i, tt := range tests {
		if _, got := evalPlatformBreak(PlatformInput{Alias: 1, Operation: "record:demo", Release: "4.0.1", Recent: tt.recent}); got != tt.want {
			t.Errorf("case %d: fired=%t, want %t", i, got, tt.want)
		}
	}
}

func TestQualityFloorsOnlyForFullDemoPresets(t *testing.T) {
	tests := []struct {
		quality string
		want    bool
	}{
		{"preset=gameplay-pov-60 width=1920 height=1080 fps=60 bitrate_kbps=40000 yavg_mean=90 black_ratio=0.01", false},
		{"preset=gameplay-pov-60 width=1920 height=1080 fps=60 bitrate_kbps=9000 yavg_mean=90 black_ratio=0.01", true},
		{"preset=gameplay-pov-60 width=1920 height=1080 fps=60 bitrate_kbps=40000 yavg_mean=90 black_ratio=0.6", true},
		{"preset=gameplay-pov-60 width=1280 height=720 fps=30 bitrate_kbps=6000 yavg_mean=90 black_ratio=0.01", false},
		{"preset=gameplay-pov-60 width=1280 height=720 fps=30 bitrate_kbps=6000 yavg_mean=12 black_ratio=0.01", true},
		{"preset=viral-aggressive width=1080 height=1920 fps=60 bitrate_kbps=900 yavg_mean=5 black_ratio=0.9", false},
	}
	for _, tt := range tests {
		if _, got := evalQuality(parseKV(tt.quality), JobContext{}, 1, "5.0.0", ""); got != tt.want {
			t.Errorf("%s: fired=%t, want %t", tt.quality, got, tt.want)
		}
	}
}

func TestClassifyCrashFamily(t *testing.T) {
	tests := []struct {
		record    Record
		wantOK    bool
		wantCrash bool
	}{
		{Record{Stream: "log", Event: "desktop.backend_crashed", Level: "error", Message: "child=orchestrator exit=2 exit_class=crashed"}, true, true},
		{Record{Stream: "log", Event: "desktop.backend_crashed", Level: "info", Message: "child=orchestrator exit=1 exit_class=shutdown_kill"}, false, false},
		{Record{Stream: "log", Event: "runtime.fatal_previous", Level: "error", Message: "source=orchestrator\nfatal error: concurrent map writes"}, true, true},
		{Record{Stream: "log", Event: "http.panicked", Level: "error", Message: "GET [path] panic: nil map"}, true, true},
		{Record{Stream: "event", Event: "desktop.boot_failed", Level: "error", Message: "web terminó inesperadamente (código 1073807364)"}, false, false},
		{Record{Stream: "event", Event: "desktop.boot_failed", Level: "error", Message: "web terminó inesperadamente (código 1)"}, true, true},
		{Record{Stream: "event", Event: "renderer.process_gone", Level: "error"}, true, false},
		{Record{Stream: "log", Event: "http.completed", Level: "error", Message: "GET [path] status=404"}, false, false},
		{Record{Stream: "log", Event: "attempt.finished", Level: "error", Outcome: "error", Operation: "record:demo"}, true, false},
		{Record{Stream: "log", Event: "attempt.finished", Level: "info", Outcome: "cancelled", Operation: "record:demo"}, false, false},
	}
	for _, tt := range tests {
		_, crash, ok := classify(tt.record)
		if ok != tt.wantOK || crash != tt.wantCrash {
			t.Errorf("%s %q: ok=%t crash=%t", tt.record.Event, tt.record.Message, ok, crash)
		}
	}
}
