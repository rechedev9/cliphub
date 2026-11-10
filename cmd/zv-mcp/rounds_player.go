package main

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// playerMatches reads the matches a player's find_rounds walks: the play's or the
// listed ones, else the newest up to the cap, with whether older ones were left out.
func (in findRoundsIn) playerMatches(call roundsCall) ([]tuiclient.LibraryMatch, bool, error) {
	ctx, c, q, play := call.ctx, call.client, call.query, call.play
	ids := q.Matches
	if play != nil {
		ids = playMatches(q, play)
	}
	if play == nil && len(ids) == 0 {
		matches, err := c.SearchLibraryMatches(ctx, tuiclient.LibraryMatchQuery{
			Map: q.Map, SteamID: in.SteamID, From: q.From, To: q.To, Limit: maxRoundMatches + 1,
		})
		if err != nil {
			return nil, false, toolError(err)
		}
		return matches[:min(len(matches), maxRoundMatches)], len(matches) > maxRoundMatches, nil
	}
	more := len(ids) > maxRoundMatches
	out := []tuiclient.LibraryMatch{}
	for _, id := range ids[:min(len(ids), maxRoundMatches)] {
		m, err := c.GetLibraryMatch(ctx, id)
		if tuiclient.StatusCode(err) == 404 {
			continue
		}
		if err != nil {
			return nil, false, toolError(err)
		}
		if (q.Map == "" || strings.EqualFold(m.Map, q.Map)) && (q.From.IsZero() || !m.PlayedAt.Before(q.From)) && (q.To.IsZero() || !m.PlayedAt.After(q.To)) {
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b tuiclient.LibraryMatch) int { return b.PlayedAt.Compare(a.PlayedAt) })
	return out, more, nil
}

// playerRounds walks a player's matches, one tactical document each.
func (in findRoundsIn) playerRounds(call roundsCall) (findRoundsOut, error) {
	out := findRoundsOut{Rounds: []roundRefOut{}, MatchCap: maxRoundMatches}
	matches, more, err := in.playerMatches(call)
	if err != nil {
		return out, err
	}
	out.MoreMatches = more
	limit := cmp.Or(in.Limit, defaultRoundsLimit)
	for _, m := range matches {
		if len(in.OpponentID) > 0 && !slices.ContainsFunc(m.Teams, func(t tuiclient.LibraryMatchTeam) bool {
			return slices.Contains(in.OpponentID, t.TeamID)
		}) {
			continue
		}
		doc, err := call.client.LibraryTactical(call.ctx, m.ID)
		if tuiclient.StatusCode(err) == 409 || tuiclient.StatusCode(err) == 404 {
			out.MatchesSkipped++
			continue
		}
		if err != nil {
			return out, toolError(err)
		}
		out.MatchesRead++
		found := in.matchRounds(matchRead{match: m, doc: doc, side: call.query.Side, play: call.play})
		if len(out.Rounds)+len(found) > limit {
			out.Rounds = append(out.Rounds, found[:limit-len(out.Rounds)]...)
			out.Truncated = true
			break
		}
		out.Rounds = append(out.Rounds, found...)
	}
	return out, nil
}

// subjectOf finds the team key and slot of the player in a match.
func (in findRoundsIn) subjectOf(doc tuiclient.TacticalDocument) (roundSubject, bool) {
	for _, p := range doc.Players {
		if p.SteamID64 == in.SteamID {
			slot := p.Slot
			return roundSubject{teamKey: p.TeamKey, slot: &slot}, true
		}
	}
	return roundSubject{}, false
}

// keeps says whether the round passes the buys, the round range and the result.
func (in findRoundsIn) keeps(ref roundRefOut) bool {
	for _, f := range []struct {
		list []string
		buy  string
	}{{in.Buy, ref.Buy}, {in.OpponentBuy, ref.OpponentBuy}} {
		if len(f.list) > 0 && !slices.ContainsFunc(f.list, func(b string) bool { return strings.EqualFold(strings.TrimSpace(b), f.buy) }) {
			return false
		}
	}
	if (in.RoundFrom > 0 && ref.Round < in.RoundFrom) || (in.RoundTo > 0 && ref.Round > in.RoundTo) {
		return false
	}
	return in.resultKeeps(ref.Won)
}

// matchRead is one match find_rounds walks for a player, with the side and play that narrow it.
type matchRead struct {
	match tuiclient.LibraryMatch
	doc   tuiclient.TacticalDocument
	side  string
	play  *playRounds
}

// matchRounds lists the player's rounds of one match that pass the filter, in round order.
func (in findRoundsIn) matchRounds(read matchRead) []roundRefOut {
	m, doc, side, play := read.match, read.doc, read.side, read.play
	subject, ok := in.subjectOf(doc)
	if !ok {
		return nil
	}
	opponent := ""
	for _, t := range m.Teams {
		if t.Key != subject.teamKey {
			opponent = cleanName(teamName(t))
		}
	}
	out := []roundRefOut{}
	for _, r := range doc.Rounds {
		if play != nil && !play.keys[playRoundKey(m.ID, r.Number)] {
			continue
		}
		ref := roundRefOut{MatchID: m.ID, Round: r.Number, Map: cleanName(m.Map), PlayedAt: m.PlayedAt.Format(time.DateOnly), Opponent: opponent}
		for _, pr := range r.Players {
			if pr.Slot == *subject.slot {
				kills, deaths := pr.Kills, pr.Deaths
				ref.Side, ref.Kills, ref.Deaths = pr.Side, &kills, &deaths
			}
		}
		if ref.Side == "" || (side != "" && ref.Side != side) {
			continue
		}
		ref.Buy, ref.OpponentBuy, ref.Pattern = r.Economy.TBuy, r.Economy.CTBuy, r.Class.TSide
		if ref.Side == "CT" {
			ref.Buy, ref.OpponentBuy, ref.Pattern = r.Economy.CTBuy, r.Economy.TBuy, r.Class.CTSide
		}
		ref.Won, ref.Site = r.Winner == ref.Side, r.Class.Site
		if !in.keeps(ref) {
			continue
		}
		ref.Ref = playRoundKey(m.ID, r.Number)
		out = append(out, ref)
	}
	return out
}
