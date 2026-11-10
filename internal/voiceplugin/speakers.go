package voiceplugin

import (
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// TieInput is a recording, the match it goes to and what the match's teams remember.
type TieInput struct {
	Recording library.VoiceRecording
	Match     library.Match
	Saved     []library.VoiceSpeaker
}

// TiePlayers says which match player speaks on each track, by file: the user's own track by the
// SteamID CS2 reported, the others by what the match's teams remember. A player gets one track.
func TiePlayers(in TieInput) map[string]string {
	inMatch := map[string]bool{}
	for _, p := range in.Match.Players {
		inMatch[p.SteamID64] = true
	}
	known := map[string]string{}
	for _, s := range in.Saved {
		if inMatch[s.SteamID64] {
			known[s.SpeakerKey] = s.SteamID64
		}
	}
	out := map[string]string{}
	taken := map[string]bool{}
	tie := func(file, steamID string) {
		if steamID != "" && inMatch[steamID] && !taken[steamID] {
			out[file], taken[steamID] = steamID, true
		}
	}
	for _, t := range in.Recording.Tracks {
		if t.Self {
			tie(t.File, in.Recording.SelfSteamID64)
		}
	}
	for _, t := range in.Recording.Tracks {
		if _, done := out[t.File]; !done {
			if key := t.SpeakerKey(); key != "" {
				tie(t.File, known[key])
			}
		}
	}
	return out
}

// TeamOf is the library team of a match player, if the side is tied to one.
func TeamOf(m library.Match, steamID string) (uuid.UUID, bool) {
	for _, p := range m.Players {
		if p.SteamID64 != steamID {
			continue
		}
		for _, t := range m.Teams {
			if t.Key == p.TeamKey && t.TeamID != nil {
				return *t.TeamID, true
			}
		}
	}
	return uuid.UUID{}, false
}

// RememberInput is a plugin track of a match and the player the user said speaks on it.
type RememberInput struct {
	Match     library.Match
	Track     library.RecordingTrack
	SteamID64 string
}

// Remember is what a team keeps when the user says who speaks on a plugin track; nothing when
// the track has no stable key or the player's side has no team.
func Remember(in RememberInput) (library.VoiceSpeaker, bool) {
	key := in.Track.SpeakerKey()
	if key == "" {
		return library.VoiceSpeaker{}, false
	}
	team, ok := TeamOf(in.Match, in.SteamID64)
	if !ok {
		return library.VoiceSpeaker{}, false
	}
	return library.VoiceSpeaker{TeamID: team, SpeakerKey: key, Nickname: in.Track.Nickname, SteamID64: in.SteamID64}, true
}
