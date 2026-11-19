package main

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const maxDemoPhrases = 300

type transcribeDemoVoiceIn struct {
	MatchID string `json:"match_id" jsonschema:"library match id"`
	Force   bool   `json:"force,omitempty" jsonschema:"transcribe again a demo already transcribed; corrections are lost"`
}

type demoTranscriptIn struct {
	MatchID string `json:"match_id" jsonschema:"library match id"`
	Round   int    `json:"round,omitempty" jsonschema:"only this round; 0 lists every round"`
}

type correctDemoPhraseIn struct {
	MatchID  string `json:"match_id" jsonschema:"library match id"`
	PhraseID string `json:"phrase_id" jsonschema:"phrase id from demo_transcript"`
	Text     string `json:"text" jsonschema:"the corrected phrase, 1 to 2000 characters"`
}

type demoPhraseOut struct {
	ID         string `json:"id"`
	Speaker    string `json:"speaker"`
	SteamID64  string `json:"steamid64"`
	Round      int    `json:"round"`
	Tick       int    `json:"tick"`
	Text       string `json:"text"`
	Corrected  bool   `json:"corrected,omitempty"`
	StudioPath string `json:"studio_path" jsonschema:"opens the Replay at the moment the phrase was said"`
}

type demoTranscriptOut struct {
	MatchID     string          `json:"match_id"`
	State       string          `json:"state" jsonschema:"none, queued, running, ready or failed"`
	Error       string          `json:"error,omitempty"`
	Progress    float64         `json:"progress,omitempty"`
	Voice       string          `json:"voice" jsonschema:"what the demo holds: none, extracting, ready, failed or unavailable"`
	Transcriber string          `json:"transcriber" jsonschema:"install state of install_transcriber"`
	Phrases     []demoPhraseOut `json:"phrases"`
	Truncated   bool            `json:"truncated,omitempty"`
}

func demoPhraseOf(matchID string, l tuiclient.DemoVoiceLine) demoPhraseOut {
	return demoPhraseOut{ID: l.ID, Speaker: cleanName(l.Name), SteamID64: l.SteamID64, Round: l.Round, Tick: l.Tick,
		Text: cleanText(l.Text, maxCommsTextRunes), Corrected: l.Corrected,
		StudioPath: "/replay?r=" + matchID + ":" + strconv.Itoa(max(l.Round, 1)) + ":" + strconv.Itoa(l.Tick)}
}

func demoTranscriptOf(t tuiclient.DemoTranscript, round int) demoTranscriptOut {
	out := demoTranscriptOut{MatchID: t.MatchID, State: t.State, Error: cleanText(t.Error, maxErrorRunes),
		Progress: t.Progress, Voice: t.Voice, Transcriber: t.Transcriber.State, Phrases: []demoPhraseOut{}}
	for _, l := range t.Lines {
		if round > 0 && l.Round != round {
			continue
		}
		if len(out.Phrases) == maxDemoPhrases {
			out.Truncated = true
			break
		}
		out.Phrases = append(out.Phrases, demoPhraseOf(t.MatchID, l))
	}
	return out
}

func registerDemoTranscriptTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "transcribe_demo_voice",
		Description: "Transcribe the team voices a match's demo recorded, on this PC, as the Subtítulos panel does. One transcript per demo: a done one is reused, and a render with subtitles queues it on its own. " +
			"Needs install_transcriber. It runs in the background; read demo_transcript until state is ready."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in transcribeDemoVoiceIn) (*mcp.CallToolResult, demoTranscriptOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, demoTranscriptOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoTranscriptOut{}, toolError(err)
			}
			t, err := c.TranscribeDemoVoice(ctx, tuiclient.DemoTranscribe{MatchID: in.MatchID, Force: in.Force})
			if err != nil {
				return nil, demoTranscriptOut{}, toolError(err)
			}
			return nil, demoTranscriptOf(t, 0), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "demo_transcript", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the transcript of the team voices a demo recorded: each phrase, in the language it was said, with its id, speaker, round and text. Pass an id to correct_demo_phrase, " +
			"or to subtitles_hidden of generate_short or plan_full_demo to leave it out of one video."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in demoTranscriptIn) (*mcp.CallToolResult, demoTranscriptOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, demoTranscriptOut{}, err
			}
			if in.Round < 0 {
				return nil, demoTranscriptOut{}, errors.New("round must be a round number, or 0 for every round")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoTranscriptOut{}, toolError(err)
			}
			t, err := c.DemoTranscript(ctx, in.MatchID)
			if err != nil {
				return nil, demoTranscriptOut{}, toolError(err)
			}
			return nil, demoTranscriptOf(t, in.Round), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "correct_demo_phrase", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Correct a phrase of a demo transcript, as the Subtítulos panel does. The correction is kept for every later video of the demo and for the comms tools."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in correctDemoPhraseIn) (*mcp.CallToolResult, demoPhraseOut, error) {
			for field, id := range map[string]string{"match_id": in.MatchID, "phrase_id": in.PhraseID} {
				if err := checkUUID(field, id); err != nil {
					return nil, demoPhraseOut{}, err
				}
			}
			if strings.TrimSpace(in.Text) == "" {
				return nil, demoPhraseOut{}, errors.New("a corrected phrase cannot be empty; hide it with subtitles_hidden instead")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoPhraseOut{}, toolError(err)
			}
			l, err := c.CorrectDemoVoiceLine(ctx, tuiclient.DemoLineEdit{MatchID: in.MatchID, LineID: in.PhraseID, Text: in.Text})
			if err != nil {
				return nil, demoPhraseOut{}, toolError(err)
			}
			return nil, demoPhraseOf(in.MatchID, l), nil
		})
}
