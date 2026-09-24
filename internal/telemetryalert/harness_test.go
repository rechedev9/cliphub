package telemetryalert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

const testAdminToken = "admin-token-with-at-least-32-characters-long"

// fakeAdmin implements the admin API contract (contracts.md §1) over fixture
// records, showing only what the collector had received by `now`.
type fakeAdmin struct {
	mu     sync.Mutex
	now    time.Time
	events []ErrorEvent
	// eventRow is the collector's rowid: assigned when an event becomes
	// visible (committed), so the feed pages in commit order.
	eventRow map[string]int64
	logs     []LogRecord
	health   Health
	noErrors bool // an old collector without /v1/errors
	requests []string
}

type fixtureFile struct {
	Events []ErrorEvent `json:"events"`
	Logs   []LogRecord  `json:"logs"`
}

func loadFixture(t *testing.T, name string) fixtureFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "incidents", name))
	if err != nil {
		t.Fatal(err)
	}
	var f fixtureFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return f
}

func (f *fakeAdmin) add(fixture fixtureFile) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, event := range fixture.Events {
		if event.ID == "" {
			event.ID = fmt.Sprintf("e0000000-0000-4000-8000-%012d", len(f.events)+1)
		}
		event.Kind = "error"
		f.events = append(f.events, event)
	}
	for _, record := range fixture.Logs {
		if record.ID == "" {
			record.ID = fmt.Sprintf("10900000-0000-4000-8000-%012d", len(f.logs)+1)
		}
		f.logs = append(f.logs, record)
	}
	sort.SliceStable(f.events, func(i, j int) bool { return eventCursorLess(f.events[i], f.events[j]) })
	sort.SliceStable(f.logs, func(i, j int) bool { return f.logs[i].ReceivedAt.Before(f.logs[j].ReceivedAt) })
	for i := range f.logs {
		f.logs[i].Cursor = int64(i + 1)
	}
}

func eventCursorLess(a, b ErrorEvent) bool {
	if !a.ReceivedAt.Equal(b.ReceivedAt) {
		return a.ReceivedAt.Before(b.ReceivedAt)
	}
	return a.ID < b.ID
}

func (f *fakeAdmin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer "+testAdminToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	switch r.URL.Path {
	case "/healthz":
		_ = json.NewEncoder(w).Encode(f.health)
	case "/v1/errors":
		if f.noErrors {
			http.NotFound(w, r)
			return
		}
		if f.eventRow == nil {
			f.eventRow = map[string]int64{}
		}
		for _, event := range f.events {
			if _, ok := f.eventRow[event.ID]; !ok && !event.ReceivedAt.After(f.now) {
				f.eventRow[event.ID] = int64(len(f.eventRow) + 1)
			}
		}
		committed := make([]ErrorEvent, 0, len(f.eventRow))
		for _, event := range f.events {
			if _, ok := f.eventRow[event.ID]; ok {
				committed = append(committed, event)
			}
		}
		sort.Slice(committed, func(i, j int) bool { return f.eventRow[committed[i].ID] < f.eventRow[committed[j].ID] })
		after, _ := strconv.ParseInt(query.Get("after"), 10, 64)
		page := errorPage{Events: []ErrorEvent{}, NextAfter: query.Get("after")}
		for _, event := range committed {
			row := f.eventRow[event.ID]
			if row <= after {
				continue
			}
			if len(page.Events) == limit {
				page.HasMore = true
				break
			}
			page.Events = append(page.Events, event)
			page.NextAfter = strconv.FormatInt(row, 10)
		}
		_ = json.NewEncoder(w).Encode(page)
	case "/v1/logs":
		after, _ := strconv.ParseInt(query.Get("after"), 10, 64)
		page := logPage{Records: []LogRecord{}, NextCursor: after}
		for _, record := range f.logs {
			if record.Cursor <= after || record.ReceivedAt.After(f.now) {
				continue
			}
			if len(page.Records) == limit {
				page.HasMore = true
				break
			}
			page.Records = append(page.Records, record)
			page.NextCursor = record.Cursor
		}
		_ = json.NewEncoder(w).Encode(page)
	default:
		http.NotFound(w, r)
	}
}

// recorder is an in-memory Notifier.
type recorder struct {
	mu   sync.Mutex
	sent []Alert
	fail bool
}

func (r *recorder) Send(_ context.Context, a Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return fmt.Errorf("channel down")
	}
	r.sent = append(r.sent, a)
	return nil
}

func (r *recorder) take() []Alert {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.sent
	r.sent = nil
	return out
}

type harness struct {
	t        *testing.T
	admin    *fakeAdmin
	server   *httptest.Server
	cfg      Config
	notifier Notifier
	rec      *recorder
}

func newHarness(t *testing.T, fixtures ...string) *harness {
	t.Helper()
	ok := true
	admin := &fakeAdmin{health: Health{Service: "cliphub-telemetry-admin", Status: "ok", Version: "test", StartedAt: "2026-09-01T00:00:00Z", DBOK: &ok}}
	for _, name := range fixtures {
		admin.add(loadFixture(t, name))
	}
	server := httptest.NewServer(admin)
	t.Cleanup(server.Close)
	rec := &recorder{}
	return &harness{t: t, admin: admin, server: server, rec: rec, notifier: rec, cfg: Config{
		AdminURL: server.URL, AdminToken: testAdminToken, StateDir: t.TempDir(), TelegramChat: 42,
		ReportBase: "https://report.invalid/alerts",
	}}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// run executes one alerter pass as if the timer fired at `at`.
func (h *harness) run(at string, mutate ...func(*Options)) Result {
	h.t.Helper()
	now := mustTime(h.t, at)
	h.admin.mu.Lock()
	h.admin.now = now
	h.admin.mu.Unlock()
	opts := Options{Now: func() time.Time { return now }, Notifier: h.notifier, Logf: func(string, ...any) {}}
	for _, m := range mutate {
		m(&opts)
	}
	result, err := Run(context.Background(), h.cfg, opts)
	if err != nil {
		h.t.Fatalf("run at %s: %v", at, err)
	}
	return result
}

func summary(alerts []Alert, keepDigest bool) []string {
	var out []string
	for _, a := range alerts {
		if a.Rule == RuleDigest && !keepDigest {
			continue
		}
		silent := ""
		if a.Priority == 2 || a.Silent {
			silent = "/silent"
		}
		out = append(out, fmt.Sprintf("%s:P%d%s", a.Rule, a.Priority, silent))
	}
	return out
}
