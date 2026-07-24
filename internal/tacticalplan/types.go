// Package tacticalplan defines the durable tactical-analysis document emitted
// from a demo: the round index with its deterministic classification, the
// per-round event list, the map geometry derived from observed play, and the
// descriptor of the sidecar position blob.
//
// The package is pure schema plus the filtering and aggregation defined over
// it. It never imports a demo parser, so the HTTP API, the CLI, and the web
// proxy can all read a document without pulling the parsing stack in.
package tacticalplan

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SchemaVersion identifies the tactical document format. Readers must reject
// versions they do not understand rather than render a partial analysis.
const SchemaVersion = "1.0"

// Side is a playing side. Spectators never appear in a tactical document.
type Side string

// The two playing sides.
const (
	SideCT Side = "CT"
	SideT  Side = "T"
)

// Opponent returns the other side. The zero Side answers the zero Side.
func (s Side) Opponent() Side {
	switch s {
	case SideCT:
		return SideT
	case SideT:
		return SideCT
	default:
		return ""
	}
}

// BuyType is a team's economic state for one round. The vocabulary matches the
// one the CS analysis ecosystem converged on, so numbers stay comparable with
// published team statistics.
type BuyType string

// Buy types, from poorest to richest. ForceBuy is deliberately not a value
// band: it is a half-buy made after losing the previous round while broke,
// which separates desperation from a planned save.
const (
	BuyPistol  BuyType = "pistol"
	BuyEco     BuyType = "eco"
	BuySemi    BuyType = "semi"
	BuyForce   BuyType = "force"
	BuyFull    BuyType = "full"
	BuyUnknown BuyType = "unknown"
)

// BuyTypes lists every buy type in economic order.
var BuyTypes = []BuyType{BuyPistol, BuyEco, BuySemi, BuyForce, BuyFull, BuyUnknown}

// TPattern is the attacking side's round shape.
type TPattern string

// T-side round patterns. Anti-eco is not among them: it describes the economic
// matchup, not the round's shape, and is carried as a tag.
const (
	TExecute TPattern = "execute"  // utility burst then a committed site take
	TDefault TPattern = "default"  // map control first, late commit
	TSplit   TPattern = "split"    // one site entered from two corridors
	TFast    TPattern = "fast"     // site contact inside the opening window
	TEcoRush TPattern = "eco_rush" // eco or force with an immediate commit
	TSave    TPattern = "save"     // no committed take; equipment preserved
	TUnknown TPattern = "unknown"  // not enough evidence to classify
)

// CTPattern is the defending side's round shape.
type CTPattern string

// CT-side round patterns.
const (
	CTHold       CTPattern = "hold"       // standard default setup held
	CTRetake     CTPattern = "retake"     // bomb planted, site retaken or attempted
	CTAggression CTPattern = "aggression" // early off-site contact in T territory
	CTStack      CTPattern = "stack"      // three or more defenders on one site
	CTSave       CTPattern = "save"       // equipment preserved after losing map control
	CTUnknown    CTPattern = "unknown"
)

// Site names a bombsite or the neutral middle of the map.
type Site string

// Sites. SiteNone means the round produced no site commitment at all.
const (
	SiteA    Site = "A"
	SiteB    Site = "B"
	SiteMid  Site = "mid"
	SiteNone Site = "none"
)

// EventKind is the closed vocabulary of round events carried in the index.
// Per-tick movement lives in the position blob, never here.
type EventKind string

// Round event kinds.
const (
	EventKill     EventKind = "kill"
	EventPlant    EventKind = "plant"
	EventDefuse   EventKind = "defuse"
	EventExplode  EventKind = "explode"
	EventSmoke    EventKind = "smoke"
	EventFlash    EventKind = "flash"
	EventHE       EventKind = "he"
	EventMolotov  EventKind = "molotov"
	EventDecoy    EventKind = "decoy"
	EventDropBomb EventKind = "bomb_drop"
)

// Round tags are orthogonal facts that do not fit the pattern vocabulary.
const (
	TagPostPlant     = "postplant"
	TagRetakeWon     = "retake_won"
	TagFullSave      = "full_save"
	TagAce           = "ace"
	TagOpeningTraded = "opening_traded"
	TagAntiEco       = "anti_eco"
	TagOvertime      = "overtime"
	TagPistol        = "pistol"
	TagTimeout       = "time_expired"
)

// Document is the top-level tactical analysis of one demo.
type Document struct {
	SchemaVersion string      `json:"schema_version"`
	GeneratedAt   time.Time   `json:"generated_at"`
	JobID         uuid.UUID   `json:"job_id,omitempty"`
	Demo          Demo        `json:"demo"`
	Teams         []Team      `json:"teams"`
	Players       []Player    `json:"players"`
	Rounds        []Round     `json:"rounds"`
	Geometry      MapGeometry `json:"geometry"`
	Positions     Positions   `json:"positions"`
	// Warnings records every ambiguity the scan resolved by convention, so a
	// surprising number can be traced instead of guessed at.
	Warnings []string `json:"warnings,omitempty"`
}

// Demo is the source demo's identity and timing.
type Demo struct {
	Path          string  `json:"path"`
	SHA256        string  `json:"sha256"`
	Map           string  `json:"map"`
	Tickrate      float64 `json:"tickrate"`
	DurationTicks int     `json:"duration_ticks"`
	Format        string  `json:"format"`
	// MaxRounds is the detected regulation length (24 for MR12, 30 for MR15),
	// and OvertimeRounds the detected overtime half length. Both drive half
	// detection, which drives pistol-round classification.
	MaxRounds       int `json:"max_rounds"`
	OvertimeRounds  int `json:"overtime_rounds,omitempty"`
	RegulationEnded int `json:"regulation_ended_round,omitempty"`
}

// Team is a team identity, keyed so tendencies can later be aggregated across
// several demos of the same opponent. Sides swap at halftime, so a team is
// identified by clan name and by the players it fielded, never by side.
type Team struct {
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	StartSide Side     `json:"start_side"`
	Slots     []uint8  `json:"slots"`
	SteamIDs  []string `json:"steamids"`
}

// Player is the identity table. Every per-tick sample and every event refers to
// a Slot rather than a SteamID: it is the single biggest size win in the blob
// and the only stable key across a name change.
type Player struct {
	Slot      uint8  `json:"slot"`
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	TeamKey   string `json:"team_key"`
	StartSide Side   `json:"start_side"`
}

// Round is one played round: its tick boundaries, its outcome, its economy,
// its deterministic classification, and everything that happened in it.
type Round struct {
	Number        int    `json:"number"`
	TickStart     int    `json:"tick_start"`
	TickFreezeEnd int    `json:"tick_freeze_end"`
	TickEnd       int    `json:"tick_end"`
	TickOfficial  int    `json:"tick_official"`
	ScoreCTBefore int    `json:"score_ct_before"`
	ScoreTBefore  int    `json:"score_t_before"`
	Winner        Side   `json:"winner"`
	EndReason     string `json:"end_reason"`
	Half          int    `json:"half"`
	Overtime      int    `json:"overtime,omitempty"`

	Bomb    *Bomb         `json:"bomb,omitempty"`
	Economy Economy       `json:"economy"`
	Class   Class         `json:"class"`
	Players []PlayerRound `json:"players"`
	Events  []Event       `json:"events"`
}

// Duration returns the round's live duration in seconds at the given tickrate,
// measured from freeze-time end to the round-end event.
func (r Round) Duration(tickrate float64) float64 {
	if tickrate <= 0 || r.TickEnd <= r.TickFreezeEnd {
		return 0
	}
	return float64(r.TickEnd-r.TickFreezeEnd) / tickrate
}

// Bomb records the bomb's fate. A round with no plant carries no Bomb.
type Bomb struct {
	PlantTick   int    `json:"plant_tick"`
	Site        Site   `json:"site"`
	PlanterSlot *uint8 `json:"planter_slot,omitempty"`
	DefuseTick  int    `json:"defuse_tick,omitempty"`
	DefuserSlot *uint8 `json:"defuser_slot,omitempty"`
	ExplodeTick int    `json:"explode_tick,omitempty"`
}

// Economy is both sides' buy state, sampled once per round.
type Economy struct {
	CTEquipValue int     `json:"ct_equip_value"`
	TEquipValue  int     `json:"t_equip_value"`
	CTMoney      int     `json:"ct_money"`
	TMoney       int     `json:"t_money"`
	CTBuy        BuyType `json:"ct_buy"`
	TBuy         BuyType `json:"t_buy"`
	// SampleTick is the tick the values were read at: freeze-time end plus a
	// fixed delay, because players keep buying for a few seconds after it.
	SampleTick int `json:"sample_tick"`
}

// Buy returns one side's buy type.
func (e Economy) Buy(side Side) BuyType {
	if side == SideCT {
		return e.CTBuy
	}
	return e.TBuy
}

// EquipValue returns one side's sampled equipment value.
func (e Economy) EquipValue(side Side) int {
	if side == SideCT {
		return e.CTEquipValue
	}
	return e.TEquipValue
}

// Class is the deterministic round taxonomy. Every field is a closed
// vocabulary, and Reasons records why the classifier landed where it did so a
// rule change shows up as a reviewable diff rather than a silent relabelling.
type Class struct {
	TSide         TPattern  `json:"t_side"`
	CTSide        CTPattern `json:"ct_side"`
	Site          Site      `json:"site"`
	OpeningSlot   *uint8    `json:"opening_slot,omitempty"`
	OpeningSide   Side      `json:"opening_side,omitempty"`
	OpeningTick   int       `json:"opening_tick,omitempty"`
	OpeningTraded bool      `json:"opening_traded"`
	// FirstContactTick is when the round's first kill happened, the standard
	// proxy for when map control turned into a fight.
	FirstContactTick int      `json:"first_contact_tick,omitempty"`
	Tags             []string `json:"tags"`
	Reasons          []string `json:"reasons"`
}

// HasTag reports whether the round carries a tag.
func (c Class) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// PlayerRound is one player's round: what they bought, what they did, and how
// the opening duel and the trade went.
type PlayerRound struct {
	Slot         uint8 `json:"slot"`
	Side         Side  `json:"side"`
	Kills        int   `json:"kills"`
	Deaths       int   `json:"deaths"`
	Assists      int   `json:"assists"`
	Damage       int   `json:"damage"`
	EquipValue   int   `json:"equip_value"`
	Money        int   `json:"money"`
	DeathTick    int   `json:"death_tick,omitempty"`
	Survived     bool  `json:"survived"`
	OpeningKill  bool  `json:"opening_kill,omitempty"`
	OpeningDeath bool  `json:"opening_death,omitempty"`
	// Traded reports that this player died and a teammate killed the killer
	// inside the trade window; TradeKills counts trades this player made.
	Traded     bool `json:"traded,omitempty"`
	TradeKills int  `json:"trade_kills,omitempty"`
}

// Event is one thing that happened at a tick. Positions are world coordinates,
// transformed to radar pixels by the reader.
type Event struct {
	Tick          int        `json:"tick"`
	Kind          EventKind  `json:"kind"`
	ActorSlot     *uint8     `json:"actor_slot,omitempty"`
	TargetSlot    *uint8     `json:"target_slot,omitempty"`
	Side          Side       `json:"side,omitempty"`
	Weapon        string     `json:"weapon,omitempty"`
	Pos           [3]float64 `json:"pos"`
	TargetPos     [3]float64 `json:"target_pos,omitempty"`
	Place         string     `json:"place,omitempty"`
	Site          Site       `json:"site,omitempty"`
	Headshot      bool       `json:"headshot,omitempty"`
	Wallbang      bool       `json:"wallbang,omitempty"`
	ThroughSmoke  bool       `json:"through_smoke,omitempty"`
	AttackerBlind bool       `json:"attacker_blind,omitempty"`
	NoScope       bool       `json:"no_scope,omitempty"`
	Traded        bool       `json:"traded,omitempty"`
	Opening       bool       `json:"opening,omitempty"`
}

// MarshalJSON guarantees every emitted document carries the current
// SchemaVersion, even when the caller built the Document as a zero value.
func (d Document) MarshalJSON() ([]byte, error) {
	type alias Document
	d.SchemaVersion = SchemaVersion
	return json.Marshal(alias(d))
}

// NewDocument returns a Document pre-populated with the current schema version
// and a UTC timestamp.
func NewDocument() Document {
	return Document{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Rounds:        []Round{},
		Players:       []Player{},
		Teams:         []Team{},
	}
}

// PlayerBySlot returns the identity for a slot.
func (d Document) PlayerBySlot(slot uint8) (Player, bool) {
	for _, p := range d.Players {
		if p.Slot == slot {
			return p, true
		}
	}
	return Player{}, false
}

// RoundByNumber returns the round with the given 1-based number.
func (d Document) RoundByNumber(number int) (Round, bool) {
	for _, r := range d.Rounds {
		if r.Number == number {
			return r, true
		}
	}
	return Round{}, false
}

// TeamSide returns the side a team played in a given round. Teams swap sides at
// half time, so this is the only correct way to attribute a round to a team.
func (d Document) TeamSide(teamKey string, round Round) Side {
	for _, t := range d.Teams {
		if t.Key != teamKey {
			continue
		}
		if round.Half%2 == 1 {
			return t.StartSide
		}
		return t.StartSide.Opponent()
	}
	return ""
}
