package community

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

type partRound struct {
	number int
	start  int
	reason string
}

func mergePart(t *testing.T, entry string, players []tacticalplan.Player, rounds []partRound) ProPart {
	t.Helper()
	doc := tacticalplan.NewDocument()
	doc.Demo = tacticalplan.Demo{Map: "de_anubis", Tickrate: 64, DurationTicks: rounds[len(rounds)-1].start + 50}
	doc.Teams = []tacticalplan.Team{{Key: "a", Name: "Alpha", StartSide: tacticalplan.SideCT}, {Key: "b", Name: "Beta", StartSide: tacticalplan.SideT}}
	doc.Players = players
	frames := []tacticalplan.RoundFrames{}
	for _, r := range rounds {
		round := tacticalplan.Round{Number: r.number, TickStart: r.start, TickFreezeEnd: r.start + 10, TickEnd: r.start + 40, EndReason: r.reason}
		samples := []tacticalplan.Sample{}
		for _, p := range players {
			round.Players = append(round.Players, tacticalplan.PlayerRound{Slot: p.Slot, Side: p.StartSide})
			samples = append(samples, tacticalplan.Sample{Slot: p.Slot, X: float64(p.Slot), Health: 100, Flags: tacticalplan.FlagAlive})
		}
		actor := players[0].Slot
		round.Events = []tacticalplan.Event{{Tick: r.start + 20, Kind: tacticalplan.EventKind("kill"), ActorSlot: &actor}}
		doc.Rounds = append(doc.Rounds, round)
		frames = append(frames, tacticalplan.RoundFrames{Round: r.number, Frames: []tacticalplan.Frame{{Tick: r.start + 12, Samples: samples}}})
	}
	blob, err := tacticalplan.EncodePositions(frames, 8, 64)
	if err != nil {
		t.Fatal(err)
	}
	doc.Positions = blob.Descriptor
	return ProPart{Header: ProAnalysisHeader{Format: ProAnalysisFormat, Entry: entry, Document: doc, Positions: blob.Descriptor}, Blob: blob.Data}
}

func TestMergeProPartsKeepsTheReplayedRoundOfTheLaterPart(t *testing.T) {
	first := mergePart(t, "a-vs-b-m1-anubis.dem", []tacticalplan.Player{
		{Slot: 0, SteamID64: "1", Name: "one", TeamKey: "a", StartSide: tacticalplan.SideCT},
		{Slot: 1, SteamID64: "2", Name: "two", TeamKey: "b", StartSide: tacticalplan.SideT},
	}, []partRound{{1, 100, "first"}, {2, 200, "first"}, {3, 300, "first"}})
	// The restart demo numbers its slots apart, fields a stand in, and starts its ticks over.
	second := mergePart(t, "a-vs-b-m1-anubis-p2.dem", []tacticalplan.Player{
		{Slot: 0, SteamID64: "2", Name: "two", TeamKey: "b", StartSide: tacticalplan.SideT},
		{Slot: 1, SteamID64: "1", Name: "one", TeamKey: "a", StartSide: tacticalplan.SideCT},
		{Slot: 2, SteamID64: "3", Name: "sub", TeamKey: "b", StartSide: tacticalplan.SideT},
	}, []partRound{{3, 100, "second"}, {4, 200, "second"}})
	second.Header.Match.Veto = []ProSidecarVeto{{Action: ProVetoDecider, Map: "Anubis"}}
	header, blob, err := MergeProParts([]ProPart{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(header.Match.Veto) != 1 {
		t.Fatalf("the merge must keep the sidecar with the veto: %+v", header.Match)
	}
	doc := header.Document
	if len(doc.Rounds) != 4 || doc.Rounds[2].EndReason != "second" || doc.Rounds[0].EndReason != "first" {
		t.Fatalf("rounds = %+v", doc.Rounds)
	}
	if len(doc.Players) != 3 || doc.Players[2].SteamID64 != "3" || doc.Players[2].Slot != 2 || doc.Players[2].TeamKey != "b" {
		t.Fatalf("the stand in must join the merged players: %+v", doc.Players)
	}
	r3 := doc.Rounds[2]
	if r3.TickStart <= doc.Rounds[1].TickEnd || *r3.Events[0].ActorSlot != 1 || r3.Players[0].Slot != 1 {
		t.Fatalf("the later part must move after the first and use its slots: %+v", r3)
	}
	if err := CheckProAnalysis(header, blob); err != nil && err.Error() != "analysis has no radar calibration" {
		t.Fatalf("merged blob does not match its document: %v", err)
	}
	off, ok := doc.Positions.Offset(3)
	if !ok {
		t.Fatal("round 3 has no positions")
	}
	frames, err := tacticalplan.DecodeFrames(blob, off.ByteOffset, off.FrameCount, doc.Positions)
	if err != nil || len(frames) != 1 || frames[0].Tick != r3.TickStart+12 {
		t.Fatalf("frames = %+v %v", frames, err)
	}
	for _, s := range frames[0].Samples {
		// X was the part's slot: SteamID 2 sat in slot 0 there and is slot 1 here.
		if s.X == 0 && s.Slot != 1 {
			t.Fatalf("a sample kept its part's slot: %+v", frames[0].Samples)
		}
	}
}
