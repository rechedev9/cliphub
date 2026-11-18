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
	Subtitles        *bool    `json:"subtitles,omitempty" jsonschema:"burn what the team voices say, with the speaker's name in front; needs the team voices in the video (voice_comms, or team_voices in plan_full_demo) and a demo that recorded them; the render transcribes the demo first when it was not (install_transcriber); default false"`
	SubtitlesEnglish *bool    `json:"subtitles_english,omitempty" jsonschema:"show the subtitles only in English; needs subtitles and install_translator; default false"`
	SubtitlesHidden  []string `json:"subtitles_hidden,omitempty" jsonschema:"phrase ids from demo_transcript to leave out of this video only; needs subtitles"`
}

// resolved checks the choices; voices says whether the video carries the team voices.
func (c subtitleChoices) resolved(voices bool) (on, english bool, hidden []string, err error) {
	on, english = orDefault(c.Subtitles, false), orDefault(c.SubtitlesEnglish, false)
	if len(c.SubtitlesHidden) > recapplan.MaxHiddenSubtitles {
		return false, false, nil, fmt.Errorf("subtitles_hidden takes at most %d ids", recapplan.MaxHiddenSubtitles)
	}
	for _, id := range c.SubtitlesHidden {
		id = strings.ToLower(strings.TrimSpace(id))
		if err := checkUUID("subtitles_hidden", id); err != nil {
			return false, false, nil, err
		}
		hidden = append(hidden, id)
	}
	switch {
	case !on && (english || len(hidden) > 0):
		return false, false, nil, errors.New("subtitles_english and subtitles_hidden need subtitles")
	case on && !voices:
		return false, false, nil, errors.New("subtitles need the team voices in the video: they write what the voices say")
	}
	return on, english, hidden, nil
}

func resolveSubtitles(edit *tuiclient.ShortEdit, c subtitleChoices) error {
	var err error
	edit.Subtitles, edit.SubtitlesEnglish, edit.SubtitlesHidden, err = c.resolved(edit.VoiceComms)
	return err
}

func applySubtitles(options *recapplan.Options, c subtitleChoices) error {
	on, english, hidden, err := c.resolved(options.Audio.Voice.Enabled)
	if on {
		options.Subtitles = &recapplan.SubtitleOptions{Enabled: true, English: english, Hidden: hidden}
	}
	return err
}

// describeSubtitles words the subtitles for the approval summary.
func describeSubtitles(on, english bool, hidden int) string {
	if !on {
		return ""
	}
	out := ", subtítulos de las voces"
	if english {
		out += " en inglés"
	}
	if hidden > 0 {
		out += fmt.Sprintf(" sin %s", plural(hidden, "frase oculta", "frases ocultas"))
	}
	return out
}
