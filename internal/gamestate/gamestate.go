// Package gamestate reads CS2's Game State Integration: the JSON CS2 posts while the user plays,
// turned into whether a match is live, its map, score and when each round went live.
package gamestate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// Payload is the part of a CS2 post Studio reads.
type Payload struct {
	Provider struct {
		SteamID string `json:"steamid"`
	} `json:"provider"`
	Map *struct {
		Name   string `json:"name"`
		Mode   string `json:"mode"`
		Phase  string `json:"phase"`
		Round  int    `json:"round"`
		TeamCT struct {
			Score int `json:"score"`
		} `json:"team_ct"`
		TeamT struct {
			Score int `json:"score"`
		} `json:"team_t"`
	} `json:"map"`
	Round *struct {
		Phase string `json:"phase"`
	} `json:"round"`
	Auth struct {
		Token string `json:"token"`
	} `json:"auth"`
}

// Parse reads a CS2 post.
func Parse(body []byte) (Payload, error) {
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Payload{}, fmt.Errorf("gamestate: %w", err)
	}
	return p, nil
}

// notMatchModes are CS2 modes without a team match worth recording.
var notMatchModes = []string{"casual", "deathmatch", "gungameprogressive", "gungametrbomb", "armsrace", "training", "survival", "coopmission"}

// matchPhases are the map phases of a match being played; gameover ends it.
var matchPhases = []string{"warmup", "live", "intermission"}

// StaleAfter ends a match CS2 stopped reporting: CS2 posts a heartbeat every 10 seconds.
const StaleAfter = 90 * time.Second

// RoundMark is when a round's freeze time ended, by this PC's wall clock.
type RoundMark struct {
	Round  int       `json:"round"`
	LiveAt time.Time `json:"live_at"`
}

// Live is what Studio tells the TeamSpeak plugin.
type Live struct {
	Live    bool        `json:"live"`
	Session string      `json:"session,omitempty"`
	Map     string      `json:"map,omitempty"`
	SteamID string      `json:"steamid,omitempty"`
	ScoreCT int         `json:"score_ct"`
	ScoreT  int         `json:"score_t"`
	Rounds  []RoundMark `json:"rounds"`
	// LastSeen is the last post from CS2; zero when none came since Studio started.
	LastSeen time.Time `json:"last_seen"`
}

// Tracker follows CS2's posts. It is safe for concurrent use.
type Tracker struct {
	mu         sync.Mutex
	state      Live
	roundPhase string
}

// Observe takes one post received at the given time.
func (t *Tracker) Observe(p Payload, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.LastSeen = at
	if p.Map == nil || p.Map.Name == "" || slices.Contains(notMatchModes, strings.ToLower(p.Map.Mode)) ||
		!slices.Contains(matchPhases, strings.ToLower(p.Map.Phase)) {
		t.end()
		return
	}
	session := sessionID(p.Map.Name, p.Provider.SteamID)
	if !t.state.Live || t.state.Session != session {
		t.state = Live{Live: true, Session: session, Map: p.Map.Name, Rounds: []RoundMark{}, LastSeen: at}
		t.roundPhase = ""
	}
	t.state.SteamID = p.Provider.SteamID
	t.state.ScoreCT, t.state.ScoreT = p.Map.TeamCT.Score, p.Map.TeamT.Score
	phase := ""
	if p.Round != nil {
		phase = strings.ToLower(p.Round.Phase)
	}
	// Only a seen freeze time gives the exact moment; a round first seen live has none.
	if phase == "live" && t.roundPhase == "freezetime" && strings.EqualFold(p.Map.Phase, "live") {
		t.mark(RoundMark{Round: p.Map.Round + 1, LiveAt: at})
	}
	if phase != "" {
		t.roundPhase = phase
	}
}

// mark stores a round, replacing an earlier one with its number (a restarted game counts again).
func (t *Tracker) mark(m RoundMark) {
	t.state.Rounds = slices.DeleteFunc(t.state.Rounds, func(r RoundMark) bool { return r.Round >= m.Round })
	t.state.Rounds = append(t.state.Rounds, m)
}

func (t *Tracker) end() {
	seen := t.state.LastSeen
	t.state = Live{Rounds: []RoundMark{}, LastSeen: seen}
	t.roundPhase = ""
}

// State is the match as of now: a match CS2 stopped reporting is over.
func (t *Tracker) State(now time.Time) Live {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state.Live && now.Sub(t.state.LastSeen) > StaleAfter {
		t.end()
	}
	out := t.state
	out.Rounds = append([]RoundMark{}, t.state.Rounds...)
	return out
}

// sessionID is the same for one map of one player, so a Studio restart mid-match keeps it.
func sessionID(mapName, steamID string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(mapName) + "|" + steamID))
	return hex.EncodeToString(sum[:6])
}

// Text is the plain answer the plugin parses: one record per line.
func (l Live) Text() string {
	if !l.Live {
		return "live 0\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "live 1\nsession %s\nmap %s\n", l.Session, oneWord(l.Map))
	if l.SteamID != "" {
		fmt.Fprintf(&b, "steamid %s\n", oneWord(l.SteamID))
	}
	fmt.Fprintf(&b, "score %d %d\n", l.ScoreCT, l.ScoreT)
	for _, r := range l.Rounds {
		fmt.Fprintf(&b, "round %d %d\n", r.Round, r.LiveAt.UnixMilli())
	}
	return b.String()
}

// oneWord keeps a value on its record: CS2 map names and SteamIDs have no spaces.
func oneWord(s string) string {
	return strings.Join(strings.Fields(s), "_")
}
