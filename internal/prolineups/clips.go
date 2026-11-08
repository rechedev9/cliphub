package prolineups

import (
	"fmt"
	"regexp"
	"strings"
)

// Seconds a throw clip shows before the release and after the grenade lands.
const (
	clipLeadSeconds = 3
	clipTailSeconds = 2
)

var (
	clipMatch   = regexp.MustCompile(`^\d{1,12}$`)
	clipSteamID = regexp.MustCompile(`^\d{17}$`)
	clipUnsafe  = regexp.MustCompile(`[^a-z0-9-]`)
	clipPath    = regexp.MustCompile(`^\d{1,12}/[a-z0-9-]{1,220}-r\d{1,3}-t\d{1,9}-\d{17}(-combo)?\.mp4$`)
)

// ValidClipName reports whether name has the shape ClipName gives, so it is safe as a path.
func ValidClipName(name string) bool { return clipPath.MatchString(name) }

// ClipName is the stored path of an example's throw clip: one folder per match,
// a file per map, round, tick and thrower. It is empty when the example cannot name one.
func ClipName(e Example) string {
	entry, ok := strings.CutPrefix(e.MapKey, e.Match+"/")
	if !ok || !clipMatch.MatchString(e.Match) || !clipSteamID.MatchString(e.SteamID64) || e.Round < 1 || e.Tick < 1 {
		return ""
	}
	entry = clipUnsafe.ReplaceAllString(strings.TrimSuffix(strings.ToLower(entry), ".dem"), "-")
	if strings.Trim(entry, "-") == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s-r%d-t%d-%s.mp4", e.Match, entry, e.Round, e.Tick, e.SteamID64)
}

// ComboClipName is the stored path of a combo example's clip: its throws one after another.
func ComboClipName(e Example) string {
	name := ClipName(e)
	if name == "" || len(e.Throws) < 2 {
		return ""
	}
	return strings.TrimSuffix(name, ".mp4") + "-combo.mp4"
}

// ThrowExample is one throw of a combo example as an example of its own, to name and record its clip.
func ThrowExample(e Example, t ThrowRef) Example {
	return Example{Match: e.Match, MapKey: e.MapKey, Event: e.Event, PlayedAt: e.PlayedAt, Team: e.Team, Player: t.Player,
		SteamID64: t.SteamID64, Round: e.Round, Tick: t.Tick, AirTime: t.AirTime}
}

// AttachClips marks the lineup and combo examples whose clip is recorded; each item's clip is its newest recorded example.
func AttachClips(idx *MapIndex, recorded func(name string) bool) {
	for i := range idx.Lineups {
		l := &idx.Lineups[i]
		l.Clip = ""
		for j := range l.Examples {
			e := &l.Examples[j]
			e.Clip = ""
			if name := ClipName(*e); name != "" && recorded(name) {
				e.Clip = name
				if l.Clip == "" {
					l.Clip = name
				}
			}
		}
	}
	for i := range idx.Combos {
		c := &idx.Combos[i]
		c.Clip = ""
		for j := range c.Examples {
			e := &c.Examples[j]
			e.Clip = ""
			if name := ComboClipName(*e); name != "" && recorded(name) {
				e.Clip = name
				if c.Clip == "" {
					c.Clip = name
				}
			}
		}
	}
}

// ClipWindow is the tick range a throw clip records: the wind-up, the release and the flight.
func ClipWindow(throwTick, tickRate int, airSeconds float64) (start, end int) {
	return throwTick - clipLeadSeconds*tickRate, throwTick + int((airSeconds+clipTailSeconds)*float64(tickRate))
}
