package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryProMatchContract(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := library.Demo{SHA256: "pro-sha", FileName: "vitality-vs-faze-m1-mirage.dem", Source: library.SourcePro, SourceManual: true, State: library.ParseParsing}
			if _, err := repo.CreateDemo(ctx, &d); err != nil {
				t.Fatal(err)
			}
			played := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)
			first := &library.ProMap{MatchID: 2400001, Entry: "vitality-vs-faze-m1-mirage", DemoID: &d.ID, Map: "de_mirage", Event: "IEM",
				PlayedAt: played, Teams: []library.ProTeam{{ID: 9565, Name: "Vitality", Rank: 1}, {ID: 6667, Name: "FaZe"}}, AnalysisVersion: 6, SHA256: "a"}
			second := &library.ProMap{MatchID: 2400001, Entry: "vitality-vs-faze-m2-nuke", Map: "de_nuke", PlayedAt: played, SHA256: "b"}
			older := &library.ProMap{MatchID: 2300000, Entry: "a-vs-b-m1-inferno", Map: "de_inferno", PlayedAt: played.AddDate(0, 0, -3), SHA256: "c"}
			for _, m := range []*library.ProMap{first, second, older} {
				if err := repo.SaveProMap(ctx, m); err != nil {
					t.Fatal(err)
				}
			}
			got, err := repo.GetProMap(ctx, 2400001, "vitality-vs-faze-m1-mirage")
			if err != nil || got.DemoID == nil || *got.DemoID != d.ID || len(got.Teams) != 2 || got.Teams[0].Rank != 1 || !got.PlayedAt.Equal(played) {
				t.Fatalf("GetProMap = %+v, %v", got, err)
			}
			if byDemo, err := repo.ProMapForDemo(ctx, d.ID); err != nil || byDemo.Entry != first.Entry {
				t.Fatalf("ProMapForDemo = %+v, %v", byDemo, err)
			}
			if _, err := repo.GetProMap(ctx, 1, "x"); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("GetProMap(missing) = %v", err)
			}
			all, err := repo.ListProMaps(ctx, 0)
			if err != nil || len(all) != 3 || all[0].Entry != first.Entry || all[2].MatchID != 2300000 {
				t.Fatalf("ListProMaps(all) = %+v, %v", all, err)
			}
			if one, err := repo.ListProMaps(ctx, 2400001); err != nil || len(one) != 2 || one[1].Entry != second.Entry {
				t.Fatalf("ListProMaps(match) = %+v, %v", one, err)
			}
			if err := repo.DeleteDemo(ctx, d.ID); err != nil {
				t.Fatal(err)
			}
			if gone, err := repo.GetProMap(ctx, 2400001, first.Entry); err != nil || gone.DemoID != nil {
				t.Fatalf("a deleted demo must leave its pro record without a demo: %+v, %v", gone, err)
			}

			f := &library.ProFollow{TeamID: 9565, Name: "Vitality"}
			if err := repo.FollowProTeam(ctx, f); err != nil || f.FollowedAt.IsZero() || f.SyncedAt != nil {
				t.Fatalf("FollowProTeam = %+v, %v", f, err)
			}
			if err := repo.FollowProTeam(ctx, &library.ProFollow{TeamID: 6667, Name: "faze"}); err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
			if err := repo.MarkProFollowSynced(ctx, 9565, at); err != nil {
				t.Fatal(err)
			}
			renamed := &library.ProFollow{TeamID: 9565, Name: "Team Vitality"}
			if err := repo.FollowProTeam(ctx, renamed); err != nil || renamed.SyncedAt == nil || !renamed.SyncedAt.Equal(at) || !renamed.FollowedAt.Equal(f.FollowedAt) {
				t.Fatalf("following again must keep the dates: %+v, %v", renamed, err)
			}
			follows, err := repo.ListProFollows(ctx)
			if err != nil || len(follows) != 2 || follows[0].Name != "faze" || follows[1].Name != "Team Vitality" {
				t.Fatalf("ListProFollows = %+v, %v", follows, err)
			}
			if err := repo.UnfollowProTeam(ctx, 6667); err != nil {
				t.Fatal(err)
			}
			if err := repo.UnfollowProTeam(ctx, 6667); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("UnfollowProTeam(again) = %v", err)
			}
			if err := repo.MarkProFollowSynced(ctx, 6667, at); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("MarkProFollowSynced(unfollowed) = %v", err)
			}
		})
	}
}
