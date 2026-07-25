package anticheat

import (
	"strings"
	"testing"
)

func sampleReport() (Report, PlayerReport) {
	player := PlayerReport{
		SteamID64:  "76561198012345678",
		Name:       "sospechoso",
		Team:       "T",
		Rounds:     26,
		GunKills:   28,
		Score:      84.2,
		Confidence: 0.91,
		Verdict:    VerdictHighlyAnomalous,
		Metrics: []MetricScore{
			{
				ID: MetricWallTracking, Label: "Seguimiento a través de muros", Unit: "%",
				Value: 24.5, Samples: 40000, Baseline: MetricBaseline{Mean: 1.8, StdDev: 0.9},
				Z: 6, Suspicion: 98.2, Weight: 0.22, Applied: true,
			},
			{
				ID: MetricReaction, Label: "Tiempo de reacción", Unit: "ms",
				Value: 0, Samples: 1, Baseline: MetricBaseline{Mean: 420, StdDev: 130},
				Weight: 0.12, Applied: false,
			},
		},
		Evidence: []Evidence{
			{Kind: EvidenceWallPreaim, Round: 4, Tick: 12345, Victim: "v", Weapon: "AK-47", Detail: "detalle"},
		},
	}
	report := Report{
		SchemaVersion: SchemaVersion,
		Source:        Source{DemoPath: "match.dem", SHA256: "deadbeef"},
		Baseline:      DefaultBaseline().header(),
		Match:         MatchSummary{Map: "de_mirage", Rounds: 26, TickRate: 64, SampledTicks: 90000},
		Players:       []PlayerReport{player},
		Limitations:   standardLimitations(),
	}
	return report, player
}

func TestDossierCarriesTheIdentifyingFacts(t *testing.T) {
	report, player := sampleReport()
	d := BuildDossier(report, player)

	for _, want := range []string{
		player.SteamID64,
		"de_mirage",
		"match.dem",
		"deadbeef",
		DefaultBaseline().ID,
		"tick 12345",
	} {
		if !strings.Contains(d.Markdown, want) {
			t.Fatalf("dossier markdown is missing %q", want)
		}
	}
	if d.ProfileURL != "https://steamcommunity.com/profiles/76561198012345678" {
		t.Fatalf("profile url = %q", d.ProfileURL)
	}
}

func TestDossierMarksUnscoredMetricsInsteadOfShowingAFakeZ(t *testing.T) {
	report, player := sampleReport()
	d := BuildDossier(report, player)
	if !strings.Contains(d.Markdown, "muestra insuficiente") {
		t.Fatal("a metric with too few samples must be shown as unscored, not as a z of 0")
	}
}

func TestDossierRefusesToPromiseAMassReportingPath(t *testing.T) {
	report, player := sampleReport()
	d := BuildDossier(report, player)

	if !strings.Contains(d.Policy.Rejected, "no envía denuncias automáticamente") {
		t.Fatalf("policy must state the tool does not submit reports, got %q", d.Policy.Rejected)
	}
	if !strings.Contains(d.Markdown, "no aumenta la probabilidad de baneo") {
		t.Fatal("the dossier must say that mass reporting does not raise the ban odds")
	}
	if !strings.Contains(d.Markdown, "no una prueba") {
		t.Fatal("the dossier must carry the report limitations")
	}
}

func TestDossierChannelsPreferTheInGameReport(t *testing.T) {
	report, player := sampleReport()
	d := BuildDossier(report, player)

	if len(d.Channels) == 0 {
		t.Fatal("dossier has no reporting channels")
	}
	if d.Channels[0].ID != "cs2_in_game" || !d.Channels[0].Effective {
		t.Fatalf("first channel = %+v, want the effective in-game report first", d.Channels[0])
	}
	for _, c := range d.Channels {
		if c.ID == "steam_profile" && c.Effective {
			t.Fatal("the Steam profile report must not be presented as an anti-cheat channel")
		}
	}
}

func TestDossierOmitsAProfileURLForAMalformedSteamID(t *testing.T) {
	report, player := sampleReport()
	player.SteamID64 = "not-a-steam-id"
	d := BuildDossier(report, player)

	if d.ProfileURL != "" {
		t.Fatalf("profile url = %q, want empty for a malformed steamid", d.ProfileURL)
	}
	for _, c := range d.Channels {
		if c.ID == "steam_profile" && c.URL != "" {
			t.Fatalf("steam channel url = %q, want empty for a malformed steamid", c.URL)
		}
	}
}

func TestDossierFallsBackToTheSteamIDWhenTheNameIsBlank(t *testing.T) {
	report, player := sampleReport()
	player.Name = "   "
	d := BuildDossier(report, player)
	if !strings.Contains(d.Markdown, "# Expediente CheaterDetect — 76561198012345678") {
		t.Fatalf("blank name should fall back to the steamid, got header:\n%s", firstLine(d.Markdown))
	}
}

func TestVerdictLabelsCoverEveryBand(t *testing.T) {
	for _, v := range []Verdict{
		VerdictInsufficient, VerdictClean, VerdictInconclusive, VerdictAnomalous, VerdictHighlyAnomalous,
	} {
		if VerdictLabel(v) == "" {
			t.Fatalf("verdict %q has no label", v)
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
