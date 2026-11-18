package renderplan

import (
	"fmt"
	"slices"
)

// DefaultPreRollSeconds is the lead before a play's first kill that the analysis cuts the kill plan with.
const DefaultPreRollSeconds = 3

// PreRollChoices are the seconds a video may start each play before its first kill.
var PreRollChoices = []int{DefaultPreRollSeconds, 5, 8}

// ValidatePreRoll accepts PreRollChoices and 0, the windows the analysis cut.
func ValidatePreRoll(seconds int) error {
	if seconds == 0 || slices.Contains(PreRollChoices, seconds) {
		return nil
	}
	return fmt.Errorf("pre_roll_seconds must be 3, 5 or 8")
}

// normalizePreRoll stores the analysis default as unset, so equal choices give equal requests, and clears the
// pre-roll of a death reel, a match recap and a Full Demo, which never cut kill plays.
func normalizePreRoll(req EditRequest) EditRequest {
	if req.PreRollSeconds == DefaultPreRollSeconds || req.DeathReel || req.MatchRecap || req.FullDemo != nil {
		req.PreRollSeconds = 0
	}
	return req
}
