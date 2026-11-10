package teamvoice

import (
	"fmt"
	"math"

	"github.com/rechedev9/cliphub/internal/library"
)

// SyncInput is what the automatic sync compares: the recording's speech and the demo.
type SyncInput struct {
	Speech   []library.VoiceSpan
	AudioMS  int64
	Timeline Timeline
	// DemoVoice is each team's in-game voice in demo time, by team key; most demos have none.
	DemoVoice map[string][]library.VoiceSpan
}

// SyncResult is the sync to store; Mismatch says why the audio looks like another match.
type SyncResult struct {
	Sync     library.VoiceSync
	Mismatch string
}

const (
	coarseMS = 1000
	fineMS   = 100
	// A peak counts when it stands this many deviations over the other offsets and over the runner-up.
	minPeakZ    = 6.0
	minPeakLead = 1.5
	// The runner-up is the best offset farther than this from the peak.
	runnerUpMS     = 5000
	minDemoVoiceMS = 60_000
	minAudioMS     = 120_000
	// Speech right after a kill or plant: callouts follow it.
	eventWindowMS = 2500
)

// Estimate finds the offset that puts the recording on the demo: against the demo's own voice
// when it has some, else by the speech after kills, else at round 1's start (the usual habit).
func Estimate(in SyncInput) SyncResult {
	prior := roundStartGuess(in.Timeline)
	if mismatch := lengthMismatch(in.AudioMS, in.Timeline.DurationMS); mismatch != "" {
		prior.Note = mismatch
		return SyncResult{Sync: prior, Mismatch: mismatch}
	}
	audio := spanSignal(in.Speech, in.AudioMS)
	strongVoice := false
	var best *peak
	for _, key := range []string{"ct-start", "t-start"} {
		spans := in.DemoVoice[key]
		if SpeechMS(spans) < minDemoVoiceMS {
			continue
		}
		strongVoice = true
		p := findPeak(audio, spanSignal(spans, in.Timeline.DurationMS))
		if p.ok() && (best == nil || p.z > best.z) {
			p.teamKey = key
			best = &p
		}
	}
	if best != nil {
		return SyncResult{Sync: acceptedSync(*best, library.VoiceMethodDemoVoice, prior)}
	}
	if strongVoice {
		note := "La voz de la grabación no coincide con la voz que trae la demo: ¿es de otra partida?"
		prior.Note = note
		return SyncResult{Sync: prior, Mismatch: note}
	}
	p := findPeak(audio, eventSignal(in.Timeline))
	if p.ok() {
		return SyncResult{Sync: acceptedSync(p, library.VoiceMethodKills, prior)}
	}
	return SyncResult{Sync: prior}
}

func acceptedSync(p peak, method string, prior library.VoiceSync) library.VoiceSync {
	return library.VoiceSync{State: library.VoiceAuto, Method: method, OffsetMS: p.offsetMS, AutoOffsetMS: p.offsetMS,
		Confidence: math.Round(min(1, (p.z-3)/9)*100) / 100, TeamKey: p.teamKey,
		Note: fmt.Sprintf("Ajustada sola; sin ella habría empezado en %s.", Clock(prior.OffsetMS))}
}

// roundStartGuess plays audio 0 at round 1's freeze time, where teams usually press record.
func roundStartGuess(t Timeline) library.VoiceSync {
	var at int64
	if len(t.Rounds) > 0 {
		at = t.Rounds[0].StartMS
	}
	return library.VoiceSync{State: library.VoiceUnsynced, Method: library.VoiceMethodRoundStart, OffsetMS: at, AutoOffsetMS: at,
		Note: "Estimada: la grabación empieza con el tiempo de compra de la ronda 1. Compruébala con un kill."}
}

func lengthMismatch(audioMS, demoMS int64) string {
	switch {
	case audioMS < minAudioMS:
		return fmt.Sprintf("La grabación dura %s: es demasiado corta para sincronizarla con la partida.", Clock(audioMS))
	case demoMS > 0 && audioMS < demoMS/4:
		return fmt.Sprintf("La grabación dura %s y la partida %s: no parece de esta partida.", Clock(audioMS), Clock(demoMS))
	case demoMS > 0 && audioMS > 2*demoMS+30*60_000:
		return fmt.Sprintf("La grabación dura %s y la partida %s: no parece de esta partida.", Clock(audioMS), Clock(demoMS))
	}
	return ""
}

// Clock writes a duration as m:ss, or h:mm:ss, with a sign when negative.
func Clock(ms int64) string {
	sign := ""
	if ms < 0 {
		sign, ms = "-", -ms
	}
	s := ms / 1000
	if s >= 3600 {
		return fmt.Sprintf("%s%d:%02d:%02d", sign, s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%s%d:%02d", sign, s/60, s%60)
}

// signal is a time series sampled at a step; at resamples it.
type signal struct {
	at func(stepMS int64) []float64
}

// spanSignal is the share of each step covered by spans.
func spanSignal(spans []library.VoiceSpan, lengthMS int64) signal {
	return signal{at: func(step int64) []float64 {
		out := make([]float64, max(1, (lengthMS+step-1)/step))
		for _, s := range spans {
			for b := max(0, s.StartMS/step); b < int64(len(out)) && b*step < s.EndMS; b++ {
				lo, hi := max(s.StartMS, b*step), min(s.EndMS, (b+1)*step)
				if hi > lo {
					out[b] += float64(hi-lo) / float64(step)
				}
			}
		}
		return out
	}}
}

// eventSignal is 1 in the window after each kill or plant.
func eventSignal(t Timeline) signal {
	var spans []library.VoiceSpan
	for _, r := range t.Rounds {
		for _, k := range r.KillMS {
			spans = append(spans, library.VoiceSpan{StartMS: k, EndMS: k + eventWindowMS})
		}
	}
	return spanSignal(UnionSpans(spans), t.DurationMS)
}

type peak struct {
	offsetMS int64
	z, lead  float64
	teamKey  string
}

func (p peak) ok() bool { return p.z >= minPeakZ && p.lead >= minPeakLead }

// findPeak correlates the audio with the demo signal over every offset that overlaps half the
// shorter one, coarse first, then refines around the best coarse offset.
func findPeak(audio, demo signal) peak {
	a, d := audio.at(coarseMS), demo.at(coarseMS)
	minOverlap := max(1, min(len(a), len(d))/2)
	lo, hi := -(len(a) - minOverlap), len(d)-minOverlap
	if hi < lo {
		return peak{}
	}
	scores := make([]float64, 0, hi-lo+1)
	for o := lo; o <= hi; o++ {
		scores = append(scores, pearson(a, d, o))
	}
	best := 0
	for i, s := range scores {
		if s > scores[best] {
			best = i
		}
	}
	mean, std := meanStd(scores)
	if std == 0 {
		return peak{}
	}
	runner := math.Inf(-1)
	for i, s := range scores {
		if abs(i-best)*coarseMS > runnerUpMS {
			runner = max(runner, s)
		}
	}
	p := peak{z: (scores[best] - mean) / std, lead: (scores[best] - runner) / std}
	coarseOffset := int64(lo+best) * coarseMS
	fa, fd := audio.at(fineMS), demo.at(fineMS)
	fineBest, fineScore := coarseOffset/fineMS, math.Inf(-1)
	for o := (coarseOffset - 2*coarseMS) / fineMS; o <= (coarseOffset+2*coarseMS)/fineMS; o++ {
		if s := pearson(fa, fd, int(o)); s > fineScore {
			fineBest, fineScore = o, s
		}
	}
	p.offsetMS = fineBest * fineMS
	return p
}

// pearson correlates a[i] with d[i+o] over their overlap.
func pearson(a, d []float64, o int) float64 {
	from, to := max(0, -o), min(len(a), len(d)-o)
	n := float64(to - from)
	if n < 2 {
		return 0
	}
	var sa, sd, saa, sdd, sad float64
	for i := from; i < to; i++ {
		x, y := a[i], d[i+o]
		sa, sd, saa, sdd, sad = sa+x, sd+y, saa+x*x, sdd+y*y, sad+x*y
	}
	cov := sad - sa*sd/n
	va, vd := saa-sa*sa/n, sdd-sd*sd/n
	if va <= 0 || vd <= 0 {
		return 0
	}
	return cov / math.Sqrt(va*vd)
}

func meanStd(xs []float64) (float64, float64) {
	var sum, sq float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	for _, x := range xs {
		sq += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(sq / float64(len(xs)))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
