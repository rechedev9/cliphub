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

// path writes the link as the web's findingHref does; a finding without rounds has none.
func (l findingLink) path() string {
	seen := map[string]bool{}
	parts := []string{}
	for _, e := range l.refs {
		if len(parts) == maxPlaylistRounds {
			break
		}
		key := e.MatchID + ":" + strconv.Itoa(e.Round)
		if seen[key] || e.Round < 1 || checkUUID("match_id", e.MatchID) != nil {
			continue
		}
		seen[key] = true
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
