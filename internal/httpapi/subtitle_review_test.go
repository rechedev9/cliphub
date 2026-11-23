package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

func (e demoTranscriptEnv) shortJob(segments ...killplan.Segment) job.Job {
	e.t.Helper()
	plan := killplan.NewPlan()
	plan.Segments, plan.Demo.Tickrate = segments, 64
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan, TargetSteamID: "76561198000000002"}
	if err := e.jobs.Create(context.Background(), &j); err != nil {
		e.t.Fatal(err)
	}
	if err := e.repo.LinkDemoJob(context.Background(), e.match.DemoID, j.ID); err != nil {
		e.t.Fatal(err)
	}
	return j
}

func (e demoTranscriptEnv) storeLines(lines ...library.DemoVoiceLine) {
	e.t.Helper()
	if err := e.repo.SetDemoVoice(context.Background(), library.DemoVoiceUpdate{MatchID: e.match.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
		e.t.Fatal(err)
	}
}

func reviewTexts(lines []demoTranscriptLine) []string {
	out := []string{}
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return out
}

func TestSubtitleReviewListsOnlyThePhrasesOfTheChosenSegmentsOrRounds(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	team := teamOf(env.match, "76561198000000002")
	say := func(round, tick int, text string) library.DemoVoiceLine {
		return library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "two", TeamKey: team, StartMS: int64(tick) * 1000 / 64, EndMS: int64(tick)*1000/64 + 1000, Tick: tick, Round: round, Text: text}
	}
	enemy := say(1, 1_100, "enemy talk")
	enemy.TeamKey, enemy.SteamID64 = "ct-start", "76561198000000001"
	gone := say(1, 1_200, "invented")
	gone.Discarded = true
	env.storeLines(say(1, 1_000, "one tap"), enemy, gone, say(1, 5_000, "after the kill"), say(2, 9_000, "rotate B"))
	j := env.shortJob(killplan.Segment{ID: "r1-k1", Round: 1, TickStart: 900, TickEnd: 1_500}, killplan.Segment{ID: "r2-k1", Round: 2, TickStart: 8_000, TickEnd: 9_500})
	path := "/api/jobs/" + j.ID.String() + "/subtitle-review"

	var got demoSubtitleReview
	env.json(http.MethodGet, path+"?segments=r1-k1", nil, http.StatusOK, &got)
	if got.MatchID != env.match.ID || got.State != library.TranscriptReady || !slices.Equal(reviewTexts(got.Lines), []string{"one tap"}) {
		t.Fatalf("review of one segment = %+v; want only the team's phrase inside it", got)
	}
	if !slices.Contains(got.Glossary.Players, "one") || !slices.Contains(got.Glossary.Players, "two") || len(got.Glossary.Lexicon) == 0 {
		t.Fatalf("glossary = %+v; want the match's nicks and the CS2 lexicon", got.Glossary)
	}
	env.json(http.MethodGet, path+"?rounds=2", nil, http.StatusOK, &got)
	if !slices.Equal(reviewTexts(got.Lines), []string{"rotate B"}) {
		t.Fatalf("review of round 2 = %v", reviewTexts(got.Lines))
	}
	env.json(http.MethodGet, path, nil, http.StatusOK, &got)
	if !slices.Equal(reviewTexts(got.Lines), []string{"one tap", "after the kill", "rotate B"}) {
		t.Fatalf("review of every round = %v", reviewTexts(got.Lines))
	}
	env.json(http.MethodGet, path+"?segments=nope", nil, http.StatusBadRequest, nil)
	env.json(http.MethodGet, path+"?segments=r1-k1&rounds=1", nil, http.StatusBadRequest, nil)
	env.json(http.MethodGet, path+"?deaths=1", nil, http.StatusBadRequest, nil)
}

func TestSubtitleReviewOfAShortCoversThePreRollItIsCutWith(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	team := teamOf(env.match, "76561198000000002")
	lead := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "two", TeamKey: team, StartMS: 10_000, EndMS: 10_500, Tick: 640, Round: 1, Text: "push now"}
	env.storeLines(lead)
	j := env.shortJob(killplan.Segment{ID: "r1-k1", Round: 1, TickStart: 1_000, TickEnd: 1_400, Kills: []killplan.Kill{{Tick: 1_100}}})
	path := "/api/jobs/" + j.ID.String() + "/subtitle-review?segments=r1-k1"

	var got demoSubtitleReview
	env.json(http.MethodGet, path+"&pre_roll_seconds=8", nil, http.StatusOK, &got)
	if !slices.Equal(reviewTexts(got.Lines), []string{"push now"}) {
		t.Fatalf("review with an 8 s pre-roll = %v; want the phrase said in the lead", reviewTexts(got.Lines))
	}
	env.json(http.MethodGet, path+"&pre_roll_seconds=3", nil, http.StatusOK, &got)
	if len(got.Lines) != 0 {
		t.Fatalf("review with the 3 s pre-roll = %v; want the lead left out", reviewTexts(got.Lines))
	}
	env.json(http.MethodGet, path+"&pre_roll_seconds=4", nil, http.StatusBadRequest, nil)
}

func TestAssistantCorrectionsApplyTogetherAndAHandCorrectionMakesThemTheUsers(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	a := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "two", TeamKey: "t-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "flesh on Palas"}
	b := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "two", TeamKey: "t-start", StartMS: 3_000, EndMS: 4_000, Tick: 192, Round: 1, Text: "drop the vesty"}
	env.storeLines(a, b)
	post := env.base + "/corrections"
	fix := func(id uuid.UUID, text string) map[string]any { return map[string]any{"line": id, "text": text} }

	env.json(http.MethodPost, post, map[string]any{"lines": []any{fix(a.ID, "flash on Palace"), fix(uuid.New(), "x")}}, http.StatusNotFound, nil)
	env.json(http.MethodPost, post, map[string]any{"lines": []any{fix(a.ID, "flash"), fix(a.ID, "flash on")}}, http.StatusBadRequest, nil)
	env.json(http.MethodPost, post, map[string]any{"lines": []any{fix(a.ID, "  ")}}, http.StatusBadRequest, nil)
	env.json(http.MethodPost, post, map[string]any{"lines": []any{}}, http.StatusBadRequest, nil)
	var untouched demoTranscript
	env.json(http.MethodGet, env.base, nil, http.StatusOK, &untouched)
	if untouched.Lines[0].Corrected || untouched.Lines[0].Text != a.Text {
		t.Fatalf("after refused batches = %+v; want nothing written", untouched.Lines[0])
	}

	var done correctedDemoLines
	env.json(http.MethodPost, post, map[string]any{"lines": []any{fix(a.ID, " flash on  Palace "), fix(b.ID, "drop the vest")}}, http.StatusOK, &done)
	if len(done.Lines) != 2 || done.Lines[0].Text != "flash on Palace" || done.Lines[0].CorrectedBy != library.CorrectedByAssistant || done.Lines[1].OriginalText != b.Text {
		t.Fatalf("batch = %+v; want both phrases corrected by the assistant", done.Lines)
	}
	var hand library.DemoVoiceLine
	env.json(http.MethodPatch, env.base+"/lines/"+a.ID.String(), map[string]string{"text": "flash on Palace now"}, http.StatusOK, &hand)
	if hand.CorrectedBy != library.CorrectedByUser {
		t.Fatalf("hand correction = %+v; want it the user's", hand)
	}
	var undone library.DemoVoiceLine
	env.json(http.MethodPatch, env.base+"/lines/"+b.ID.String(), map[string]string{"text": b.Text}, http.StatusOK, &undone)
	if undone.Corrected || undone.CorrectedBy != "" {
		t.Fatalf("undo = %+v; want the heard text back with no author", undone)
	}
}

func TestDemoTranscriptDoesNotLearnFromAssistantCorrections(t *testing.T) {
	env := newLearningEnv(t)
	env.transcriber.heard = []teamvoice.Line{heardLine(1000, "flesh on Palas")}
	first := env.transcribe(false)
	var fixes []any
	for _, l := range first.Lines {
		fixes = append(fixes, map[string]any{"line": l.ID, "text": "flash on palacio"})
	}
	if len(fixes) != 2 {
		t.Fatalf("first demo = %+v; want one phrase per player", first.Lines)
	}
	env.json(http.MethodPost, env.base+"/corrections", map[string]any{"lines": fixes}, http.StatusOK, nil)

	next := env.withMatch("next-demo").onMirage()
	next.transcriber.heard = []teamvoice.Line{heardLine(1000, "Flesh, Palas now")}
	if call := lineAt(t, next.transcribe(false), 1000); strings.Contains(call.Text, "Palacio") {
		t.Fatalf("next demo = %+v; want no rule learned from the assistant's corrections", call.DemoVoiceLine)
	}
}

func (e streamTranscriptEnv) putJSON(key string, v any) {
	e.t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.storage.Put(key, bytes.NewReader(raw)); err != nil {
		e.t.Fatal(err)
	}
}

func streamReviewTexts(lines []streamTranscriptLine) []string {
	out := []string{}
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return out
}

func TestStreamSubtitleReviewListsOnlyThePhrasesOfTheChosenMomentosOrJugadas(t *testing.T) {
	env := newStreamTranscriptEnv(t)
	base := env.stream(micStream)
	id := uuid.MustParse(strings.TrimPrefix(base, "/api/stream-jobs/"))
	say := func(startSeconds int64, text string) streamclips.VoiceLine {
		return streamclips.VoiceLine{ID: uuid.New(), StartMS: startSeconds * 1000, EndMS: startSeconds*1000 + 1500, Text: text}
	}
	gone := say(66, "invented")
	gone.Discarded = true
	lines := []streamclips.VoiceLine{say(65, "moly left"), gone, say(305, "vaya tiro"), say(500, "ace")}
	spans := []library.VoiceSpan{{StartMS: 0, EndMS: 400_000}, {StartMS: 490_000, EndMS: 600_000}}
	if err := env.streams.SetStreamVoice(t.Context(), streamclips.VoiceUpdate{JobID: id, Transcript: streamclips.VoiceTranscript{State: library.TranscriptReady, Spans: spans}, Replaced: spans, Lines: &lines}); err != nil {
		t.Fatal(err)
	}
	env.putJSON(streamclips.EditPlanKey(id), streamclips.EditPlan{Variant: "vertical", Clips: []streamclips.ClipRange{{ID: "clip-a", StartSeconds: 60, EndSeconds: 70}, {ID: "clip-b", StartSeconds: 300, EndSeconds: 310}}})
	env.putJSON(streamclips.PlaysKey(id), streamPlayScan{Status: playScanDone, Plays: []vodplays.Play{{ID: "play-1", StartSeconds: 495, EndSeconds: 505}, {ID: "play-2", StartSeconds: 300, EndSeconds: 309}}})
	path := base + "/subtitle-review"

	var got streamSubtitleReview
	env.json(http.MethodGet, path+"?clips=clip-a", nil, http.StatusOK, &got)
	if got.StreamID != id || got.State != library.TranscriptReady || !slices.Equal(streamReviewTexts(got.Lines), []string{"moly left"}) || len(got.Glossary.Lexicon) == 0 || len(got.Untranscribed) != 0 {
		t.Fatalf("review of one momento = %+v", got)
	}
	env.json(http.MethodGet, path, nil, http.StatusOK, &got)
	if !slices.Equal(streamReviewTexts(got.Lines), []string{"moly left", "vaya tiro"}) {
		t.Fatalf("review of every saved momento = %v", streamReviewTexts(got.Lines))
	}
	env.json(http.MethodGet, path+"?clips=clip-b&plays=play-1", nil, http.StatusOK, &got)
	if !slices.Equal(streamReviewTexts(got.Lines), []string{"vaya tiro", "ace"}) {
		t.Fatalf("review of a momento and an added jugada = %v", streamReviewTexts(got.Lines))
	}
	env.putJSON(streamclips.PlaysKey(id), streamPlayScan{Status: playScanDone, Plays: []vodplays.Play{{ID: "play-1", StartSeconds: 480, EndSeconds: 505}, {ID: "play-2", StartSeconds: 300, EndSeconds: 309}}})
	env.json(http.MethodGet, path+"?plays=play-1", nil, http.StatusOK, &got)
	if len(got.Untranscribed) != 1 || got.Untranscribed[0].EndSeconds != 490 {
		t.Fatalf("untranscribed = %+v; want the start of the jugada not transcribed yet", got.Untranscribed)
	}
	env.json(http.MethodGet, path+"?summary=1&plays=play-1", nil, http.StatusOK, &got)
	if !slices.Equal(streamReviewTexts(got.Lines), []string{"ace"}) {
		t.Fatalf("review of the resumen of one jugada = %v", streamReviewTexts(got.Lines))
	}
	if err := env.streams.SetStreamSubtitles(t.Context(), streamclips.Subtitles{JobID: id, Hidden: map[string][]uuid.UUID{"clip-b": {lines[2].ID}}}); err != nil {
		t.Fatal(err)
	}
	env.json(http.MethodGet, path, nil, http.StatusOK, &got)
	if !slices.Equal(streamReviewTexts(got.Lines), []string{"moly left"}) {
		t.Fatalf("review with a phrase hidden in its only momento = %v", streamReviewTexts(got.Lines))
	}
	env.json(http.MethodGet, path+"?clips=nope", nil, http.StatusBadRequest, nil)
	env.json(http.MethodGet, path+"?plays=nope", nil, http.StatusBadRequest, nil)
	env.json(http.MethodGet, path+"?summary=1&clips=clip-a", nil, http.StatusBadRequest, nil)

	post := base + "/transcript/corrections"
	env.json(http.MethodPost, post, map[string]any{"lines": []any{map[string]any{"line": lines[0].ID, "text": "molly left"}, map[string]any{"line": uuid.New(), "text": "x"}}}, http.StatusNotFound, nil)
	var done correctedStreamLines
	env.json(http.MethodPost, post, map[string]any{"lines": []any{map[string]any{"line": lines[0].ID, "text": "molly left"}}}, http.StatusOK, &done)
	if len(done.Lines) != 1 || done.Lines[0].CorrectedBy != library.CorrectedByAssistant || done.Lines[0].OriginalText != "moly left" {
		t.Fatalf("batch = %+v; want the phrase corrected by the assistant", done.Lines)
	}
	var hand streamclips.VoiceLine
	env.json(http.MethodPatch, base+"/transcript/lines/"+lines[0].ID.String(), map[string]string{"text": "molotov left"}, http.StatusOK, &hand)
	if hand.CorrectedBy != library.CorrectedByUser {
		t.Fatalf("hand correction = %+v; want it the user's", hand)
	}
}
