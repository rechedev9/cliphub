package tacticalplan

import (
	"math"
	"slices"

	"github.com/google/uuid"
)

// FirstMovementDistance is how far, in world units, a player must be from the spot the round
// started on to count as having moved: one heatmap cell.
const FirstMovementDistance = 64.0

// FirstMovementLead is how many seconds before the move a replay of it opens.
const FirstMovementLead = 3.0

// FirstMovement is the moment a player first left their starting spot in one round.
type FirstMovement struct {
	MatchID   uuid.UUID
	Round     int
	SteamID64 string
	Name      string
	// Seconds is the time since freeze end.
	Seconds float64
	// Tick is where a replay of the move opens, a few seconds before it.
	Tick int
}

// FirstMovementFilter narrows the first movements of an overlay.
type FirstMovementFilter struct {
	// Players keeps these SteamID64s; empty keeps every track.
	Players []string
	// From and To bound the move on the round clock; To 0 runs to the round's end.
	From, To float64
	// Area keeps only the rounds in which a kept player passes through the zone; Geometry names its places.
	Area     *PatternArea
	Geometry MapGeometry
}

// FirstMovementSeconds is when a track first moves FirstMovementDistance from its first point.
func FirstMovementSeconds(track OverlayTrack, step float64) (float64, bool) {
	if len(track.Points) == 0 || step <= 0 {
		return 0, false
	}
	first := track.Points[0]
	for i, p := range track.Points[1:] {
		if math.Hypot(float64(p[0]-first[0]), float64(p[1]-first[1])) >= FirstMovementDistance {
			return float64(track.Start+i+1) * step, true
		}
	}
	return 0, false
}

// FirstMovements lists, round by round in the overlay's order, the first move of every kept player.
func FirstMovements(overlay Overlay, f FirstMovementFilter) []FirstMovement {
	out := []FirstMovement{}
	for _, round := range overlay.Rounds {
		kept := make([]OverlayTrack, 0, len(round.Tracks))
		for _, track := range round.Tracks {
			if len(f.Players) == 0 || slices.Contains(f.Players, track.SteamID64) {
				kept = append(kept, track)
			}
		}
		if f.Area != nil && !slices.ContainsFunc(kept, func(t OverlayTrack) bool { return passesThrough(t, *f.Area, f.Geometry) }) {
			continue
		}
		for _, track := range kept {
			seconds, moved := FirstMovementSeconds(track, overlay.StepSeconds)
			if !moved || seconds < f.From || (f.To > 0 && seconds > f.To) {
				continue
			}
			lead := FirstMovementLead * round.Tickrate
			out = append(out, FirstMovement{MatchID: round.MatchID, Round: round.Round, SteamID64: track.SteamID64, Name: track.Name, Seconds: seconds,
				Tick: max(round.TickFreezeEnd, round.TickFreezeEnd+int(math.Round(seconds*round.Tickrate-lead)))})
		}
	}
	return out
}

func passesThrough(track OverlayTrack, area PatternArea, g MapGeometry) bool {
	return slices.ContainsFunc(track.Points, func(p [3]int) bool {
		return area.Contains(g, [3]float64{float64(p[0]), float64(p[1]), float64(p[2])})
	})
}
