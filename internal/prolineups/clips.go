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
)

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

// AttachClips marks the examples whose clip is recorded; a lineup's clip is its newest recorded example.
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
}

// ClipWindow is the tick range a throw clip records: the wind-up, the release and the flight.
func ClipWindow(throwTick, tickRate int, airSeconds float64) (start, end int) {
	return throwTick - clipLeadSeconds*tickRate, throwTick + int((airSeconds+clipTailSeconds)*float64(tickRate))
}
