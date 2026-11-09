package library

import (
	"time"

	"github.com/google/uuid"
)

// ProMap is a map of an HLTV pro match imported from the community server's
// catalog. DemoID is nil once the user deleted its match, so the follow sync
// does not bring it back; an explicit import does.
type ProMap struct {
	MatchID         int        `json:"hltv_match_id"`
	Entry           string     `json:"entry"`
	DemoID          *uuid.UUID `json:"demo_id,omitempty"`
	Map             string     `json:"map"`
	Event           string     `json:"event"`
	PlayedAt        time.Time  `json:"played_at"`
	Teams           []ProTeam  `json:"teams"`
	AnalysisVersion int        `json:"analysis_version"`
	SHA256          string     `json:"sha256"`
	ImportedAt      time.Time  `json:"imported_at"`
}

// ProTeam is an HLTV team as the catalog names it.
type ProTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Rank int    `json:"rank,omitempty"`
}

// ProFollow is an HLTV team whose new pro matches Studio imports on its own.
type ProFollow struct {
	TeamID     int        `json:"team_id"`
	Name       string     `json:"name"`
	FollowedAt time.Time  `json:"followed_at"`
	SyncedAt   *time.Time `json:"synced_at,omitempty"`
}

// radarCalibrationVersions is the analysis version that changed a map's radar calibration.
var radarCalibrationVersions = map[string]int{"de_cache": 8}

// RadarMayBeOff reports an analysis older than its map's last radar calibration
// change, whose positions Studio may draw off the radar.
func RadarMayBeOff(mapName string, analysisVersion int) bool {
	v, ok := radarCalibrationVersions[mapName]
	return ok && analysisVersion < v
}
