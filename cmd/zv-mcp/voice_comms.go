package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// voicePhases are the round phases talk time is split by.
var voicePhases = []string{"freezetime", "midround", "post_plant", "dead", "after_round"}

const (
	maxRoundCommsLines = 300
	maxCommsTextRunes  = 400
)

type transcribeVoiceIn struct {
	MatchID  string `json:"match_id" jsonschema:"library match id"`
	Language string `json:"language,omitempty" jsonschema:"spoken language: es (default), en, pt, fr, de or auto"`
}

type roundCommsIn struct {
	MatchID string `json:"match_id" jsonschema:"library match id"`
	Round   int    `json:"round" jsonschema:"round number"`
}

type searchCommsIn struct {
	Text    string `json:"text" jsonschema:"words to find in the transcripts, as a phrase, case-insensitive"`
	TeamID  string `json:"team_id,omitempty" jsonschema:"only matches of this team (list_teams)"`
	MatchID string `json:"match_id,omitempty" jsonschema:"only this match"`
	From    string `json:"from,omitempty" jsonschema:"only matches played on or after this date, YYYY-MM-DD"`
	To      string `json:"to,omitempty" jsonschema:"only matches played on or before this date, YYYY-MM-DD"`
	Period  string `json:"period,omitempty" jsonschema:"only phrases said in this round phase: freezetime, midround, post_plant, dead or after_round"`
	Limit   int    `json:"limit,omitempty" jsonschema:"at most this many hits, 1 to 200; default 50"`
}

type installTranscriberIn struct {
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type transcriberOut struct {
	pendingOut
	State         string `json:"state" jsonschema:"not_installed, installing, installed, failed or unsupported"`
	Version       string `json:"version"`
	Model         string `json:"model"`
	DownloadBytes int64  `json:"download_bytes"`
	DoneBytes     int64  `json:"done_bytes,omitempty"`
	ErrorCode     string `json:"error_code,omitempty" jsonschema:"runtime_missing (install the Visual C++ runtime), start_failed or download_failed"`
	Error         string `json:"error,omitempty"`
}

type speakerTalkOut struct {
	Track     int    `json:"track"`
	Label     string `json:"label"`
	SteamID64 string `json:"steamid64,omitempty"`
	tuiclient.VoicePhaseTalk
	TotalMS int64 `json:"total_ms"`
}

type commsLineOut struct {
	Track      int    `json:"track"`
	Speaker    string `json:"speaker"`
	Round      int    `json:"round"`
	RoundClock string `json:"round_clock" jsonschema:"time since the round started, m:ss"`
	Phase      string `json:"phase,omitempty"`
	Tick       int    `json:"tick"`
	Text       string `json:"text"`
	StudioPath string `json:"studio_path" jsonschema:"opens the Replay at the moment the phrase was said"`
}

type roundCommsOut struct {
	MatchID    string           `json:"match_id"`
	Round      int              `json:"round"`
	Sync       string           `json:"sync" jsonschema:"what the Replay shows about the sync; times are off while the voice is unsynced"`
	Transcript string           `json:"transcript" jsonschema:"none, queued, running, ready or failed"`
	Speakers   []speakerTalkOut `json:"speakers"`
	Lines      []commsLineOut   `json:"lines"`
	Truncated  bool             `json:"truncated,omitempty"`
}

type roundTalkOut struct {
	Round    int              `json:"round"`
	Speakers []speakerTalkOut `json:"speakers"`
}

type matchTalkOut struct {
	MatchID string           `json:"match_id"`
	Sync    string           `json:"sync"`
	Totals  []speakerTalkOut `json:"totals"`
	Rounds  []roundTalkOut   `json:"rounds"`
}

type commsHitOut struct {
	MatchID  string   `json:"match_id"`
	Map      string   `json:"map"`
	PlayedAt string   `json:"played_at"`
	Teams    []string `json:"teams"`
	commsLineOut
}

type searchCommsOut struct {
	Hits      []commsHitOut `json:"hits"`
	Truncated bool          `json:"truncated,omitempty"`
}

func transcriberOutOf(t tuiclient.VoiceTranscriber) transcriberOut {
	return transcriberOut{State: t.State, Version: t.Version, Model: t.Model, DownloadBytes: t.DownloadBytes, DoneBytes: t.DoneBytes, ErrorCode: t.ErrorCode, Error: cleanText(t.Error, maxErrorRunes)}
}

func speakersOf(talk []tuiclient.VoiceSpeakerTalk) []speakerTalkOut {
	out := make([]speakerTalkOut, 0, len(talk))
	for _, t := range talk {
		p := t.VoicePhaseTalk
		total := p.FreezetimeMS + p.MidroundMS + p.PostPlantMS + p.DeadMS + p.AfterRoundMS
		out = append(out, speakerTalkOut{Track: t.Track, Label: cleanName(t.Label), SteamID64: t.SteamID64, VoicePhaseTalk: p, TotalMS: total})
	}
	return out
}

func commsLineOf(matchID string, l tuiclient.VoiceLine) commsLineOut {
	return commsLineOut{Track: l.Track, Speaker: cleanName(l.Label), Round: l.Round, RoundClock: clockMS(l.RoundMS), Phase: l.Phase, Tick: l.Tick,
		Text: cleanText(l.Text, maxCommsTextRunes), StudioPath: "/replay?r=" + matchID + ":" + strconv.Itoa(max(l.Round, 1)) + ":" + strconv.Itoa(l.Tick)}
}

func registerVoiceCommsTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "install_transcriber", Annotations: &mcp.ToolAnnotations{},
		Description: "Install local transcription (whisper.cpp with its small multilingual model, about 200 MB, fetched once into Studio's folder) as Instalar transcripción does. " +
			"It downloads about 200 MB, so the user must approve it. " + deleteApprovalHint + " It runs in the background: read voice_status or this tool's state until installed. Nothing leaves the PC when it transcribes."},
		func(ctx context.Context, req *mcp.CallToolRequest, in installTranscriberIn) (*mcp.CallToolResult, transcriberOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			now, err := c.VoiceTranscriberStatus(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			if now.State == "installed" || now.State == "installing" || now.State == "unsupported" {
				return nil, transcriberOutOf(now), nil
			}
			summary := fmt.Sprintf("Descargar la transcripción local (whisper.cpp %s, unos %d MB) en la carpeta de Studio.", now.Version, max(1, now.DownloadBytes>>20))
			ask, pending, err := gate(req, approvalSpec{Tool: "install_transcriber", JobID: now.Version, Request: now.Model, Summary: summary, Given: in.Approve, Question: "¿Descargar ahora?"})
			if err != nil || ask != nil {
				return ask, transcriberOut{}, err
			}
			if pending != nil {
				out := transcriberOutOf(now)
				out.pendingOut = *pending
				return nil, out, nil
			}
			t, err := c.InstallVoiceTranscriber(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			return nil, transcriberOutOf(t), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "transcribe_voice",
		Description: "Transcribe a match's team voice on this PC, as Transcribir does in the Replay. Needs install_transcriber first. It runs in the background; voice_status shows the progress until the transcript is ready."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in transcribeVoiceIn) (*mcp.CallToolResult, voiceOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, voiceOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, voiceOut{}, toolError(err)
			}
			v, err := c.TranscribeMatchVoice(ctx, in.MatchID, strings.ToLower(strings.TrimSpace(in.Language)))
			if err != nil {
				return nil, voiceOut{}, toolError(err)
			}
			return nil, voiceOutOf(v), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_round_comms", Annotations: readOnly,
		Description: "Read one round's comms: talk time of each speaker by phase (freezetime, midround, post_plant, dead, after_round) and the transcribed phrases with their round clock and a Replay path to each moment."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in roundCommsIn) (*mcp.CallToolResult, roundCommsOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, roundCommsOut{}, err
			}
			if in.Round < 1 {
				return nil, roundCommsOut{}, errors.New("round must be a round number")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, roundCommsOut{}, toolError(err)
			}
			comms, err := c.MatchVoiceComms(ctx, in.MatchID, in.Round)
			if err != nil {
				return nil, roundCommsOut{}, toolError(err)
			}
			out := roundCommsOut{MatchID: in.MatchID, Round: in.Round, Sync: voiceSyncLabel(comms.Sync), Transcript: comms.Transcript.State,
				Speakers: []speakerTalkOut{}, Lines: []commsLineOut{}}
			for _, r := range comms.Rounds {
				if r.Round == in.Round {
					out.Speakers = speakersOf(r.Speakers)
				}
			}
			for _, l := range comms.Lines {
				if len(out.Lines) == maxRoundCommsLines {
					out.Truncated = true
					break
				}
				out.Lines = append(out.Lines, commsLineOf(in.MatchID, l))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "match_talk_time", Annotations: readOnly,
		Description: "Read a match's talk time: each speaker's total by phase, and each round's. Shows who calls, and who talks in freezetime or when dead."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in voiceMatchIn) (*mcp.CallToolResult, matchTalkOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, matchTalkOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, matchTalkOut{}, toolError(err)
			}
			comms, err := c.MatchVoiceComms(ctx, in.MatchID, 0)
			if err != nil {
				return nil, matchTalkOut{}, toolError(err)
			}
			out := matchTalkOut{MatchID: in.MatchID, Sync: voiceSyncLabel(comms.Sync), Totals: speakersOf(comms.Totals), Rounds: []roundTalkOut{}}
			for _, r := range comms.Rounds {
				out.Rounds = append(out.Rounds, roundTalkOut{Round: r.Round, Speakers: speakersOf(r.Speakers)})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "search_comms", Annotations: readOnly,
		Description: "Find a phrase in every transcribed match, as the comms search does: narrow by team, match, dates and round phase (period). Each hit has the round, round clock and a Replay path to the moment."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchCommsIn) (*mcp.CallToolResult, searchCommsOut, error) {
			text := strings.Join(strings.Fields(in.Text), " ")
			if text == "" || len([]rune(text)) > 120 {
				return nil, searchCommsOut{}, errors.New("text must be 1 to 120 characters")
			}
			for field, id := range map[string]string{"team_id": in.TeamID, "match_id": in.MatchID} {
				if id == "" {
					continue
				}
				if err := checkUUID(field, id); err != nil {
					return nil, searchCommsOut{}, err
				}
			}
			if in.Period != "" && !slices.Contains(voicePhases, in.Period) {
				return nil, searchCommsOut{}, errors.New("period must be freezetime, midround, post_plant, dead or after_round")
			}
			limit := in.Limit
			if limit == 0 {
				limit = 50
			}
			if limit < 1 || limit > 200 {
				return nil, searchCommsOut{}, errors.New("limit must be from 1 to 200")
			}
			from, to, err := parseDateRange(in.From, in.To)
			if err != nil {
				return nil, searchCommsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, searchCommsOut{}, toolError(err)
			}
			query := tuiclient.VoiceSearchQuery{Query: text, TeamID: in.TeamID, MatchID: in.MatchID, Phase: in.Period, Limit: limit}
			if !from.IsZero() {
				query.From = from.Format(time.RFC3339)
			}
			if !to.IsZero() {
				query.To = to.Format(time.RFC3339)
			}
			hits, truncated, err := c.SearchVoice(ctx, query)
			if err != nil {
				return nil, searchCommsOut{}, toolError(err)
			}
			out := searchCommsOut{Hits: []commsHitOut{}, Truncated: truncated}
			for _, h := range hits {
				teams := make([]string, 0, len(h.Teams))
				for _, t := range h.Teams {
					teams = append(teams, cleanName(t))
				}
				out.Hits = append(out.Hits, commsHitOut{MatchID: h.MatchID, Map: h.Map, PlayedAt: h.PlayedAt.Format("2006-01-02"), Teams: teams,
					commsLineOut: commsLineOf(h.MatchID, h.VoiceLine)})
			}
			return nil, out, nil
		})
}
