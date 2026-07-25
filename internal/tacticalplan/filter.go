package tacticalplan

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Outcome selects rounds by result, seen from the filter's perspective.
type Outcome string

// Round outcomes.
const (
	OutcomeWin  Outcome = "win"
	OutcomeLoss Outcome = "loss"
)

// Phase selects regulation or overtime rounds.
type Phase string

// Match phases.
const (
	PhaseRegulation Phase = "regulation"
	PhaseOvertime   Phase = "overtime"
)

// Filter selects rounds. Every field is optional and they compose with AND;
// within a field, values compose with OR.
//
// Most fields are read from a perspective: the side a team was playing that
// round. Set TeamKey to follow one team across the side swap, or Side to look
// at one side of the server regardless of who was on it. With neither, a
// perspective-dependent field matches if either side satisfies it.
type Filter struct {
	TeamKey      string      `json:"team_key,omitempty"`
	Side         Side        `json:"side,omitempty"`
	Buys         []BuyType   `json:"buys,omitempty"`
	OpponentBuys []BuyType   `json:"opponent_buys,omitempty"`
	Sites        []Site      `json:"sites,omitempty"`
	Outcome      Outcome     `json:"outcome,omitempty"`
	TPatterns    []TPattern  `json:"t_patterns,omitempty"`
	CTPatterns   []CTPattern `json:"ct_patterns,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
	Slots        []uint8     `json:"slots,omitempty"`
	RoundFrom    int         `json:"round_from,omitempty"`
	RoundTo      int         `json:"round_to,omitempty"`
	Phase        Phase       `json:"phase,omitempty"`
}

// Empty reports whether the filter selects everything.
func (f Filter) Empty() bool {
	return f.TeamKey == "" && f.Side == "" && len(f.Buys) == 0 && len(f.OpponentBuys) == 0 &&
		len(f.Sites) == 0 && f.Outcome == "" && len(f.TPatterns) == 0 && len(f.CTPatterns) == 0 &&
		len(f.Tags) == 0 && len(f.Slots) == 0 && f.RoundFrom == 0 && f.RoundTo == 0 && f.Phase == ""
}

// Perspective returns the side the filter looks from in a given round, or the
// empty Side when the filter is side-agnostic.
func (f Filter) Perspective(d Document, r Round) Side {
	if f.TeamKey != "" {
		return d.TeamSide(f.TeamKey, r)
	}
	return f.Side
}

// Match reports whether a round passes the filter.
func (f Filter) Match(d Document, r Round) bool {
	if f.RoundFrom > 0 && r.Number < f.RoundFrom {
		return false
	}
	if f.RoundTo > 0 && r.Number > f.RoundTo {
		return false
	}
	switch f.Phase {
	case PhaseRegulation:
		if r.Overtime > 0 {
			return false
		}
	case PhaseOvertime:
		if r.Overtime == 0 {
			return false
		}
	}

	side := f.Perspective(d, r)
	if f.TeamKey != "" && side == "" {
		// The team did not play this round (a series document, another map).
		return false
	}

	if len(f.Buys) > 0 && !matchBuy(r.Economy, side, f.Buys) {
		return false
	}
	if len(f.OpponentBuys) > 0 && !matchBuy(r.Economy, side.Opponent(), f.OpponentBuys) {
		return false
	}
	if len(f.Sites) > 0 && !containsSite(f.Sites, r.Class.Site) {
		return false
	}
	if f.Outcome != "" && !matchOutcome(f.Outcome, side, r.Winner) {
		return false
	}
	if len(f.TPatterns) > 0 && !containsPattern(f.TPatterns, r.Class.TSide) {
		return false
	}
	if len(f.CTPatterns) > 0 && !containsCTPattern(f.CTPatterns, r.Class.CTSide) {
		return false
	}
	for _, tag := range f.Tags {
		if !r.Class.HasTag(tag) {
			return false
		}
	}
	if len(f.Slots) > 0 && !matchSlots(r, f.Slots, side) {
		return false
	}
	return true
}

// Apply returns the rounds that pass the filter, in document order.
func (f Filter) Apply(d Document) []Round {
	out := make([]Round, 0, len(d.Rounds))
	for _, r := range d.Rounds {
		if f.Match(d, r) {
			out = append(out, r)
		}
	}
	return out
}

func matchBuy(e Economy, side Side, want []BuyType) bool {
	if side == "" {
		return containsBuy(want, e.CTBuy) || containsBuy(want, e.TBuy)
	}
	return containsBuy(want, e.Buy(side))
}

func matchOutcome(outcome Outcome, side Side, winner Side) bool {
	if side == "" {
		// Without a perspective a round is neither a win nor a loss; asking for
		// one is a filter that cannot be satisfied rather than one that matches
		// everything.
		return false
	}
	if outcome == OutcomeWin {
		return winner == side
	}
	return winner != "" && winner != side
}

func matchSlots(r Round, slots []uint8, side Side) bool {
	for _, pr := range r.Players {
		if side != "" && pr.Side != side {
			continue
		}
		for _, slot := range slots {
			if pr.Slot == slot {
				return true
			}
		}
	}
	return false
}

func containsBuy(list []BuyType, v BuyType) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func containsSite(list []Site, v Site) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func containsPattern(list []TPattern, v TPattern) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func containsCTPattern(list []CTPattern, v CTPattern) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// FilterFromValues parses a filter from query parameters. The CLI and the HTTP
// API both call it so there is exactly one filter vocabulary in the product.
//
// Repeated or comma-separated values are ORed: "buy=eco&buy=force" and
// "buy=eco,force" are the same filter.
func FilterFromValues(values url.Values) (Filter, error) {
	f := Filter{
		TeamKey: strings.TrimSpace(values.Get("team")),
	}
	var err error
	if f.Side, err = parseSide(values.Get("side")); err != nil {
		return Filter{}, err
	}
	if f.Buys, err = parseBuys(multi(values, "buy")); err != nil {
		return Filter{}, err
	}
	if f.OpponentBuys, err = parseBuys(multi(values, "opponent_buy")); err != nil {
		return Filter{}, err
	}
	if f.Sites, err = parseSites(multi(values, "site")); err != nil {
		return Filter{}, err
	}
	if f.Outcome, err = parseOutcome(values.Get("outcome")); err != nil {
		return Filter{}, err
	}
	if f.TPatterns, err = parseTPatterns(multi(values, "t_pattern")); err != nil {
		return Filter{}, err
	}
	if f.CTPatterns, err = parseCTPatterns(multi(values, "ct_pattern")); err != nil {
		return Filter{}, err
	}
	f.Tags = multi(values, "tag")
	if f.Slots, err = parseSlots(multi(values, "slot")); err != nil {
		return Filter{}, err
	}
	if f.RoundFrom, err = parseBound(values.Get("round_from")); err != nil {
		return Filter{}, err
	}
	if f.RoundTo, err = parseBound(values.Get("round_to")); err != nil {
		return Filter{}, err
	}
	if f.Phase, err = parsePhase(values.Get("phase")); err != nil {
		return Filter{}, err
	}
	if f.RoundFrom > 0 && f.RoundTo > 0 && f.RoundFrom > f.RoundTo {
		return Filter{}, fmt.Errorf("filter: round_from %d is after round_to %d", f.RoundFrom, f.RoundTo)
	}
	return f, nil
}

func multi(values url.Values, key string) []string {
	var out []string
	for _, raw := range values[key] {
		for _, part := range strings.Split(raw, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func parseSide(raw string) (Side, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case "CT":
		return SideCT, nil
	case "T":
		return SideT, nil
	default:
		return "", fmt.Errorf("filter: unknown side %q", raw)
	}
}

func parseBuys(raw []string) ([]BuyType, error) {
	var out []BuyType
	for _, item := range raw {
		v := BuyType(strings.ToLower(item))
		if !containsBuy(BuyTypes, v) {
			return nil, fmt.Errorf("filter: unknown buy type %q", item)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseSites(raw []string) ([]Site, error) {
	var out []Site
	for _, item := range raw {
		var v Site
		switch strings.ToLower(item) {
		case "a":
			v = SiteA
		case "b":
			v = SiteB
		case "mid":
			v = SiteMid
		case "none":
			v = SiteNone
		default:
			return nil, fmt.Errorf("filter: unknown site %q", item)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseOutcome(raw string) (Outcome, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case string(OutcomeWin):
		return OutcomeWin, nil
	case string(OutcomeLoss):
		return OutcomeLoss, nil
	default:
		return "", fmt.Errorf("filter: unknown outcome %q", raw)
	}
}

func parseTPatterns(raw []string) ([]TPattern, error) {
	known := []TPattern{TExecute, TDefault, TSplit, TFast, TEcoRush, TSave, TUnknown}
	var out []TPattern
	for _, item := range raw {
		v := TPattern(strings.ToLower(item))
		if !containsPattern(known, v) {
			return nil, fmt.Errorf("filter: unknown T pattern %q", item)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseCTPatterns(raw []string) ([]CTPattern, error) {
	known := []CTPattern{CTHold, CTRetake, CTAggression, CTStack, CTSave, CTUnknown}
	var out []CTPattern
	for _, item := range raw {
		v := CTPattern(strings.ToLower(item))
		if !containsCTPattern(known, v) {
			return nil, fmt.Errorf("filter: unknown CT pattern %q", item)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseSlots(raw []string) ([]uint8, error) {
	var out []uint8
	for _, item := range raw {
		n, err := strconv.Atoi(item)
		if err != nil || n < 0 || n >= maxSlots {
			return nil, fmt.Errorf("filter: slot %q must be an integer in 0..%d", item, maxSlots-1)
		}
		out = append(out, uint8(n))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func parseBound(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("filter: round bound %q must be a positive integer", raw)
	}
	return n, nil
}

func parsePhase(raw string) (Phase, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case string(PhaseRegulation):
		return PhaseRegulation, nil
	case string(PhaseOvertime):
		return PhaseOvertime, nil
	default:
		return "", fmt.Errorf("filter: unknown phase %q", raw)
	}
}
