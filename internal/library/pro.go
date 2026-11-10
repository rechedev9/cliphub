package library

import (
	"time"

	"github.com/google/uuid"
)

// ProMap is a pro map imported from the community catalog. DemoID is nil once the
// user deleted its match, so the follow sync leaves it out and only an explicit import brings it back.
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

// ProTeam is an HLTV team as the catalog names it; SteamIDs are the players it
// fielded in the map, which pair the sides of its later demos.
type ProTeam struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Rank     int      `json:"rank,omitempty"`
	SteamIDs []string `json:"steamids,omitempty"`
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
