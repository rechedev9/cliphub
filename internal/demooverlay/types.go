// Package demooverlay builds Full Demo intro roster and outro scoreboard
// overlays from demo roster facts plus optional FACEIT enrichment.
package demooverlay

const (
	SchemaVersion = "cliphub.full-demo-overlay/v1"

	// FadeFromBlackSeconds is the opening fade. The roster slides in
	// IntroOverlayAfterFadeSeconds after that fade ends, then leaves before
	// live action. Parser IntroFreezeSeconds must stay in sync with
	// IntroFreezeSeconds here so the first compiled seconds are freeze/buy.
	FadeFromBlackSeconds         = 1.0
	IntroOverlayAfterFadeSeconds = 4.0
	IntroOverlaySlideSeconds     = 0.4
	IntroFreezeSeconds           = 15
	IntroLeaveBeforeLiveSeconds  = 1.0
	OutroSeconds                 = 8
	BannerHoldSeconds            = 4

	FrameWidth  = 1920
	FrameHeight = 1080
)

// IntroOverlayStart is fade duration + the post-fade delay (~4s).
func IntroOverlayStart() float64 {
	return FadeFromBlackSeconds + IntroOverlayAfterFadeSeconds
}

// IntroOverlayEnd is 1s before the typical freeze prefix ends, so the
// roster leaves before live action.
func IntroOverlayEnd() float64 {
	return float64(IntroFreezeSeconds) - IntroLeaveBeforeLiveSeconds
}

const (
	ColName    = "name"
	ColELO     = "elo"
	ColLevel   = "level"
	ColCountry = "country"
	ColMatches = "matches"
	ColWinPct  = "winpct"
	ColRating  = "rating"
	ColSwing   = "swing"
	ColKDA     = "kda"
	ColKD      = "kd"
	ColKR      = "kr"
	ColADR     = "adr"
	ColHS      = "hs"
	ColHSPct   = "hspct"
	ColMulti   = "multi"
	ColMVP     = "mvp"
)

// Document is the durable Full Demo overlay contract. FACEIT fields are
// omitted unless a real FACEIT value was supplied; nothing is invented.
type Document struct {
	SchemaVersion   string     `json:"schema_version"`
	TargetSteamID64 string     `json:"target_steamid64"`
	TargetName      string     `json:"target_name"`
	TargetKills     int        `json:"target_kills"`
	TargetDeaths    int        `json:"target_deaths"`
	TargetELO       *int       `json:"target_elo,omitempty"`
	Map             string     `json:"map,omitempty"`
	ScoreCT         int        `json:"score_ct"`
	ScoreT          int        `json:"score_t"`
	Intro           Intro      `json:"intro"`
	Outro           Scoreboard `json:"outro"`
}

type Intro struct {
	Left    []PlayerCard `json:"left"`
	Right   []PlayerCard `json:"right"`
	Columns []string     `json:"columns"`
}

type Scoreboard struct {
	Teams   []TeamBoard `json:"teams"`
	Columns []string    `json:"columns"`
}

type TeamBoard struct {
	Name       string       `json:"name"`
	Side       string       `json:"side"`
	Score      int          `json:"score"`
	AverageELO *int         `json:"average_elo,omitempty"`
	Players    []PlayerCard `json:"players"`
}

type PlayerCard struct {
	SteamID64  string  `json:"steamid64"`
	Name       string  `json:"name"`
	Team       string  `json:"team,omitempty"`
	Country    string  `json:"country,omitempty"`
	Kills      int     `json:"kills"`
	Deaths     int     `json:"deaths"`
	Assists    int     `json:"assists"`
	Headshots  int     `json:"headshots,omitempty"`
	MVPs       int     `json:"mvps,omitempty"`
	Rounds     int     `json:"rounds,omitempty"`
	ADR        float64 `json:"adr,omitempty"`
	HSPct      float64 `json:"hs_pct,omitempty"`
	Rating     float64 `json:"rating,omitempty"`
	Rounds2K   int     `json:"rounds_2k,omitempty"`
	Rounds3K   int     `json:"rounds_3k,omitempty"`
	Rounds4K   int     `json:"rounds_4k,omitempty"`
	Rounds5K   int     `json:"rounds_5k,omitempty"`
	HasADR     bool    `json:"has_adr,omitempty"`
	HasHSPct   bool    `json:"has_hs_pct,omitempty"`
	HasRating  bool    `json:"has_rating,omitempty"`
	ELO        *int    `json:"elo,omitempty"`
	SkillLevel *int    `json:"skill_level,omitempty"`
	Last20     *Last20 `json:"last20,omitempty"`
}

type Last20 struct {
	Matches *int     `json:"matches,omitempty"`
	WinPct  *float64 `json:"win_pct,omitempty"`
	Rating  *float64 `json:"rating,omitempty"`
	Swing   *float64 `json:"swing,omitempty"`
	Kills   *int     `json:"kills,omitempty"`
	Deaths  *int     `json:"deaths,omitempty"`
	Assists *int     `json:"assists,omitempty"`
	KD      *float64 `json:"kd,omitempty"`
	KR      *float64 `json:"kr,omitempty"`
	ADR     *float64 `json:"adr,omitempty"`
}

// Roster is the demo-truth input. Rates of 0 are treated as absent unless
// the matching Has* flag is set (or the raw tally implies they exist).
type Roster struct {
	TargetSteamID64 string
	Players         []RosterPlayer
	Map             string
	ScoreCT         int
	ScoreT          int
	Rounds          int
}

type RosterPlayer struct {
	SteamID64 string
	Name      string
	Team      string
	Kills     int
	Deaths    int
	Assists   int
	Headshots int
	MVPs      int
	Rounds    int
	ADR       float64
	HSPct     float64
	Rating    float64
	Rounds2K  int
	Rounds3K  int
	Rounds4K  int
	Rounds5K  int
}

// Enrichment is optional FACEIT data keyed by SteamID64. Zero ELO/level are
// absent; last-20 fields are omitted unless set on Last20.
type Enrichment struct {
	Nickname   string
	Country    string
	ELO        int
	SkillLevel int
	Last20     *Last20
}

func intPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}
