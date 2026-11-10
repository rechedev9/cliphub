package httpapi

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/gamestate"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/voiceplugin"
)

// libraryVoiceRecordingRepository is the plugin recordings part of LibraryRepository.
type libraryVoiceRecordingRepository interface {
	CreateVoiceRecording(ctx context.Context, rec library.VoiceRecording) error
	GetVoiceRecording(ctx context.Context, id string) (library.VoiceRecording, error)
	ListVoiceRecordings(ctx context.Context) ([]library.VoiceRecording, error)
	UpdateVoiceRecording(ctx context.Context, rec library.VoiceRecording) (library.VoiceRecording, error)
	DeleteVoiceRecording(ctx context.Context, id string) error
	SaveVoiceSpeakers(ctx context.Context, speakers []library.VoiceSpeaker) error
	ListVoiceSpeakers(ctx context.Context, teamIDs []uuid.UUID) ([]library.VoiceSpeaker, error)
}

// VoicePluginMedia turns a plugin track's raw speech into the Ogg Opus the Replay plays.
type VoicePluginMedia interface {
	EncodePCM(ctx context.Context, job teamvoice.PCMEncodeJob) error
}

// VoicePluginDeps enables the TeamSpeak plugin: Dir is Studio's plugin folder in the profile and
// WebOrigin the web server CS2 posts to and the plugin polls.
type VoicePluginDeps struct {
	Dir       voiceplugin.Dir
	WebOrigin string
	Media     VoicePluginMedia
}

// voicePlugin is the plugin's live state: what CS2 last said, and one inbox scan at a time.
type voicePlugin struct {
	game   gamestate.Tracker
	scanMu sync.Mutex
}

// inboxScanEvery is how often Studio looks for finished recordings and for the demos they wait for.
const inboxScanEvery = 15 * time.Second

// codeVoicePresent is an attach to a match that already has team voice.
const codeVoicePresent = "voice_present"

func (h *Handlers) pluginReady() bool {
	return h.voice != nil && h.voice.deps.Plugin.Dir != "" && h.voice.deps.Plugin.Media != nil && h.libraryRepo != nil
}

// gameStateURL is where CS2 posts, through the web server's local request guard.
func (h *Handlers) gameStateURL() string {
	return h.voice.deps.Plugin.WebOrigin + "/api/voice-plugin/gamestate"
}

// startVoicePlugin tells the plugin where Studio listens, refreshes an installed CS2 config whose port
// moved, and watches the inbox.
func (h *Handlers) startVoicePlugin(ctx context.Context) {
	if !h.pluginReady() {
		return
	}
	deps := h.voice.deps.Plugin
	if deps.WebOrigin != "" {
		if err := deps.Dir.Publish(deps.WebOrigin); err != nil {
			log.Printf("httpapi: publish the TeamSpeak plugin discovery file: %v", err)
		}
		if st, err := h.gameStateConfig(); err == nil && st.Present && !st.Current {
			if _, err := h.installGameState(); err != nil {
				log.Printf("httpapi: refresh the CS2 game state config: %v", err)
			}
		}
	}
	go func() {
		tick := time.NewTicker(inboxScanEvery)
		defer tick.Stop()
		for {
			h.scanVoiceInbox(ctx)
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}

func (h *Handlers) gameStateOptions() (gamestate.ConfigOptions, error) {
	tok, err := h.voice.deps.Plugin.Dir.Token()
	return gamestate.ConfigOptions{URL: h.gameStateURL(), Token: tok}, err
}

func (h *Handlers) gameStateConfig() (gamestate.ConfigState, error) {
	opts, err := h.gameStateOptions()
	if err != nil {
		return gamestate.ConfigState{}, err
	}
	return gamestate.Check(h.cs2Path(), opts)
}

func (h *Handlers) installGameState() (string, error) {
	opts, err := h.gameStateOptions()
	if err != nil {
		return "", err
	}
	return gamestate.Install(h.cs2Path(), opts)
}

// scanVoiceInbox stores every finished recording and attaches the waiting ones whose match is in.
func (h *Handlers) scanVoiceInbox(ctx context.Context) {
	p := h.voice.plugin
	p.scanMu.Lock()
	defer p.scanMu.Unlock()
	found, errs := h.voice.deps.Plugin.Dir.Finished()
	for _, err := range errs {
		log.Printf("httpapi: TeamSpeak recording skipped: %v", err)
	}
	for _, f := range found {
		if _, err := h.libraryRepo.GetVoiceRecording(ctx, f.ID); !errors.Is(err, library.ErrNotFound) {
			continue
		}
		if err := h.storeRecording(ctx, f); err != nil {
			log.Printf("httpapi: store TeamSpeak recording %s: %v", f.ID, err)
		}
	}
	recs, err := h.libraryRepo.ListVoiceRecordings(ctx)
	if err != nil {
		log.Printf("httpapi: list TeamSpeak recordings: %v", err)
		return
	}
	for _, rec := range recs {
		if rec.MatchID != nil || ctx.Err() != nil {
			continue
		}
		h.autoAttach(ctx, rec)
	}
}

// trackFailedNote is what Ajustes and the MCP show for a track Studio could not read.
const trackFailedNote = "No se pudo convertir el audio de esta pista; el resto de la grabación sí se guardó."

// storeRecording encodes each track next to the manifest, removes the raw speech and stores the
// recording. A track that fails is stored as failed, with its raw files kept, and never retried.
func (h *Handlers) storeRecording(ctx context.Context, f voiceplugin.Found) error {
	rec := f.Manifest.Recording(f.ID)
	failed := 0
	for i, t := range rec.Tracks {
		ms, err := h.encodePluginTrack(ctx, encodeTarget{dir: f.Dir, file: t.File})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			log.Printf("httpapi: TeamSpeak recording %s track %s: %v", f.ID, t.File, err)
			rec.Tracks[i].Failed = trackFailedNote
			failed++
			continue
		}
		rec.Tracks[i].DurationMS = ms
	}
	rec.Tracks = slices.DeleteFunc(rec.Tracks, func(t library.RecordingTrack) bool { return t.DurationMS == 0 && t.Failed == "" })
	if err := h.libraryRepo.CreateVoiceRecording(ctx, rec); err != nil && !errors.Is(err, library.ErrConflict) {
		return err
	}
	keep := map[string]bool{}
	for _, t := range rec.Tracks {
		keep[t.File] = t.Failed != ""
	}
	for _, t := range f.Manifest.Tracks {
		if !keep[t.File] {
			_ = os.Remove(filepath.Join(f.Dir, t.File+".pcm"))
			_ = os.Remove(filepath.Join(f.Dir, t.File+".idx"))
		}
	}
	log.Printf("httpapi: TeamSpeak recording %s stored: %d tracks (%d failed), map %q", rec.ID, len(rec.Tracks), failed, rec.Map)
	return nil
}

type encodeTarget struct {
	dir, file string
}

// encodePluginTrack writes <file>.ogg; a track already encoded by an earlier scan keeps its file.
func (h *Handlers) encodePluginTrack(ctx context.Context, t encodeTarget) (int64, error) {
	dst := filepath.Join(t.dir, t.file+".ogg")
	track, err := voiceplugin.OpenTrack(t.dir, t.file)
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(dst); statErr == nil {
			_, ms, err := h.voice.deps.Media.Speech(ctx, dst)
			return ms, err
		}
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer track.Close()
	if track.Samples == 0 {
		return 0, nil
	}
	tmp := dst + ".part"
	if err := h.voice.deps.Plugin.Media.EncodePCM(ctx, teamvoice.PCMEncodeJob{PCM: track.PCM(), SampleRate: voiceplugin.SampleRate, Dst: tmp}); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return track.DurationMS(), nil
}

// autoAttach adds a waiting recording to the one match it fits, or keeps why it waits.
func (h *Handlers) autoAttach(ctx context.Context, rec library.VoiceRecording) {
	candidates, err := h.recordingCandidates(ctx, rec)
	if err != nil {
		log.Printf("httpapi: matches for TeamSpeak recording %s: %v", rec.ID, err)
		return
	}
	pick := voiceplugin.PickMatch(rec, candidates)
	if pick.MatchID == nil {
		if pick.Note != rec.Note {
			rec.Note = pick.Note
			if _, err := h.libraryRepo.UpdateVoiceRecording(ctx, rec); err != nil {
				log.Printf("httpapi: note TeamSpeak recording %s: %v", rec.ID, err)
			}
		}
		return
	}
	m, err := h.libraryRepo.GetMatch(ctx, *pick.MatchID)
	if err != nil {
		log.Printf("httpapi: get match for TeamSpeak recording %s: %v", rec.ID, err)
		return
	}
	if _, err := h.attachRecording(ctx, attachRequest{rec: rec, match: m, note: "Asociada sola por mapa, hora y marcador."}); err != nil {
		log.Printf("httpapi: attach TeamSpeak recording %s to match %s: %v", rec.ID, m.ID, err)
	}
}

// recordingCandidates are the matches of the recording's map, near its time, without team voice or another recording.
func (h *Handlers) recordingCandidates(ctx context.Context, rec library.VoiceRecording) ([]library.Match, error) {
	if rec.Map == "" {
		return nil, nil
	}
	from, to := rec.StartedAt.Add(-3*time.Hour), rec.EndedAt.Add(15*24*time.Hour)
	matches, err := h.libraryRepo.ListMatches(ctx, library.MatchFilter{Map: rec.Map, From: &from, To: &to, Limit: 200})
	if err != nil {
		return nil, err
	}
	recs, err := h.libraryRepo.ListVoiceRecordings(ctx)
	if err != nil {
		return nil, err
	}
	linked := map[uuid.UUID]bool{}
	for _, r := range recs {
		if r.MatchID != nil {
			linked[*r.MatchID] = true
		}
	}
	out := matches[:0]
	for _, m := range matches {
		if linked[m.ID] {
			continue
		}
		if _, err := h.libraryRepo.GetMatchVoice(ctx, m.ID); !errors.Is(err, library.ErrNotFound) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

type attachRequest struct {
	rec   library.VoiceRecording
	match library.Match
	note  string
}

// errVoicePresent is a match that already has team voice: the plugin's tracks would not share its clock.
var errVoicePresent = errors.New("the match already has team voice")

// attachRecording copies a recording's tracks into a match's voice, ties them to players and places them.
func (h *Handlers) attachRecording(ctx context.Context, req attachRequest) (library.VoiceRecording, error) {
	rec, m := req.rec, req.match
	if _, err := h.libraryRepo.GetMatchVoice(ctx, m.ID); !errors.Is(err, library.ErrNotFound) {
		if err == nil {
			err = errVoicePresent
		}
		return rec, err
	}
	dir, err := h.voice.deps.Plugin.Dir.Recording(rec.ID)
	if err != nil {
		return rec, err
	}
	saved, err := h.libraryRepo.ListVoiceSpeakers(ctx, matchTeamIDs(m))
	if err != nil {
		return rec, err
	}
	ties := voiceplugin.TiePlayers(voiceplugin.TieInput{Recording: rec, Match: m, Saved: saved})
	readable := slices.DeleteFunc(slices.Clone(rec.Tracks), func(t library.RecordingTrack) bool { return t.Failed != "" })
	picked := loudestTracks(readable, library.MaxVoiceTracks)
	tracks := make([]library.VoiceTrack, 0, len(picked))
	for _, t := range picked {
		vt, err := h.copyPluginTrack(ctx, pluginTrackCopy{dir: dir, matchID: m.ID, track: t})
		if err != nil {
			h.discardVoiceTracks(tracks)
			return rec, err
		}
		vt.SteamID64 = ties[t.File]
		tracks = append(tracks, vt)
	}
	if len(tracks) == 0 {
		return rec, errors.New("the recording has no speech")
	}
	sync, err := h.pluginSync(ctx, pluginSyncSubject{rec: rec, match: m, tracks: tracks})
	if err != nil {
		h.discardVoiceTracks(tracks)
		return rec, err
	}
	v, err := h.libraryRepo.AddVoiceTracks(ctx, library.VoiceAdd{MatchID: m.ID, Tracks: tracks, Sync: sync})
	if err != nil {
		h.discardVoiceTracks(tracks)
		return rec, err
	}
	for i := range rec.Tracks {
		rec.Tracks[i].MatchTrack = 0
		for _, vt := range v.Tracks {
			if vt.FileName == rec.Tracks[i].File+".ogg" {
				rec.Tracks[i].MatchTrack = vt.Track
			}
		}
	}
	rec.MatchID, rec.Note = &m.ID, req.note
	out, err := h.libraryRepo.UpdateVoiceRecording(ctx, rec)
	if err != nil {
		return rec, err
	}
	log.Printf("httpapi: TeamSpeak recording %s added to match %s: %d tracks, %d tied to players, placed at %s by %s",
		rec.ID, m.ID, len(tracks), len(ties), teamvoice.Clock(sync.OffsetMS), sync.Method)
	return out, nil
}

func matchTeamIDs(m library.Match) []uuid.UUID {
	var ids []uuid.UUID
	for _, t := range m.Teams {
		if t.TeamID != nil {
			ids = append(ids, *t.TeamID)
		}
	}
	return ids
}

// loudestTracks keeps the n longest tracks, in their recording order, when a server had more people.
func loudestTracks(tracks []library.RecordingTrack, n int) []library.RecordingTrack {
	if len(tracks) <= n {
		return tracks
	}
	byLength := slices.Clone(tracks)
	slices.SortStableFunc(byLength, func(a, b library.RecordingTrack) int { return int(b.DurationMS - a.DurationMS) })
	keep := map[string]bool{}
	for _, t := range byLength[:n] {
		keep[t.File] = true
	}
	return slices.DeleteFunc(slices.Clone(tracks), func(t library.RecordingTrack) bool { return !keep[t.File] })
}

type pluginTrackCopy struct {
	dir     string
	matchID uuid.UUID
	track   library.RecordingTrack
}

// copyPluginTrack copies an encoded plugin track into the match's voice and finds its speech.
func (h *Handlers) copyPluginTrack(ctx context.Context, c pluginTrackCopy) (library.VoiceTrack, error) {
	src, err := os.Open(filepath.Join(c.dir, c.track.File+".ogg"))
	if err != nil {
		return library.VoiceTrack{}, err
	}
	defer src.Close()
	matchDir := h.voice.matchDir(c.matchID)
	if err := os.MkdirAll(matchDir, 0o750); err != nil {
		return library.VoiceTrack{}, err
	}
	stored := uuid.NewString() + ".ogg"
	dst := filepath.Join(matchDir, stored)
	out, err := os.Create(dst)
	if err != nil {
		return library.VoiceTrack{}, err
	}
	size, copyErr := io.Copy(out, src)
	if err := errors.Join(copyErr, out.Close()); err != nil {
		_ = os.Remove(dst)
		return library.VoiceTrack{}, err
	}
	spans, duration, err := h.voice.deps.Media.Speech(ctx, dst)
	if err != nil {
		_ = os.Remove(dst)
		return library.VoiceTrack{}, err
	}
	return library.VoiceTrack{Label: truncateRunes(c.track.Nickname, library.MaxVoiceLabelRunes), FileName: c.track.File + ".ogg", Channel: -1,
		StoredKey: path.Join(c.matchID.String(), stored), DurationMS: max(duration, c.track.DurationMS), SizeBytes: size,
		Source: library.VoiceSourcePlugin, Speech: spans}, nil
}

type pluginSyncSubject struct {
	rec    library.VoiceRecording
	match  library.Match
	tracks []library.VoiceTrack
}

// pluginSync places a plugin recording by CS2's round clock when it saw enough rounds, else as an
// upload is placed; an analysis it cannot read only costs the round clock.
func (h *Handlers) pluginSync(ctx context.Context, s pluginSyncSubject) (library.VoiceSync, error) {
	doc, err := h.readLibraryDocument(s.match.DemoID)
	if err != nil {
		log.Printf("httpapi: TeamSpeak recording %s: read the match analysis, placing it by the audio instead: %v", s.rec.ID, err)
	} else if sync, ok := teamvoice.GameClock(teamvoice.GameClockInput{StartedAt: s.rec.StartedAt, Rounds: s.rec.Rounds, Timeline: teamvoice.TimelineOf(doc)}); ok {
		return sync, nil
	}
	res, err := h.estimateVoiceSync(ctx, voiceSyncSubject{match: s.match, tracks: s.tracks})
	return res.Sync, err
}

// speakerChoice is a match voice track and the player the user said speaks on it.
type speakerChoice struct {
	match     library.Match
	track     int
	steamID64 string
}

// rememberVoiceSpeaker keeps on the player's team who a plugin track's TeamSpeak identity is.
func (h *Handlers) rememberVoiceSpeaker(ctx context.Context, c speakerChoice) {
	m, track, steamID := c.match, c.track, c.steamID64
	if steamID == "" {
		return
	}
	recs, err := h.libraryRepo.ListVoiceRecordings(ctx)
	if err != nil {
		log.Printf("httpapi: list TeamSpeak recordings: %v", err)
		return
	}
	for _, rec := range recs {
		if rec.MatchID == nil || *rec.MatchID != m.ID {
			continue
		}
		for _, t := range rec.Tracks {
			if t.MatchTrack != track {
				continue
			}
			if s, ok := voiceplugin.Remember(voiceplugin.RememberInput{Match: m, Track: t, SteamID64: steamID}); ok {
				if err := h.libraryRepo.SaveVoiceSpeakers(ctx, []library.VoiceSpeaker{s}); err != nil {
					log.Printf("httpapi: remember TeamSpeak speaker %q: %v", t.Nickname, err)
				}
			}
		}
	}
}
