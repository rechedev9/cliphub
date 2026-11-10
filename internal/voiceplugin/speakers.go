package voiceplugin

import (
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// TiePlayers says which match player speaks on each track, by file: the user's own track by the
// SteamID CS2 reported, the others by what the match's teams remember. A player gets one track.
func TiePlayers(rec library.VoiceRecording, m library.Match, saved []library.VoiceSpeaker) map[string]string {
	inMatch := map[string]bool{}
	for _, p := range m.Players {
		inMatch[p.SteamID64] = true
	}
	known := map[string]string{}
	for _, s := range saved {
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
	for _, t := range rec.Tracks {
		if t.Self {
			tie(t.File, rec.SelfSteamID64)
		}
	}
	for _, t := range rec.Tracks {
		if _, done := out[t.File]; !done {
			tie(t.File, known[t.SpeakerKey()])
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

// Remember is what a team keeps when the user says who speaks on a plugin track.
func Remember(m library.Match, t library.RecordingTrack, steamID string) (library.VoiceSpeaker, bool) {
	team, ok := TeamOf(m, steamID)
	if !ok {
		return library.VoiceSpeaker{}, false
	}
	return library.VoiceSpeaker{TeamID: team, SpeakerKey: t.SpeakerKey(), Nickname: t.Nickname, SteamID64: steamID}, true
}
