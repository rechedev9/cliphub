package library

import (
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Inferred roles. A role is a reading of the player's rounds, never a label
// the demo carries, so every role ships with the counts it was read from.
const (
	RoleAWP     = "awp"
	RoleEntry   = "entry"
	RoleSupport = "support"
	RoleRifler  = "rifler"
	RoleUnknown = "unknown"
)

// Role thresholds, in the order they are tried. They are part of the response
// so the UI can show the rule next to the numbers.
const (
	// RoleMinRounds is the sample below which no role is read.
	RoleMinRounds = 20
	// RoleAWPKillShare is the share of a player's kills made with the AWP.
	RoleAWPKillShare = 0.30
	// RoleEntryOpeningShare is the share of T rounds in which the player took
	// the opening duel. Five players share one duel, so 0.20 is the average.
	RoleEntryOpeningShare = 0.30
	// RoleSupportUtilityRatio is the player's grenades per round over the
	// grenades per round of their teammates in the same matches.
	RoleSupportUtilityRatio = 1.25
	// roleExampleLimit caps the example rounds kept for each metric.
	roleExampleLimit = 5
)

// RoleEvidence is one round a role metric counted, by match, round and tick.
type RoleEvidence struct {
	MatchID uuid.UUID `json:"match_id"`
	Round   int       `json:"round"`
	Tick    int       `json:"tick"`
}

// RoleMetric is a count over a denominator with the first rounds it counted.
type RoleMetric struct {
	Count    int            `json:"count"`
	Total    int            `json:"total"`
	Examples []RoleEvidence `json:"examples"`
}

// RoleThresholds are the rules InferPlayerRole applied.
type RoleThresholds struct {
	MinRounds           int     `json:"min_rounds"`
	AWPKillShare        float64 `json:"awp_kill_share"`
	EntryOpeningShare   float64 `json:"entry_opening_share"`
	SupportUtilityRatio float64 `json:"support_utility_ratio"`
}

// PlayerRole is a player's inferred role and the metrics behind it.
type PlayerRole struct {
	SteamID64  string `json:"steamid64"`
	Role       string `json:"role"`
	MatchCount int    `json:"match_count"`
	Rounds     int    `json:"rounds"`
	// AWPKills counts AWP kills over all kills.
	AWPKills RoleMetric `json:"awp_kills"`
	// TOpenings counts T rounds with the player in the opening duel over T rounds.
	TOpenings RoleMetric `json:"t_openings"`
	// Utility counts the player's grenades over the player's rounds.
	Utility RoleMetric `json:"utility"`
	// TeamUtility counts the teammates' grenades over the teammates' rounds.
	TeamUtility RoleMetric     `json:"team_utility"`
	Thresholds  RoleThresholds `json:"thresholds"`
}

// RoleDocument is one match's tactical document, named by its library match.
type RoleDocument struct {
	MatchID  uuid.UUID
	Document tacticalplan.Document
}

func isUtilityEvent(kind tacticalplan.EventKind) bool {
	switch kind {
	case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov:
		return true
	default:
		return false
	}
}

func (m *RoleMetric) add(e RoleEvidence) {
	m.Count++
	if len(m.Examples) < roleExampleLimit {
		m.Examples = append(m.Examples, e)
	}
}

// share is count over total, or 0 for an empty total.
func (m RoleMetric) share() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.Count) / float64(m.Total)
}

// InferPlayerRole reads a player's role from the tactical documents of the
// matches they played. It is a pure function of the documents and their
// order: the examples are the first rounds counted, in the order given.
func InferPlayerRole(steamID string, docs []RoleDocument) PlayerRole {
	role := PlayerRole{
		SteamID64: steamID,
		Role:      RoleUnknown,
		AWPKills:  RoleMetric{Examples: []RoleEvidence{}}, TOpenings: RoleMetric{Examples: []RoleEvidence{}},
		Utility: RoleMetric{Examples: []RoleEvidence{}}, TeamUtility: RoleMetric{Examples: []RoleEvidence{}},
		Thresholds: RoleThresholds{
			MinRounds: RoleMinRounds, AWPKillShare: RoleAWPKillShare,
			EntryOpeningShare: RoleEntryOpeningShare, SupportUtilityRatio: RoleSupportUtilityRatio,
		},
	}
	for _, entry := range docs {
		doc := entry.Document
		slot, teamKey, ok := playerSlot(doc, steamID)
		if !ok {
			continue
		}
		role.MatchCount++
		teammates := map[uint8]bool{}
		for _, p := range doc.Players {
			if p.TeamKey == teamKey && p.Slot != slot {
				teammates[p.Slot] = true
			}
		}
		for _, round := range doc.Rounds {
			played := false
			for _, pr := range round.Players {
				switch {
				case pr.Slot == slot:
					played = true
					role.Rounds++
					role.Utility.Total++
					if pr.Side == tacticalplan.SideT {
						role.TOpenings.Total++
						if pr.OpeningKill || pr.OpeningDeath {
							role.TOpenings.add(RoleEvidence{MatchID: entry.MatchID, Round: round.Number, Tick: round.Class.OpeningTick})
						}
					}
				case teammates[pr.Slot]:
					role.TeamUtility.Total++
				}
			}
			if !played {
				continue
			}
			for _, ev := range round.Events {
				if ev.ActorSlot == nil {
					continue
				}
				evidence := RoleEvidence{MatchID: entry.MatchID, Round: round.Number, Tick: ev.Tick}
				actor := *ev.ActorSlot
				switch {
				case ev.Kind == tacticalplan.EventKill && actor == slot:
					role.AWPKills.Total++
					if ev.Weapon == "awp" {
						role.AWPKills.add(evidence)
					}
				case isUtilityEvent(ev.Kind) && actor == slot:
					role.Utility.add(evidence)
				case isUtilityEvent(ev.Kind) && teammates[actor]:
					role.TeamUtility.add(evidence)
				}
			}
		}
	}
	role.Role = readRole(role)
	return role
}

func playerSlot(doc tacticalplan.Document, steamID string) (uint8, string, bool) {
	for _, p := range doc.Players {
		if p.SteamID64 == steamID {
			return p.Slot, p.TeamKey, true
		}
	}
	return 0, "", false
}

func readRole(r PlayerRole) string {
	if r.Rounds < RoleMinRounds {
		return RoleUnknown
	}
	if r.AWPKills.share() >= RoleAWPKillShare {
		return RoleAWP
	}
	if r.TOpenings.share() >= RoleEntryOpeningShare {
		return RoleEntry
	}
	if team := r.TeamUtility.share(); team > 0 && r.Utility.share()/team >= RoleSupportUtilityRatio {
		return RoleSupport
	}
	return RoleRifler
}
