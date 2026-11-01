package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestParseAgentSession(t *testing.T) {
	token := strings.Repeat("ab", 32)
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"valid", `{"version":1,"enabled":true,"orchestratorUrl":"http://127.0.0.1:4100","token":"` + token + `"}`, nil},
		{"switch off", `{"version":1,"enabled":false}`, errAgentOff},
		{"unknown version", `{"version":2,"enabled":true}`, errStudioClosed},
		{"not json", `{`, errStudioClosed},
	}
	for _, tc := range cases {
		_, err := parseAgentSession([]byte(tc.raw))
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
	for _, raw := range []string{
		`{"version":1,"enabled":true,"orchestratorUrl":"http://evil.example:80","token":"` + token + `"}`,
		`{"version":1,"enabled":true,"orchestratorUrl":"http://127.0.0.1:4100","token":"short"}`,
	} {
		if _, err := parseAgentSession([]byte(raw)); err == nil {
			t.Errorf("accepted invalid session %s", raw)
		}
	}
}

func TestCleanNameStripsControlAndCaps(t *testing.T) {
	if got := cleanName(" s1mple‮\n\x00 "); got != "s1mple" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("x", 200)
	if got := []rune(cleanName(long)); len(got) != maxNameRunes+1 || got[maxNameRunes] != '…' {
		t.Errorf("not capped: %d runes", len(got))
	}
}

func TestCheckDemoPath(t *testing.T) {
	dir := t.TempDir()
	demo := filepath.Join(dir, "match.dem")
	if err := os.WriteFile(demo, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkDemoPath(demo); err != nil {
		t.Errorf("valid demo refused: %v", err)
	}
	for _, p := range []string{"match.dem", filepath.Join(dir, "notes.txt"), filepath.Join(dir, "missing.dem")} {
		if checkDemoPath(p) == nil {
			t.Errorf("accepted %s", p)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.dem"), 0o700); err != nil {
		t.Fatal(err)
	}
	if checkDemoPath(filepath.Join(dir, "folder.dem")) == nil {
		t.Error("accepted a directory")
	}
}

func TestCheckDemoPathRefusesNetworkAndOversize(t *testing.T) {
	for _, p := range []string{`\\evil.example\share\match.dem`, `//evil.example/share/match.dem`, `\\?\C:\match.dem`} {
		if err := checkDemoPath(p); err == nil || !strings.Contains(err.Error(), "UNC") {
			t.Errorf("%s: got %v, want a UNC refusal", p, err)
		}
	}
	if runtime.GOOS == "windows" {
		if checkDemoPath(`relative\match.dem`) == nil || checkDemoPath(`1:\match.dem`) == nil {
			t.Error("accepted a path without a drive letter")
		}
	}
	big := filepath.Join(t.TempDir(), "big.dem")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxDemoBytes + 1); err != nil {
		t.Skipf("cannot make a sparse file: %v", err)
	}
	_ = f.Close()
	if err := checkDemoPath(big); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Errorf("oversize demo: got %v", err)
	}
}

func TestDescribeJobDecidesRecordedFromRenders(t *testing.T) {
	progress := &tuiclient.JobProgress{Stage: "encode", Percent: 70}
	render := func(status, errText string) []tuiclient.RenderVariantState {
		return []tuiclient.RenderVariantState{{Variant: "viral-60-clean", Status: status, Error: errText}}
	}
	cases := []struct {
		name     string
		status   string
		progress *tuiclient.JobProgress
		renders  []tuiclient.RenderVariantState
		running  bool
		code     string
		percent  int
	}{
		{"parsing is running", "parsing", nil, nil, true, "", 0},
		{"parsed is settled", "parsed", nil, nil, false, "", 0},
		{"failed is settled", "failed", nil, nil, false, "", 0},
		{"stale progress alone is not running", statusRecorded, progress, nil, false, "", 0},
		{"stale progress after a ready render", statusRecorded, progress, render("ready", ""), false, "", 0},
		{"queued render is the gap after capture", statusRecorded, nil, render("queued", ""), true, "", 0},
		{"rendering reports progress", statusRecorded, progress, render("rendering", ""), true, "", 70},
		{"failed render", statusRecorded, progress, render("failed", "encoder crashed"), false, "render_failed", 0},
		{"legacy review_required counts as done", statusRecorded, progress, render("review_required", ""), false, "", 0},
	}
	for _, tc := range cases {
		out := describeJob("job", tuiclient.JobStatus{Status: tc.status, Progress: tc.progress}, tc.renders)
		if out.Running != tc.running || out.FailureCode != tc.code || out.Percent != tc.percent {
			t.Errorf("%s: got running=%v code=%q percent=%d", tc.name, out.Running, out.FailureCode, out.Percent)
		}
	}
	failed := describeJob("job", tuiclient.JobStatus{Status: statusRecorded}, render("failed", "encoder\x00 crashed"))
	if failed.FailureReason != "encoder crashed" || len(failed.Renders) != 1 {
		t.Errorf("failure details: %+v", failed)
	}
	mixed := describeJob("job", tuiclient.JobStatus{Status: statusRecorded}, []tuiclient.RenderVariantState{
		{Variant: "a", Status: "ready"}, {Variant: "b", Status: "rendering"}, {Variant: "c", Status: "failed"}})
	if !mixed.Running {
		t.Error("a rendering variant among settled ones should keep the job running")
	}
}

func TestDescribePlays(t *testing.T) {
	moments := []tuiclient.Moment{
		{SegmentID: "seg-001", Player: "s1mple\x00", Map: "de_mirage", Round: 5, Events: tuiclient.MomentEvents{Kills: 4}},
		{SegmentID: "seg-002", Round: 9, Events: tuiclient.MomentEvents{Kills: 2}},
	}
	got, err := describePlays(moments, []string{"seg-002", "seg-001"})
	if err != nil {
		t.Fatal(err)
	}
	want := "seg-002 (ronda 9, 2 bajas); seg-001 (s1mple, de_mirage, ronda 5, 4 bajas)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if _, err := describePlays(moments, []string{"seg-009"}); err == nil {
		t.Error("accepted a segment that is not a play of the job")
	}
}

func TestCheckIDs(t *testing.T) {
	if checkUUID("id", "9d4f1a52-2c1e-4a5e-8d6f-3b7a1c0e9f21") != nil {
		t.Error("refused a UUID")
	}
	for _, bad := range []string{"", "job-1", "../x", "9D4F1A52-2C1E-4A5E-8D6F-3B7A1C0E9F21", "00000000-0000-0000-0000-000000000000"} {
		if checkUUID("id", bad) == nil {
			t.Errorf("accepted %q as a UUID", bad)
		}
	}
	if err := checkSegmentIDs([]string{"seg-001", "seg-18", "seg--07"}); err != nil {
		t.Errorf("refused plan segment ids: %v", err)
	}
	for _, bad := range [][]string{nil, {"seg-1/../x"}, {"../seg-1"}, {"seg-1", "seg-1"}, {"segment-1"}} {
		if checkSegmentIDs(bad) == nil {
			t.Errorf("accepted segment ids %q", bad)
		}
	}
	for _, name := range []string{"viral-60-clean", "short-001-s1mple_mirage_4k"} {
		if checkArtifactName("name", name) != nil {
			t.Errorf("refused %q", name)
		}
	}
	for _, bad := range []string{"", "..", ".hidden", "a/b", "a\\b", "a.mp4", "a b"} {
		if checkArtifactName("name", bad) == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if checkSteamID("76561198000000000") != nil || checkSteamID("7656119800000000x") != nil == false {
		t.Error("steamid check wrong")
	}
}

func TestTeamNameFallsBackToStartSide(t *testing.T) {
	cases := map[string]tuiclient.LibraryMatchTeam{
		"Navi":       {TeamName: "Navi", ClanName: "navi.gg", StartSide: "ct"},
		"navi.gg":    {ClanName: "navi.gg", StartSide: "ct"},
		"Empieza CT": {Key: "ct-start", StartSide: "ct"},
		"t-start":    {Key: "t-start"},
	}
	for want, team := range cases {
		if got := teamName(team); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
