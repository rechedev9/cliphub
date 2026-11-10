package voiceplugin

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const (
	// A match may carry the time it started, a little before the plugin did, or the time it was imported.
	playedBefore = 3 * time.Hour
	playedAfter  = 14 * 24 * time.Hour
)

// Pick is the match a recording goes to, or why it waits.
type Pick struct {
	MatchID *uuid.UUID
	Note    string
}

// PickMatch finds the one library match a recording belongs to, by map, time and final score.
// candidates are the matches without team voice yet.
func PickMatch(rec library.VoiceRecording, candidates []library.Match) Pick {
	if rec.Map == "" {
		return Pick{Note: "Studio no vio la partida en CS2 mientras grababas: asóciala a mano."}
	}
	var inWindow []library.Match
	for _, m := range candidates {
		if !strings.EqualFold(m.Map, rec.Map) || m.PlayedAt.Before(rec.StartedAt.Add(-playedBefore)) || m.PlayedAt.After(rec.EndedAt.Add(playedAfter)) {
			continue
		}
		inWindow = append(inWindow, m)
	}
	scored := rec.ScoreCT >= 0 && rec.ScoreT >= 0 && rec.ScoreCT+rec.ScoreT > 0
	var sameScore []library.Match
	if scored {
		for _, m := range inWindow {
			if scoreMatches(m, rec) {
				sameScore = append(sameScore, m)
			}
		}
		// An interrupted recording saw only part of the match, so its score is not final.
		if len(sameScore) > 0 || !rec.Interrupted {
			inWindow = sameScore
		}
	}
	switch len(inWindow) {
	case 0:
		return Pick{Note: fmt.Sprintf("Esperando la demo: importa la partida de %s jugada el %s y se asociará sola.", rec.Map, rec.StartedAt.Local().Format("02/01/2006"))}
	case 1:
		id := inWindow[0].ID
		return Pick{MatchID: &id}
	}
	return Pick{Note: fmt.Sprintf("%d partidas de %s cuadran con esta grabación: elige la suya.", len(inWindow), rec.Map)}
}

func scoreMatches(m library.Match, rec library.VoiceRecording) bool {
	if len(m.Teams) != 2 {
		return false
	}
	a, b := m.Teams[0].Score, m.Teams[1].Score
	return (a == rec.ScoreCT && b == rec.ScoreT) || (a == rec.ScoreT && b == rec.ScoreCT)
}
