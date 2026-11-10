package teamvoice

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// pcm writes 16 kHz s16le: a hiss floor with a 300 Hz tone inside each span.
func pcm(durationMS int64, spans []library.VoiceSpan) []byte {
	rng := rand.New(rand.NewPCG(1, 2))
	var buf bytes.Buffer
	n := durationMS * SampleRate / 1000
	for i := int64(0); i < n; i++ {
		ms := i * 1000 / SampleRate
		v := rng.NormFloat64() * 0.002
		for _, s := range spans {
			if ms >= s.StartMS && ms < s.EndMS {
				v += 0.3 * math.Sin(2*math.Pi*300*float64(i)/SampleRate)
			}
		}
		_ = binary.Write(&buf, binary.LittleEndian, int16(max(-1, min(1, v))*32767))
	}
	return buf.Bytes()
}

func TestDetectSpeechFindsTalkOverHissAndBridgesShortPauses(t *testing.T) {
	spans := []library.VoiceSpan{{StartMS: 1000, EndMS: 2000}, {StartMS: 2150, EndMS: 3000}, {StartMS: 5000, EndMS: 5050}, {StartMS: 7000, EndMS: 8000}}
	got, duration, err := DetectSpeech(bytes.NewReader(pcm(10_000, spans)))
	if err != nil {
		t.Fatal(err)
	}
	if duration != 10_000 {
		t.Fatalf("duration = %d, want 10000", duration)
	}
	// The 150 ms pause joins the first two runs; the 50 ms blip is dropped; runs are padded.
	if len(got) != 2 || abs(int(got[0].StartMS-920)) > 40 || abs(int(got[0].EndMS-3080)) > 40 || abs(int(got[1].StartMS-6920)) > 40 {
		t.Fatalf("spans = %+v, want about 920-3080 and 6920-8080", got)
	}
}

func TestDetectSpeechOfSilenceIsEmpty(t *testing.T) {
	got, _, err := DetectSpeech(bytes.NewReader(make([]byte, SampleRate*2*3)))
	if err != nil || len(got) != 0 {
		t.Fatalf("silence = %+v, %v; want no speech", got, err)
	}
}

// chatter is random talk over a length, seeded so a test is repeatable.
func chatter(seed uint64, lengthMS int64) []library.VoiceSpan {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	var out []library.VoiceSpan
	for at := int64(0); at < lengthMS; {
		at += int64(500 + rng.IntN(6000))
		end := at + int64(400+rng.IntN(3000))
		out = append(out, library.VoiceSpan{StartMS: at, EndMS: min(end, lengthMS)})
		at = end
	}
	return out
}

func shift(spans []library.VoiceSpan, by int64) []library.VoiceSpan {
	out := make([]library.VoiceSpan, 0, len(spans))
	for _, s := range spans {
		out = append(out, library.VoiceSpan{StartMS: s.StartMS + by, EndMS: s.EndMS + by})
	}
	return out
}

func timeline(durationMS int64) Timeline {
	t := Timeline{Tickrate: 64, DurationMS: durationMS}
	rng := rand.New(rand.NewPCG(7, 8))
	for n, at := 1, int64(30_000); at+110_000 < durationMS; n, at = n+1, at+110_000 {
		r := Round{Number: n, StartMS: at, FreezeEndMS: at + 15_000, EndMS: at + 100_000, NextStartMS: at + 110_000, DeathMS: map[string]int64{}}
		for k := 0; k < 7; k++ {
			r.KillMS = append(r.KillMS, at+20_000+int64(rng.IntN(75_000)))
		}
		t.Rounds = append(t.Rounds, r)
	}
	return t
}

func TestEstimateAlignsWithTheDemoVoice(t *testing.T) {
	audioMS, demoMS := int64(35*60_000), int64(40*60_000)
	talk := chatter(3, audioMS)
	// The demo heard the same team 73.4 s later, with a few phrases missing.
	demo := shift(talk, 73_400)
	demo = append(demo[:40], demo[45:]...)
	res := Estimate(SyncInput{Speech: talk, AudioMS: audioMS, Timeline: timeline(demoMS), DemoVoice: map[string][]library.VoiceSpan{"t-start": demo}})
	s := res.Sync
	if s.State != library.VoiceAuto || s.Method != library.VoiceMethodDemoVoice || abs(int(s.OffsetMS-73_400)) > 100 || s.TeamKey != "t-start" || res.Mismatch != "" {
		t.Fatalf("sync = %+v, mismatch %q; want auto at 73.4 s from the T team's voice", s, res.Mismatch)
	}
}

func TestEstimateRefusesAudioThatIsNotTheDemosVoice(t *testing.T) {
	audioMS := int64(35 * 60_000)
	res := Estimate(SyncInput{Speech: chatter(3, audioMS), AudioMS: audioMS, Timeline: timeline(40 * 60_000),
		DemoVoice: map[string][]library.VoiceSpan{"ct-start": chatter(99, 40*60_000)}})
	if res.Sync.State != library.VoiceUnsynced || res.Mismatch == "" || res.Sync.Method != library.VoiceMethodRoundStart || res.Sync.OffsetMS != 30_000 {
		t.Fatalf("result = %+v; want the round 1 guess, unsynced, with a mismatch", res)
	}
}

func TestEstimateFollowsTheCallsAfterKills(t *testing.T) {
	demoMS := int64(40 * 60_000)
	tl := timeline(demoMS)
	offset := int64(28_700)
	var talk []library.VoiceSpan
	for _, r := range tl.Rounds {
		for _, k := range r.KillMS {
			at := k - offset + 300
			talk = append(talk, library.VoiceSpan{StartMS: at, EndMS: at + 1500})
		}
	}
	audioMS := demoMS - offset
	talk = UnionSpans(talk, chatter(5, audioMS)[:60])
	res := Estimate(SyncInput{Speech: talk, AudioMS: audioMS, Timeline: tl})
	if res.Sync.State != library.VoiceAuto || res.Sync.Method != library.VoiceMethodKills || abs(int(res.Sync.OffsetMS-offset)) > 1000 {
		t.Fatalf("sync = %+v; want auto within a second of %d from the kills", res.Sync, offset)
	}
}

func TestEstimateGuessesRoundOneWithoutEvidence(t *testing.T) {
	audioMS := int64(35 * 60_000)
	res := Estimate(SyncInput{Speech: chatter(11, audioMS), AudioMS: audioMS, Timeline: timeline(40 * 60_000)})
	if res.Sync.State != library.VoiceUnsynced || res.Sync.OffsetMS != 30_000 || res.Mismatch != "" {
		t.Fatalf("result = %+v; want the round 1 guess with no mismatch", res)
	}
}

func TestEstimateCallsAShortRecordingAMismatch(t *testing.T) {
	res := Estimate(SyncInput{AudioMS: 90_000, Timeline: timeline(40 * 60_000)})
	if res.Mismatch == "" || !strings.Contains(res.Sync.Note, "1:30") {
		t.Fatalf("result = %+v; want a mismatch that names the length", res)
	}
}

func TestTalkTimeSplitsByPhaseAndDeath(t *testing.T) {
	r := Round{Number: 4, StartMS: 10_000, FreezeEndMS: 25_000, EndMS: 100_000, PlantMS: 70_000, NextStartMS: 105_000,
		DeathMS: map[string]int64{"7656": 80_000}}
	tl := Timeline{Tickrate: 64, DurationMS: 200_000, Rounds: []Round{r}}
	// Audio starts 5 s into the demo; this speech covers 20-90 s of demo time.
	tracks := []library.VoiceTrack{
		{Track: 1, Label: "mix", Speech: []library.VoiceSpan{{StartMS: 15_000, EndMS: 85_000}, {StartMS: 97_000, EndMS: 120_000}}},
		{Track: 2, Label: "mopoz", SteamID64: "7656", Speech: []library.VoiceSpan{{StartMS: 15_000, EndMS: 85_000}}},
	}
	rounds, totals := TalkTime(TalkInput{Tracks: tracks, OffsetMS: 5_000, Timeline: tl})
	mix, player := rounds[0].Speakers[0], rounds[0].Speakers[1]
	if mix.FreezetimeMS != 5_000 || mix.MidroundMS != 45_000 || mix.PostPlantMS != 20_000 || mix.DeadMS != 0 || mix.AfterRoundMS != 3_000 {
		t.Fatalf("mix = %+v", mix.PhaseTalk)
	}
	if player.PostPlantMS != 10_000 || player.DeadMS != 10_000 || player.MidroundMS != 45_000 {
		t.Fatalf("player = %+v; want 10 s post plant alive and 10 s dead", player.PhaseTalk)
	}
	if totals[0].TotalMS() != 73_000 || totals[1].Label != "mopoz" {
		t.Fatalf("totals = %+v", totals)
	}
	if m := Locate(tl, 60_000, 5_000); m.Round != 4 || m.Phase != PhaseMidround || m.RoundMS != 55_000 || m.Tick != 4160 {
		t.Fatalf("Locate = %+v", m)
	}
	if m := Locate(tl, 0, 5_000); m.Round != 0 {
		t.Fatalf("Locate before round 1 = %+v, want no round", m)
	}
}

func TestTimelineOfReadsRoundsKillsAndDeaths(t *testing.T) {
	slot := uint8(3)
	doc := tacticalplan.Document{
		Demo:    tacticalplan.Demo{Tickrate: 64, DurationTicks: 64 * 300},
		Players: []tacticalplan.Player{{Slot: 3, SteamID64: "7656"}},
		Rounds: []tacticalplan.Round{
			{Number: 1, TickStart: 640, TickFreezeEnd: 1600, TickEnd: 6400, Bomb: &tacticalplan.Bomb{PlantTick: 5000},
				Events:  []tacticalplan.Event{{Tick: 3200, Kind: tacticalplan.EventKill, TargetSlot: &slot}, {Tick: 3300, Kind: tacticalplan.EventSmoke}},
				Players: []tacticalplan.PlayerRound{{Slot: 3, DeathTick: 3200}}},
			{Number: 2, TickStart: 7040, TickFreezeEnd: 8000, TickEnd: 12800},
		},
	}
	tl := TimelineOf(doc)
	r := tl.Rounds[0]
	if r.StartMS != 10_000 || r.FreezeEndMS != 25_000 || r.PlantMS != 78_125 || r.NextStartMS != 110_000 || len(r.KillMS) != 1 || r.DeathMS["7656"] != 50_000 {
		t.Fatalf("round 1 = %+v", r)
	}
	if last := tl.Rounds[1]; last.NextStartMS != 210_000 || tl.DurationMS != 300_000 {
		t.Fatalf("round 2 = %+v, duration %d", last, tl.DurationMS)
	}
}
