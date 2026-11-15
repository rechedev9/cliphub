// Package moments derives reviewable, scored clip candidates from kill plans.
package moments

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
)

const SchemaVersion = "1.0"
const DefaultVariant = "viral-60-clean"

func ArtifactKey(jobID uuid.UUID) string {
	return artifacts.MomentsKey(jobID)
}

type Document struct {
	SchemaVersion string    `json:"schema_version"`
	JobID         uuid.UUID `json:"job_id"`
	GeneratedAt   time.Time `json:"generated_at"`
	Moments       []Moment  `json:"moments"`
}

type Moment struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	SegmentID string `json:"segment_id"`
	Player    string `json:"player,omitempty"`
	// SteamID64 is the moment's player, set only in multi-POV plans.
	SteamID64 string `json:"steamid64,omitempty"`
	Map       string `json:"map,omitempty"`
	Round     int    `json:"round"`
	Side      string `json:"side,omitempty"`
	ClutchVs  int    `json:"clutch_vs,omitempty"`
	// ClutchWon says whether the player's team took a clutch round; nil when not a clutch or unknown.
	ClutchWon *bool `json:"clutch_won,omitempty"`
	// WinProbSwing adds the round-win chance the play's kills gave the player's side.
	WinProbSwing    float64        `json:"win_prob_swing,omitempty"`
	TickStart       int            `json:"tick_start"`
	TickEnd         int            `json:"tick_end"`
	TimeStart       float64        `json:"time_start_seconds,omitempty"`
	TimeEnd         float64        `json:"time_end_seconds,omitempty"`
	DurationSeconds float64        `json:"duration_seconds"`
	Score           float64        `json:"score"`
	ReasonCodes     []string       `json:"reason_codes"`
	Events          Events         `json:"events"`
	Weapons         []string       `json:"weapons,omitempty"`
	Victims         []string       `json:"victims,omitempty"`
	Utility         []UtilityEvent `json:"utility,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
	DefaultVariant  string         `json:"default_variant"`
	Death           *Death         `json:"death,omitempty"`
}

// Death is how the target died in a death clip.
type Death struct {
	Tick     int    `json:"tick"`
	Killer   string `json:"killer,omitempty"`
	Weapon   string `json:"weapon,omitempty"`
	Headshot bool   `json:"headshot,omitempty"`
}

type Events struct {
	Kills          int      `json:"kills"`
	Headshots      int      `json:"headshots,omitempty"`
	Wallbangs      int      `json:"wallbangs,omitempty"`
	NoScopes       int      `json:"no_scopes,omitempty"`
	ThroughSmoke   int      `json:"through_smoke,omitempty"`
	AttackerBlind  int      `json:"attacker_blind,omitempty"`
	Weapons        []string `json:"weapons,omitempty"`
	Utility        int      `json:"utility,omitempty"`
	KnownLineups   int      `json:"known_lineups,omitempty"`
	UnknownLineups int      `json:"unknown_lineups,omitempty"`
}

type UtilityEvent struct {
	ID          string  `json:"id,omitempty"`
	Type        string  `json:"type,omitempty"`
	Destination string  `json:"destination,omitempty"`
	FromArea    string  `json:"from_area,omitempty"`
	Side        string  `json:"side,omitempty"`
	Action      string  `json:"action,omitempty"`
	ThrowPlace  string  `json:"throw_place,omitempty"`
	ThrowTick   int     `json:"throw_tick,omitempty"`
	PopTick     int     `json:"pop_tick,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
	KnownLineup bool    `json:"known_lineup"`
}

func Build(jobID uuid.UUID, plan killplan.Plan) Document {
	return Document{
		SchemaVersion: SchemaVersion,
		JobID:         jobID,
		GeneratedAt:   time.Now().UTC(),
		Moments:       Rank(plan),
	}
}

// BuildDeaths lists the clips of a death plan in match order: a death reel
// follows the match, so it is not ranked.
func BuildDeaths(jobID uuid.UUID, plan killplan.Plan) Document {
	out := make([]Moment, 0, len(plan.Segments))
	for i, segment := range plan.Segments {
		moment := buildMoment(i+1, segment, plan)
		if d := segment.Death; d != nil {
			moment.Death = &Death{Tick: d.Tick, Killer: d.Killer.NameInDemo, Weapon: d.Weapon, Headshot: d.Headshot}
		}
		out = append(out, moment)
	}
	return Document{SchemaVersion: SchemaVersion, JobID: jobID, GeneratedAt: time.Now().UTC(), Moments: out}
}

// Rank scores every planned segment and returns a deterministic best-first
// order. Segment ID is the stable tie-breaker so capture, CLI review, and
// editor compilation cannot disagree when scores are equal.
func Rank(plan killplan.Plan) []Moment {
	ranked := make([]Moment, 0, len(plan.Segments))
	for i, segment := range plan.Segments {
		ranked = append(ranked, buildMoment(i+1, segment, plan))
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].SegmentID < ranked[j].SegmentID
	})
	return ranked
}

func buildMoment(index int, segment killplan.Segment, plan killplan.Plan) Moment {
	tickrate := plan.Demo.Tickrate
	events := segmentEvents(segment)
	fast := fastMultiKill(segment, tickrate)
	clutchWon := segmentClutchWon(segment)
	reasons := reasonCodes(segment, events)
	if fast != "" {
		reasons = insertReason(reasons, fast)
	}
	if clutchWon != nil && *clutchWon {
		reasons = insertReason(reasons, ReasonClutchWon)
	}
	warnings := warnings(segment, tickrate)
	duration := durationSeconds(segment, tickrate)
	timeStart, timeEnd := timeRangeSeconds(segment, tickrate)
	swing := segmentWinSwing(segment)
	return Moment{
		ID:              fmt.Sprintf("mom-%03d", index),
		Source:          "killplan",
		SegmentID:       segment.ID,
		Player:          plan.SegmentPOV(segment).NameInDemo,
		SteamID64:       multiPOVSteamID(plan, segment),
		Map:             plan.Demo.Map,
		Round:           segment.Round,
		Side:            segmentSide(segment),
		ClutchVs:        segmentClutchVs(segment),
		ClutchWon:       clutchWon,
		WinProbSwing:    swing,
		TickStart:       segment.TickStart,
		TickEnd:         segment.TickEnd,
		TimeStart:       timeStart,
		TimeEnd:         timeEnd,
		DurationSeconds: duration,
		Score:           score(scoreInput{events: events, duration: duration, clutchVs: segmentClutchVs(segment), clutchWon: clutchWon, fast: fast, swing: swing}),
		ReasonCodes:     reasons,
		Events:          events,
		Weapons:         append([]string(nil), events.Weapons...),
		Victims:         victims(segment),
		Utility:         utilityEvents(segment),
		Warnings:        warnings,
		DefaultVariant:  DefaultVariant,
	}
}

func segmentEvents(segment killplan.Segment) Events {
	weaponSet := map[string]bool{}
	events := Events{
		Kills:   len(segment.Kills),
		Utility: len(segment.Utility),
	}
	for _, kill := range segment.Kills {
		weapon := strings.TrimSpace(strings.ToLower(kill.Weapon))
		if weapon != "" {
			weaponSet[weapon] = true
		}
		if kill.Headshot {
			events.Headshots++
		}
		if kill.Wallbang {
			events.Wallbangs++
		}
		if kill.NoScope {
			events.NoScopes++
		}
		if kill.ThroughSmoke {
			events.ThroughSmoke++
		}
		if kill.AttackerBlind {
			events.AttackerBlind++
		}
	}
	for _, u := range segment.Utility {
		if u.LineupMatch != nil && u.LineupMatch.ID != "" {
			events.KnownLineups++
		} else {
			events.UnknownLineups++
		}
	}
	for weapon := range weaponSet {
		events.Weapons = append(events.Weapons, weapon)
	}
	sort.Strings(events.Weapons)
	return events
}

// segmentSide is the target's side, CT or T, at the segment's first event.
func segmentSide(segment killplan.Segment) string {
	team := ""
	if len(segment.Kills) > 0 {
		team = segment.Kills[0].Killer.TeamAtKill
	} else if len(segment.Utility) > 0 {
		team = segment.Utility[0].Thrower.TeamAtKill
	}
	switch team {
	case SideCT, SideT:
		return team
	default:
		return ""
	}
}

// segmentClutchVs is the largest clutch a segment's kills were made in.
func segmentClutchVs(segment killplan.Segment) int {
	vs := 0
	for _, kill := range segment.Kills {
		vs = max(vs, kill.ClutchVs)
	}
	return vs
}

func victims(segment killplan.Segment) []string {
	set := map[string]bool{}
	for _, kill := range segment.Kills {
		name := strings.TrimSpace(kill.Victim.NameInDemo)
		if name != "" {
			set[name] = true
		}
	}
	out := make([]string, 0, len(set))
	for victim := range set {
		out = append(out, victim)
	}
	sort.Strings(out)
	return out
}

func utilityEvents(segment killplan.Segment) []UtilityEvent {
	out := make([]UtilityEvent, 0, len(segment.Utility))
	for _, u := range segment.Utility {
		event := UtilityEvent{
			ID:         u.ID,
			Type:       u.Type,
			Action:     u.ThrowAction,
			ThrowPlace: u.ThrowPlace,
			ThrowTick:  u.ThrowTick,
			PopTick:    u.PopTick,
		}
		if u.LineupMatch != nil {
			event.Destination = u.LineupMatch.Destination
			event.FromArea = u.LineupMatch.FromArea
			event.Side = u.LineupMatch.Side
			event.Confidence = u.LineupMatch.Confidence
			event.KnownLineup = u.LineupMatch.ID != ""
		}
		out = append(out, event)
	}
	return out
}

func reasonCodes(segment killplan.Segment, events Events) []string {
	reasons := map[string]bool{}
	if events.Kills >= 2 {
		reasons["multi_kill"] = true
	}
	if events.Kills >= 4 {
		reasons["quad"] = true
	}
	if events.Kills >= 5 {
		reasons["ace"] = true
	}
	if segmentClutchVs(segment) > 0 {
		reasons["clutch"] = true
	}
	if events.Headshots > 0 {
		reasons["headshot"] = true
	}
	if events.Wallbangs > 0 {
		reasons["wallbang"] = true
	}
	if events.NoScopes > 0 {
		reasons[ReasonNoScope] = true
	}
	if events.ThroughSmoke > 0 {
		reasons[ReasonThroughSmoke] = true
	}
	if events.AttackerBlind > 0 {
		reasons[ReasonAttackerBlind] = true
	}
	for _, weapon := range events.Weapons {
		switch weapon {
		case "awp", "weapon_awp":
			reasons["awp"] = true
		case "ak47", "weapon_ak47", "m4a1", "m4a1_silencer", "weapon_m4a1", "weapon_m4a1_silencer":
			reasons["rifle"] = true
		case "deagle", "weapon_deagle", "usp_silencer", "glock", "weapon_usp_silencer", "weapon_glock":
			reasons["pistol"] = true
		}
	}
	if len(segment.Utility) > 0 {
		reasons["utility_lineup"] = true
	}
	if events.KnownLineups > 0 {
		reasons["known_lineup"] = true
	}
	if events.UnknownLineups > 0 && len(segment.Utility) > 0 {
		reasons["unmatched_lineup"] = true
	}
	out := make([]string, 0, len(reasons))
	for reason := range reasons {
		out = append(out, reason)
	}
	sort.Strings(out)
	return out
}

func warnings(segment killplan.Segment, tickrate int) []string {
	var out []string
	if segment.ID == "" {
		out = append(out, "missing_segment_id")
	}
	if tickrate <= 0 {
		out = append(out, "missing_tickrate")
	}
	if segment.TickEnd <= segment.TickStart {
		out = append(out, "invalid_tick_range")
	}
	if len(segment.Kills) == 0 && len(segment.Utility) == 0 && segment.Death == nil {
		out = append(out, "empty_segment")
	}
	return out
}

func durationSeconds(segment killplan.Segment, tickrate int) float64 {
	if tickrate <= 0 || segment.TickEnd <= segment.TickStart {
		return 0
	}
	return math.Round((float64(segment.TickEnd-segment.TickStart)/float64(tickrate))*1000) / 1000
}

func timeRangeSeconds(segment killplan.Segment, tickrate int) (float64, float64) {
	if tickrate <= 0 {
		return 0, 0
	}
	start := math.Round((float64(segment.TickStart)/float64(tickrate))*1000) / 1000
	end := math.Round((float64(segment.TickEnd)/float64(tickrate))*1000) / 1000
	return start, end
}

type scoreInput struct {
	events    Events
	duration  float64
	clutchVs  int
	clutchWon *bool
	fast      string
	swing     float64
}

// fastBonus weighs each named fast multi-kill; a play takes only its best one.
var fastBonus = map[string]float64{ReasonFastDouble: 0.06, ReasonFastTriple: 0.12, ReasonFastQuad: 0.18, ReasonFastAce: 0.25}

// score adds points for what makes a play worth watching and maps them to
// 0..1 with 1-e^-x, so big plays keep their order instead of tying at a cap.
func score(in scoreInput) float64 {
	e := in.events
	points := 0.2
	points += float64(e.Kills) * 0.22
	points += float64(e.Headshots) * 0.06
	points += float64(e.Wallbangs) * 0.08
	points += float64(e.NoScopes) * 0.1
	points += float64(e.ThroughSmoke) * 0.08
	points += float64(e.AttackerBlind) * 0.08
	points += float64(e.KnownLineups) * 0.18
	points += float64(e.UnknownLineups) * 0.08
	if in.duration > 0 && in.duration <= 18 {
		points += 0.08
	}
	if in.clutchVs > 0 {
		points += 0.08 * float64(in.clutchVs)
		if in.clutchWon != nil && *in.clutchWon {
			points += 0.15 + 0.05*float64(in.clutchVs)
		}
	}
	points += fastBonus[in.fast]
	points += 0.6 * in.swing
	value := 1 - math.Exp(-math.Max(points, 0))
	return math.Round(value*1000) / 1000
}

func multiPOVSteamID(plan killplan.Plan, segment killplan.Segment) string {
	if !plan.IsMultiPOV() {
		return ""
	}
	return plan.SegmentPOV(segment).SteamID64
}
