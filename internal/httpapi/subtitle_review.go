package httpapi

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/demovocab"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

type demoSubtitleReview struct {
	MatchID  uuid.UUID            `json:"match_id"`
	State    string               `json:"state"`
	Lines    []demoTranscriptLine `json:"lines"`
	Glossary demovocab.Glossary   `json:"glossary"`
}

type streamSubtitleReview struct {
	StreamID      uuid.UUID              `json:"stream_id"`
	State         string                 `json:"state"`
	Untranscribed []streamSpan           `json:"untranscribed"`
	Lines         []streamTranscriptLine `json:"lines"`
	Glossary      demovocab.Glossary     `json:"glossary"`
}

var errReviewScope = errors.New("bad review scope")

type tickRange struct {
	start, end int
}

func queryList(r *http.Request, key string) []string {
	var out []string
	for _, part := range strings.Split(r.URL.Query().Get(key), ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func segmentRanges(plan killplan.Plan, ids []string) ([]tickRange, error) {
	out := make([]tickRange, 0, len(ids))
	for _, id := range ids {
		i := slices.IndexFunc(plan.Segments, func(s killplan.Segment) bool { return s.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("unknown segment id %q", id)
		}
		out = append(out, tickRange{start: plan.Segments[i].TickStart, end: plan.Segments[i].TickEnd})
	}
	return out, nil
}

func reviewRounds(r *http.Request) ([]int, error) {
	var out []int
	for _, part := range queryList(r, "rounds") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("rounds takes round numbers, not %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

func (h *Handlers) reviewGlossary(r *http.Request, m library.Match) demovocab.Glossary {
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		doc = tacticalplan.Document{}
		doc.Demo.Map = m.Map
		for _, p := range m.Players {
			doc.Players = append(doc.Players, tacticalplan.Player{SteamID64: p.SteamID64, Name: p.Name, TeamKey: p.TeamKey})
		}
	}
	return demovocab.DemoGlossary(r.Context(), demovocab.Sources{Document: doc, Names: h.libraryRepo})
}

func (h *Handlers) GetJobSubtitleReview(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	segments, deaths := queryList(r, "segments"), r.URL.Query().Get("deaths") == "1"
	rounds, err := reviewRounds(r)
	if err != nil || (len(segments) > 0 && len(rounds) > 0) || (deaths && len(segments) == 0) {
		writeError(w, http.StatusBadRequest, "send segments (with deaths=1 for a death reel) for a Short, or rounds for a Full Demo, not both")
		return
	}
	preRoll, err := parsePreRollQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var ranges []tickRange
	if len(segments) > 0 {
		plan := killplan.Plan{}
		switch {
		case deaths:
			if plan, ok = h.loadDeathPlan(w, j); !ok {
				return
			}
		case j.KillPlan != nil && preRoll != 0:
			facts, _, err := recapplan.LoadFacts(h.storage, j.ID)
			if err != nil {
				internalError(w, "load round starts for the pre-roll", err)
				return
			}
			plan = parser.WithPreRoll(*j.KillPlan, preRoll, parser.FactsRoundStarts(facts))
			segments = parser.PreRollIDs(*j.KillPlan, preRoll, segments)
		case j.KillPlan != nil:
			plan = *j.KillPlan
		}
		if ranges, err = segmentRanges(plan, segments); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	m, err := h.subtitleMatch(r.Context(), j.ID)
	if err != nil {
		writeCodedError(w, http.StatusConflict, codeNoVoice, "this job's demo is not an analysed library demo")
		return
	}
	t, err := h.demoVoiceState(r.Context(), m.ID)
	if err != nil {
		internalError(w, "get demo transcript", err)
		return
	}
	lines, err := h.libraryRepo.ListDemoVoiceLines(r.Context(), library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
	if err != nil {
		internalError(w, "list demo transcript lines", err)
		return
	}
	rate, team := m.Tickrate, teamOf(m, j.TargetSteamID)
	if rate <= 0 {
		rate = 64
	}
	in := lines[:0]
	for _, l := range lines {
		end := l.Tick + int(math.Round(float64(l.EndMS-l.StartMS)*rate/1000))
		switch {
		case l.TeamKey != team || l.Discarded || l.Text == "":
		case len(segments) > 0 && !slices.ContainsFunc(ranges, func(s tickRange) bool { return l.Tick <= s.end && end >= s.start }):
		case len(rounds) > 0 && !slices.Contains(rounds, l.Round):
		default:
			in = append(in, l)
		}
	}
	writeJSON(w, http.StatusOK, demoSubtitleReview{MatchID: m.ID, State: t.State, Lines: demoTranscriptLines(in), Glossary: h.reviewGlossary(r, m)})
}

func (h *Handlers) streamReviewVideos(r *http.Request, j streamclips.Job) ([]streamclips.SubtitleVideo, error) {
	clips, playIDs, summary := queryList(r, "clips"), queryList(r, "plays"), r.URL.Query().Get("summary") == "1"
	if summary && len(clips) > 0 {
		return nil, fmt.Errorf("%w: the resumen is made of jugadas: send plays, not clips", errReviewScope)
	}
	var plays []vodplays.Play
	if summary || len(playIDs) > 0 {
		scan, err := h.currentStreamPlayScan(j.ID)
		if err != nil {
			return nil, err
		}
		for _, id := range playIDs {
			if !slices.ContainsFunc(scan.Plays, func(p vodplays.Play) bool { return p.ID == id }) {
				return nil, fmt.Errorf("%w: unknown play id %q", errReviewScope, id)
			}
		}
		plays = scan.Plays
		if len(playIDs) > 0 {
			plays = chosenPlays(scan.Plays, playIDs)
		}
	}
	if summary {
		return []streamclips.SubtitleVideo{summarySubtitleVideo(plays)}, nil
	}
	plan, _, err := h.savedStreamEditPlan(j)
	if err != nil {
		return nil, err
	}
	var videos []streamclips.SubtitleVideo
	outputs := plan.Outputs()
	for _, id := range clips {
		if !slices.ContainsFunc(outputs, func(o streamclips.PlanOutput) bool { return o.ID == id }) {
			return nil, fmt.Errorf("%w: unknown momento or group id %q", errReviewScope, id)
		}
	}
	for _, o := range outputs {
		if len(clips) == 0 || slices.Contains(clips, o.ID) {
			videos = append(videos, streamclips.SubtitleVideo{ID: o.ID, Clips: o.Clips})
		}
	}
	for _, p := range plays {
		videos = append(videos, streamclips.SubtitleVideo{ID: p.ID, Clips: []streamclips.ClipRange{{ID: p.ID, StartSeconds: p.StartSeconds, EndSeconds: p.EndSeconds}}})
	}
	return videos, nil
}

func (h *Handlers) GetStreamSubtitleReview(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	repo, ok := h.streamVoiceRepo(w)
	if !ok {
		return
	}
	videos, err := h.streamReviewVideos(r, j)
	if errors.Is(err, errReviewScope) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		internalError(w, "read the stream review scope", err)
		return
	}
	t, err := repo.GetStreamVoice(r.Context(), j.ID)
	if err != nil {
		internalError(w, "get stream transcript", err)
		return
	}
	lines, err := repo.ListStreamVoiceLines(r.Context(), j.ID)
	if err != nil {
		internalError(w, "list stream transcript lines", err)
		return
	}
	choice, err := repo.GetStreamSubtitles(r.Context(), j.ID)
	if err != nil {
		internalError(w, "get stream subtitles", err)
		return
	}
	drawn := func(l streamclips.VoiceLine) bool {
		return slices.ContainsFunc(videos, func(v streamclips.SubtitleVideo) bool {
			return !slices.Contains(choice.Hidden[v.ID], l.ID) && slices.ContainsFunc(streamclips.SubtitleStretches([]streamclips.SubtitleVideo{v}), func(s library.VoiceSpan) bool { return l.StartMS < s.EndMS && l.EndMS > s.StartMS })
		})
	}
	in := lines[:0]
	for _, l := range lines {
		if !l.Discarded && l.Text != "" && drawn(l) {
			in = append(in, l)
		}
	}
	untranscribed := []streamSpan{}
	if wanted, ok := streamSpansOf(streamclips.SubtitleStretches(videos), j.Probe.DurationSeconds); ok {
		for _, s := range streamclips.MissingVoiceSpans(wanted, t.Spans) {
			untranscribed = append(untranscribed, streamSpan{StartSeconds: float64(s.StartMS) / 1000, EndSeconds: float64(s.EndMS) / 1000})
		}
	}
	writeJSON(w, http.StatusOK, streamSubtitleReview{StreamID: j.ID, State: t.State, Untranscribed: untranscribed, Lines: streamTranscriptLines(in), Glossary: demovocab.StreamGlossary()})
}
