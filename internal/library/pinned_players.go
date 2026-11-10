package library

import (
	"fmt"
	"regexp"
)

// MaxPinnedPlayers bounds the Ranking's pinned list.
const MaxPinnedPlayers = 100

// maxPinnedNameRunes cuts the name kept beside a pin, shown when the player is not in the table.
const maxPinnedNameRunes = 64

var pinnedSteamIDPattern = regexp.MustCompile(`^[0-9]{17}$`)

// PinnedPlayer is a player the Ranking keeps on top of its table.
type PinnedPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
}

// CleanPinnedPlayers checks a whole pinned list: SteamID64s are 17 digits, a repeated one
// keeps its first place, and a name is trimmed and cut. The order is the table order.
func CleanPinnedPlayers(in []PinnedPlayer) ([]PinnedPlayer, error) {
	if len(in) > MaxPinnedPlayers {
		return nil, fmt.Errorf("at most %d pinned players", MaxPinnedPlayers)
	}
	seen := map[string]bool{}
	out := make([]PinnedPlayer, 0, len(in))
	for i, p := range in {
		if !pinnedSteamIDPattern.MatchString(p.SteamID64) {
			return nil, fmt.Errorf("player %d: steamid64 must be a 17 digit SteamID64", i+1)
		}
		if seen[p.SteamID64] {
			continue
		}
		seen[p.SteamID64] = true
		name := []rune(CleanName(p.Name))
		out = append(out, PinnedPlayer{SteamID64: p.SteamID64, Name: string(name[:min(len(name), maxPinnedNameRunes)])})
	}
	return out, nil
}
