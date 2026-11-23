package main

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	maxStreamPhrases = 300
	maxStreamSpans   = 500
)

type streamSpanIn struct {
	StartSeconds float64 `json:"start_seconds" jsonschema:"where the stretch starts, in seconds of the stream"`
	EndSeconds   float64 `json:"end_seconds" jsonschema:"where the stretch ends, in seconds of the stream"`
}

type transcribeStreamIn struct {
	ID    string         `json:"id" jsonschema:"stream id from list_streams or import_stream"`
	Spans []streamSpanIn `json:"spans" jsonschema:"the stretches that go into a video: the momentos of get_stream_edit_plan for Shorts, or the jugadas of find_stream_plays for the resumen"`
	Force bool           `json:"force,omitempty" jsonschema:"transcribe again stretches already transcribed; a phrase heard again with the same words keeps its correction and its discard"`
}

type streamTranscriptIn struct {
	ID          string  `json:"id" jsonschema:"stream id from list_streams or import_stream"`
	FromSeconds float64 `json:"from_seconds,omitempty" jsonschema:"list only the phrases that start at this second of the stream or later; for the next page when truncated is true, pass the start_seconds of the last phrase listed"`
}

type correctStreamPhraseIn struct {
	ID       string `json:"id" jsonschema:"stream id"`
	PhraseID string `json:"phrase_id" jsonschema:"phrase id from stream_transcript"`
	Text     string `json:"text" jsonschema:"the corrected phrase, 1 to 2000 characters"`
}

type discardStreamPhraseIn struct {
	ID       string `json:"id" jsonschema:"stream id"`
	PhraseID string `json:"phrase_id" jsonschema:"phrase id from stream_transcript"`
	Restore  bool   `json:"restore,omitempty" jsonschema:"bring a discarded phrase back"`
}

type setStreamSubtitlesIn struct {
	ID       string `json:"id" jsonschema:"stream id"`
	Shorts   *bool  `json:"shorts,omitempty" jsonschema:"turn subtitles on or off for the Shorts of this stream"`
	Summary  *bool  `json:"summary,omitempty" jsonschema:"turn subtitles on or off for the resumen of this stream"`
	Video    string `json:"video,omitempty" jsonschema:"the one video to hide a phrase in: a momento id or a group id of get_stream_edit_plan for a Short, or summary for the resumen; needs phrase_id and hidden"`
	PhraseID string `json:"phrase_id,omitempty" jsonschema:"phrase id from stream_transcript"`
	Hidden   *bool  `json:"hidden,omitempty" jsonschema:"true hides the phrase in that video only, false shows it again"`
}

type streamPhraseOut struct {
	ID           string               `json:"id"`
	StartSeconds float64              `json:"start_seconds" jsonschema:"when it was said, in seconds of the stream"`
	EndSeconds   float64              `json:"end_seconds"`
	Text         string               `json:"text" jsonschema:"what a video draws: the correction when corrected"`
	Corrected    bool                 `json:"corrected,omitempty"`
	OriginalText string               `json:"original_text,omitempty" jsonschema:"what the transcriber heard, when the text differs from it"`
	VocabText    string               `json:"vocab_text,omitempty" jsonschema:"what Studio's CS2 vocabulary corrector wrote over the heard text; empty when it changed nothing"`
	VocabChanges []demoVocabChangeOut `json:"vocab_changes,omitempty" jsonschema:"the words the corrector changed: heard and written"`
	Discarded    bool                 `json:"discarded,omitempty" jsonschema:"drawn in no video of the stream"`
}

type streamSubtitlesOut struct {
	Shorts  bool                `json:"shorts" jsonschema:"subtitles are on for the Shorts of this stream"`
	Summary bool                `json:"summary" jsonschema:"subtitles are on for the resumen of this stream"`
	Hidden  map[string][]string `json:"hidden" jsonschema:"phrase ids hidden in one video, by video: a momento or group id, or summary"`
}

type streamTranscriptOut struct {
	ID           string             `json:"id"`
	State        string             `json:"state" jsonschema:"none, queued, running, ready or failed"`
	Error        string             `json:"error,omitempty"`
	Progress     float64            `json:"progress,omitempty"`
	Availability string             `json:"availability" jsonschema:"available when the stream has a mic track; mixed_track when it has only the mix of voice and game, transcribed too, where teammates and the game announcer can come in as phrases; no_audio"`
	Transcriber  string             `json:"transcriber" jsonschema:"install state of install_transcriber"`
	Transcribed  []streamSpanIn     `json:"transcribed" jsonschema:"the stretches already transcribed; the rest of the stream has no phrases"`
	Phrases      []streamPhraseOut  `json:"phrases"`
	Subtitles    streamSubtitlesOut `json:"subtitles"`
	Stale        streamStaleOut     `json:"stale" jsonschema:"videos rendered before their subtitles changed: render them again to draw the current ones"`
	Truncated    bool               `json:"truncated,omitempty" jsonschema:"true when the stream has more phrases than the 300 listed; call again with from_seconds"`
}

type streamStaleOut struct {
	Shorts  bool `json:"shorts" jsonschema:"the rendered Shorts of the saved plan's layout"`
	Summary bool `json:"summary" jsonschema:"the rendered resumen"`
}

func streamPhraseOf(l tuiclient.StreamVoiceLine) streamPhraseOut {
	return streamPhraseOut{ID: l.ID, StartSeconds: float64(l.StartMS) / 1000, EndSeconds: float64(l.EndMS) / 1000,
		Text: cleanText(l.Text, maxCommsTextRunes), Corrected: l.Corrected,
		OriginalText: cleanText(l.OriginalText, maxCommsTextRunes), VocabText: cleanText(l.VocabText, maxCommsTextRunes),
		VocabChanges: vocabChangesOf(l.VocabChanges), Discarded: l.Discarded}
}

func streamSubtitlesOf(s tuiclient.StreamSubtitles) streamSubtitlesOut {
	out := streamSubtitlesOut{Shorts: s.Shorts, Summary: s.Summary, Hidden: map[string][]string{}}
	for video, ids := range s.Hidden {
		out.Hidden[cleanName(video)] = ids
	}
	return out
}

func streamTranscriptOf(t tuiclient.StreamTranscript, fromSeconds float64) streamTranscriptOut {
	out := streamTranscriptOut{ID: t.JobID, State: t.State, Error: cleanText(t.Error, maxErrorRunes), Progress: t.Progress,
		Availability: t.Availability, Transcriber: t.Transcriber.State, Transcribed: []streamSpanIn{}, Phrases: []streamPhraseOut{},
		Subtitles: streamSubtitlesOf(t.Subtitles), Stale: streamStaleOut{Shorts: t.Stale.Shorts, Summary: t.Stale.Summary}}
	for _, s := range t.Spans {
		out.Transcribed = append(out.Transcribed, streamSpanIn{StartSeconds: float64(s.StartMS) / 1000, EndSeconds: float64(s.EndMS) / 1000})
	}
	for _, l := range t.Lines {
		if float64(l.StartMS)/1000 < fromSeconds {
			continue
		}
		if len(out.Phrases) == maxStreamPhrases {
			out.Truncated = true
			break
		}
		out.Phrases = append(out.Phrases, streamPhraseOf(l))
	}
	return out
}

func checkStreamPhrase(streamID, phraseID string) error {
	if err := checkUUID("id", streamID); err != nil {
		return err
	}
	return checkUUID("phrase_id", phraseID)
}

func registerStreamTranscriptTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "transcribe_stream",
		Description: "Transcribe what the streamer says in the stretches of a stream that go into a video, on this PC, as the Subtítulos panel of Clips de stream does. " +
			"It reads the mic track, or the only track of a stream with voice and game mixed (availability mixed_track), where teammates and the game announcer can come in: offer to discard those phrases. Stretches already transcribed are skipped unless force. " +
			"Studio downloads the transcription on its own the first time, and install_transcriber reads that download. It runs in the background; read stream_transcript until state is ready."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in transcribeStreamIn) (*mcp.CallToolResult, streamTranscriptOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamTranscriptOut{}, err
			}
			if len(in.Spans) == 0 || len(in.Spans) > maxStreamSpans {
				return nil, streamTranscriptOut{}, errors.New("spans takes 1 to 500 stretches: the momentos or jugadas that go into a video")
			}
			req := tuiclient.StreamTranscribe{JobID: in.ID, Force: in.Force}
			for _, span := range in.Spans {
				if span.StartSeconds < 0 || span.EndSeconds <= span.StartSeconds {
					return nil, streamTranscriptOut{}, errors.New("each stretch needs start_seconds before end_seconds")
				}
				req.Spans = append(req.Spans, tuiclient.StreamStretch{StartSeconds: span.StartSeconds, EndSeconds: span.EndSeconds})
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamTranscriptOut{}, toolError(err)
			}
			t, err := c.TranscribeStream(ctx, req)
			if err != nil {
				return nil, streamTranscriptOut{}, toolError(err)
			}
			return nil, streamTranscriptOf(t, 0), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "stream_transcript", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the transcript of a stream: each phrase the streamer said in the transcribed stretches, in the language it was said, with no speaker names, " +
			"with the original_text the transcriber heard when the text differs from it, vocab_text and vocab_changes when Studio's CS2 vocabulary corrector changed words (sending original_text to correct_stream_phrase undoes them), and discarded when no video draws it, plus the subtitles switches and the phrases hidden in one video. " +
			"Pass a phrase id to correct_stream_phrase, discard_stream_phrase or set_stream_subtitles. It lists 300 phrases at most: when truncated is true, call again with from_seconds."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in streamTranscriptIn) (*mcp.CallToolResult, streamTranscriptOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamTranscriptOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamTranscriptOut{}, toolError(err)
			}
			t, err := c.StreamTranscript(ctx, in.ID)
			if err != nil {
				return nil, streamTranscriptOut{}, toolError(err)
			}
			return nil, streamTranscriptOf(t, in.FromSeconds), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "correct_stream_phrase", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Correct a phrase of a stream transcript, as the Subtítulos panel does. The correction counts for every video of the stream and keeps what the transcriber heard as original_text; sending the original text again undoes it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in correctStreamPhraseIn) (*mcp.CallToolResult, streamPhraseOut, error) {
			if err := checkStreamPhrase(in.ID, in.PhraseID); err != nil {
				return nil, streamPhraseOut{}, err
			}
			if strings.TrimSpace(in.Text) == "" {
				return nil, streamPhraseOut{}, errors.New("a corrected phrase cannot be empty; hide it with set_stream_subtitles or discard it instead")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamPhraseOut{}, toolError(err)
			}
			l, err := c.EditStreamVoiceLine(ctx, tuiclient.StreamLineEdit{JobID: in.ID, LineID: in.PhraseID, Text: &in.Text})
			if err != nil {
				return nil, streamPhraseOut{}, toolError(err)
			}
			return nil, streamPhraseOf(l), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "discard_stream_phrase", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Discard a phrase the transcriber invented, as the Subtítulos panel does: no video of the stream draws it, and transcribing again keeps it discarded. " +
			"The phrase keeps its text in stream_transcript; restore brings it back. To leave a real phrase out of one video, use set_stream_subtitles instead."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in discardStreamPhraseIn) (*mcp.CallToolResult, streamPhraseOut, error) {
			if err := checkStreamPhrase(in.ID, in.PhraseID); err != nil {
				return nil, streamPhraseOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamPhraseOut{}, toolError(err)
			}
			discarded := !in.Restore
			l, err := c.EditStreamVoiceLine(ctx, tuiclient.StreamLineEdit{JobID: in.ID, LineID: in.PhraseID, Discarded: &discarded})
			if err != nil {
				return nil, streamPhraseOut{}, toolError(err)
			}
			return nil, streamPhraseOf(l), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_stream_subtitles", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Turn the subtitles of a stream on or off, as the switch of its Subtítulos panel does (shorts for the Shorts, summary for the resumen), " +
			"or hide a phrase in one video only (video, phrase_id and hidden). The next render_stream or render_stream_summary draws them; " +
			"stream_transcript stale says which videos already rendered must be rendered again."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setStreamSubtitlesIn) (*mcp.CallToolResult, streamSubtitlesOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamSubtitlesOut{}, err
			}
			edit := tuiclient.StreamSubtitlesEdit{JobID: in.ID, Shorts: in.Shorts, Summary: in.Summary, Video: strings.TrimSpace(in.Video), LineID: in.PhraseID, Hidden: in.Hidden}
			switch {
			case edit.Video == "" && (in.PhraseID != "" || in.Hidden != nil):
				return nil, streamSubtitlesOut{}, errors.New("hiding a phrase needs video, phrase_id and hidden together")
			case edit.Video == "" && in.Shorts == nil && in.Summary == nil:
				return nil, streamSubtitlesOut{}, errors.New("send shorts or summary to switch subtitles, or video, phrase_id and hidden to hide a phrase in one video")
			case edit.Video != "" && in.Hidden == nil:
				return nil, streamSubtitlesOut{}, errors.New("hiding a phrase needs video, phrase_id and hidden together")
			case edit.Video != "":
				if err := checkUUID("phrase_id", in.PhraseID); err != nil {
					return nil, streamSubtitlesOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamSubtitlesOut{}, toolError(err)
			}
			subs, err := c.EditStreamSubtitles(ctx, edit)
			if err != nil {
				return nil, streamSubtitlesOut{}, toolError(err)
			}
			return nil, streamSubtitlesOf(subs), nil
		})
}
