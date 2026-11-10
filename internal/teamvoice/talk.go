package teamvoice

import (
	"github.com/rechedev9/cliphub/internal/library"
)

// Round phases talk time is split by. Dead is only told apart for a track tied to a player.
const (
	PhaseFreezetime = "freezetime"
	PhaseMidround   = "midround"
	PhasePostPlant  = "post_plant"
	PhaseDead       = "dead"
	PhaseAfterRound = "after_round"
)

// Phases are every round phase, in round order.
var Phases = []string{PhaseFreezetime, PhaseMidround, PhasePostPlant, PhaseDead, PhaseAfterRound}

// PhaseTalk is milliseconds of speech in each phase.
type PhaseTalk struct {
	FreezetimeMS int64 `json:"freezetime_ms"`
	MidroundMS   int64 `json:"midround_ms"`
	PostPlantMS  int64 `json:"post_plant_ms"`
	DeadMS       int64 `json:"dead_ms"`
	AfterRoundMS int64 `json:"after_round_ms"`
}

// TotalMS is the speech of every phase.
func (p PhaseTalk) TotalMS() int64 {
	return p.FreezetimeMS + p.MidroundMS + p.PostPlantMS + p.DeadMS + p.AfterRoundMS
}

func (p *PhaseTalk) add(phase string, ms int64) {
	switch phase {
	case PhaseFreezetime:
		p.FreezetimeMS += ms
	case PhaseMidround:
		p.MidroundMS += ms
	case PhasePostPlant:
		p.PostPlantMS += ms
	case PhaseDead:
		p.DeadMS += ms
	case PhaseAfterRound:
		p.AfterRoundMS += ms
	}
}

func (p *PhaseTalk) sum(o PhaseTalk) {
	p.FreezetimeMS += o.FreezetimeMS
	p.MidroundMS += o.MidroundMS
	p.PostPlantMS += o.PostPlantMS
	p.DeadMS += o.DeadMS
	p.AfterRoundMS += o.AfterRoundMS
}

// SpeakerTalk is one track's talk time.
type SpeakerTalk struct {
	Track     int    `json:"track"`
	Label     string `json:"label"`
	SteamID64 string `json:"steamid64,omitempty"`
	PhaseTalk
}

// RoundTalk is every track's talk time in one round.
type RoundTalk struct {
	Round    int           `json:"round"`
	Speakers []SpeakerTalk `json:"speakers"`
}

// TalkInput places tracks on the demo: demo time = audio time + OffsetMS.
type TalkInput struct {
	Tracks   []library.VoiceTrack
	OffsetMS int64
	Timeline Timeline
}

// TalkTime splits each track's speech by round and phase; the totals sum every round.
func TalkTime(in TalkInput) ([]RoundTalk, []SpeakerTalk) {
	rounds := make([]RoundTalk, len(in.Timeline.Rounds))
	totals := make([]SpeakerTalk, len(in.Tracks))
	for ri, r := range in.Timeline.Rounds {
		rounds[ri] = RoundTalk{Round: r.Number, Speakers: make([]SpeakerTalk, len(in.Tracks))}
		for ti, t := range in.Tracks {
			talk := SpeakerTalk{Track: t.Track, Label: t.Label, SteamID64: t.SteamID64}
			for _, s := range t.Speech {
				for _, part := range phaseParts(phaseSpan{round: r, steamID: t.SteamID64, from: s.StartMS + in.OffsetMS, to: s.EndMS + in.OffsetMS}) {
					talk.add(part.phase, part.ms)
				}
			}
			rounds[ri].Speakers[ti] = talk
		}
	}
	for ti, t := range in.Tracks {
		totals[ti] = SpeakerTalk{Track: t.Track, Label: t.Label, SteamID64: t.SteamID64}
		for _, r := range rounds {
			totals[ti].sum(r.Speakers[ti].PhaseTalk)
		}
	}
	return rounds, totals
}

type phasePart struct {
	phase string
	ms    int64
}

// phaseSpan is a demo-time span of one speaker's voice in one round.
type phaseSpan struct {
	round    Round
	steamID  string
	from, to int64
}

// phaseParts cuts a demo-time span by the round's phases; time outside the round is dropped.
func phaseParts(s phaseSpan) []phasePart {
	var out []phasePart
	for _, b := range phaseBounds(s.round, s.steamID) {
		lo, hi := max(s.from, b.from), min(s.to, b.to)
		if hi > lo {
			out = append(out, phasePart{phase: b.phase, ms: hi - lo})
		}
	}
	return out
}

type phaseBound struct {
	phase    string
	from, to int64
}

func phaseBounds(r Round, steamID string) []phaseBound {
	liveEnd := r.EndMS
	if r.PlantMS > 0 {
		liveEnd = min(r.PlantMS, r.EndMS)
	}
	death, dead := r.DeathMS[steamID]
	if steamID == "" || !dead {
		death = r.EndMS
	}
	death = max(death, r.FreezeEndMS)
	return []phaseBound{
		{PhaseFreezetime, r.StartMS, r.FreezeEndMS},
		{PhaseMidround, r.FreezeEndMS, min(liveEnd, death)},
		{PhasePostPlant, liveEnd, max(liveEnd, min(r.EndMS, death))},
		{PhaseDead, death, r.EndMS},
		{PhaseAfterRound, r.EndMS, r.NextStartMS},
	}
}

// Moment is where an audio time lands on the demo.
type Moment struct {
	DemoMS int64
	Tick   int
	Round  int
	// RoundMS is the time since the round's start, freeze time included.
	RoundMS int64
	Phase   string
}

// LocateInput is an audio time of a track; SteamID64 is its speaker, which tells dead time apart.
type LocateInput struct {
	Timeline  Timeline
	AudioMS   int64
	OffsetMS  int64
	SteamID64 string
}

// Locate places an audio time of a track on the demo; Round is 0 outside every round.
func Locate(in LocateInput) Moment {
	t, demoMS := in.Timeline, in.AudioMS+in.OffsetMS
	m := Moment{DemoMS: demoMS, Tick: t.Tick(demoMS)}
	r, ok := t.RoundAt(demoMS)
	if !ok {
		return m
	}
	m.Round, m.RoundMS = r.Number, demoMS-r.StartMS
	for _, b := range phaseBounds(r, in.SteamID64) {
		if demoMS >= b.from && demoMS < b.to {
			m.Phase = b.phase
		}
	}
	return m
}
