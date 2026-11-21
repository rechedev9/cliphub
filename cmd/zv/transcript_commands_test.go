package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/transcriptmeasure"
)

type transcriptLibraryFixture struct {
	t    *testing.T
	ctx  context.Context
	repo store.LibraryRepository
}

func (f transcriptLibraryFixture) match(sha, mapName string) library.Match {
	f.t.Helper()
	played := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	demo := library.Demo{SHA256: sha, FileName: sha + ".dem", Path: "library/" + sha, PlayedAt: &played}
	if _, err := f.repo.CreateDemo(f.ctx, &demo); err != nil {
		f.t.Fatal(err)
	}
	m, err := f.repo.SaveAnalysis(f.ctx, library.Analysis{
		DemoID: demo.ID, Source: library.SourceFaceit, CS2Build: 14100, PatchVersion: 1,
		Map: mapName, Tickrate: 64, DurationTicks: 100000, RoundCount: 21,
		Teams: []library.AnalysisTeam{
			{Key: "ct-start", StartSide: "CT", ClanName: "FUT", Score: 13},
			{Key: "t-start", StartSide: "T", ClanName: "OG", Score: 8},
		},
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return m
}

func (f transcriptLibraryFixture) frases(m library.Match, texts ...string) []library.DemoVoiceLine {
	f.t.Helper()
	lines := make([]library.DemoVoiceLine, 0, len(texts))
	for i, text := range texts {
		start := int64(i) * 60_000
		lines = append(lines, library.DemoVoiceLine{
			ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start",
			StartMS: start, EndMS: start + 60_000, Round: i + 1, Text: text,
		})
	}
	update := library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}
	if err := f.repo.SetDemoVoice(f.ctx, update); err != nil {
		f.t.Fatal(err)
	}
	return lines
}

func (f transcriptLibraryFixture) edit(patch library.DemoVoiceLinePatch) {
	f.t.Helper()
	if _, err := f.repo.UpdateDemoVoiceLine(f.ctx, patch); err != nil {
		f.t.Fatal(err)
	}
}

func correctedLibrary(t *testing.T) (string, library.Match) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jobs.db")
	repos, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	f := transcriptLibraryFixture{t: t, ctx: context.Background(), repo: repos.Library}
	reviewed := f.match("reviewed", "de_mirage")
	lines := f.frases(reviewed, "rush be no stop", "eco", "gracias por ver")
	fixed, discarded := "Rush B, no stop", true
	f.edit(library.DemoVoiceLinePatch{MatchID: reviewed.ID, ID: lines[0].ID, Text: &fixed})
	f.edit(library.DemoVoiceLinePatch{MatchID: reviewed.ID, ID: lines[2].ID, Discarded: &discarded})
	f.frases(f.match("untouched", "de_nuke"), "nobody reviewed this one")
	if err := repos.Close(); err != nil {
		t.Fatal(err)
	}
	return path, reviewed
}

func TestTranscriptMeasureReportsEachCorrectedMatch(t *testing.T) {
	path, reviewed := correctedLibrary(t)
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "transcript", "measure", "--library", path, "--format", "json"}, &stdout, &stderr, strings.NewReader(""), nil)
	if code != exitSuccess {
		t.Fatalf("code = %d; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var got transcriptMeasureResult
	if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	want := transcriptmeasure.Result{
		Frases: 3, CorrectedFrases: 1, DiscardedFrases: 1, Words: 5, CorrectedWords: 1,
		CorrectedShare: 0.2, SpeechMS: 120_000, DiscardedPerMinute: 0.5,
	}
	if len(got.Transcripts) != 1 {
		t.Fatalf("transcripts = %+v; want only the match a person corrected", got.Transcripts)
	}
	row := got.Transcripts[0]
	if row.Source != "demo" || row.ID != reviewed.ID.String() || row.Name != "de_mirage FUT 13-8 OG" || row.Result != want {
		t.Fatalf("row = %+v; want the reviewed match with %+v", row, want)
	}
	if got.Total != want {
		t.Fatalf("total = %+v, want %+v", got.Total, want)
	}
}

func TestTranscriptMeasureTextShowsShareAndDiscardRate(t *testing.T) {
	path, reviewed := correctedLibrary(t)
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "transcript", "measure", "--library", path}, &stdout, &stderr, strings.NewReader(""), nil)
	if code != exitSuccess {
		t.Fatalf("code = %d; stderr=%s", code, stderr.String())
	}
	wantRow := "demo\t" + reviewed.ID.String() + "\tde_mirage FUT 13-8 OG\t5\t1\t20.0%\t3\t1\t2.00\t0.50"
	if !strings.Contains(stdout.String(), wantRow+"\n") {
		t.Fatalf("text output = %q; want the row %q", stdout.String(), wantRow)
	}
	if strings.Contains(stdout.String(), "de_nuke") {
		t.Fatalf("text output lists a match nobody corrected: %q", stdout.String())
	}
}

func TestTranscriptMeasureRefusesBadInput(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "jobs.db")
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantSub  string
	}{
		{name: "no library", args: []string{"transcript", "measure"}, wantCode: exitInvalidArgs, wantSub: "--library"},
		{name: "unknown subcommand", args: []string{"transcript", "fix"}, wantCode: exitInvalidArgs, wantSub: "transcript measure"},
		{name: "library that does not exist", args: []string{"transcript", "measure", "--library", missing}, wantCode: exitUnexpected, wantSub: "jobs.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := Run(append([]string{"zv"}, tt.args...), &stdout, &stderr, strings.NewReader(""), nil)
			if code != tt.wantCode || !strings.Contains(stderr.String(), tt.wantSub) {
				t.Fatalf("code = %d, stderr = %q; want %d mentioning %q", code, stderr.String(), tt.wantCode, tt.wantSub)
			}
		})
	}
}
