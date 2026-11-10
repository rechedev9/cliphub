package library

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// Report caps: the notes hold a whole match prep in Markdown, and each round
// list is one finding the coach names and opens in the Replay as a playlist.
const (
	MaxReportNotesRunes     = 20000
	MaxReportRoundLists     = 30
	MaxReportListRounds     = 200
	MaxReportListTitleRunes = 120
	maxReportRoundNumber    = 999
)

// ReportRound is one round of a report's list.
type ReportRound struct {
	MatchID uuid.UUID `json:"match_id"`
	Round   int       `json:"round"`
}

// ReportRoundList is a named list of rounds on a report, such as "Split B con humo en ventana".
type ReportRoundList struct {
	Title  string        `json:"title"`
	Rounds []ReportRound `json:"rounds"`
}

// CleanReportRoundLists validates a report's lists: a title each, rounds in
// order without repeats, within the caps. A nil list comes back empty.
func CleanReportRoundLists(lists []ReportRoundList) ([]ReportRoundList, error) {
	if len(lists) > MaxReportRoundLists {
		return nil, fmt.Errorf("a report holds at most %d round lists", MaxReportRoundLists)
	}
	out := make([]ReportRoundList, 0, len(lists))
	for i, list := range lists {
		title := strings.Join(strings.Fields(list.Title), " ")
		if title == "" || len([]rune(title)) > MaxReportListTitleRunes {
			return nil, fmt.Errorf("round list %d needs a title of 1 to %d characters", i+1, MaxReportListTitleRunes)
		}
		if len(list.Rounds) == 0 || len(list.Rounds) > MaxReportListRounds {
			return nil, fmt.Errorf("round list %q holds from 1 to %d rounds", title, MaxReportListRounds)
		}
		seen := map[ReportRound]bool{}
		rounds := make([]ReportRound, 0, len(list.Rounds))
		for _, r := range list.Rounds {
			if r.MatchID == uuid.Nil || r.Round < 1 || r.Round > maxReportRoundNumber {
				return nil, errors.New("a listed round needs a match id and a round number from 1 to 999")
			}
			if !seen[r] {
				seen[r] = true
				rounds = append(rounds, r)
			}
		}
		out = append(out, ReportRoundList{Title: title, Rounds: rounds})
	}
	return out, nil
}

// CheckReportListMatches refuses a listed round whose match is not one the report reads.
func CheckReportListMatches(lists []ReportRoundList, matchIDs []uuid.UUID) error {
	for _, list := range lists {
		for _, r := range list.Rounds {
			if !slices.Contains(matchIDs, r.MatchID) {
				return fmt.Errorf("round list %q: round %d of match %s is not in the report's match_ids; add the match or drop the round", list.Title, r.Round, r.MatchID)
			}
		}
	}
	return nil
}

// KeepReportListMatches drops listed rounds of matches the report no longer reads, and lists left empty.
func KeepReportListMatches(lists []ReportRoundList, matchIDs []uuid.UUID) []ReportRoundList {
	out := make([]ReportRoundList, 0, len(lists))
	for _, list := range lists {
		rounds := slices.DeleteFunc(append([]ReportRound{}, list.Rounds...), func(r ReportRound) bool { return !slices.Contains(matchIDs, r.MatchID) })
		if len(rounds) > 0 {
			out = append(out, ReportRoundList{Title: list.Title, Rounds: rounds})
		}
	}
	return out
}

// CopyReportRoundLists copies the lists, so a caller owns what it reads.
func CopyReportRoundLists(lists []ReportRoundList) []ReportRoundList {
	out := make([]ReportRoundList, 0, len(lists))
	for _, list := range lists {
		out = append(out, ReportRoundList{Title: list.Title, Rounds: append([]ReportRound{}, list.Rounds...)})
	}
	return out
}
