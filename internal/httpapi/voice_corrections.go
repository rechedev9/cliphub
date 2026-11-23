package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type phraseCorrection struct {
	Line uuid.UUID `json:"line"`
	Text string    `json:"text"`
}

type phraseCorrections struct {
	Lines []phraseCorrection `json:"lines"`
}

type correctedDemoLines struct {
	Lines []demoTranscriptLine `json:"lines"`
}

type correctedStreamLines struct {
	Lines []streamTranscriptLine `json:"lines"`
}

func decodePhraseCorrections(w http.ResponseWriter, r *http.Request) ([]library.VoiceLineText, bool) {
	var req phraseCorrections
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || len(req.Lines) == 0 || len(req.Lines) > library.MaxVoiceCorrections {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("send lines: 1 to %d phrases, each with line and text", library.MaxVoiceCorrections))
		return nil, false
	}
	seen := make(map[uuid.UUID]bool, len(req.Lines))
	out := make([]library.VoiceLineText, 0, len(req.Lines))
	for _, c := range req.Lines {
		text := strings.Join(strings.Fields(c.Text), " ")
		switch {
		case c.Line == uuid.Nil:
			writeError(w, http.StatusBadRequest, "each correction needs the line id of its phrase")
			return nil, false
		case seen[c.Line]:
			writeError(w, http.StatusBadRequest, fmt.Sprintf("phrase %s is corrected twice", c.Line))
			return nil, false
		case text == "" || len([]rune(text)) > library.MaxVoiceLineRunes:
			writeError(w, http.StatusBadRequest, "a phrase takes 1 to 2000 characters")
			return nil, false
		}
		seen[c.Line] = true
		out = append(out, library.VoiceLineText{ID: c.Line, Text: text})
	}
	return out, true
}

func (h *Handlers) CorrectLibraryMatchDemoTranscript(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	lines, ok := decodePhraseCorrections(w, r)
	if !ok {
		return
	}
	got, err := h.libraryRepo.CorrectDemoVoiceLines(r.Context(), library.DemoVoiceCorrections{MatchID: m.ID, By: library.CorrectedByAssistant, Lines: lines})
	if errors.Is(err, library.ErrNotFound) {
		writeError(w, http.StatusNotFound, "a phrase is not in this demo's transcript; nothing was corrected")
		return
	}
	if err != nil {
		internalError(w, "correct demo transcript lines", err)
		return
	}
	writeJSON(w, http.StatusOK, correctedDemoLines{Lines: demoTranscriptLines(got)})
}

func (h *Handlers) CorrectStreamTranscript(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	repo, ok := h.streamVoiceRepo(w)
	if !ok {
		return
	}
	lines, ok := decodePhraseCorrections(w, r)
	if !ok {
		return
	}
	got, err := repo.CorrectStreamVoiceLines(r.Context(), streamclips.VoiceCorrections{JobID: j.ID, By: library.CorrectedByAssistant, Lines: lines})
	if errors.Is(err, streamclips.ErrVoiceLineNotFound) {
		writeError(w, http.StatusNotFound, "a phrase is not in this stream's transcript; nothing was corrected")
		return
	}
	if err != nil {
		internalError(w, "correct stream transcript lines", err)
		return
	}
	writeJSON(w, http.StatusOK, correctedStreamLines{Lines: streamTranscriptLines(got)})
}
