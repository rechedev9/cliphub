package recapplan

const (
	HoldEvidenceVersion = 1
	HoldSampleHz        = 8
	MaxKillHolds        = 10000
)

type HoldEvidence struct {
	Version int        `json:"version"`
	Kills   []KillHold `json:"kills"`
}

type KillHold struct {
	Tick            int     `json:"tick"`
	VictimSteamID64 string  `json:"victim_steamid64"`
	MoveUnits       float64 `json:"move_units"`
	Covered         bool    `json:"covered"`
}

func (f Facts) Hash() (string, error) {
	f.Holds = nil
	return HashValue(f)
}

type holdKey struct {
	tick   int
	victim string
}

func (h *HoldEvidence) byKill() (map[holdKey]KillHold, bool) {
	if h == nil || h.Version != HoldEvidenceVersion {
		return nil, false
	}
	holds := make(map[holdKey]KillHold, len(h.Kills))
	for _, hold := range h.Kills {
		key := holdKey{hold.Tick, hold.VictimSteamID64}
		if _, seen := holds[key]; !seen {
			holds[key] = hold
		}
	}
	return holds, true
}
