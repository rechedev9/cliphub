package tacticalplan_test

import (
	"context"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

var updateProReference = flag.Bool("update-pro-reference", false, "rewrite pro_reference_default.json from the demos in CLIPHUB_PRO_DEMOS")

// The demos are local files that cannot be versioned, so the description names
// them; change it whenever the demo list changes.
const (
	proReferenceID          = "pro-2026-isog-lavked-tnc"
	proReferenceDescription = "4 mapas profesionales de 2026: Iberian Soul contra OG (Anubis, Nuke e Inferno) y Lavked contra TNC (Nuke)."
	// proReferenceMinRounds keeps every player of one full map.
	proReferenceMinRounds = 15
)

var rateMetrics = []string{
	tacticalplan.MetricKPR, tacticalplan.MetricDPR, tacticalplan.MetricADR,
	tacticalplan.MetricUtilityDamage, tacticalplan.MetricFlashAssists, tacticalplan.MetricWPA,
}

// TestGenerateProReference measures the bundled pro reference. Run it with
//
//	CLIPHUB_PRO_DEMOS="a.dem;b.dem" go test ./internal/tacticalplan -run TestGenerateProReference -update-pro-reference -timeout 30m
func TestGenerateProReference(t *testing.T) {
	if !*updateProReference {
		t.Skip("run with -update-pro-reference and CLIPHUB_PRO_DEMOS to regenerate the bundled pro reference")
	}
	paths := filepath.SplitList(os.Getenv("CLIPHUB_PRO_DEMOS"))
	if len(paths) == 0 {
		t.Fatal("CLIPHUB_PRO_DEMOS lists no demo")
	}
	var all tacticalplan.BaselineSample
	perMap := map[string]*tacticalplan.BaselineSample{}
	within := map[string]map[string][]float64{}
	teams := map[string]string{}
	ref := tacticalplan.ProReference{ID: proReferenceID, Description: proReferenceDescription, Maps: map[string]tacticalplan.ProReferenceMap{}}
	for i, path := range paths {
		res, err := tactical.ScanFile(context.Background(), path, tactical.Options{})
		if err != nil {
			t.Fatalf("scan %s: %v", path, err)
		}
		doc := res.Document
		id := uuid.UUID{byte(i + 1)}
		report := tacticalplan.Metrics([]tacticalplan.MetricsMatch{{MatchID: id, Document: doc}}, tacticalplan.MetricsOptions{})
		all.Add(report)
		if perMap[doc.Demo.Map] == nil {
			perMap[doc.Demo.Map] = &tacticalplan.BaselineSample{}
		}
		perMap[doc.Demo.Map].Add(report)
		names := []string{}
		teamName := map[string]string{}
		for _, team := range doc.Teams {
			names = append(names, team.Name)
			teamName[team.Key] = team.Name
		}
		for _, p := range doc.Players {
			teams[p.SteamID64] = teamName[p.TeamKey]
		}
		ref.Matches = append(ref.Matches, tacticalplan.ProReferenceMatch{Map: doc.Demo.Map, Teams: names, Rounds: len(doc.Rounds)})
		addRoundValues(within, doc, id)
		t.Logf("%s: %s, %s, %d rounds", filepath.Base(path), doc.Demo.Map, strings.Join(names, " vs "), len(doc.Rounds))
	}
	spread := tacticalplan.ReferenceSpread{RoundStddev: map[string]float64{}}
	for _, metric := range rateMetrics {
		spread.RoundStddev[metric] = pooledWithinStddev(within[metric])
	}
	ref.Baseline = all.Baseline(proReferenceMinRounds, spread)
	ref.Players = referenceRows(&all, teams)
	for name, sample := range perMap {
		b := sample.Baseline(proReferenceMinRounds, spread)
		if b.Players >= tacticalplan.ProReferenceMinPlayers {
			ref.Maps[name] = tacticalplan.ProReferenceMap{Baseline: b, Players: referenceRows(sample, teams)}
		}
	}
	out, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tacticalplan.DecodeProReference(out); err != nil {
		t.Fatalf("generated reference does not load: %v", err)
	}
	if err := os.WriteFile("pro_reference_default.json", append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// addRoundValues reads each round alone, so every player's per-round values
// of the rate metrics show how much one round swings.
func addRoundValues(within map[string]map[string][]float64, doc tacticalplan.Document, id uuid.UUID) {
	for i := range doc.Rounds {
		one := doc
		one.Rounds = doc.Rounds[i : i+1]
		var sample tacticalplan.BaselineSample
		sample.Add(tacticalplan.Metrics([]tacticalplan.MetricsMatch{{MatchID: id, Document: one}}, tacticalplan.MetricsOptions{}))
		for _, metric := range rateMetrics {
			if within[metric] == nil {
				within[metric] = map[string][]float64{}
			}
			for _, row := range sample.Leaderboard(metric, 1, 0) {
				within[metric][row.SteamID64] = append(within[metric][row.SteamID64], row.Value)
			}
		}
	}
}

// pooledWithinStddev is the spread of a round around each player's own mean.
func pooledWithinStddev(byPlayer map[string][]float64) float64 {
	var dev float64
	var df int
	for _, values := range byPlayer {
		if len(values) < 2 {
			continue
		}
		var sum float64
		for _, v := range values {
			sum += v
		}
		mean := sum / float64(len(values))
		for _, v := range values {
			dev += (v - mean) * (v - mean)
		}
		df += len(values) - 1
	}
	if df == 0 {
		return 0
	}
	return math.Sqrt(dev / float64(df))
}

// referenceRows is every player's value of every metric, in SteamID order.
func referenceRows(sample *tacticalplan.BaselineSample, teams map[string]string) []tacticalplan.ProReferenceRow {
	rows := map[string]*tacticalplan.ProReferenceRow{}
	for _, metric := range tacticalplan.ReferenceMetrics {
		for _, line := range sample.Leaderboard(metric, 1, 0) {
			row := rows[line.SteamID64]
			if row == nil {
				row = &tacticalplan.ProReferenceRow{SteamID64: line.SteamID64, Name: line.Name, Team: teams[line.SteamID64], Rounds: line.Rounds, Matches: line.Matches, Values: map[string]float64{}}
				rows[line.SteamID64] = row
			}
			row.Values[metric] = line.Value
		}
	}
	out := make([]tacticalplan.ProReferenceRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SteamID64 < out[j].SteamID64 })
	return out
}
