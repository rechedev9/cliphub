package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// Query keys the Replay playlist reads beside r and t, as web/lib/replay/zone-marks.ts names them.
const (
	zoneQuery       = "zona"
	steamFocusQuery = "jugador"
	tramoQuery      = "tramo"
	// maxZoneMarks mirrors ZONE_MARKS_MAX in the web.
	maxZoneMarks = 12
)

// findingLink is what a finding's studio_path opens: its rounds as a Replay
// playlist, with players in focus, a tramo of the round and zones marked.
type findingLink struct {
	title string
	refs  []tuiclient.Evidence
	focus []string
	tramo string
	zones []string
}

// maxExampleRefs is the round refs a row lists in place of a link, as team_prep does.
const maxExampleRefs = 5

// linkOut is a studio_path and, when its rounds were cut at maxPlaylistRounds, how many it opens.
type linkOut struct {
	StudioPath   string `json:"studio_path,omitempty"`
	RoundsLinked int    `json:"rounds_linked,omitempty" jsonschema:"set only when studio_path was cut: the rounds it opens, fewer than rounds"`
}

// roundKeys is the link's valid rounds as match:round, once each, in order.
func (l findingLink) roundKeys() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range l.refs {
		key := e.MatchID + ":" + strconv.Itoa(e.Round)
		if seen[key] || e.Round < 1 || checkUUID("match_id", e.MatchID) != nil {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func (l findingLink) out() linkOut {
	keys := l.roundKeys()
	o := linkOut{StudioPath: l.path()}
	if len(keys) > maxPlaylistRounds {
		o.RoundsLinked = maxPlaylistRounds
	}
	return o
}

// exampleRefs is up to maxExampleRefs of the rounds as match_id/round.
func exampleRefs(refs []tuiclient.Evidence) []string { return examplesOf(refs, maxExampleRefs) }

func examplesOf(refs []tuiclient.Evidence, n int) []string {
	keys := findingLink{refs: refs}.roundKeys()
	if len(keys) > n {
		keys = keys[:n]
	}
	for i, k := range keys {
		keys[i] = strings.Replace(k, ":", "/", 1)
	}
	return keys
}

// path writes the link as the web's findingHref does; a finding without rounds has none.
func (l findingLink) path() string {
	keys := l.roundKeys()
	if len(keys) > maxPlaylistRounds {
		keys = keys[:maxPlaylistRounds]
	}
	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, "r="+key)
	}
	if len(parts) == 0 {
		return ""
	}
	extra := url.Values{}
	if title := strings.TrimSpace(l.title); title != "" {
		extra.Set("t", title)
	}
	for _, id := range l.focus {
		if steamIDPattern.MatchString(id) {
			extra.Add(steamFocusQuery, id)
		}
	}
	if l.tramo != "" {
		extra.Set(tramoQuery, l.tramo)
	}
	for i, zone := range l.zones {
		if i == maxZoneMarks {
			break
		}
		if zone != "" {
			extra.Add(zoneQuery, zone)
		}
	}
	if len(extra) > 0 {
		parts = append(parts, extra.Encode())
	}
	return "/replay?" + strings.Join(parts, "&")
}

// windowTramo is a prep window as the Replay's tramo query writes it; an open end is the round's.
func windowTramo(from, to int) string {
	if to == 0 {
		to = roundClockSeconds
	}
	return fmt.Sprintf("%d-%d", from, to)
}

// siteZone is the callout of a bombsite key, so a link can mark it.
func siteZone(site string) []string {
	switch site {
	case "A":
		return []string{"BombsiteA"}
	case "B":
		return []string{"BombsiteB"}
	case "both":
		return []string{"BombsiteA", "BombsiteB"}
	default:
		return nil
	}
}
