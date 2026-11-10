package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxVoiceFileBytes mirrors the orchestrator's cap on one voice import.
const maxVoiceFileBytes = 2 << 30

type voiceMatchIn struct {
	MatchID string `json:"match_id" jsonschema:"library match id from list_matches or find_rounds"`
}

type importVoiceIn struct {
	MatchID   string `json:"match_id" jsonschema:"library match id from list_matches or find_rounds"`
	AudioPath string `json:"audio_path" jsonschema:"absolute path on this PC to a TeamSpeak recording (wav, ogg, mp3, m4a or flac); a file with more than two channels gives one track per channel"`
}

type syncVoiceIn struct {
	MatchID  string `json:"match_id" jsonschema:"library match id"`
	OffsetMS *int64 `json:"offset_ms,omitempty" jsonschema:"confirmed offset in milliseconds: the demo time at which the recording starts; negative when the recording started before the demo"`
	Auto     bool   `json:"auto,omitempty" jsonschema:"true to estimate the offset again instead of setting one"`
}

type setVoiceTrackIn struct {
	MatchID   string  `json:"match_id" jsonschema:"library match id"`
	Track     int     `json:"track" jsonschema:"track number from voice_status"`
	Label     *string `json:"label,omitempty" jsonschema:"name shown for the track, at most 60 characters"`
	SteamID64 *string `json:"steamid64,omitempty" jsonschema:"SteamID64 of the match player who speaks on it (get_match), or empty to clear"`
}

type deleteVoiceIn struct {
	MatchID string `json:"match_id" jsonschema:"library match id"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type voiceTrackOut struct {
	Track      int    `json:"track"`
	Label      string `json:"label"`
	SteamID64  string `json:"steamid64,omitempty"`
	FileName   string `json:"file_name"`
	Duration   string `json:"duration"`
	DurationMS int64  `json:"duration_ms"`
}

type voiceSyncOut struct {
	State        string  `json:"state" jsonschema:"unsynced (a guess the user has not confirmed), auto (estimated with confidence) or manual (set by hand)"`
	Method       string  `json:"method" jsonschema:"demo_voice, kills, round_start or manual"`
	OffsetMS     int64   `json:"offset_ms"`
	AutoOffsetMS int64   `json:"auto_offset_ms" jsonschema:"the automatic estimate Reset goes back to"`
	Confidence   float64 `json:"confidence"`
	Note         string  `json:"note,omitempty"`
	Label        string  `json:"label" jsonschema:"what the Replay shows about the sync"`
}

type voiceOut struct {
	MatchID     string                      `json:"match_id"`
	Present     bool                        `json:"present"`
	Tracks      []voiceTrackOut             `json:"tracks"`
	Duration    string                      `json:"duration,omitempty"`
	SpeechMS    int64                       `json:"speech_ms"`
	Sync        voiceSyncOut                `json:"sync"`
	Transcript  string                      `json:"transcript" jsonschema:"none, queued, running, ready or failed"`
	Progress    float64                     `json:"progress,omitempty" jsonschema:"share of a running transcription done, 0 to 1"`
	Error       string                      `json:"error,omitempty"`
	Transcriber string                      `json:"transcriber" jsonschema:"not_installed, installing, installed, failed or unsupported"`
	Files       []tuiclient.VoiceFileResult `json:"files,omitempty"`
	StudioPath  string                      `json:"studio_path" jsonschema:"opens the match's first round in the Replay, where the voice plays"`
}

func voiceSyncLabel(s tuiclient.VoiceSync) string {
	switch s.State {
	case "manual":
		return "Sincronizada a mano"
	case "auto":
		return fmt.Sprintf("Sincronizada sola (confianza %.0f %%)", s.Confidence*100)
	}
	return "La voz aún no está sincronizada"
}

func voiceOutOf(v tuiclient.MatchVoice) voiceOut {
	out := voiceOut{MatchID: v.MatchID, Present: v.Present, Tracks: []voiceTrackOut{}, SpeechMS: v.SpeechMS,
		Sync: voiceSyncOut{State: v.Sync.State, Method: v.Sync.Method, OffsetMS: v.Sync.OffsetMS, AutoOffsetMS: v.Sync.AutoOffsetMS,
			Confidence: v.Sync.Confidence, Note: cleanText(v.Sync.Note, 300), Label: voiceSyncLabel(v.Sync)},
		Transcript: v.Transcript.State, Progress: v.Transcript.Progress, Error: cleanText(v.Transcript.Error, maxErrorRunes),
		Transcriber: v.Transcriber.State, Files: v.Files, StudioPath: "/replay?r=" + v.MatchID + ":1"}
	if v.DurationMS > 0 {
		out.Duration = clockMS(v.DurationMS)
	}
	for _, t := range v.Tracks {
		out.Tracks = append(out.Tracks, voiceTrackOut{Track: t.Track, Label: cleanName(t.Label), SteamID64: t.SteamID64,
			FileName: cleanName(t.FileName), Duration: clockMS(t.DurationMS), DurationMS: t.DurationMS})
	}
	return out
}

// clockMS writes milliseconds as m:ss, or h:mm:ss past an hour; negative times keep their sign.
func clockMS(ms int64) string {
	sign := ""
	if ms < 0 {
		sign, ms = "-", -ms
	}
	s := ms / 1000
	if s >= 3600 {
		return fmt.Sprintf("%s%d:%02d:%02d", sign, s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%s%d:%02d", sign, s/60, s%60)
}

func registerVoiceTools(server *mcp.Server, s *studio) {
	registerVoiceCommsTools(server, s)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	call := func(ctx context.Context, matchID string, do func(*tuiclient.Client) (tuiclient.MatchVoice, error)) (*mcp.CallToolResult, voiceOut, error) {
		if err := checkUUID("match_id", matchID); err != nil {
			return nil, voiceOut{}, err
		}
		c, err := s.client(ctx)
		if err != nil {
			return nil, voiceOut{}, toolError(err)
		}
		v, err := do(c)
		if err != nil {
			return nil, voiceOut{}, toolError(err)
		}
		return nil, voiceOutOf(v), nil
	}

	mcp.AddTool(server, &mcp.Tool{Name: "import_voice",
		Description: "Import a TeamSpeak recording of a library match, as Importar voz does in the Replay. The first import places it on the demo on its own " +
			"(by the demo's in-game voice, the kills, or round 1 start) and returns the sync state; confirm or fix it with sync_voice. A recording that does not fit the demo stays unsynced with a note."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importVoiceIn) (*mcp.CallToolResult, voiceOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, voiceOut{}, err
			}
			if err := checkLocalFile(in.AudioPath, "audio", maxVoiceFileBytes); err != nil {
				return nil, voiceOut{}, err
			}
			c, err := s.uploadClient(ctx)
			if err != nil {
				return nil, voiceOut{}, toolError(err)
			}
			v, err := c.ImportMatchVoice(ctx, in.MatchID, in.AudioPath)
			if err != nil {
				return nil, voiceOut{}, toolError(err)
			}
			return nil, voiceOutOf(v), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "sync_voice",
		Description: "Set when a match's team voice starts on its demo, as the Sincronizar bar does: offset_ms confirms an offset (the user's nudge), auto true estimates it again. Returns the sync state."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in syncVoiceIn) (*mcp.CallToolResult, voiceOut, error) {
			if (in.OffsetMS == nil) == !in.Auto {
				return nil, voiceOut{}, errors.New("pass offset_ms, or auto true")
			}
			return call(ctx, in.MatchID, func(c *tuiclient.Client) (tuiclient.MatchVoice, error) {
				if in.Auto {
					return c.AutoSyncVoice(ctx, in.MatchID)
				}
				return c.SetVoiceOffset(ctx, in.MatchID, *in.OffsetMS)
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "voice_status", Annotations: readOnly,
		Description: "Read a match's team voice: its tracks, the sync state and offset, and the transcript and transcriber states."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in voiceMatchIn) (*mcp.CallToolResult, voiceOut, error) {
			return call(ctx, in.MatchID, func(c *tuiclient.Client) (tuiclient.MatchVoice, error) { return c.GetMatchVoice(ctx, in.MatchID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_voice_track",
		Description: "Name a voice track or tie it to the match player who speaks on it, as the track menu in the Replay does. A track tied to a player splits talk time into alive and dead."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setVoiceTrackIn) (*mcp.CallToolResult, voiceOut, error) {
			if in.Track < 1 || (in.Label == nil && in.SteamID64 == nil) {
				return nil, voiceOut{}, errors.New("pass a track number and label or steamid64")
			}
			return call(ctx, in.MatchID, func(c *tuiclient.Client) (tuiclient.MatchVoice, error) {
				return c.UpdateVoiceTrack(ctx, tuiclient.VoiceTrackEdit{MatchID: in.MatchID, Track: in.Track, Label: in.Label, SteamID64: in.SteamID64})
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_voice", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a match's team voice: its audio files, sync and transcript, as Quitar voz does in the Replay. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteVoiceIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			v, err := c.GetMatchVoice(ctx, in.MatchID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			if !v.Present {
				return nil, deleteOut{}, errors.New("this match has no team voice")
			}
			summary := fmt.Sprintf("Borrar la voz del equipo de esta partida (%s, %s) y su transcripción. No se puede deshacer.",
				plural(len(v.Tracks), "pista", "pistas"), clockMS(v.DurationMS))
			return approveAndDelete(req, approvalSpec{Tool: "delete_voice", JobID: in.MatchID, Request: len(v.Tracks), Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteMatchVoice(ctx, in.MatchID) })
		})
}
