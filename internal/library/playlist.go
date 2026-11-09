package library

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Playlist limits. MaxPlaylistRounds matches the rounds one Replay strip carries.
const (
	MaxPlaylistNameRunes    = 120
	MaxPlaylistFolderRunes  = 60
	MaxPlaylistAuthorRunes  = 60
	MaxPlaylistCommentRunes = 1000
	MaxPlaylistRounds       = 200
	MaxPlaylistList         = 1000
	MaxPlaylistRound        = 999
)

var (
	playlistMapPattern = regexp.MustCompile(`^[a-z0-9_]{0,64}$`)
	demoSHAPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Playlist is a saved list of rounds of any matches, in the order the team
// watches them. Maps derive from the rounds.
type Playlist struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Folder    string          `json:"folder"`
	Favorite  bool            `json:"favorite"`
	Author    string          `json:"author"`
	Maps      []string        `json:"maps"`
	Rounds    []PlaylistRound `json:"rounds"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// PlaylistRound is one round of a playlist with the team's comment. Map and
// DemoSHA256 come from the match when the round is added, so a round whose
// match is gone, or that came from another PC, still says what it was and
// finds its match again once that demo is imported. Missing is set on read.
type PlaylistRound struct {
	MatchID    uuid.UUID `json:"match_id"`
	Round      int       `json:"round"`
	Map        string    `json:"map"`
	DemoSHA256 string    `json:"demo_sha256,omitempty"`
	Comment    string    `json:"comment"`
	Missing    bool      `json:"missing,omitempty"`
}

// PlaylistFilter selects playlists for a list read; zero fields do not filter.
// Query matches the name, author or folder, case-insensitively.
type PlaylistFilter struct {
	Query    string
	Map      string
	Folder   string
	NoFolder bool
	Favorite bool
	Limit    int
	Offset   int
}

// PlaylistPatch is a user edit of a playlist. Nil fields are left unchanged;
// Rounds replaces the whole list, in its order.
type PlaylistPatch struct {
	Name     *string
	Folder   *string
	Author   *string
	Favorite *bool
	Rounds   *[]PlaylistRound
}

// Edits reports whether the patch changes more than the star, which does not
// count as an edit: starring a playlist keeps its place in the list.
func (p PlaylistPatch) Edits() bool {
	return p.Name != nil || p.Folder != nil || p.Author != nil || p.Rounds != nil
}

// ApplyPlaylistPatch returns p with the patch's fields set.
func ApplyPlaylistPatch(p Playlist, patch PlaylistPatch) Playlist {
	for _, f := range []struct {
		dst *string
		src *string
	}{{&p.Name, patch.Name}, {&p.Folder, patch.Folder}, {&p.Author, patch.Author}} {
		if f.src != nil {
			*f.dst = *f.src
		}
	}
	if patch.Favorite != nil {
		p.Favorite = *patch.Favorite
	}
	if patch.Rounds != nil {
		p.Rounds = append([]PlaylistRound{}, *patch.Rounds...)
	}
	p.Maps = PlaylistMaps(p.Rounds)
	return p
}

// SamePlaylistRound reports whether two rounds are the same round of the same match.
func SamePlaylistRound(a, b PlaylistRound) bool {
	return a.MatchID == b.MatchID && a.Round == b.Round
}

// PlaylistMaps lists the maps of the rounds once each, in round order.
func PlaylistMaps(rounds []PlaylistRound) []string {
	out := []string{}
	for _, r := range rounds {
		if r.Map != "" && !slices.Contains(out, r.Map) {
			out = append(out, r.Map)
		}
	}
	return out
}

// CleanPlaylist trims the texts, drops repeated rounds (the first stays) and
// checks the limits.
func CleanPlaylist(p Playlist) (Playlist, error) {
	p.Name, p.Folder, p.Author = CleanName(p.Name), CleanName(p.Folder), CleanName(p.Author)
	switch {
	case p.Name == "" || len([]rune(p.Name)) > MaxPlaylistNameRunes:
		return p, fmt.Errorf("name takes 1 to %d characters", MaxPlaylistNameRunes)
	case len([]rune(p.Folder)) > MaxPlaylistFolderRunes:
		return p, fmt.Errorf("folder takes at most %d characters", MaxPlaylistFolderRunes)
	case len([]rune(p.Author)) > MaxPlaylistAuthorRunes:
		return p, fmt.Errorf("author takes at most %d characters", MaxPlaylistAuthorRunes)
	case strings.ContainsFunc(p.Name+p.Folder+p.Author, unicode.IsControl):
		return p, errors.New("name, folder and author must not hold control characters")
	}
	rounds := make([]PlaylistRound, 0, len(p.Rounds))
	for _, r := range p.Rounds {
		r, err := cleanPlaylistRound(r)
		if err != nil {
			return p, err
		}
		if !slices.ContainsFunc(rounds, func(v PlaylistRound) bool { return SamePlaylistRound(v, r) }) {
			rounds = append(rounds, r)
		}
	}
	if len(rounds) > MaxPlaylistRounds {
		return p, fmt.Errorf("a playlist holds at most %d rounds", MaxPlaylistRounds)
	}
	p.Rounds = rounds
	p.Maps = PlaylistMaps(rounds)
	return p, nil
}

func cleanPlaylistRound(r PlaylistRound) (PlaylistRound, error) {
	r.Comment, r.Map, r.DemoSHA256 = strings.TrimSpace(r.Comment), strings.ToLower(strings.TrimSpace(r.Map)), strings.ToLower(strings.TrimSpace(r.DemoSHA256))
	r.Missing = false
	switch {
	case r.MatchID == uuid.Nil:
		return r, errors.New("every round needs its match_id")
	case r.Round < 1 || r.Round > MaxPlaylistRound:
		return r, fmt.Errorf("round must be from 1 to %d", MaxPlaylistRound)
	case !playlistMapPattern.MatchString(r.Map):
		return r, errors.New("map must be a map name such as de_mirage")
	case r.DemoSHA256 != "" && !demoSHAPattern.MatchString(r.DemoSHA256):
		return r, errors.New("demo_sha256 must be 64 hex characters")
	case len([]rune(r.Comment)) > MaxPlaylistCommentRunes:
		return r, fmt.Errorf("a comment takes at most %d characters", MaxPlaylistCommentRunes)
	case strings.ContainsFunc(r.Comment, func(c rune) bool { return unicode.IsControl(c) && c != '\n' && c != '\t' }):
		return r, errors.New("a comment must not hold control characters")
	}
	return r, nil
}
