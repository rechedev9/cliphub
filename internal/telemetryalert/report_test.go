package telemetryalert

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite the report golden files")

func readPage(t *testing.T, h *harness, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(h.cfg.StateDir, "www", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestReportGolden(t *testing.T) {
	h := newHarness(t, "hlae.json", "faceit.json")
	for _, at := range []string{"2026-09-05T00:00:00Z", "2026-09-10T14:31:00Z", "2026-09-11T07:31:00Z", "2026-09-22T20:00:00Z", "2026-09-23T13:52:00Z"} {
		h.run(at)
	}
	hlaeKey := IssueKey(Labels{"orchestrator", "pipeline.error", "worker", "record:demo"}, "hlae_hook_incompatible")
	for page, golden := range map[string]string{
		"index.html":                     "index.golden.html",
		"issue/" + hlaeKey + ".html":     "issue-hlae.golden.html",
	} {
		// core.autocrlf may check the template or goldens out with CRLF.
		got := strings.ReplaceAll(readPage(t, h, page), "\r\n", "\n")
		path := filepath.Join("testdata", "report", golden)
		if *updateGolden {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
			t.Errorf("%s differs from %s; run go test ./internal/telemetryalert -run TestReportGolden -update", page, golden)
		}
	}
}

func TestReportEscapesMessagesAndLoadsNothing(t *testing.T) {
	h := newHarness(t)
	h.run("2026-09-20T09:00:00Z")
	message := `failed <script>alert(1)</script> <img src=x onerror=alert(2)> "quoted" & more`
	h.admin.add(fixtureFile{Events: []ErrorEvent{{
		ReceivedAt: mustTime(t, "2026-09-20T10:00:00Z"), SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000090",
		Release: "5.2.1", Component: "orchestrator", Name: "pipeline.error", Stage: "worker", Class: "render:variant",
		JobID: "e5c00001-0000-4000-8000-000000000001", Message: message,
	}}})
	h.run("2026-09-20T10:01:00Z")
	key := IssueKey(Labels{"orchestrator", "pipeline.error", "worker", "render:variant"}, "unclassified:"+Signature(message))
	for _, page := range []string{"index.html", "issue/" + key + ".html"} {
		html := readPage(t, h, page)
		if strings.Contains(html, "<script") || strings.Contains(html, "<img") {
			t.Fatalf("%s renders markup from a message:\n%s", page, html)
		}
		for _, want := range []string{
			"&lt;script&gt;alert(1)&lt;/script&gt;", "&lt;img src=x onerror=alert(2)&gt;",
			`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'">`,
			"CH-1111-2222-3333-4444-5555",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s lacks %q", page, want)
			}
		}
	}
	if html := readPage(t, h, "issue/"+key+".html"); !strings.Contains(html, "node scripts/telemetry-debug.mjs --job e5c00001-0000-4000-8000-000000000001") {
		t.Errorf("issue page lacks the debug command")
	}
}
