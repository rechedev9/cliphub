package voiceplugin

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const (
	// A real match date sits around the recording: a little before the plugin started, or after it.
	playedBefore = 3 * time.Hour
	playedAfter  = 3 * time.Hour
	// A match whose date is its import time may come in days after the recording.
	importedAfter = 14 * 24 * time.Hour
	// A match took its import time as date when the two are this close.
	importDateSlack = time.Hour
)

// Pick is the match a recording goes to, or why it waits.
type Pick struct {
	MatchID *uuid.UUID
	Note    string
}

// PickMatch finds the one library match a recording belongs to, by map, time and final score.
// Only a complete recording with a score attaches on its own; candidates have no team voice yet.
func PickMatch(rec library.VoiceRecording, candidates []library.Match) Pick {
	if rec.Map == "" {
		return Pick{Note: "Studio no vio la partida en CS2 mientras grababas: asóciala a mano."}
	}
	scored := rec.ScoreCT >= 0 && rec.ScoreT >= 0 && rec.ScoreCT+rec.ScoreT > 0
	if !scored || rec.Interrupted {
		return Pick{Note: fmt.Sprintf("La grabación no tiene el marcador final de %s: elige su partida con Asociar.", rec.Map)}
	}
	var fits []library.Match
	for _, m := range candidates {
		if strings.EqualFold(m.Map, rec.Map) && inWindow(rec, m) && scoreMatches(m, rec) {
			fits = append(fits, m)
		}
	}
	switch len(fits) {
	case 0:
		return Pick{Note: fmt.Sprintf("Esperando la demo: importa la partida de %s jugada el %s y se asociará sola.", rec.Map, rec.StartedAt.Local().Format("02/01/2006"))}
	case 1:
		id := fits[0].ID
		return Pick{MatchID: &id}
	}
	return Pick{Note: fmt.Sprintf("%d partidas de %s cuadran con esta grabación: elige la suya.", len(fits), rec.Map)}
}

// inWindow keeps a match dated around the recording, or imported in the days after it.
func inWindow(rec library.VoiceRecording, m library.Match) bool {
	if m.PlayedAt.Before(rec.StartedAt.Add(-playedBefore)) {
		return false
	}
	after := playedAfter
	if d := m.CreatedAt.Sub(m.PlayedAt); d >= 0 && d <= importDateSlack {
		after = importedAfter
	}
	return !m.PlayedAt.After(rec.EndedAt.Add(after))
}

func scoreMatches(m library.Match, rec library.VoiceRecording) bool {
	if len(m.Teams) != 2 {
		return false
	}
	a, b := m.Teams[0].Score, m.Teams[1].Score
	return (a == rec.ScoreCT && b == rec.ScoreT) || (a == rec.ScoreT && b == rec.ScoreCT)
}
