package playercard

// Request is what the render worker hands the editor: the template of each slot
// (empty leaves the slot out) and the facts of the player the Short follows.
type Request struct {
	Opening string `json:"opening,omitempty"`
	End     string `json:"end,omitempty"`
	Cover   string `json:"cover,omitempty"`
	Card    Card   `json:"card"`
	// ImagePath is the player's image beside the request; empty draws the initials.
	ImagePath string `json:"image_path,omitempty"`
}

// Card is one player's facts from one match. A nil figure is unknown for that
// demo and the card leaves it out.
type Card struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Team      string `json:"team,omitempty"`
	// Role is the library's read of the player (library.Role*), empty when it knows none.
	Role   string  `json:"role,omitempty"`
	Map    string  `json:"map,omitempty"`
	Result *Result `json:"result,omitempty"`
	Rounds int     `json:"rounds,omitempty"`
	Kills  int     `json:"kills"`
	Deaths int     `json:"deaths"`
	// Rating is ClipHub's own rating (tacticalplan.Rating), shown as it is.
	Rating *float64 `json:"rating,omitempty"`
	ADR    *float64 `json:"adr,omitempty"`
	KAST   *float64 `json:"kast,omitempty"`
	HSPct  *float64 `json:"hs_pct,omitempty"`
	// OpeningKills are the rounds the player won the first duel; UtilityDamage is the enemy damage of their grenades.
	OpeningKills  *int    `json:"opening_kills,omitempty"`
	UtilityDamage *int    `json:"utility_damage,omitempty"`
	FACEIT        *FACEIT `json:"faceit,omitempty"`
}

type Result struct {
	Own   int `json:"own"`
	Rival int `json:"rival"`
}

// FACEIT is the player's FACEIT level and ELO, drawn as a badge.
type FACEIT struct {
	Level int `json:"level,omitempty"`
	ELO   int `json:"elo,omitempty"`
}
