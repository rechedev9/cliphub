// Package library defines the shared records of Studio's demo library: the
// imported demo, the match it holds, the series it belongs to, and the teams
// and players that appear across matches. Both areas of Studio read these
// records: Crear picks a demo from them and Analizar aggregates over them.
//
// The package is pure schema plus the deterministic derivations over it (team
// name normalization, source detection, a match summary from a tactical
// document). Persistence lives in internal/store.
package library

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors shared by every repository implementation.
var (
	ErrNotFound = errors.New("library record not found")
	// ErrConflict reports a write that would break a uniqueness rule, such as
	// a team alias another team already owns.
	ErrConflict = errors.New("library record conflict")
)

// Source is where a demo came from. Detection only ever proposes valve,
// faceit, hltv or unknown; scrim is a fact only the user knows.
type Source string

const (
	SourceValve   Source = "valve"
	SourceFaceit  Source = "faceit"
	SourceHLTV    Source = "hltv"
	SourceScrim   Source = "scrim"
	SourceUnknown Source = "unknown"
)

// ValidSource reports whether s is one of the known sources.
func ValidSource(s Source) bool {
	switch s {
	case SourceValve, SourceFaceit, SourceHLTV, SourceScrim, SourceUnknown:
		return true
	default:
		return false
	}
}

// ParseState is the analysis state of an imported demo.
type ParseState string

// Parse states. A demo moves queued -> parsing -> ready or failed; a retry
// moves it back to queued.
const (
	ParseQueued  ParseState = "queued"
	ParseParsing ParseState = "parsing"
	ParseReady   ParseState = "ready"
	ParseFailed  ParseState = "failed"
)

// ValidParseState reports whether s is one of the parse states.
func ValidParseState(s ParseState) bool {
	switch s {
	case ParseQueued, ParseParsing, ParseReady, ParseFailed:
		return true
	default:
		return false
	}
}

const (
	AssignmentAuto   = "auto"   // resolved from the clan name in the demo
	AssignmentManual = "manual" // set by the user; a re-analysis keeps it
	AssignmentNone   = "none"   // no clan name and no correction yet
)

const (
	FormatBO1 = "bo1"
	FormatBO3 = "bo3"
	FormatBO5 = "bo5"
)

// ValidSeriesFormat reports whether f is a known series format.
func ValidSeriesFormat(f string) bool {
	return f == FormatBO1 || f == FormatBO3 || f == FormatBO5
}

// Demo is one imported demo file, keyed by the SHA-256 of its decompressed
// bytes so importing the same demo twice yields one record.
type Demo struct {
	ID           uuid.UUID  `json:"id"`
	SHA256       string     `json:"sha256"`
	FileName     string     `json:"file_name"`
	Path         string     `json:"path"`
	SizeBytes    int64      `json:"size_bytes"`
	Source       Source     `json:"source"`
	SourceManual bool       `json:"source_manual"`
	CS2Build     int        `json:"cs2_build,omitempty"`
	PatchVersion int        `json:"patch_version,omitempty"`
	State        ParseState `json:"state"`
	ErrorCode    string     `json:"error_code,omitempty"`
	Error        string     `json:"error,omitempty"`
	Tags         []string   `json:"tags"`
	MatchID      *uuid.UUID `json:"match_id,omitempty"`
	// JobIDs are the Crear jobs made from this demo, oldest link first, so a
	// match can list the videos made from it.
	JobIDs []uuid.UUID `json:"job_ids"`
	// PlayedAt is the match date the importer was told, if any; the match
	// takes it on its first analysis and falls back to ImportedAt.
	PlayedAt   *time.Time `json:"played_at,omitempty"`
	ImportedAt time.Time  `json:"imported_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type DemoFilter struct {
	States []ParseState
	Source Source
	Tag    string
	Limit  int
	Offset int
}

// DemoPatch is a user edit of a demo. Nil fields are left unchanged.
type DemoPatch struct {
	Source *Source
	Tags   *[]string
}

// Match is one map played, as read from one demo.
type Match struct {
	ID            uuid.UUID     `json:"id"`
	DemoID        uuid.UUID     `json:"demo_id"`
	SeriesID      *uuid.UUID    `json:"series_id,omitempty"`
	Map           string        `json:"map"`
	PlayedAt      time.Time     `json:"played_at"`
	Source        Source        `json:"source"`
	Tickrate      float64       `json:"tickrate"`
	DurationTicks int           `json:"duration_ticks"`
	RoundCount    int           `json:"round_count"`
	Teams         []MatchTeam   `json:"teams"`
	Players       []MatchPlayer `json:"players,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// MatchTeam is one side of a match. Key is the team key of the tactical
// document ("ct-start" or "t-start"), so every number here can be traced to
// the rounds of that document.
type MatchTeam struct {
	Key        string     `json:"key"`
	StartSide  string     `json:"start_side"`
	ClanName   string     `json:"clan_name"`
	TeamID     *uuid.UUID `json:"team_id"`
	TeamName   string     `json:"team_name,omitempty"`
	Assignment string     `json:"assignment"`
	Score      int        `json:"score"`
}

// MatchPlayer is one player's line in a match, summed over the rounds of the
// tactical document.
type MatchPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	TeamKey   string `json:"team_key"`
	Slot      uint8  `json:"slot"`
	Rounds    int    `json:"rounds"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Assists   int    `json:"assists"`
	Damage    int    `json:"damage"`
}

// MatchFilter selects matches for a list read. Zero fields do not filter;
// From and To bound PlayedAt inclusively.
type MatchFilter struct {
	Map      string
	TeamID   *uuid.UUID
	SteamID  string
	SeriesID *uuid.UUID
	Source   Source
	Tag      string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// MatchPatch is a user edit of a match. ClearSeries detaches the match from
// its series; SeriesID attaches it to another one.
type MatchPatch struct {
	PlayedAt    *time.Time
	SeriesID    *uuid.UUID
	ClearSeries bool
}

// TeamAssignment is a manual correction of the team on one side of a match.
// A nil TeamID clears the side. RememberAlias also records the side's clan
// name as an alias of the team, so later imports resolve it on their own.
type TeamAssignment struct {
	MatchID       uuid.UUID
	TeamKey       string
	TeamID        *uuid.UUID
	RememberAlias bool
}

// Series groups the maps of one BO1, BO3 or BO5. Veto is the user's free
// notes; Vetoes is the known pick and ban order, by team slot.
type Series struct {
	ID        uuid.UUID    `json:"id"`
	Format    string       `json:"format"`
	Event     string       `json:"event"`
	Veto      string       `json:"veto,omitempty"`
	TeamIDs   []uuid.UUID  `json:"team_ids"`
	Teams     []SeriesTeam `json:"teams"`
	Vetoes    []VetoStep   `json:"vetoes"`
	MatchIDs  []uuid.UUID  `json:"match_ids"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Team is a team identity across matches. Aliases are the clan names that
// resolve to it; the roster is derived from the matches it played.
type Team struct {
	ID         uuid.UUID     `json:"id"`
	Name       string        `json:"name"`
	Aliases    []string      `json:"aliases"`
	MatchCount int           `json:"match_count"`
	Roster     []RosterEntry `json:"roster,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// RosterEntry is one player who played for a team, with the dates of the
// first and last match they played for it.
type RosterEntry struct {
	SteamID64     string    `json:"steamid64"`
	Nick          string    `json:"nick"`
	FirstPlayedAt time.Time `json:"first_played_at"`
	LastPlayedAt  time.Time `json:"last_played_at"`
	Matches       int       `json:"matches"`
}

type TeamFilter struct {
	Query  string
	Limit  int
	Offset int
}

// TeamPatch is a user edit of a team. Nil fields are left unchanged.
type TeamPatch struct {
	Name    *string
	Aliases *[]string
}

// Player is a player identity, keyed by SteamID64.
type Player struct {
	SteamID64      string       `json:"steamid64"`
	Nick           string       `json:"nick"`
	Nicks          []PlayerNick `json:"nicks,omitempty"`
	FaceitPlayerID string       `json:"faceit_player_id,omitempty"`
	FaceitNickname string       `json:"faceit_nickname,omitempty"`
	MatchCount     int          `json:"match_count"`
	Teams          []PlayerTeam `json:"teams,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// PlayerNick is a name a player used, with the dates of the matches it was
// first and last seen in.
type PlayerNick struct {
	Nick        string    `json:"nick"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// PlayerTeam is a team a player played for, by date.
type PlayerTeam struct {
	TeamID        uuid.UUID `json:"team_id"`
	TeamName      string    `json:"team_name"`
	FirstPlayedAt time.Time `json:"first_played_at"`
	LastPlayedAt  time.Time `json:"last_played_at"`
	Matches       int       `json:"matches"`
}

type PlayerFilter struct {
	Query  string
	TeamID *uuid.UUID
	Limit  int
	Offset int
}

// Analysis is what one tactical scan of a demo contributes to the library.
// Saving it creates the match on the first scan and refreshes it on a later
// one, keeping the user's corrections (series, date, manual teams).
type Analysis struct {
	DemoID        uuid.UUID
	Source        Source
	CS2Build      int
	PatchVersion  int
	Map           string
	Tickrate      float64
	DurationTicks int
	RoundCount    int
	Teams         []AnalysisTeam
	Players       []MatchPlayer
}

// AnalysisTeam is one side as the scan saw it. ClanName is empty when the
// demo carried no clan name for that side.
type AnalysisTeam struct {
	Key       string
	StartSide string
	ClanName  string
	Score     int
}

const (
	DefaultListLimit = 50
	MaxListLimit     = 200
)

// ClampLimit maps a requested page size onto the supported range.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

// MaxReportMatches caps the matches one scouting report reads.
const MaxReportMatches = 50

// Report is a saved scouting report: the rival team, the matches it is read
// from and the side it looks at. Its sections are computed on every read from
// the matches' tactical documents, so a re-analysis shows up without a resave.
type Report struct {
	ID       uuid.UUID   `json:"id"`
	Title    string      `json:"title"`
	TeamID   uuid.UUID   `json:"team_id"`
	TeamName string      `json:"team_name"`
	MatchIDs []uuid.UUID `json:"match_ids"`
	// Side is "CT", "T" or empty for both sides.
	Side      string    `json:"side"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReportFilter selects reports for a list read.
type ReportFilter struct {
	TeamID *uuid.UUID
	Limit  int
	Offset int
}

// ReportPatch is a user edit of a report. Nil fields are left unchanged.
type ReportPatch struct {
	Title    *string
	MatchIDs *[]uuid.UUID
	Side     *string
	Notes    *string
}
