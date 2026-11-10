package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// TestPatternCategoryPlayAndDetectionContract runs a pattern's category, its play link and its
// stored detection rows against both repositories.
func TestPatternCategoryPlayAndDetectionContract(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			f := libraryFixture{t: t, ctx: ctx, repo: repo}
			m := f.analyze(f.demo("p1", 1), "de_ancient", side{clan: "Rivales", score: 13}, side{clan: "Nosotros", score: 9})
			play := &library.Play{Title: "A Finish", Map: "de_ancient", Side: "T", Kind: "execute", Site: "A", Origin: "manual"}
			if err := repo.CreatePlay(ctx, play); err != nil {
				t.Fatal(err)
			}
			area := &tacticalplan.PatternArea{Polygon: [][2]float64{{0, 0}, {100, 0}, {0, 100}}}
			finish := &library.Pattern{Name: "A Finish", Map: "de_ancient", Category: "Ejecuciones", PlayID: &play.ID,
				Pattern: tacticalplan.Pattern{Side: tacticalplan.SideT, Blocks: []tacticalplan.PatternBlock{
					{Kind: tacticalplan.PatternPlayer, MinPlayers: 3, Area: area, To: 55},
					{Kind: tacticalplan.PatternUtility, Area: &tacticalplan.PatternArea{Callouts: []string{"CTSpawn"}}},
				}}}
			other := &library.Pattern{Name: "Rush B", Map: "de_ancient", Category: "Rush",
				Pattern: tacticalplan.Pattern{Side: tacticalplan.SideT, Blocks: []tacticalplan.PatternBlock{{Kind: tacticalplan.PatternPlant}}}}
			for _, p := range []*library.Pattern{finish, other} {
				if err := repo.CreatePattern(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
			got, err := repo.GetPattern(ctx, finish.ID)
			b := got.Pattern.Blocks
			if err != nil || got.Category != "Ejecuciones" || got.PlayID == nil || *got.PlayID != play.ID || b[0].MinPlayers != 3 ||
				len(b[0].Area.Polygon) != 3 || b[1].Area.Callouts[0] != "CTSpawn" {
				t.Fatalf("GetPattern = %+v, %v", got, err)
			}
			got.Pattern.Blocks[0].Area.Polygon[0][0] = 999
			if again, _ := repo.GetPattern(ctx, finish.ID); again.Pattern.Blocks[0].Area.Polygon[0][0] != 0 {
				t.Fatal("a read shares its polygon with the store")
			}
			for _, tc := range []struct {
				filter library.PatternFilter
				want   string
			}{{library.PatternFilter{Category: "Rush"}, "Rush B"}, {library.PatternFilter{PlayID: &play.ID}, "A Finish"}} {
				list, err := repo.ListPatterns(ctx, tc.filter)
				if err != nil || len(list) != 1 || list[0].Name != tc.want {
					t.Fatalf("ListPatterns(%+v) = %+v, %v", tc.filter, list, err)
				}
			}
			category := ""
			patched, err := repo.UpdatePattern(ctx, finish.ID, library.PatternPatch{Category: &category, ClearPlay: true})
			if err != nil || patched.Category != "" || patched.PlayID != nil {
				t.Fatalf("UpdatePattern(clear) = %+v, %v", patched, err)
			}
			if patched, err = repo.UpdatePattern(ctx, finish.ID, library.PatternPatch{PlayID: &play.ID}); err != nil || patched.PlayID == nil {
				t.Fatalf("UpdatePattern(play) = %+v, %v", patched, err)
			}

			row := func(team string, rounds ...int) library.PatternMatchRow {
				read := tacticalplan.PatternMatchRead{Match: tacticalplan.PatternMatch{MatchID: m.ID, TeamKey: team, Rounds: 12, Matched: []tacticalplan.PatternRound{}},
					Blocks: []tacticalplan.PatternCount{{Matched: len(rounds), Rounds: 12}}}
				for _, n := range rounds {
					read.Match.Matched = append(read.Match.Matched, tacticalplan.PatternRound{Round: n, Side: tacticalplan.SideT, Won: n%2 == 0})
				}
				return library.PatternMatchRow{TeamKey: team, Revision: library.PatternRevision(*finish), MatchUpdatedAt: m.UpdatedAt, PlayedAt: m.PlayedAt, Read: read}
			}
			set := library.PatternMatchSet{PatternID: finish.ID, MatchID: m.ID, Rows: []library.PatternMatchRow{row("t-start", 3, 4), row("ct-start")}}
			if err := repo.SavePatternMatches(ctx, set); err != nil {
				t.Fatal(err)
			}
			if err := repo.SavePatternMatches(ctx, library.PatternMatchSet{PatternID: other.ID, MatchID: m.ID, Rows: []library.PatternMatchRow{row("t-start")}}); err != nil {
				t.Fatal(err)
			}
			rows, err := repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{finish.ID}})
			if err != nil || len(rows) != 2 || rows[0].TeamKey != "ct-start" || rows[1].Matched() != 2 || rows[1].Read.Match.Matched[1].Round != 4 ||
				rows[1].MatchUpdatedAt.UnixMilli() != m.UpdatedAt.UnixMilli() || rows[1].Revision != library.PatternRevision(*finish) {
				t.Fatalf("ListPatternMatches(pattern) = %+v, %v", rows, err)
			}
			hits, err := repo.ListPatternMatches(ctx, library.PatternMatchFilter{MatchIDs: []uuid.UUID{m.ID}, MatchedOnly: true})
			if err != nil || len(hits) != 1 || hits[0].PatternID != finish.ID || hits[0].TeamKey != "t-start" {
				t.Fatalf("ListPatternMatches(match, matched) = %+v, %v", hits, err)
			}
			// A second save replaces the pattern's rows over the match.
			set.Rows = []library.PatternMatchRow{row("t-start", 7)}
			if err := repo.SavePatternMatches(ctx, set); err != nil {
				t.Fatal(err)
			}
			if rows, _ := repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{finish.ID}}); len(rows) != 1 || rows[0].Read.Match.Matched[0].Round != 7 {
				t.Fatalf("after replace = %+v", rows)
			}
			if err := repo.SavePatternMatches(ctx, library.PatternMatchSet{PatternID: uuid.New(), MatchID: m.ID}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("SavePatternMatches(missing pattern) = %v, want ErrNotFound", err)
			}
			if err := repo.SavePatternMatches(ctx, library.PatternMatchSet{PatternID: finish.ID, MatchID: uuid.New(), Rows: []library.PatternMatchRow{row("t-start")}}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("SavePatternMatches(missing match) = %v, want ErrNotFound", err)
			}
			// A newer match lists first, and both filters together keep their crossing only.
			newer := f.analyze(f.demo("p2", 5), "de_ancient", side{clan: "Rivales", score: 13}, side{clan: "Nosotros", score: 4})
			late := row("t-start", 2)
			late.PlayedAt = newer.PlayedAt
			for _, pid := range []uuid.UUID{finish.ID, other.ID} {
				if err := repo.SavePatternMatches(ctx, library.PatternMatchSet{PatternID: pid, MatchID: newer.ID, Rows: []library.PatternMatchRow{late}}); err != nil {
					t.Fatal(err)
				}
			}
			ordered, err := repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{finish.ID}})
			if err != nil || len(ordered) != 2 || ordered[0].MatchID != newer.ID || ordered[1].MatchID != m.ID {
				t.Fatalf("ListPatternMatches(newest first) = %+v, %v", ordered, err)
			}
			both, err := repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{other.ID}, MatchIDs: []uuid.UUID{newer.ID}})
			if err != nil || len(both) != 1 || both[0].PatternID != other.ID || both[0].MatchID != newer.ID {
				t.Fatalf("ListPatternMatches(pattern and match) = %+v, %v", both, err)
			}
			many, err := repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{finish.ID, other.ID}, MatchIDs: []uuid.UUID{m.ID, newer.ID}})
			if err != nil || len(many) != 4 {
				t.Fatalf("ListPatternMatches(lists) = %+v, %v", many, err)
			}
			if err := repo.DeleteDemo(ctx, newer.DemoID); err != nil {
				t.Fatal(err)
			}

			missing := uuid.New()
			if err := repo.CreatePattern(ctx, &library.Pattern{Name: "Sin jugada", Map: "de_ancient", PlayID: &missing, Pattern: other.Pattern}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("CreatePattern(missing play) = %v, want ErrNotFound", err)
			}
			if _, err := repo.UpdatePattern(ctx, other.ID, library.PatternPatch{PlayID: &missing}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("UpdatePattern(missing play) = %v, want ErrNotFound", err)
			}

			if err := repo.DeletePlay(ctx, play.ID); err != nil {
				t.Fatal(err)
			}
			if got, _ := repo.GetPattern(ctx, finish.ID); got.PlayID != nil {
				t.Fatalf("a deleted play stays linked: %+v", got.PlayID)
			}
			if err := repo.DeletePattern(ctx, finish.ID); err != nil {
				t.Fatal(err)
			}
			if rows, _ := repo.ListPatternMatches(ctx, library.PatternMatchFilter{MatchIDs: []uuid.UUID{m.ID}}); len(rows) != 1 || rows[0].PatternID != other.ID {
				t.Fatalf("a deleted pattern keeps its rows: %+v", rows)
			}
			if err := repo.DeleteDemo(ctx, m.DemoID); err != nil {
				t.Fatal(err)
			}
			if rows, _ := repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{other.ID}}); len(rows) != 0 {
				t.Fatalf("a deleted match keeps its rows: %+v", rows)
			}
		})
	}
}
