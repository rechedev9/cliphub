package main

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

// maxSegmentIDs bounds one request; a kill plan holds far fewer plays.
const maxSegmentIDs = 200

// Every id a model sends is checked here before it reaches a URL or a body.
var (
	// Plan segment ids are "seg-<n>", and "seg--<nn>" for negative indexes (killplan).
	segmentIDPattern = regexp.MustCompile(`^seg--?[0-9]{1,6}$`)
	steamIDPattern   = regexp.MustCompile(`^[0-9]{17}$`)
	mapNamePattern   = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	// Same token shape as internal/artifacts: variants and video names are path segments.
	artifactNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
)

// checkUUID accepts only the canonical lowercase UUID the orchestrator issues.
func checkUUID(field, value string) error {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return fmt.Errorf("%s must be an id returned by an earlier tool, not %q", field, cleanName(value))
	}
	return nil
}

func checkSegmentIDs(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("segment_ids is empty; pick plays from list_moments")
	}
	if len(ids) > maxSegmentIDs {
		return fmt.Errorf("segment_ids takes at most %d ids", maxSegmentIDs)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !segmentIDPattern.MatchString(id) {
			return fmt.Errorf("segment_ids must come from list_moments, not %q", cleanName(id))
		}
		if seen[id] {
			return fmt.Errorf("segment id %s is listed twice", id)
		}
		seen[id] = true
	}
	return nil
}

func checkArtifactName(field, value string) error {
	if !artifactNamePattern.MatchString(value) {
		return fmt.Errorf("%s is not a valid name: %q; use a value an earlier tool returned", field, cleanName(value))
	}
	return nil
}

// checkOptionalFilter validates a list_matches filter; empty means no filter.
func checkOptionalFilter(field, value string, pattern *regexp.Regexp) error {
	if value != "" && !pattern.MatchString(value) {
		return fmt.Errorf("%s is not valid: %q", field, cleanName(value))
	}
	return nil
}

func checkSteamID(value string) error {
	if !steamIDPattern.MatchString(value) {
		return fmt.Errorf("steamid must be a 17 digit SteamID64 from get_match, not %q", cleanName(value))
	}
	return nil
}
