package syncclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/clipmind"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/syncclient"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

type mailbox struct {
	mu   sync.Mutex
	mail map[string]string
}

func (m *mailbox) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mail == nil {
		m.mail = map[string]string{}
	}
	m.mail[to+"|"+subject] = body
	return nil
}

var tokenLink = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

func (m *mailbox) token(t *testing.T, to, subject string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		for key, body := range m.mail {
			if strings.HasPrefix(key, to+"|") && strings.Contains(key, subject) {
				if match := tokenLink.FindStringSubmatch(body); match != nil {
					m.mu.Unlock()
					return match[1]
				}
			}
		}
		m.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %q email for %s", subject, to)
	return ""
}

func startHub(t *testing.T) (*httptest.Server, *mailbox) {
	t.Helper()
	box := &mailbox{}
	srv, err := clipmind.New(clipmind.Config{DataDir: t.TempDir(), PublicURL: "https://clipmind.test", Mailer: box, PBKDF2Iterations: 1000})
	if err != nil {
		t.Fatal(err)
	}
	hub := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hub.Close()
		_ = srv.Close()
	})
	return hub, box
}

func postJSON(t *testing.T, url string, body any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s = %d %s", url, resp.StatusCode, out)
	}
}

// device is a desktop library: its own database, its own files, its own sync link.
type device struct {
	repos  *store.Repositories
	files  *storage.Local
	voice  *storage.Local
	client *syncclient.Client
}

func newDevice(t *testing.T, name string) *device {
	t.Helper()
	dir := t.TempDir()
	repos, err := store.OpenSQLite(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repos.Close() })
	files, err := storage.NewLocal(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	voice, err := storage.NewLocal(filepath.Join(dir, "voice"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := syncclient.New(syncclient.Config{StatePath: filepath.Join(dir, "clipmind-sync.json"), DB: repos.DB(), Library: repos.Library, Files: files, VoiceFiles: voice, DeviceName: name})
	if err != nil {
		t.Fatal(err)
	}
	return &device{repos: repos, files: files, voice: voice, client: c}
}

// addMatch stores one analysed demo the way the library worker does.
func (d *device) addMatch(t *testing.T, sha string) library.Match {
	t.Helper()
	ctx := context.Background()
	blob, err := tacticalplan.EncodePositions([]tacticalplan.RoundFrames{{
		Round:  1,
		Frames: []tacticalplan.Frame{{Tick: 100, Samples: []tacticalplan.Sample{{Slot: 0, X: 1, Y: 2, Health: 100, Flags: tacticalplan.FlagAlive}}}},
	}}, 8, 64)
	if err != nil {
		t.Fatal(err)
	}
	doc := tacticalplan.Document{
		SchemaVersion: "1",
		Demo:          tacticalplan.Demo{Path: `C:\Users\luis\demos\x.dem`, SHA256: sha, Map: "de_mirage", Tickrate: 64, DurationTicks: 500},
		Teams:         []tacticalplan.Team{{Key: "a", Name: "Alpha", StartSide: tacticalplan.SideCT}, {Key: "b", Name: "Bravo", StartSide: tacticalplan.SideT}},
		Rounds:        []tacticalplan.Round{{Number: 1, TickStart: 0, TickEnd: 400, Winner: tacticalplan.SideCT}},
		Geometry:      tacticalplan.MapGeometry{Calibration: radarmap.Calibration{Map: "de_mirage", PosX: -3230, PosY: 1713, Scale: 5, Size: 1024}},
		Positions:     blob.Descriptor,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	demo := library.Demo{SHA256: sha, FileName: "x.dem", Path: `C:\Users\luis\demos\x.dem`, SizeBytes: 10, Source: library.SourceUnknown, State: library.ParseQueued}
	if _, err := d.repos.Library.CreateDemo(ctx, &demo); err != nil {
		t.Fatal(err)
	}
	if err := d.files.Put(artifacts.LibraryTacticalPositionsKey(demo.ID), bytes.NewReader(blob.Data)); err != nil {
		t.Fatal(err)
	}
	if err := d.files.Put(artifacts.LibraryTacticalIndexKey(demo.ID), bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	match, err := d.repos.Library.SaveAnalysis(ctx, library.AnalysisFromDocument(demo.ID, doc))
	if err != nil {
		t.Fatal(err)
	}
	return match
}

func (d *device) matchCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := d.repos.DB().QueryRow(`SELECT COUNT(*) FROM library_matches`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestTwoDevicesShareOneAccount(t *testing.T) {
	hub, box := startHub(t)
	postJSON(t, hub.URL+"/auth/signup", map[string]string{"email": "luis@example.com", "password": "correct horse battery"})
	postJSON(t, hub.URL+"/auth/confirm", map[string]string{"token": box.token(t, "luis@example.com", "Confirma"), "password": "correct horse battery"})
	ctx := context.Background()

	pc := newDevice(t, "PC de sobremesa")
	match := pc.addMatch(t, strings.Repeat("ab", 32))
	if _, err := pc.client.Connect(ctx, syncclient.ConnectInput{ServerURL: hub.URL, Email: "luis@example.com", Password: "wrong password!!"}); err == nil {
		t.Fatal("Connect accepted a wrong password")
	}
	if st := pc.client.Status(ctx); st.Connected {
		t.Fatalf("status after a failed connect = %+v", st)
	}
	st, err := pc.client.Connect(ctx, syncclient.ConnectInput{ServerURL: hub.URL, Email: "luis@example.com", Password: "correct horse battery"})
	if err != nil || !st.Connected || st.WorkspaceName != "Mi espacio" {
		t.Fatalf("Connect = %+v, %v", st, err)
	}
	if err := pc.client.Sync(ctx); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if st := pc.client.Status(ctx); st.LastSyncAt == 0 || st.LastError != "" || st.Pending != 0 {
		t.Fatalf("status after Sync = %+v", st)
	}

	// A second computer joins the same account and receives the match and its files.
	laptop := newDevice(t, "Portátil")
	if _, err := laptop.client.Connect(ctx, syncclient.ConnectInput{ServerURL: hub.URL, Email: "luis@example.com", Password: "correct horse battery"}); err != nil {
		t.Fatal(err)
	}
	if err := laptop.client.Sync(ctx); err != nil {
		t.Fatalf("laptop Sync: %v", err)
	}
	if laptop.matchCount(t) != 1 {
		t.Fatalf("laptop has %d matches, want 1", laptop.matchCount(t))
	}
	for _, key := range []string{artifacts.LibraryTacticalIndexKey(match.DemoID), artifacts.LibraryTacticalPositionsKey(match.DemoID)} {
		if ok, _ := laptop.files.Exists(key); !ok {
			t.Fatalf("laptop is missing %s", key)
		}
	}
	var path string
	if err := laptop.repos.DB().QueryRow(`SELECT path FROM library_demos WHERE id = ?`, match.DemoID.String()).Scan(&path); err != nil || path != "" {
		t.Fatalf("the laptop's demo path = %q (%v); the PC's path must not travel", path, err)
	}
	stored, _ := laptop.files.Open(artifacts.LibraryTacticalIndexKey(match.DemoID))
	doc, _ := io.ReadAll(stored)
	_ = stored.Close()
	if bytes.Contains(doc, []byte("luis")) {
		t.Fatalf("the PC's file path reached the hub: %s", doc)
	}

	// An edit on the laptop reaches the PC, and the laptop does not get its own edit back.
	if _, err := laptop.repos.DB().Exec(`UPDATE library_matches SET round_count = 30 WHERE id = ?`, match.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := laptop.client.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pc.client.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	var rounds int
	if err := pc.repos.DB().QueryRow(`SELECT round_count FROM library_matches WHERE id = ?`, match.ID.String()).Scan(&rounds); err != nil || rounds != 30 {
		t.Fatalf("the PC's round_count = %d (%v), want the laptop's 30", rounds, err)
	}
	if st := laptop.client.Status(ctx); st.Pending != 0 {
		t.Fatalf("laptop still has %d changes pending", st.Pending)
	}

	// Disconnecting forgets the link but keeps the data.
	if err := laptop.client.Disconnect(ctx); err != nil {
		t.Fatal(err)
	}
	if st := laptop.client.Status(ctx); st.Connected || laptop.matchCount(t) != 1 {
		t.Fatalf("after Disconnect: %+v, %d matches", st, laptop.matchCount(t))
	}
	if err := laptop.client.Sync(ctx); err != syncclient.ErrNotConnected {
		t.Fatalf("Sync after Disconnect = %v, want ErrNotConnected", err)
	}
}

func TestConnectRefusesPlainHTTPToARemoteServer(t *testing.T) {
	d := newDevice(t, "PC")
	for _, bad := range []string{"http://clipmind.example.com", "ftp://x", "not a url", ""} {
		if _, err := d.client.Connect(context.Background(), syncclient.ConnectInput{ServerURL: bad, Email: "a@b.co", Password: "x"}); err == nil {
			t.Errorf("Connect accepted %q", bad)
		}
	}
}
