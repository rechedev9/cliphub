package patterndetect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const freeze = 1000

// files is a storage over a map; failing makes an Open fail, and onOpen runs before each one.
type files struct {
	blobs   map[string][]byte
	failing map[string]error
	onOpen  func(key string)
}

func (f *files) Put(key string, r io.Reader) error {
	b, err := io.ReadAll(r)
	f.blobs[key] = b
	return err
}

func (f *files) Open(key string) (io.ReadCloser, error) {
	if f.onOpen != nil {
		f.onOpen(key)
	}
	if err := f.failing[key]; err != nil {
		return nil, err
	}
	b, ok := f.blobs[key]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: key, Err: os.ErrNotExist}
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (f *files) Exists(key string) (bool, error) {
	_, ok := f.blobs[key]
	return ok, nil
}

type fixture struct {
	t     *testing.T
	ctx   context.Context
	repo  *store.MemoryLibraryRepository
	files *files
	demo  library.Demo
}

// newFixture is one de_ancient match: team "Nosotros" (t-start) stands in A in rounds 1 to 3.
func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{t: t, ctx: context.Background(), repo: store.NewMemoryLibraryRepository(), files: &files{blobs: map[string][]byte{}, failing: map[string]error{}}}
	played := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	f.demo = library.Demo{SHA256: "pd1", FileName: "pd1.dem", Path: "library/pd1", PlayedAt: &played}
	if _, err := f.repo.CreateDemo(f.ctx, &f.demo); err != nil {
		t.Fatal(err)
	}
	doc := tacticalplan.Document{
		Demo:  tacticalplan.Demo{Map: "de_ancient", Tickrate: 64},
		Teams: []tacticalplan.Team{{Key: "t-start", StartSide: tacticalplan.SideT}, {Key: "ct-start", StartSide: tacticalplan.SideCT}},
		Players: []tacticalplan.Player{{Slot: 0, SteamID64: "76561198000000010", TeamKey: "t-start"},
			{Slot: 1, SteamID64: "76561198000000013", TeamKey: "ct-start"}},
		Geometry: tacticalplan.MapGeometry{Calibration: radarmap.Calibration{Map: "de_ancient", Scale: 5, Size: 1024}},
	}
	var rounds []tacticalplan.RoundFrames
	for n := range 3 {
		doc.Rounds = append(doc.Rounds, tacticalplan.Round{Number: n + 1, Half: 1, TickFreezeEnd: freeze, TickEnd: freeze + 64*30, Winner: tacticalplan.SideT,
			Players: []tacticalplan.PlayerRound{{Slot: 0, Side: tacticalplan.SideT}, {Slot: 1, Side: tacticalplan.SideCT}}})
		var frames []tacticalplan.Frame
		for s := 0; s <= 5; s++ {
			frames = append(frames, tacticalplan.Frame{Tick: freeze + s*64, Samples: []tacticalplan.Sample{
				{Slot: 0, X: 300, Flags: tacticalplan.FlagAlive | tacticalplan.FlagSideT}, {Slot: 1, X: 900, Y: 900, Flags: tacticalplan.FlagAlive}}})
		}
		rounds = append(rounds, tacticalplan.RoundFrames{Round: n + 1, Frames: frames})
	}
	blob, err := tacticalplan.EncodePositions(rounds, 8, 64)
	if err != nil {
		t.Fatal(err)
	}
	doc.Positions = blob.Descriptor
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	f.files.blobs[artifacts.LibraryTacticalIndexKey(f.demo.ID)] = raw
	f.files.blobs[artifacts.LibraryTacticalPositionsKey(f.demo.ID)] = blob.Data
	return f
}

// analyze saves (or saves again) the demo's analysis, as a scan does.
func (f fixture) analyze() library.Match {
	f.t.Helper()
	m, err := f.repo.SaveAnalysis(f.ctx, library.Analysis{DemoID: f.demo.ID, Source: library.SourceFaceit, Map: "de_ancient", Tickrate: 64,
		DurationTicks: 100000, RoundCount: 3, Teams: []library.AnalysisTeam{
			{Key: "ct-start", StartSide: "CT", ClanName: "Rivales", Score: 0}, {Key: "t-start", StartSide: "T", ClanName: "Nosotros", Score: 3}}})
	if err != nil {
		f.t.Fatal(err)
	}
	return m
}

func (f fixture) pattern(name string, teamID *uuid.UUID) library.Pattern {
	f.t.Helper()
	p := library.Pattern{Name: name, Map: "de_ancient", TeamID: teamID, Pattern: tacticalplan.Pattern{Side: tacticalplan.SideT,
		Blocks: []tacticalplan.PatternBlock{{Kind: tacticalplan.PatternPlayer, Area: &tacticalplan.PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}}}}}
	if err := f.repo.CreatePattern(f.ctx, &p); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f fixture) rows(p library.Pattern) []library.PatternMatchRow {
	f.t.Helper()
	rows, err := f.repo.ListPatternMatches(f.ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{p.ID}})
	if err != nil {
		f.t.Fatal(err)
	}
	return rows
}

func TestStoredRowsAnswerUntilTheMatchIsAnalysedAgain(t *testing.T) {
	f := newFixture(t)
	m := f.analyze()
	p := f.pattern("A", nil)
	d := New(f.repo, f.files)
	got, err := d.Results(f.ctx, []library.Pattern{p})
	if err != nil || got[0].Matched != 3 {
		t.Fatalf("Results = %+v, %v; want 3 rounds", got, err)
	}
	// A stored row stands in for the match while the match and the pattern stay as they are.
	rows := f.rows(p)
	forged := rows[len(rows)-1]
	forged.Read.Match.Matched = forged.Read.Match.Matched[:1]
	forged.Read.Match.Team = "Viejo nombre"
	if err := f.repo.SavePatternMatches(f.ctx, library.PatternMatchSet{PatternID: p.ID, MatchID: m.ID, Rows: []library.PatternMatchRow{forged}}); err != nil {
		t.Fatal(err)
	}
	got, err = d.Results(f.ctx, []library.Pattern{p})
	if err != nil || got[0].Matched != 1 || got[0].Matches[0].Team != "Nosotros" {
		t.Fatalf("Results from the store = %+v, %v; want the stored round under the current name", got, err)
	}
	time.Sleep(2 * time.Millisecond)
	f.analyze()
	got, err = d.Results(f.ctx, []library.Pattern{p})
	if err != nil || got[0].Matched != 3 {
		t.Fatalf("Results after a new analysis = %+v, %v; want 3 rounds again", got, err)
	}
}

func TestReadErrorsAreNotStoredButMissingFilesAre(t *testing.T) {
	f := newFixture(t)
	f.analyze()
	p := f.pattern("A", nil)
	d := New(f.repo, f.files)
	positions := artifacts.LibraryTacticalPositionsKey(f.demo.ID)
	f.files.failing[positions] = errors.New("disk busy")
	if _, err := d.Results(f.ctx, []library.Pattern{p}); err != nil {
		t.Fatal(err)
	}
	if rows := f.rows(p); len(rows) != 0 {
		t.Fatalf("a failed positions read was stored: %+v", rows)
	}
	index := artifacts.LibraryTacticalIndexKey(f.demo.ID)
	delete(f.files.failing, positions)
	f.files.failing[index] = errors.New("disk busy")
	if got, err := d.Results(f.ctx, []library.Pattern{p}); err != nil || got[0].Unreadable != 1 || len(f.rows(p)) != 0 {
		t.Fatalf("a failed document read = %+v, %v, rows %+v; want unreadable and nothing stored", got, err, f.rows(p))
	}
	delete(f.files.failing, index)
	delete(f.files.blobs, index)
	if _, err := d.Results(f.ctx, []library.Pattern{p}); err != nil {
		t.Fatal(err)
	}
	if rows := f.rows(p); len(rows) != 1 || !rows[0].Read.Unreadable {
		t.Fatalf("a missing document = %+v; want one stored unreadable row", rows)
	}
}

func TestAPatternDeletedMidPassStoresNothing(t *testing.T) {
	f := newFixture(t)
	f.analyze()
	p := f.pattern("A", nil)
	f.files.onOpen = func(key string) {
		if key == artifacts.LibraryTacticalIndexKey(f.demo.ID) {
			if err := f.repo.DeletePattern(f.ctx, p.ID); err != nil {
				t.Error(err)
			}
		}
	}
	got, err := New(f.repo, f.files).Results(f.ctx, []library.Pattern{p})
	if err != nil || got[0].Matched != 3 {
		t.Fatalf("Results = %+v, %v; want the pass to finish", got, err)
	}
	if rows := f.rows(p); len(rows) != 0 {
		t.Fatalf("a deleted pattern got rows: %+v", rows)
	}
}

func TestMatchHitsCountOnlyThePatternsOfTheMatch(t *testing.T) {
	f := newFixture(t)
	m := f.analyze()
	var own *uuid.UUID
	for _, team := range m.Teams {
		if team.Key == "t-start" {
			own = team.TeamID
		}
	}
	other := library.Team{Name: "Otros"}
	if err := f.repo.CreateTeam(f.ctx, &other); err != nil {
		t.Fatal(err)
	}
	if own == nil {
		t.Fatal("the match's team has no id")
	}
	f.pattern("De la biblioteca", nil)
	f.pattern("Nuestro", own)
	f.pattern("De otros", &other.ID)
	hits, total, err := New(f.repo, f.files).MatchHits(f.ctx, m)
	if err != nil || total != 2 || len(hits) != 2 {
		t.Fatalf("MatchHits = %d hits of %d, %v; want 2 of 2", len(hits), total, err)
	}
	for _, hit := range hits {
		if hit.Rows[0].Read.Match.Team != "Nosotros" {
			t.Errorf("%s holds for %q, want Nosotros", hit.Pattern.Name, hit.Rows[0].Read.Match.Team)
		}
	}
}

var _ storage.Storage = (*files)(nil)
