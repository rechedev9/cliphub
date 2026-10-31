package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// The library contract runs against both repositories, like the job contract:
// a difference between memory and SQLite is a bug no handler test can see.
func libraryRepositoriesUnderTest(t *testing.T) map[string]LibraryRepository {
	t.Helper()
	return map[string]LibraryRepository{
		"memory": NewMemoryLibraryRepository(),
		"sqlite": NewSQLiteLibraryRepository(newTestSQLiteRepo(t).db),
	}
}

var libraryEpoch = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

type libraryFixture struct {
	t    *testing.T
	ctx  context.Context
	repo LibraryRepository
}

// demo imports a demo with the given hash and an explicit match date, days
// after libraryEpoch, so ordering by date is deterministic.
func (f libraryFixture) demo(sha string, day int, tags ...string) library.Demo {
	f.t.Helper()
	played := libraryEpoch.AddDate(0, 0, day)
	d := library.Demo{SHA256: sha, FileName: sha + ".dem", Path: "library/" + sha, PlayedAt: &played, Tags: tags}
	created, err := f.repo.CreateDemo(f.ctx, &d)
	if err != nil || !created {
		f.t.Fatalf("CreateDemo(%s) = %v, %v; want created", sha, created, err)
	}
	return d
}

type side struct {
	clan    string
	score   int
	players []string
}

func (f libraryFixture) analyze(d library.Demo, mapName string, ct, tside side) library.Match {
	f.t.Helper()
	analysis := library.Analysis{
		DemoID: d.ID, Source: library.SourceFaceit, CS2Build: 14100, PatchVersion: 1,
		Map: mapName, Tickrate: 64, DurationTicks: 100000, RoundCount: ct.score + tside.score,
		Teams: []library.AnalysisTeam{
			{Key: "ct-start", StartSide: "CT", ClanName: ct.clan, Score: ct.score},
			{Key: "t-start", StartSide: "T", ClanName: tside.clan, Score: tside.score},
		},
	}
	slot := uint8(0)
	for _, entry := range []struct {
		key string
		s   side
	}{{"ct-start", ct}, {"t-start", tside}} {
		for _, p := range entry.s.players {
			analysis.Players = append(analysis.Players, library.MatchPlayer{
				SteamID64: p, Name: "nick-" + p, TeamKey: entry.key, Slot: slot, Rounds: analysis.RoundCount, Kills: 1,
			})
			slot++
		}
	}
	m, err := f.repo.SaveAnalysis(f.ctx, analysis)
	if err != nil {
		f.t.Fatalf("SaveAnalysis: %v", err)
	}
	return m
}

func teamOf(t *testing.T, m library.Match, key string) library.MatchTeam {
	t.Helper()
	for _, team := range m.Teams {
		if team.Key == key {
			return team
		}
	}
	t.Fatalf("match %s has no side %s: %+v", m.ID, key, m.Teams)
	return library.MatchTeam{}
}

func matchIDs(ms []library.Match) []uuid.UUID {
	out := make([]uuid.UUID, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}

func TestLibraryRepositoryContract(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, f libraryFixture)
	}{
		{
			name: "CreateDemo deduplicates by hash and normalizes tags",
			run: func(t *testing.T, f libraryFixture) {
				first := f.demo("aaa", 0, " Scrim ", "scrim", "Pistolas  B")
				if got := first.Tags; len(got) != 2 || got[0] != "Pistolas B" || got[1] != "Scrim" {
					t.Fatalf("tags = %q, want [Pistolas B Scrim]", got)
				}
				if first.State != library.ParseQueued || first.Source != library.SourceUnknown || first.MatchID != nil {
					t.Fatalf("new demo = %+v, want queued, unknown source, no match", first)
				}
				again := library.Demo{SHA256: "aaa", FileName: "copy.dem"}
				created, err := f.repo.CreateDemo(f.ctx, &again)
				if err != nil || created {
					t.Fatalf("second CreateDemo = %v, %v; want existing demo", created, err)
				}
				if again.ID != first.ID || again.FileName != "aaa.dem" {
					t.Fatalf("duplicate import returned %+v, want the first demo %s", again, first.ID)
				}
				listed, err := f.repo.ListDemos(f.ctx, library.DemoFilter{Tag: "SCRIM"})
				if err != nil || len(listed) != 1 || listed[0].ID != first.ID {
					t.Fatalf("ListDemos(tag) = %v, %v", listed, err)
				}
			},
		},
		{
			name: "LinkDemoJob links Crear jobs to a demo until the demo is deleted",
			run: func(t *testing.T, f libraryFixture) {
				d := f.demo("jobs", 0)
				first, second := uuid.MustParse("00000000-0000-0000-0000-0000000000b2"), uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
				for _, jobID := range []uuid.UUID{first, second, first} {
					if err := f.repo.LinkDemoJob(f.ctx, d.ID, jobID); err != nil {
						t.Fatalf("LinkDemoJob(%s) = %v", jobID, err)
					}
				}
				got, err := f.repo.DemoForJob(f.ctx, second)
				if err != nil || got.ID != d.ID {
					t.Fatalf("DemoForJob = %+v, %v; want demo %s", got, err, d.ID)
				}
				if len(got.JobIDs) != 2 || !containsUUID(got.JobIDs, first) || !containsUUID(got.JobIDs, second) {
					t.Fatalf("job ids = %v, want both jobs once", got.JobIDs)
				}
				if err := f.repo.LinkDemoJob(f.ctx, uuid.New(), first); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("LinkDemoJob(missing demo) = %v, want ErrNotFound", err)
				}
				if err := f.repo.DeleteDemo(f.ctx, d.ID); err != nil {
					t.Fatalf("DeleteDemo: %v", err)
				}
				if _, err := f.repo.DemoForJob(f.ctx, first); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("DemoForJob after delete = %v, want ErrNotFound", err)
				}
				fresh := f.demo("jobs-again", 1)
				if fresh.JobIDs == nil || len(fresh.JobIDs) != 0 {
					t.Fatalf("new demo job ids = %v, want empty", fresh.JobIDs)
				}
			},
		},
		{
			name: "SaveAnalysis creates the match and assigns teams by clan name",
			run: func(t *testing.T, f libraryFixture) {
				d1 := f.demo("d1", 0)
				m1 := f.analyze(d1, "de_mirage", side{"Vitality", 13, []string{"1", "2"}}, side{"", 7, []string{"3"}})
				vit := teamOf(t, m1, "ct-start")
				if vit.TeamID == nil || vit.TeamName != "Vitality" || vit.Assignment != library.AssignmentAuto || vit.Score != 13 {
					t.Fatalf("ct side = %+v, want auto Vitality with 13", vit)
				}
				if none := teamOf(t, m1, "t-start"); none.TeamID != nil || none.Assignment != library.AssignmentNone {
					t.Fatalf("side without clan = %+v, want unassigned", none)
				}
				if !m1.PlayedAt.Equal(libraryEpoch) || m1.Source != library.SourceFaceit || m1.RoundCount != 20 || len(m1.Players) != 3 {
					t.Fatalf("match = %+v", m1)
				}
				d2 := f.demo("d2", 1)
				m2 := f.analyze(d2, "de_nuke", side{"team  vitality", 5, []string{"4"}}, side{"MOUZ", 13, []string{"1"}})
				if got := teamOf(t, m2, "ct-start"); got.TeamID == nil || *got.TeamID != *vit.TeamID {
					t.Fatalf("alias spelling did not resolve to Vitality: %+v", got)
				}
				demo, err := f.repo.GetDemo(f.ctx, d2.ID)
				if err != nil || demo.State != library.ParseReady || demo.MatchID == nil || *demo.MatchID != m2.ID || demo.CS2Build != 14100 {
					t.Fatalf("demo after analysis = %+v, %v", demo, err)
				}
				teams, err := f.repo.ListTeams(f.ctx, library.TeamFilter{})
				if err != nil || len(teams) != 2 || teams[0].Name != "MOUZ" || teams[1].Name != "Vitality" || teams[1].MatchCount != 2 {
					t.Fatalf("ListTeams = %+v, %v", teams, err)
				}
			},
		},
		{
			name: "re-analysis keeps the match id, series, date and manual teams",
			run: func(t *testing.T, f libraryFixture) {
				d := f.demo("d", 0)
				m := f.analyze(d, "de_inferno", side{"FaZe", 13, []string{"1"}}, side{"G2", 10, []string{"2"}})
				navi := &library.Team{Name: "NAVI"}
				if err := f.repo.CreateTeam(f.ctx, navi); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: m.ID, TeamKey: "t-start", TeamID: &navi.ID}); err != nil {
					t.Fatal(err)
				}
				s := &library.Series{Format: library.FormatBO3, Event: "Major"}
				if err := f.repo.CreateSeries(f.ctx, s); err != nil {
					t.Fatal(err)
				}
				moved := libraryEpoch.Add(48 * time.Hour)
				if _, err := f.repo.UpdateMatch(f.ctx, m.ID, library.MatchPatch{SeriesID: &s.ID, PlayedAt: &moved}); err != nil {
					t.Fatal(err)
				}
				again := f.analyze(d, "de_inferno", side{"FaZe", 13, []string{"1"}}, side{"G2", 10, []string{"2"}})
				manual := teamOf(t, again, "t-start")
				if again.ID != m.ID || again.SeriesID == nil || *again.SeriesID != s.ID || !again.PlayedAt.Equal(moved) {
					t.Fatalf("re-analysis lost user edits: %+v", again)
				}
				if manual.TeamID == nil || *manual.TeamID != navi.ID || manual.Assignment != library.AssignmentManual || manual.ClanName != "G2" {
					t.Fatalf("manual side = %+v, want NAVI kept", manual)
				}
				series, err := f.repo.GetSeries(f.ctx, s.ID)
				if err != nil || len(series.MatchIDs) != 1 || series.MatchIDs[0] != m.ID {
					t.Fatalf("series = %+v, %v", series, err)
				}
			},
		},
		{
			name: "re-analysis never puts a manual side's team on the other side",
			run: func(t *testing.T, f libraryFixture) {
				d := f.demo("d", 0)
				m := f.analyze(d, "de_nuke", side{"Foo", 13, []string{"1"}}, side{"Bar", 10, []string{"2"}})
				x := &library.Team{Name: "X"}
				if err := f.repo.CreateTeam(f.ctx, x); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: m.ID, TeamKey: "t-start", TeamID: &x.ID}); err != nil {
					t.Fatal(err)
				}
				// Merging Foo into X moves Foo's alias to X while the t side holds X by hand.
				if _, err := f.repo.MergeTeams(f.ctx, *teamOf(t, m, "ct-start").TeamID, x.ID); err != nil {
					t.Fatal(err)
				}
				again := f.analyze(d, "de_nuke", side{"Foo", 13, []string{"1"}}, side{"Bar", 10, []string{"2"}})
				if ct := teamOf(t, again, "ct-start"); ct.TeamID != nil && *ct.TeamID == x.ID {
					t.Fatalf("ct side = %+v, want X only on the manual t side", ct)
				}
				if tt := teamOf(t, again, "t-start"); tt.TeamID == nil || *tt.TeamID != x.ID || tt.Assignment != library.AssignmentManual {
					t.Fatalf("t side = %+v, want manual X", tt)
				}
			},
		},
		{
			name: "AssignMatchTeam remembers the alias and refuses one team on both sides",
			run: func(t *testing.T, f libraryFixture) {
				m1 := f.analyze(f.demo("a", 0), "de_mirage", side{"team_apEX", 13, nil}, side{"Spirit", 3, nil})
				m2 := f.analyze(f.demo("b", 1), "de_mirage", side{"team_apex", 13, nil}, side{"Liquid", 3, nil})
				vit := &library.Team{Name: "Vitality"}
				if err := f.repo.CreateTeam(f.ctx, vit); err != nil {
					t.Fatal(err)
				}
				fixed, err := f.repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: m1.ID, TeamKey: "ct-start", TeamID: &vit.ID, RememberAlias: true})
				if err != nil || teamOf(t, fixed, "ct-start").Assignment != library.AssignmentManual {
					t.Fatalf("AssignMatchTeam = %+v, %v", fixed, err)
				}
				other, err := f.repo.GetMatch(f.ctx, m2.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got := teamOf(t, other, "ct-start"); got.TeamID == nil || *got.TeamID != vit.ID || got.Assignment != library.AssignmentAuto {
					t.Fatalf("auto side with the remembered clan = %+v, want Vitality", got)
				}
				spirit := teamOf(t, fixed, "t-start").TeamID
				if _, err := f.repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: m1.ID, TeamKey: "t-start", TeamID: &vit.ID}); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("same team on both sides = %v, want ErrConflict", err)
				}
				if _, err := f.repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: m1.ID, TeamKey: "mid", TeamID: spirit}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("unknown side = %v, want ErrNotFound", err)
				}
				team, err := f.repo.GetTeam(f.ctx, vit.ID)
				if err != nil || len(team.Aliases) != 2 || team.Aliases[0] != "team_apEX" || team.Aliases[1] != "Vitality" || team.MatchCount != 2 {
					t.Fatalf("team = %+v, %v", team, err)
				}
				// The team the import made for "team_apEX" lost its only alias and
				// its matches, so it is gone rather than left empty in the list.
				if _, err := f.repo.GetTeam(f.ctx, *teamOf(t, m1, "ct-start").TeamID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("emptied auto team = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "ListMatches filters and orders newest first",
			run: func(t *testing.T, f libraryFixture) {
				a := f.analyze(f.demo("a", 0, "scrim"), "de_mirage", side{"Vitality", 13, []string{"1"}}, side{"MOUZ", 5, []string{"2"}})
				b := f.analyze(f.demo("b", 2), "de_nuke", side{"Vitality", 13, []string{"1"}}, side{"FaZe", 5, []string{"3"}})
				c := f.analyze(f.demo("c", 4), "de_mirage", side{"G2", 13, []string{"4"}}, side{"FaZe", 5, []string{"3"}})
				vit := teamOf(t, a, "ct-start").TeamID
				from, to := libraryEpoch.AddDate(0, 0, 1), libraryEpoch.AddDate(0, 0, 4)
				for name, tc := range map[string]struct {
					filter library.MatchFilter
					want   []uuid.UUID
				}{
					"all":       {library.MatchFilter{}, []uuid.UUID{c.ID, b.ID, a.ID}},
					"map":       {library.MatchFilter{Map: "de_mirage"}, []uuid.UUID{c.ID, a.ID}},
					"team":      {library.MatchFilter{TeamID: vit}, []uuid.UUID{b.ID, a.ID}},
					"player":    {library.MatchFilter{SteamID: "3"}, []uuid.UUID{c.ID, b.ID}},
					"tag":       {library.MatchFilter{Tag: "Scrim"}, []uuid.UUID{a.ID}},
					"source":    {library.MatchFilter{Source: library.SourceValve}, []uuid.UUID{}},
					"dates":     {library.MatchFilter{From: &from, To: &to}, []uuid.UUID{c.ID, b.ID}},
					"page":      {library.MatchFilter{Limit: 1, Offset: 1}, []uuid.UUID{b.ID}},
					"past page": {library.MatchFilter{Offset: 9}, []uuid.UUID{}},
				} {
					got, err := f.repo.ListMatches(f.ctx, tc.filter)
					if err != nil {
						t.Fatalf("%s: %v", name, err)
					}
					if ids := matchIDs(got); len(ids) != len(tc.want) {
						t.Fatalf("%s: ids = %v, want %v", name, ids, tc.want)
					} else {
						for i := range ids {
							if ids[i] != tc.want[i] {
								t.Fatalf("%s: ids = %v, want %v", name, ids, tc.want)
							}
						}
					}
				}
				listed, _ := f.repo.ListMatches(f.ctx, library.MatchFilter{Limit: 1})
				if len(listed[0].Teams) != 2 || listed[0].Players != nil {
					t.Fatalf("list row = %+v, want teams and no players", listed[0])
				}
			},
		},
		{
			name: "rosters, nick history and team history derive from the matches",
			run: func(t *testing.T, f libraryFixture) {
				a := f.analyze(f.demo("a", 0), "de_mirage", side{"Vitality", 13, []string{"1", "2"}}, side{"MOUZ", 5, []string{"3"}})
				f.analyze(f.demo("b", 3), "de_nuke", side{"MOUZ", 13, []string{"1", "3"}}, side{"FaZe", 5, []string{"4"}})
				vit := *teamOf(t, a, "ct-start").TeamID
				mouz := *teamOf(t, a, "t-start").TeamID
				team, err := f.repo.GetTeam(f.ctx, mouz)
				if err != nil || len(team.Roster) != 2 {
					t.Fatalf("roster = %+v, %v", team.Roster, err)
				}
				if team.Roster[0].SteamID64 != "1" || !team.Roster[0].FirstPlayedAt.Equal(libraryEpoch.AddDate(0, 0, 3)) {
					t.Fatalf("roster[0] = %+v, want player 1 joined on day 3", team.Roster[0])
				}
				if team.Roster[1].SteamID64 != "3" || team.Roster[1].Matches != 2 || !team.Roster[1].FirstPlayedAt.Equal(libraryEpoch) {
					t.Fatalf("roster[1] = %+v, want player 3 with 2 matches since day 0", team.Roster[1])
				}
				player, err := f.repo.GetPlayer(f.ctx, "1")
				if err != nil || player.MatchCount != 2 || len(player.Teams) != 2 || player.Teams[0].TeamID != mouz || player.Teams[1].TeamID != vit {
					t.Fatalf("player = %+v, %v", player, err)
				}
				if len(player.Nicks) != 1 || player.Nick != "nick-1" {
					t.Fatalf("nicks = %+v nick = %q", player.Nicks, player.Nick)
				}
				onMouz, err := f.repo.ListPlayers(f.ctx, library.PlayerFilter{TeamID: &mouz})
				if err != nil || len(onMouz) != 2 || onMouz[0].SteamID64 != "1" || onMouz[1].SteamID64 != "3" {
					t.Fatalf("ListPlayers(team) = %+v, %v", onMouz, err)
				}
				byNick, err := f.repo.ListPlayers(f.ctx, library.PlayerFilter{Query: "NICK-4"})
				if err != nil || len(byNick) != 1 || byNick[0].SteamID64 != "4" {
					t.Fatalf("ListPlayers(query) = %+v, %v", byNick, err)
				}
				found, err := f.repo.ListTeams(f.ctx, library.TeamFilter{Query: "vita"})
				if err != nil || len(found) != 1 || found[0].ID != vit {
					t.Fatalf("ListTeams(query) = %+v, %v", found, err)
				}
			},
		},
		{
			name: "the current nick follows the most recent match",
			run: func(t *testing.T, f libraryFixture) {
				older := f.demo("old", 0)
				newer := f.demo("new", 5)
				for _, step := range []struct {
					d    library.Demo
					nick string
				}{{newer, "ZywOo"}, {older, "zywoo_old"}} {
					if _, err := f.repo.SaveAnalysis(f.ctx, library.Analysis{DemoID: step.d.ID, Map: "de_mirage",
						Players: []library.MatchPlayer{{SteamID64: "7", Name: step.nick, TeamKey: "ct-start"}}}); err != nil {
						t.Fatal(err)
					}
				}
				if p, _ := f.repo.GetPlayer(f.ctx, "7"); p.Nick != "ZywOo" || len(p.Nicks) != 2 || p.Nicks[0].Nick != "ZywOo" {
					t.Fatalf("player = %+v, want ZywOo from the newer match", p)
				}
				newerMatch, _ := f.repo.GetDemo(f.ctx, newer.ID)
				early := libraryEpoch.AddDate(0, 0, -1)
				if _, err := f.repo.UpdateMatch(f.ctx, *newerMatch.MatchID, library.MatchPatch{PlayedAt: &early}); err != nil {
					t.Fatal(err)
				}
				if p, _ := f.repo.GetPlayer(f.ctx, "7"); p.Nick != "zywoo_old" {
					t.Fatalf("nick after moving the match back = %q, want zywoo_old", p.Nick)
				}
				if err := f.repo.DeleteDemo(f.ctx, older.ID); err != nil {
					t.Fatal(err)
				}
				if p, _ := f.repo.GetPlayer(f.ctx, "7"); p.Nick != "ZywOo" || p.MatchCount != 1 {
					t.Fatalf("player after delete = %+v", p)
				}
			},
		},
		{
			name: "team aliases are unique and MergeTeams folds one team into another",
			run: func(t *testing.T, f libraryFixture) {
				a := f.analyze(f.demo("a", 0), "de_mirage", side{"team_zywoo", 13, []string{"1"}}, side{"MOUZ", 5, []string{"2"}})
				b := f.analyze(f.demo("b", 1), "de_mirage", side{"Vitality", 13, []string{"1"}}, side{"team_zywoo", 5, []string{"3"}})
				auto := *teamOf(t, a, "ct-start").TeamID
				vit := *teamOf(t, b, "ct-start").TeamID
				dup := &library.Team{Name: "Otro", Aliases: []string{"MOUZ"}}
				if err := f.repo.CreateTeam(f.ctx, dup); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("CreateTeam with a taken alias = %v, want ErrConflict", err)
				}
				aliases := []string{"Vitality", "team_zywoo"}
				if _, err := f.repo.UpdateTeam(f.ctx, vit, library.TeamPatch{Aliases: &aliases}); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("UpdateTeam with another team's alias = %v, want ErrConflict", err)
				}
				merged, err := f.repo.MergeTeams(f.ctx, auto, vit)
				if err != nil {
					t.Fatal(err)
				}
				if merged.MatchCount != 2 || len(merged.Aliases) != 2 {
					t.Fatalf("merged = %+v, want 2 matches and both aliases", merged)
				}
				if _, err := f.repo.GetTeam(f.ctx, auto); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("merged-away team = %v, want ErrNotFound", err)
				}
				head, _ := f.repo.GetMatch(f.ctx, b.ID)
				if got := teamOf(t, head, "t-start"); got.TeamID != nil || got.Assignment != library.AssignmentNone {
					t.Fatalf("head to head side after merge = %+v, want cleared", got)
				}
				name := "Team Vitality"
				renamed, err := f.repo.UpdateTeam(f.ctx, vit, library.TeamPatch{Name: &name})
				if err != nil || renamed.Name != name {
					t.Fatalf("rename = %+v, %v", renamed, err)
				}
			},
		},
		{
			name: "manual source survives analysis and state changes clear stale errors",
			run: func(t *testing.T, f libraryFixture) {
				d := f.demo("s", 0)
				scrim := library.SourceScrim
				tags := []string{"b", "A"}
				updated, err := f.repo.UpdateDemo(f.ctx, d.ID, library.DemoPatch{Source: &scrim, Tags: &tags})
				if err != nil || updated.Source != scrim || !updated.SourceManual || updated.Tags[0] != "A" {
					t.Fatalf("UpdateDemo = %+v, %v", updated, err)
				}
				if err := f.repo.SetDemoState(f.ctx, d.ID, library.ParseFailed, "demo_unreadable", "boom"); err != nil {
					t.Fatal(err)
				}
				failed, _ := f.repo.GetDemo(f.ctx, d.ID)
				if failed.State != library.ParseFailed || failed.ErrorCode != "demo_unreadable" || failed.Error != "boom" {
					t.Fatalf("failed demo = %+v", failed)
				}
				if err := f.repo.SetDemoState(f.ctx, d.ID, library.ParseQueued, "x", "y"); err != nil {
					t.Fatal(err)
				}
				queued, _ := f.repo.ListDemos(f.ctx, library.DemoFilter{States: []library.ParseState{library.ParseQueued}})
				if len(queued) != 1 || queued[0].Error != "" || queued[0].ErrorCode != "" {
					t.Fatalf("queued demos = %+v", queued)
				}
				m := f.analyze(d, "de_mirage", side{"A", 1, nil}, side{"B", 0, nil})
				if m.Source != library.SourceScrim {
					t.Fatalf("source after analysis = %q, want the manual scrim", m.Source)
				}
			},
		},
		{
			name: "missing records report ErrNotFound and DeleteDemo is idempotent",
			run: func(t *testing.T, f libraryFixture) {
				missing := uuid.New()
				if _, err := f.repo.GetDemo(f.ctx, missing); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetDemo = %v", err)
				}
				if err := f.repo.SetDemoState(f.ctx, missing, library.ParseParsing, "", ""); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("SetDemoState = %v", err)
				}
				if _, err := f.repo.SaveAnalysis(f.ctx, library.Analysis{DemoID: missing}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("SaveAnalysis = %v", err)
				}
				if _, err := f.repo.GetMatch(f.ctx, missing); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetMatch = %v", err)
				}
				if _, err := f.repo.GetPlayer(f.ctx, "404"); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetPlayer = %v", err)
				}
				if _, err := f.repo.SetPlayerFaceit(f.ctx, "404", "x", "y"); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("SetPlayerFaceit = %v", err)
				}
				if _, err := f.repo.GetSeries(f.ctx, missing); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetSeries = %v", err)
				}
				if err := f.repo.DeleteDemo(f.ctx, missing); err != nil {
					t.Fatalf("DeleteDemo(missing) = %v", err)
				}
				d := f.demo("x", 0)
				m := f.analyze(d, "de_mirage", side{"A", 1, []string{"1"}}, side{"B", 0, nil})
				if _, err := f.repo.UpdateMatch(f.ctx, m.ID, library.MatchPatch{SeriesID: &missing}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdateMatch(unknown series) = %v", err)
				}
				p, err := f.repo.SetPlayerFaceit(f.ctx, "1", "faceit-1", "one")
				if err != nil || p.FaceitPlayerID != "faceit-1" || p.FaceitNickname != "one" {
					t.Fatalf("SetPlayerFaceit = %+v, %v", p, err)
				}
				if err := f.repo.DeleteDemo(f.ctx, d.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetMatch(f.ctx, m.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("match after DeleteDemo = %v, want ErrNotFound", err)
				}
				if all, _ := f.repo.ListMatches(f.ctx, library.MatchFilter{}); len(all) != 0 {
					t.Fatalf("matches after delete = %d", len(all))
				}
			},
		},
		{
			name: "ListSeries orders newest first with match ids by date",
			run: func(t *testing.T, f libraryFixture) {
				first := &library.Series{Format: library.FormatBO1}
				if err := f.repo.CreateSeries(f.ctx, first); err != nil {
					t.Fatal(err)
				}
				time.Sleep(2 * time.Millisecond)
				second := &library.Series{Format: library.FormatBO3, Event: "IEM", Veto: "VIT ban nuke"}
				if err := f.repo.CreateSeries(f.ctx, second); err != nil {
					t.Fatal(err)
				}
				late := f.analyze(f.demo("late", 3), "de_nuke", side{"A", 1, nil}, side{"B", 0, nil})
				early := f.analyze(f.demo("early", 1), "de_mirage", side{"A", 1, nil}, side{"B", 0, nil})
				for _, m := range []library.Match{late, early} {
					if _, err := f.repo.UpdateMatch(f.ctx, m.ID, library.MatchPatch{SeriesID: &second.ID}); err != nil {
						t.Fatal(err)
					}
				}
				list, err := f.repo.ListSeries(f.ctx, library.SeriesFilter{})
				if err != nil || len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
					t.Fatalf("ListSeries = %+v, %v", list, err)
				}
				if ids := list[0].MatchIDs; len(ids) != 2 || ids[0] != early.ID || ids[1] != late.ID {
					t.Fatalf("series match ids = %v, want early then late", ids)
				}
				if _, err := f.repo.UpdateMatch(f.ctx, late.ID, library.MatchPatch{ClearSeries: true}); err != nil {
					t.Fatal(err)
				}
				if s, _ := f.repo.GetSeries(f.ctx, second.ID); len(s.MatchIDs) != 1 || s.Veto != "VIT ban nuke" {
					t.Fatalf("series after detach = %+v", s)
				}
			},
		},
		{
			name: "series keep their teams and veto, filter by team, follow a merge and detach their matches on delete",
			run: func(t *testing.T, f libraryFixture) {
				m := f.analyze(f.demo("veto", 0), "de_mirage", side{"Vitality", 13, nil}, side{"Spirit", 7, nil})
				vit, spirit := *teamOf(t, m, "ct-start").TeamID, *teamOf(t, m, "t-start").TeamID
				if err := f.repo.CreateSeries(f.ctx, &library.Series{Format: library.FormatBO3, TeamIDs: []uuid.UUID{vit, uuid.New()}}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("CreateSeries with a missing team = %v, want ErrNotFound", err)
				}
				steps := []library.VetoStep{{Team: "a", Action: library.VetoBan, Map: "de_nuke"}, {Team: "b", Action: library.VetoPick, Map: "de_mirage"}}
				s := &library.Series{Format: library.FormatBO3, Event: "Major", TeamIDs: []uuid.UUID{vit, spirit}, Vetoes: steps}
				if err := f.repo.CreateSeries(f.ctx, s); err != nil {
					t.Fatal(err)
				}
				if len(s.Teams) != 2 || s.Teams[0].Slot != "a" || s.Teams[0].TeamID != vit || s.Teams[1].Name != "Spirit" || len(s.Vetoes) != 2 || s.Vetoes[1].Map != "de_mirage" {
					t.Fatalf("created series = %+v", s)
				}
				if _, err := f.repo.UpdateMatch(f.ctx, m.ID, library.MatchPatch{SeriesID: &s.ID}); err != nil {
					t.Fatal(err)
				}
				other := &library.Series{Format: library.FormatBO1}
				if err := f.repo.CreateSeries(f.ctx, other); err != nil {
					t.Fatal(err)
				}
				if list, err := f.repo.ListSeries(f.ctx, library.SeriesFilter{TeamID: &spirit}); err != nil || len(list) != 1 || list[0].ID != s.ID {
					t.Fatalf("ListSeries(spirit) = %+v, %v", list, err)
				}
				edited := *s
				edited.Event, edited.Vetoes = "IEM", steps[:1]
				updated, err := f.repo.UpdateSeries(f.ctx, s.ID, edited)
				if err != nil || updated.Event != "IEM" || len(updated.Vetoes) != 1 || len(updated.MatchIDs) != 1 {
					t.Fatalf("UpdateSeries = %+v, %v", updated, err)
				}
				if _, err := f.repo.UpdateSeries(f.ctx, uuid.New(), edited); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdateSeries(missing) = %v", err)
				}
				fresh := &library.Team{Name: "Spirit Academy"}
				if err := f.repo.CreateTeam(f.ctx, fresh); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.MergeTeams(f.ctx, spirit, fresh.ID); err != nil {
					t.Fatal(err)
				}
				if got, _ := f.repo.GetSeries(f.ctx, s.ID); len(got.Teams) != 2 || got.Teams[1].TeamID != fresh.ID || got.Teams[1].Name != "Spirit Academy" {
					t.Fatalf("series after merge = %+v, want slot b on the merged team", got.Teams)
				}
				if _, err := f.repo.MergeTeams(f.ctx, fresh.ID, vit); err != nil {
					t.Fatal(err)
				}
				if got, _ := f.repo.GetSeries(f.ctx, s.ID); len(got.Teams) != 1 || got.Teams[0].Slot != "a" || got.Teams[0].TeamID != vit {
					t.Fatalf("series between merged teams = %+v, want only slot a", got.Teams)
				}
				if err := f.repo.DeleteSeries(f.ctx, s.ID); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.DeleteSeries(f.ctx, s.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("DeleteSeries twice = %v", err)
				}
				if again, err := f.repo.GetMatch(f.ctx, m.ID); err != nil || again.SeriesID != nil {
					t.Fatalf("match after series delete = %+v, %v", again.SeriesID, err)
				}
			},
		},
		{
			name: "reports keep their matches and follow their team through a merge and an alias move",
			run: func(t *testing.T, f libraryFixture) {
				a := f.analyze(f.demo("a", 0), "de_mirage", side{"team_apEX", 13, []string{"1"}}, side{"Spirit", 3, []string{"2"}})
				b := f.analyze(f.demo("b", 1), "de_nuke", side{"Spirit", 13, []string{"2"}}, side{"MOUZ", 3, []string{"3"}})
				auto := *teamOf(t, a, "ct-start").TeamID
				if err := f.repo.CreateReport(f.ctx, &library.Report{TeamID: uuid.New(), Title: "x"}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("CreateReport(unknown team) = %v, want ErrNotFound", err)
				}
				first := &library.Report{Title: "Apex en Mirage", TeamID: auto, MatchIDs: []uuid.UUID{a.ID, a.ID, b.ID}, Side: "T"}
				if err := f.repo.CreateReport(f.ctx, first); err != nil {
					t.Fatal(err)
				}
				if ids := first.MatchIDs; len(ids) != 2 || ids[0] != a.ID || ids[1] != b.ID || first.TeamName != "team_apEX" {
					t.Fatalf("created report = %+v, want 2 unique matches in order and the team name", first)
				}
				time.Sleep(2 * time.Millisecond)
				spirit := *teamOf(t, a, "t-start").TeamID
				second := &library.Report{Title: "Spirit", TeamID: spirit}
				if err := f.repo.CreateReport(f.ctx, second); err != nil {
					t.Fatal(err)
				}
				list, err := f.repo.ListReports(f.ctx, library.ReportFilter{})
				if err != nil || len(list) != 2 || list[0].ID != second.ID {
					t.Fatalf("ListReports = %+v, %v; want newest first", list, err)
				}
				if only, _ := f.repo.ListReports(f.ctx, library.ReportFilter{TeamID: &auto}); len(only) != 1 || only[0].ID != first.ID {
					t.Fatalf("ListReports(team) = %+v", only)
				}
				title, notes, side, ids := "Apex", "Anti-strat B", "", []uuid.UUID{b.ID}
				patched, err := f.repo.UpdateReport(f.ctx, first.ID, library.ReportPatch{Title: &title, Notes: &notes, Side: &side, MatchIDs: &ids})
				if err != nil || patched.Title != "Apex" || patched.Notes != "Anti-strat B" || patched.Side != "" || len(patched.MatchIDs) != 1 {
					t.Fatalf("UpdateReport = %+v, %v", patched, err)
				}
				if !patched.UpdatedAt.After(first.UpdatedAt) && !patched.UpdatedAt.Equal(first.UpdatedAt) {
					t.Fatalf("updated_at went back: %v before %v", patched.UpdatedAt, first.UpdatedAt)
				}

				// Moving the clan name to another team prunes the auto team only
				// when nothing else holds it; a report does.
				vit := &library.Team{Name: "Vitality"}
				if err := f.repo.CreateTeam(f.ctx, vit); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.AssignMatchTeam(f.ctx, library.TeamAssignment{MatchID: a.ID, TeamKey: "ct-start", TeamID: &vit.ID, RememberAlias: true}); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetReport(f.ctx, first.ID); err != nil {
					t.Fatalf("report after the alias move = %v, want it kept", err)
				}
				if _, err := f.repo.MergeTeams(f.ctx, auto, vit.ID); err != nil {
					t.Fatal(err)
				}
				moved, err := f.repo.GetReport(f.ctx, first.ID)
				if err != nil || moved.TeamID != vit.ID || moved.TeamName != "Vitality" {
					t.Fatalf("report after merge = %+v, %v; want it on Vitality", moved, err)
				}
				if err := f.repo.DeleteReport(f.ctx, first.ID); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.DeleteReport(f.ctx, first.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("second DeleteReport = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.UpdateReport(f.ctx, first.ID, library.ReportPatch{Title: &title}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdateReport(deleted) = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "annotations are listed by match, round and tick and go with their demo",
			run: func(t *testing.T, f libraryFixture) {
				demo := f.demo("a", 0)
				a := f.analyze(demo, "de_nuke", side{"Spirit", 13, []string{"1"}}, side{"MOUZ", 9, []string{"2"}})
				b := f.analyze(f.demo("b", 1), "de_nuke", side{"MOUZ", 13, []string{"2"}}, side{"Vitality", 3, []string{"3"}})
				if err := f.repo.CreateAnnotation(f.ctx, &library.Annotation{MatchID: uuid.New(), Round: 1, Text: "x"}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("CreateAnnotation(unknown match) = %v, want ErrNotFound", err)
				}
				line := []library.Stroke{{Color: "accent", Width: 3, Arrow: true, Points: [][2]float64{{-120.5, 300}, {40, 80}}}}
				late := &library.Annotation{MatchID: a.ID, Round: 4, Tick: 9000, Text: "rotan a B"}
				early := &library.Annotation{MatchID: a.ID, Round: 4, Tick: 8000, Strokes: line}
				other := &library.Annotation{MatchID: b.ID, Round: 2, Tick: 100, Text: "humo de outside"}
				first := &library.Annotation{MatchID: a.ID, Round: 1, Tick: 50, Text: "pistola"}
				for _, n := range []*library.Annotation{late, early, other, first} {
					if err := f.repo.CreateAnnotation(f.ctx, n); err != nil {
						t.Fatal(err)
					}
				}
				if early.Strokes[0].Points[0] != [2]float64{-120.5, 300} || !early.Strokes[0].Arrow || late.Strokes == nil {
					t.Fatalf("created annotations = %+v / %+v", early, late)
				}
				list, err := f.repo.ListAnnotations(f.ctx, library.AnnotationFilter{MatchIDs: []uuid.UUID{a.ID}})
				if err != nil || len(list) != 3 || list[0].ID != first.ID || list[1].ID != early.ID || list[2].ID != late.ID {
					t.Fatalf("ListAnnotations(a) = %+v, %v; want round 1, then round 4 by tick", list, err)
				}
				if round, _ := f.repo.ListAnnotations(f.ctx, library.AnnotationFilter{MatchIDs: []uuid.UUID{a.ID, b.ID}, Round: 2}); len(round) != 1 || round[0].ID != other.ID {
					t.Fatalf("ListAnnotations(round 2) = %+v", round)
				}
				if none, _ := f.repo.ListAnnotations(f.ctx, library.AnnotationFilter{}); len(none) != 0 {
					t.Fatalf("ListAnnotations(no match) = %+v, want none", none)
				}
				tick, text, strokes := 8500, "flash", []library.Stroke{}
				patched, err := f.repo.UpdateAnnotation(f.ctx, early.ID, library.AnnotationPatch{Tick: &tick, Text: &text, Strokes: &strokes})
				if err != nil || patched.Tick != 8500 || patched.Text != "flash" || len(patched.Strokes) != 0 || patched.MatchID != a.ID {
					t.Fatalf("UpdateAnnotation = %+v, %v", patched, err)
				}
				if err := f.repo.DeleteAnnotation(f.ctx, late.ID); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.DeleteAnnotation(f.ctx, late.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("second DeleteAnnotation = %v, want ErrNotFound", err)
				}
				if err := f.repo.DeleteDemo(f.ctx, demo.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetAnnotation(f.ctx, first.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("annotation after its demo was deleted = %v, want ErrNotFound", err)
				}
				if kept, _ := f.repo.ListAnnotations(f.ctx, library.AnnotationFilter{MatchIDs: []uuid.UUID{b.ID}}); len(kept) != 1 {
					t.Fatalf("annotations of the other match = %+v, want kept", kept)
				}
			},
		},
		{
			name: "plays keep their lists, filter by team, map and kind, and follow their team through a merge",
			run: func(t *testing.T, f libraryFixture) {
				a := f.analyze(f.demo("a", 0), "de_mirage", side{"team_apEX", 13, []string{"1"}}, side{"Spirit", 3, []string{"2"}})
				apex := *teamOf(t, a, "ct-start").TeamID
				missing := uuid.New()
				if err := f.repo.CreatePlay(f.ctx, &library.Play{Title: "x", TeamID: &missing, Map: "de_mirage", Side: "T", Kind: "execute", Origin: "round"}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("CreatePlay(unknown team) = %v, want ErrNotFound", err)
				}
				exec := &library.Play{
					Title: "A exec", TeamID: &apex, Map: "de_mirage", Side: "T", Kind: "execute", Site: "A", Origin: "round",
					PositionsSeconds: 15,
					Grenades:         []tacticalplan.PlayGrenade{{Kind: tacticalplan.EventSmoke, Seconds: 14.5, Pos: [3]float64{1, 2, 3}, Callout: "CT", LineupID: "mirage-ct"}},
					Positions:        []tacticalplan.PlayPosition{{Player: "a1", SteamID64: "1", Pos: [3]float64{4, 5, 6}, Callout: "Ramp"}},
					Examples:         []tacticalplan.Evidence{{MatchID: a.ID, Round: 3, Tick: 640}},
				}
				if err := f.repo.CreatePlay(f.ctx, exec); err != nil {
					t.Fatal(err)
				}
				if exec.TeamName != "team_apEX" || exec.Grenades[0].LineupID != "mirage-ct" || exec.Examples[0].MatchID != a.ID {
					t.Fatalf("created play = %+v", exec)
				}
				again := *exec
				if err := f.repo.CreatePlay(f.ctx, &again); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("CreatePlay(same id) = %v, want ErrConflict", err)
				}
				time.Sleep(2 * time.Millisecond)
				hold := &library.Play{Title: "B hold", Map: "de_nuke", Side: "CT", Kind: "hold", Origin: "import"}
				if err := f.repo.CreatePlay(f.ctx, hold); err != nil {
					t.Fatal(err)
				}
				if hold.TeamID != nil || hold.Grenades == nil || hold.Examples == nil {
					t.Fatalf("play without team = %+v, want no team and empty lists", hold)
				}
				list, err := f.repo.ListPlays(f.ctx, library.PlayFilter{})
				if err != nil || len(list) != 2 || list[0].ID != hold.ID {
					t.Fatalf("ListPlays = %+v, %v; want newest first", list, err)
				}
				for _, filter := range []library.PlayFilter{{TeamID: &apex}, {Map: "de_mirage"}, {Side: "T"}, {Kind: "execute"}} {
					if got, _ := f.repo.ListPlays(f.ctx, filter); len(got) != 1 || got[0].ID != exec.ID {
						t.Fatalf("ListPlays(%+v) = %+v", filter, got)
					}
				}
				title, notes, grenades := "A split", "humo CT primero", []tacticalplan.PlayGrenade{}
				patched, err := f.repo.UpdatePlay(f.ctx, exec.ID, library.PlayPatch{Title: &title, Notes: &notes, Grenades: &grenades, ClearTeam: true})
				if err != nil || patched.Title != "A split" || patched.Notes != notes || len(patched.Grenades) != 0 || patched.TeamID != nil || len(patched.Positions) != 1 {
					t.Fatalf("UpdatePlay = %+v, %v", patched, err)
				}
				if _, err := f.repo.UpdatePlay(f.ctx, exec.ID, library.PlayPatch{TeamID: &missing}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdatePlay(unknown team) = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.UpdatePlay(f.ctx, exec.ID, library.PlayPatch{TeamID: &apex}); err != nil {
					t.Fatal(err)
				}

				vit := &library.Team{Name: "Vitality"}
				if err := f.repo.CreateTeam(f.ctx, vit); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.MergeTeams(f.ctx, apex, vit.ID); err != nil {
					t.Fatal(err)
				}
				moved, err := f.repo.GetPlay(f.ctx, exec.ID)
				if err != nil || moved.TeamID == nil || *moved.TeamID != vit.ID || moved.TeamName != "Vitality" {
					t.Fatalf("play after merge = %+v, %v; want it on Vitality", moved, err)
				}
				if err := f.repo.DeletePlay(f.ctx, exec.ID); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.DeletePlay(f.ctx, exec.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("second DeletePlay = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.UpdatePlay(f.ctx, exec.ID, library.PlayPatch{Title: &title}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdatePlay(deleted) = %v, want ErrNotFound", err)
				}
			},
		},
	}
	for _, tc := range cases {
		for name, repo := range libraryRepositoriesUnderTest(t) {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				tc.run(t, libraryFixture{t: t, ctx: context.Background(), repo: repo})
			})
		}
	}
}
