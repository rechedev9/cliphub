package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// subtitleChoices are the subtitles of a Short, a summary or a Full Demo.
type subtitleChoices struct {
	Subtitles       *bool    `json:"subtitles,omitempty" jsonschema:"burn what the team voices say, in the language they speak, with the speaker's name in front and one fixed colour per player (the POV player in white), at most 2 at once in 9:16 and 3 in 16:9; needs the team voices in the video (voice_comms, or team_voices in plan_full_demo) and a demo that recorded them; creating the video queues the transcription when the demo has none (install_transcriber), and a video rendered before it is ready goes without subtitles; default false"`
	SubtitlesHidden []string `json:"subtitles_hidden,omitempty" jsonschema:"phrase ids from demo_transcript to leave out of this video only; needs subtitles"`
}

// resolved checks the choices; voices says whether the video carries the team voices.
func (c subtitleChoices) resolved(voices bool) (on bool, hidden []string, err error) {
	on = orDefault(c.Subtitles, false)
	if len(c.SubtitlesHidden) > recapplan.MaxHiddenSubtitles {
		return false, nil, fmt.Errorf("subtitles_hidden takes at most %d ids", recapplan.MaxHiddenSubtitles)
	}
	for _, id := range c.SubtitlesHidden {
		id = strings.ToLower(strings.TrimSpace(id))
		if err := checkUUID("subtitles_hidden", id); err != nil {
			return false, nil, err
		}
		hidden = append(hidden, id)
	}
	switch {
	case !on && len(hidden) > 0:
		return false, nil, errors.New("subtitles_hidden needs subtitles")
	case on && !voices:
		return false, nil, errors.New("subtitles need the team voices in the video: they write what the voices say")
	}
	return on, hidden, nil
}

func resolveSubtitles(edit *tuiclient.ShortEdit, c subtitleChoices) error {
	var err error
	edit.Subtitles, edit.SubtitlesHidden, err = c.resolved(edit.VoiceComms)
	return err
}

func applySubtitles(options *recapplan.Options, c subtitleChoices) error {
	on, hidden, err := c.resolved(options.Audio.Voice.Enabled)
	if on {
		options.Subtitles = &recapplan.SubtitleOptions{Enabled: true, Hidden: hidden}
	}
	return err
}

// describeSubtitles words the subtitles for the approval summary.
func describeSubtitles(on bool, hidden int) string {
	if !on {
		return ""
	}
	out := ", subtítulos de las voces"
	if hidden > 0 {
		out += fmt.Sprintf(" sin %s", plural(hidden, "frase oculta", "frases ocultas"))
	}
	return out
}
