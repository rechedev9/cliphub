package recapplan

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRespawnStartCoverage(t *testing.T) {
	for _, rate := range []int{64, 100, 128} {
		for _, manual := range []bool{false, true} {
			f := fixtureFacts()
			f.TickRate = rate
			f.Rounds = f.Rounds[:1]
			r := &f.Rounds[0]
			r.StartTick = 25341
			r.FreezeEndTick = r.StartTick + 20*rate
			r.RoundEndTick = r.FreezeEndTick + 30*rate
			r.NextStartTick = 0
			f.EndTick = r.RoundEndTick + 10*rate
			o := fixtureOptions()
			v := VoiceEvidence{Availability: "available", IndexHash: strings.Repeat("b", 64), IndexRef: "voice.json", ClockKind: ClockIngame, Activity: []TickRange{{Start: r.StartTick, End: r.FreezeEndTick}}}
			if manual {
				o.Editorial.ManualRanges = []ManualRange{{RoundID: r.ID, StartTick: r.StartTick, EndTick: r.RoundEndTick}}
			}
			d, err := Plan(f, o, v, nil, "facts.json")
			if manual {
				if err == nil {
					t.Fatal("manual range bypassed respawn acquisition")
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			got := d.Rounds[0]
			want := r.FreezeEndTick - FixedFreezeSeconds*rate
			if got.RequestedStartTick != want || got.CaptureStartTick != want || got.StartReason != "fixed-freeze-2s" {
				t.Fatalf("unsafe start at %d Hz: %+v", rate, got)
			}
			if got.LiveStartTick != r.FreezeEndTick || got.LiveEndTick != r.RoundEndTick {
				t.Fatal("settling trimmed live gameplay")
			}
			if got.ExcludedIntervals[0] != (TickRange{Start: r.StartTick, End: want}) {
				t.Fatal("omitted freeze was not disclosed")
			}
			item := d.Timeline[0]
			frames, _ := TickFrames(got.EffectiveEndTick-want, rate)
			if item.SourceStartTick != want || item.EndFrame-item.StartFrame != frames || item.EndSample-item.StartSample != frames*SamplesPerFrame {
				t.Fatalf("audio/video clocks diverged: %+v", item)
			}
			if err := d.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestShortFreezeBlocksRatherThanDroppingLivePlay(t *testing.T) {
	f := fixtureFacts()
	f.Rounds = f.Rounds[:1]
	f.Rounds[0].FreezeEndTick = f.Rounds[0].StartTick + 1
	d, err := Plan(f, fixtureOptions(), VoiceEvidence{Availability: "no_packets"}, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blockers) == 0 || d.Blockers[0].Code != ErrPOVContract {
		t.Fatalf("missing acquisition blocker: %+v", d.Blockers)
	}
	if len(d.Rounds) != 0 {
		t.Fatal("admitted a round without exactly two seconds of freeze")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyApprovalRequiresExplicitReplan(t *testing.T) {
	d := fixtureDocument(t)
	d.PlannerVersion = LegacyPlannerVersion
	d.PlanHash, _ = d.Hash()
	s := Snapshot{Document: d, Approval: Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now()}}
	if err := s.Validate(); err != nil {
		t.Fatalf("legacy snapshot should remain readable: %v", err)
	}
	_, err := ResolveApproval(context.Background(), nil, uuid.New(), "demo.dem", d.Input.TargetSteamID64, "", s)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != ErrPlanStale {
		t.Fatalf("legacy approval admitted: %v", err)
	}
}
