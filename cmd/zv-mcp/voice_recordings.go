package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// recordingIDPattern is the plugin's folder name for a recording: its UTC start and 4 hex digits.
var recordingIDPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{4}$`)

type recordingTrackOut struct {
	Nickname   string `json:"nickname" jsonschema:"TeamSpeak nickname"`
	Self       bool   `json:"self,omitempty" jsonschema:"the user's own microphone"`
	Duration   string `json:"duration"`
	MatchTrack int    `json:"match_track,omitempty" jsonschema:"the voice track it became in the attached match (voice_status, set_voice_speakers)"`
	Failed     string `json:"failed,omitempty" jsonschema:"why Studio could not read this track; the other tracks still attach"`
}

type recordingOut struct {
	ID          string              `json:"id"`
	Status      string              `json:"status" jsonschema:"pending (waiting for its match) or attached"`
	StartedAt   string              `json:"started_at"`
	Duration    string              `json:"duration"`
	Map         string              `json:"map,omitempty" jsonschema:"map CS2 reported while recording; empty when Studio did not see the match"`
	Server      string              `json:"server,omitempty"`
	Channel     string              `json:"channel,omitempty"`
	Trigger     string              `json:"trigger" jsonschema:"auto (CS2 said a match was live) or manual (the plugin's hotkey)"`
	Score       string              `json:"score,omitempty" jsonschema:"last score CS2 reported, CT-T at that moment"`
	Interrupted bool                `json:"interrupted,omitempty" jsonschema:"TeamSpeak closed before the recording stopped"`
	Tracks      []recordingTrackOut `json:"tracks"`
	MatchID     string              `json:"match_id,omitempty"`
	Note        string              `json:"note,omitempty" jsonschema:"why it waits, or how it was attached"`
	StudioPath  string              `json:"studio_path,omitempty" jsonschema:"opens the attached match in the Replay"`
}

func recordingOutOf(r tuiclient.VoiceRecording) recordingOut {
	out := recordingOut{ID: r.ID, Status: r.Status, StartedAt: r.StartedAt.Local().Format("2006-01-02 15:04"), Duration: clockMS(r.DurationMS),
		Map: cleanName(r.Map), Server: cleanName(r.Server), Channel: cleanName(r.Channel), Trigger: r.Trigger, Interrupted: r.Interrupted,
		Tracks: []recordingTrackOut{}, MatchID: r.MatchID, Note: cleanText(r.Note, 300)}
	if r.ScoreCT >= 0 && r.ScoreT >= 0 && r.ScoreCT+r.ScoreT > 0 {
		out.Score = fmt.Sprintf("%d-%d", r.ScoreCT, r.ScoreT)
	}
	if r.MatchID != "" {
		out.StudioPath = "/replay?r=" + r.MatchID + ":1"
	}
	for _, t := range r.Tracks {
		out.Tracks = append(out.Tracks, recordingTrackOut{Nickname: cleanName(t.Nickname), Self: t.Self, Duration: clockMS(t.DurationMS), MatchTrack: t.MatchTrack, Failed: t.Failed})
	}
	return out
}

type listRecordingsIn struct {
	Status string `json:"status,omitempty" jsonschema:"pending or attached; empty lists both"`
}

type listRecordingsOut struct {
	Recordings []recordingOut `json:"recordings"`
}

type attachRecordingIn struct {
	ID      string `json:"id" jsonschema:"recording id from list_voice_recordings"`
	MatchID string `json:"match_id" jsonschema:"library match id (list_matches) that has no team voice yet"`
}

type setSpeakersIn struct {
	MatchID  string                       `json:"match_id" jsonschema:"library match id"`
	Speakers []tuiclient.VoiceSpeakerEdit `json:"speakers" jsonschema:"track numbers from voice_status, each with the SteamID64 of the match player who speaks on it (get_match), or empty to clear"`
}

type deleteRecordingIn struct {
	ID      string `json:"id" jsonschema:"recording id from list_voice_recordings"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type voicePluginIn struct {
	SetupCS2 bool `json:"setup_cs2,omitempty" jsonschema:"true to write CS2's game state config, as Instalar plugin de TeamSpeak does; CS2 reads it at its next start"`
}

type voicePluginOut struct {
	GameState  string `json:"game_state" jsonschema:"installed, stale (another port: run with setup_cs2) or missing"`
	CS2Found   bool   `json:"cs2_found"`
	ConfigPath string `json:"config_path,omitempty"`
	Inbox      string `json:"inbox" jsonschema:"folder the plugin writes recordings into, on this PC"`
	Live       bool   `json:"live" jsonschema:"CS2 reports a match being played now"`
	LiveMap    string `json:"live_map,omitempty"`
	LiveRounds int    `json:"live_rounds,omitempty"`
	Pending    int    `json:"pending" jsonschema:"recordings waiting for their match"`
	Next       string `json:"next"`
}

func registerVoiceRecordingTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "voice_plugin_status", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Read the TeamSpeak plugin's side in Studio: whether CS2's game state config is installed, whether a match is live, the inbox and the waiting recordings. " +
			"setup_cs2 true writes the config, as Instalar plugin de TeamSpeak in Ajustes does; the plugin itself installs from that button, which opens the package in TeamSpeak 3."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in voicePluginIn) (*mcp.CallToolResult, voicePluginOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, voicePluginOut{}, toolError(err)
			}
			st, err := c.VoicePlugin(ctx, in.SetupCS2)
			if err != nil {
				return nil, voicePluginOut{}, toolError(err)
			}
			out := voicePluginOut{GameState: st.GameState, CS2Found: st.CS2Found, ConfigPath: st.ConfigPath, Inbox: st.Inbox, Live: st.Live.Live,
				LiveMap: st.Live.Map, LiveRounds: len(st.Live.Rounds), Pending: st.Pending}
			switch {
			case !st.CS2Found:
				out.Next = "Studio did not find CS2: the plugin can still record with its hotkey, but not start on its own."
			case st.GameState != "installed":
				out.Next = "Call again with setup_cs2 true, then restart CS2."
			default:
				out.Next = "Ready: with the plugin installed and recording on in TeamSpeak, the next match records on its own."
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_voice_recordings", Annotations: readOnly,
		Description: "List the TeamSpeak plugin's recordings, newest first: pending ones wait for their match (the note says why), attached ones are in a match's team voice. " +
			"Studio attaches a recording on its own only when it has its final score and exactly one imported match fits its map, time and score; a track it could not read says so in failed."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listRecordingsIn) (*mcp.CallToolResult, listRecordingsOut, error) {
			if in.Status != "" && in.Status != "pending" && in.Status != "attached" {
				return nil, listRecordingsOut{}, errors.New("status must be pending, attached or empty")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listRecordingsOut{}, toolError(err)
			}
			recs, err := c.ListVoiceRecordings(ctx)
			if err != nil {
				return nil, listRecordingsOut{}, toolError(err)
			}
			out := listRecordingsOut{Recordings: []recordingOut{}}
			for _, r := range recs {
				if in.Status == "" || r.Status == in.Status {
					out.Recordings = append(out.Recordings, recordingOutOf(r))
				}
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "attach_voice_recording",
		Description: "Attach a TeamSpeak plugin recording to a library match, as Asociar does in Ajustes: its tracks become the match's team voice, tied to the players the teams remember, " +
			"and Studio places them on the demo by CS2's round times (or as an import is placed). The match must have no team voice yet."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in attachRecordingIn) (*mcp.CallToolResult, recordingOut, error) {
			if !recordingIDPattern.MatchString(in.ID) {
				return nil, recordingOut{}, errors.New("id must be a recording id from list_voice_recordings")
			}
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, recordingOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, recordingOut{}, toolError(err)
			}
			r, err := c.AttachVoiceRecording(ctx, in.ID, in.MatchID)
			if err != nil {
				return nil, recordingOut{}, toolError(err)
			}
			return nil, recordingOutOf(r), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_voice_speakers",
		Description: "Say which match player speaks on each voice track at once, as the track list in the Replay's comms does. For TeamSpeak plugin tracks the players' teams remember the " +
			"TeamSpeak identity, so the next recordings come tied on their own. Returns the voice as voice_status does."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setSpeakersIn) (*mcp.CallToolResult, voiceOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, voiceOut{}, err
			}
			if len(in.Speakers) == 0 || len(in.Speakers) > 16 {
				return nil, voiceOut{}, errors.New("pass 1 to 16 speakers")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, voiceOut{}, toolError(err)
			}
			v, err := c.SetVoiceSpeakers(ctx, in.MatchID, in.Speakers)
			if err != nil {
				return nil, voiceOut{}, toolError(err)
			}
			return nil, voiceOutOf(v), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_voice_recording", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a TeamSpeak plugin recording's folder from this PC, as Borrar does in Ajustes. A match it was attached to keeps its team voice. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteRecordingIn) (*mcp.CallToolResult, deleteOut, error) {
			if !recordingIDPattern.MatchString(in.ID) {
				return nil, deleteOut{}, errors.New("id must be a recording id from list_voice_recordings")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			recs, err := c.ListVoiceRecordings(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			var found *tuiclient.VoiceRecording
			for i := range recs {
				if recs[i].ID == in.ID {
					found = &recs[i]
				}
			}
			if found == nil {
				return nil, deleteOut{}, errors.New("no recording with that id")
			}
			summary := fmt.Sprintf("Borrar la grabación de TeamSpeak del %s (%s, %s) de este PC. No se puede deshacer.",
				found.StartedAt.Local().Format("02/01/2006 15:04"), plural(len(found.Tracks), "pista", "pistas"), clockMS(found.DurationMS))
			return approveAndDelete(req, approvalSpec{Tool: "delete_voice_recording", JobID: in.ID, Request: len(found.Tracks), Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteVoiceRecording(ctx, in.ID) })
		})
}
