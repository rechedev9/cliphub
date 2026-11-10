package tacticalplan

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// bombsiteRound is one round of the bombsite fixture; plant is the site the
// bomb went down on, or "" for no plant.
func bombsiteRound(number, half int, winner Side, class Class, plant Site) Round {
	r := Round{Number: number, Half: half, Winner: winner, TickFreezeEnd: number * 1000, Class: class}
	if plant != "" {
		r.Bomb = &Bomb{PlantTick: number*1000 + 500, Site: plant}
	}
	return r
}

// bombsiteDocument has "home" on CT in the first half and on T in the second.
func bombsiteDocument() Document {
	d := NewDocument()
	d.Demo.Map = "de_nuke"
	d.Teams = []Team{{Key: "home", StartSide: SideCT}, {Key: "away", StartSide: SideT}}
	d.Rounds = []Round{
		bombsiteRound(1, 1, SideCT, Class{TSide: TDefault, CTSide: CTHold, Site: SiteA}, ""),
		bombsiteRound(2, 1, SideCT, Class{TSide: TExecute, CTSide: CTRetake, Site: SiteA}, SiteA),
		bombsiteRound(3, 1, SideT, Class{TSide: TExecute, CTSide: CTHold, Site: SiteA}, SiteA),
		bombsiteRound(4, 1, SideCT, Class{TSide: TSave, CTSide: CTHold, Site: SiteB}, ""),
		bombsiteRound(5, 1, SideT, Class{TSide: TFast, CTSide: CTHold, Site: SiteB}, ""),
		bombsiteRound(6, 2, SideT, Class{TSide: TExecute, CTSide: CTHold, Site: SiteB}, SiteB),
		// The plant decides the target even when the classifier read another site.
		bombsiteRound(7, 2, SideCT, Class{TSide: TSplit, CTSide: CTRetake, Site: SiteA}, SiteB),
		bombsiteRound(8, 2, SideCT, Class{TSide: TExecute, CTSide: CTHold, Site: SiteA}, ""),
		bombsiteRound(9, 2, SideCT, Class{TSide: TDefault, CTSide: CTHold, Site: SiteMid}, ""),
		bombsiteRound(10, 2, "", Class{TSide: TExecute, Site: SiteA}, SiteA),
	}
	return d
}

func wantRate(t *testing.T, name string, got Rate, count, total int) {
	t.Helper()
	if got.Count != count || got.Total != total {
		t.Fatalf("%s = %d/%d, want %d/%d", name, got.Count, got.Total, count, total)
	}
}

func TestBombsitesCountsEachSiteFromBothSides(t *testing.T) {
	matches := []ScoutMatch{{
		MatchID: uuid.MustParse("00000000-0000-0000-0000-0000000000b1"), TeamKey: "home",
		PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC), Document: bombsiteDocument(),
	}}
	report := Bombsites(matches, ScoutFilter{})
	if report.MatchCount != 1 || len(report.Maps) != 1 {
		t.Fatalf("report = %d matches, %d maps", report.MatchCount, len(report.Maps))
	}
	m := report.Maps[0]
	if m.Map != "de_nuke" || m.CTRounds != 5 || m.TRounds != 4 || len(m.Sites) != 2 {
		t.Fatalf("map = %s, %d CT / %d T rounds, %d sites; the round without a winner must not count", m.Map, m.CTRounds, m.TRounds, len(m.Sites))
	}
	a, b := m.Sites[0], m.Sites[1]
	if a.Site != SiteA || b.Site != SiteB {
		t.Fatalf("sites = %s, %s", a.Site, b.Site)
	}
	// A as CT: rounds 1 (held), 2 (plant, retake won) and 3 (plant, no defender left).
	wantRate(t, "A hit", a.CT.Hit, 3, 5)
	wantRate(t, "A held", a.CT.Held, 1, 3)
	wantRate(t, "A plants allowed", a.CT.PlantsAllowed, 2, 3)
	wantRate(t, "A CT won", a.CT.Won, 2, 3)
	wantRate(t, "A retakes attempted", a.CT.RetakesAttempted, 1, 2)
	wantRate(t, "A retakes won", a.CT.RetakesWon, 1, 1)
	// Round 4 is a T save with no plant: no site was hit.
	wantRate(t, "B hit", b.CT.Hit, 1, 5)
	wantRate(t, "B held", b.CT.Held, 0, 1)
	// B as T: rounds 6 and 7 planted, one converted.
	wantRate(t, "B attacks", b.T.Attacks, 2, 4)
	wantRate(t, "B plants", b.T.Plants, 2, 2)
	wantRate(t, "B T won", b.T.Won, 1, 2)
	wantRate(t, "B post-plant", b.T.PostPlantWon, 1, 2)
	wantRate(t, "A attacks", a.T.Attacks, 1, 4)
	wantRate(t, "A plants", a.T.Plants, 0, 1)
	if a.CT.Hit.Pct != 60 || !a.CT.Hit.Reliable || a.CT.RetakesWon.Reliable {
		t.Fatalf("A hit = %+v, A retakes won = %+v: rates under %d rounds are flagged", a.CT.Hit, a.CT.RetakesWon, MinReliableSample)
	}
	if len(a.CT.Examples) != 3 || a.CT.Examples[0].Round != 1 || len(b.T.Examples) != 2 {
		t.Fatalf("examples = %+v / %+v", a.CT.Examples, b.T.Examples)
	}
	if len(a.CT.Refs) != a.CT.Hit.Count || len(b.T.Refs) != b.T.Attacks.Count {
		t.Fatalf("refs = %d / %d, want every hit and attacked round (%d / %d)", len(a.CT.Refs), len(b.T.Refs), a.CT.Hit.Count, b.T.Attacks.Count)
	}

	ct := Bombsites(matches, ScoutFilter{Side: SideCT}).Maps[0]
	if ct.CTRounds != 5 || ct.TRounds != 0 || ct.Sites[1].T.Attacks.Total != 0 {
		t.Fatalf("CT-only map = %d CT / %d T rounds", ct.CTRounds, ct.TRounds)
	}
	if other := Bombsites(matches, ScoutFilter{Maps: []string{"de_mirage"}}); len(other.Maps) != 0 || other.MatchCount != 0 {
		t.Fatalf("a map filter that keeps no match = %+v", other)
	}
}
