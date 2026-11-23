package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	maxPhraseCorrections  = 300
	maxCorrectedRunes     = 2000
	maxSummaryCorrections = 5
	maxGlossaryWords      = 600
)

const correctionLimits = "Only mishearings: player nicks, map callouts, weapons, CS2 jargon, and words split or joined wrongly. " +
	"Never rewrite, summarise, censor or translate a phrase, and never change its meaning. " +
	"Leave a phrase you cannot understand as it is, or propose discarding it instead, and leave a phrase corrected_by user as the user wrote it. "

const reviewReady = "The list counts only when state is ready: otherwise transcribe first (%s), wait until it is ready and review again. " +
	"When truncated is true, review again with a smaller scope (%s) until every phrase of the video was read. "

type reviewDemoSubtitlesIn struct {
	JobID      string   `json:"job_id" jsonschema:"the parsed job the video is made from (create_job)"`
	SegmentIDs []string `json:"segment_ids,omitempty" jsonschema:"for a Short: the segment ids passed to generate_short; only phrases said inside them are listed"`
	Deaths     bool     `json:"deaths,omitempty" jsonschema:"the segment ids are deaths of a death reel"`
	Rounds     []int    `json:"rounds,omitempty" jsonschema:"for a Full Demo: the rounds of plan_full_demo; empty with no segment_ids lists every round"`
	PreRoll    int      `json:"pre_roll_seconds,omitempty" jsonschema:"the pre_roll_seconds generate_short will use (3, 5 or 8), so each play starts where the Short cuts it; ignored for a death reel or rounds"`
}

type reviewStreamSubtitlesIn struct {
	ID      string   `json:"id" jsonschema:"stream_id"`
	ClipIDs []string `json:"clip_ids,omitempty" jsonschema:"leave empty before render_stream, which renders every saved momento; only narrows the review to some momento or group ids of get_stream_edit_plan"`
	PlayIDs []string `json:"play_ids,omitempty" jsonschema:"jugadas of find_stream_plays: the ones render_stream adds, or the ones of the resumen"`
	Summary bool     `json:"summary,omitempty" jsonschema:"review the resumen of render_stream_summary: play_ids, or every jugada when empty"`
}

type phraseCorrectionIn struct {
	PhraseID string `json:"phrase_id" jsonschema:"phrase id from the review"`
	Text     string `json:"text" jsonschema:"the whole corrected phrase, 1 to 2000 characters"`
}

type correctDemoPhrasesIn struct {
	MatchID     string               `json:"match_id" jsonschema:"match_id from review_demo_subtitles"`
	Corrections []phraseCorrectionIn `json:"corrections" jsonschema:"1 to 300 phrases, each once"`
	Approve     string               `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type correctStreamPhrasesIn struct {
	ID          string               `json:"id" jsonschema:"stream_id"`
	Corrections []phraseCorrectionIn `json:"corrections" jsonschema:"1 to 300 phrases, each once"`
	Approve     string               `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type glossaryOut struct {
	Players  []string `json:"players" jsonschema:"the nicks of the match's players, written as in the demo"`
	Callouts []string `json:"callouts" jsonschema:"the map's callouts, with the team's own names"`
	Lexicon  []string `json:"lexicon" jsonschema:"the CS2 words Studio's vocabulary corrector knows: weapons, utility, jargon"`
}

type demoSubtitleReviewOut struct {
	MatchID   string          `json:"match_id"`
	State     string          `json:"state" jsonschema:"the transcript: none, queued, running, ready or failed"`
	Phrases   []demoPhraseOut `json:"phrases" jsonschema:"the phrases of the player's team said inside the video, not discarded"`
	Glossary  glossaryOut     `json:"glossary"`
	Truncated bool            `json:"truncated,omitempty" jsonschema:"only the first 300 phrases are listed: review again with fewer segment_ids or rounds"`
}

type streamSubtitleReviewOut struct {
	ID            string            `json:"id"`
	State         string            `json:"state" jsonschema:"the transcript: none, queued, running, ready or failed"`
	Untranscribed []streamSpanIn    `json:"untranscribed" jsonschema:"stretches of the video not transcribed yet: pass them to transcribe_stream and review again once it is ready"`
	Phrases       []streamPhraseOut `json:"phrases" jsonschema:"the phrases said inside the video's stretches, not discarded or hidden in it"`
	Glossary      glossaryOut       `json:"glossary" jsonschema:"a stream has no map or roster: lexicon only"`
	Truncated     bool              `json:"truncated,omitempty" jsonschema:"only the first 300 phrases are listed: review again with fewer clip_ids or play_ids"`
}

type correctDemoPhrasesOut struct {
	NeedsApproval bool            `json:"needs_approval,omitempty"`
	ApprovalCode  string          `json:"approval_code,omitempty"`
	Summary       string          `json:"summary"`
	Phrases       []demoPhraseOut `json:"phrases,omitempty"`
}

type correctStreamPhrasesOut struct {
	NeedsApproval bool              `json:"needs_approval,omitempty"`
	ApprovalCode  string            `json:"approval_code,omitempty"`
	Summary       string            `json:"summary"`
	Phrases       []streamPhraseOut `json:"phrases,omitempty"`
}

func cleanWords(words []string) []string {
	out := make([]string, 0, min(len(words), maxGlossaryWords))
	for _, w := range words[:min(len(words), maxGlossaryWords)] {
		if w = cleanName(w); w != "" {
			out = append(out, w)
		}
	}
	return out
}

func glossaryOf(g tuiclient.SubtitleGlossary) glossaryOut {
	return glossaryOut{Players: cleanWords(g.Players), Callouts: cleanWords(g.Callouts), Lexicon: cleanWords(g.Lexicon)}
}

func checkCorrections(corrections []phraseCorrectionIn) ([]tuiclient.PhraseCorrection, error) {
	if len(corrections) == 0 || len(corrections) > maxPhraseCorrections {
		return nil, fmt.Errorf("corrections takes 1 to %d phrases", maxPhraseCorrections)
	}
	seen := make(map[string]bool, len(corrections))
	out := make([]tuiclient.PhraseCorrection, 0, len(corrections))
	for _, c := range corrections {
		id := strings.ToLower(strings.TrimSpace(c.PhraseID))
		if err := checkUUID("phrase_id", id); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("phrase %s is corrected twice; send each phrase once", id)
		}
		text := strings.Join(strings.Fields(c.Text), " ")
		if text == "" || utf8.RuneCountInString(text) > maxCorrectedRunes {
			return nil, errors.New("a corrected phrase takes 1 to 2000 characters; to drop a phrase, discard it instead")
		}
		seen[id] = true
		out = append(out, tuiclient.PhraseCorrection{Line: id, Text: text})
	}
	return out, nil
}

func correctionSummary(corrections []tuiclient.PhraseCorrection, before map[string]string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Corregir %s de los subtítulos (quedan como correcciones del asistente):", plural(len(corrections), "frase", "frases"))
	for i, c := range corrections {
		old, ok := before[c.Line]
		if !ok {
			return "", fmt.Errorf("phrase %s is not in this transcript; read the review again", c.Line)
		}
		if utf8.RuneCountInString(old) > maxCommsTextRunes {
			return "", fmt.Errorf("phrase %s is longer than the %d characters the review shows; leave it to the user in the Subtítulos panel", c.Line, maxCommsTextRunes)
		}
		if i < maxSummaryCorrections {
			fmt.Fprintf(&b, " %s → %s;", quotedName(old, "«»"), quotedName(c.Text, "«»"))
		}
	}
	if extra := len(corrections) - maxSummaryCorrections; extra > 0 {
		fmt.Fprintf(&b, " y %s más.", plural(extra, "frase", "frases"))
	}
	return strings.TrimSuffix(b.String(), ";") + ".", nil
}

func registerSubtitleReviewTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "review_demo_subtitles", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the subtitles a demo video will draw before it is rendered: the phrases of the player's team said inside the chosen segments of a Short (segment_ids, deaths for a death reel) or the rounds of a Full Demo, not discarded, " +
			"with what the transcriber heard and who corrected each one (corrected_by user or assistant), plus a glossary of the match's player nicks, the map's callouts and the CS2 lexicon to check them against. " +
			"Pass the pre_roll_seconds of generate_short. Propose to the user only the corrections you are sure of, and apply them with correct_demo_phrases after they agree. " + correctionLimits +
			fmt.Sprintf(reviewReady, "transcribe_demo_voice", "fewer segment_ids or rounds")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in reviewDemoSubtitlesIn) (*mcp.CallToolResult, demoSubtitleReviewOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, demoSubtitleReviewOut{}, err
			}
			switch {
			case len(in.SegmentIDs) > 0 && len(in.Rounds) > 0:
				return nil, demoSubtitleReviewOut{}, errors.New("send segment_ids for a Short or rounds for a Full Demo, not both")
			case in.Deaths && len(in.SegmentIDs) == 0:
				return nil, demoSubtitleReviewOut{}, errors.New("deaths needs the segment_ids of the death reel")
			}
			if err := renderplan.ValidatePreRoll(in.PreRoll); err != nil {
				return nil, demoSubtitleReviewOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoSubtitleReviewOut{}, toolError(err)
			}
			r, err := c.DemoSubtitleReview(ctx, tuiclient.DemoSubtitleScope{JobID: in.JobID, SegmentIDs: in.SegmentIDs, Deaths: in.Deaths, Rounds: in.Rounds, PreRoll: in.PreRoll})
			if err != nil {
				return nil, demoSubtitleReviewOut{}, toolError(err)
			}
			out := demoSubtitleReviewOut{MatchID: r.MatchID, State: r.State, Phrases: []demoPhraseOut{}, Glossary: glossaryOf(r.Glossary)}
			for _, l := range r.Lines {
				if len(out.Phrases) == maxDemoPhrases {
					out.Truncated = true
					break
				}
				out.Phrases = append(out.Phrases, demoPhraseOf(r.MatchID, l))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "correct_demo_phrases",
		Description: "Apply the subtitle corrections the user agreed to, all at once or none: each phrase gets the whole corrected text, as correct_demo_phrase does, and is marked corrected_by assistant. " +
			"Studio's vocabulary corrector does not learn from assistant corrections; a later hand correction (correct_demo_phrase or the Subtítulos panel) makes the phrase the user's, and sending original_text undoes it. " +
			correctionLimits + "It changes what every later video of the demo draws, so the user must approve it: if the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree."},
		func(ctx context.Context, req *mcp.CallToolRequest, in correctDemoPhrasesIn) (*mcp.CallToolResult, correctDemoPhrasesOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, correctDemoPhrasesOut{}, err
			}
			corrections, err := checkCorrections(in.Corrections)
			if err != nil {
				return nil, correctDemoPhrasesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, correctDemoPhrasesOut{}, toolError(err)
			}
			t, err := c.DemoTranscript(ctx, in.MatchID)
			if err != nil {
				return nil, correctDemoPhrasesOut{}, toolError(err)
			}
			before := make(map[string]string, len(t.Lines))
			for _, l := range t.Lines {
				before[strings.ToLower(l.ID)] = l.Text
			}
			summary, err := correctionSummary(corrections, before)
			if err != nil {
				return nil, correctDemoPhrasesOut{}, err
			}
			ask, pending, err := gate(req, approvalSpec{Tool: "correct_demo_phrases", JobID: in.MatchID, Request: corrections, Summary: summary, Given: in.Approve, Question: "¿Aplicar las correcciones?"})
			if err != nil || ask != nil {
				return ask, correctDemoPhrasesOut{}, err
			}
			if pending != nil {
				return nil, correctDemoPhrasesOut{NeedsApproval: true, ApprovalCode: pending.ApprovalCode, Summary: summary}, nil
			}
			lines, err := c.CorrectDemoPhrases(ctx, tuiclient.PhraseCorrections{ID: in.MatchID, Lines: corrections})
			if err != nil {
				return nil, correctDemoPhrasesOut{}, toolError(err)
			}
			out := correctDemoPhrasesOut{Summary: summary, Phrases: []demoPhraseOut{}}
			for _, l := range lines {
				out.Phrases = append(out.Phrases, demoPhraseOf(in.MatchID, l))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "review_stream_subtitles", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the subtitles a stream video will draw before it is rendered: the phrases said inside the momentos of render_stream (every saved momento; clip_ids only narrows it) and the jugadas it adds (play_ids), or the jugadas of the resumen (summary), not discarded or hidden in that video, " +
			"with what the transcriber heard and who corrected each one (corrected_by user or assistant), plus the CS2 lexicon (a stream has no map or roster). " +
			"Propose to the user only the corrections you are sure of, and apply them with correct_stream_phrases after they agree. " + correctionLimits +
			fmt.Sprintf(reviewReady, "transcribe_stream with the untranscribed stretches", "fewer clip_ids or play_ids")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in reviewStreamSubtitlesIn) (*mcp.CallToolResult, streamSubtitleReviewOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamSubtitleReviewOut{}, err
			}
			if in.Summary && len(in.ClipIDs) > 0 {
				return nil, streamSubtitleReviewOut{}, errors.New("the resumen is made of jugadas: send play_ids, not clip_ids")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamSubtitleReviewOut{}, toolError(err)
			}
			r, err := c.StreamSubtitleReview(ctx, tuiclient.StreamSubtitleScope{JobID: in.ID, ClipIDs: in.ClipIDs, PlayIDs: in.PlayIDs, Summary: in.Summary})
			if err != nil {
				return nil, streamSubtitleReviewOut{}, toolError(err)
			}
			out := streamSubtitleReviewOut{ID: r.StreamID, State: r.State, Untranscribed: []streamSpanIn{}, Phrases: []streamPhraseOut{}, Glossary: glossaryOf(r.Glossary)}
			for _, span := range r.Untranscribed[:min(len(r.Untranscribed), maxStreamSpans)] {
				out.Untranscribed = append(out.Untranscribed, streamSpanIn{StartSeconds: span.StartSeconds, EndSeconds: span.EndSeconds})
			}
			for _, l := range r.Lines {
				if len(out.Phrases) == maxStreamPhrases {
					out.Truncated = true
					break
				}
				out.Phrases = append(out.Phrases, streamPhraseOf(l))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "correct_stream_phrases",
		Description: "Apply the stream subtitle corrections the user agreed to, all at once or none: each phrase gets the whole corrected text, as correct_stream_phrase does, and is marked corrected_by assistant. " +
			"Studio's vocabulary corrector does not learn from assistant corrections; a later hand correction (correct_stream_phrase or the Subtítulos panel) makes the phrase the user's, and sending original_text undoes it. " +
			correctionLimits + "It changes what every video of the stream draws, so the user must approve it: if the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree."},
		func(ctx context.Context, req *mcp.CallToolRequest, in correctStreamPhrasesIn) (*mcp.CallToolResult, correctStreamPhrasesOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, correctStreamPhrasesOut{}, err
			}
			corrections, err := checkCorrections(in.Corrections)
			if err != nil {
				return nil, correctStreamPhrasesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, correctStreamPhrasesOut{}, toolError(err)
			}
			t, err := c.StreamTranscript(ctx, in.ID)
			if err != nil {
				return nil, correctStreamPhrasesOut{}, toolError(err)
			}
			before := make(map[string]string, len(t.Lines))
			for _, l := range t.Lines {
				before[strings.ToLower(l.ID)] = l.Text
			}
			summary, err := correctionSummary(corrections, before)
			if err != nil {
				return nil, correctStreamPhrasesOut{}, err
			}
			ask, pending, err := gate(req, approvalSpec{Tool: "correct_stream_phrases", JobID: in.ID, Request: corrections, Summary: summary, Given: in.Approve, Question: "¿Aplicar las correcciones?"})
			if err != nil || ask != nil {
				return ask, correctStreamPhrasesOut{}, err
			}
			if pending != nil {
				return nil, correctStreamPhrasesOut{NeedsApproval: true, ApprovalCode: pending.ApprovalCode, Summary: summary}, nil
			}
			lines, err := c.CorrectStreamPhrases(ctx, tuiclient.PhraseCorrections{ID: in.ID, Lines: corrections})
			if err != nil {
				return nil, correctStreamPhrasesOut{}, toolError(err)
			}
			out := correctStreamPhrasesOut{Summary: summary, Phrases: []streamPhraseOut{}}
			for _, l := range lines {
				out.Phrases = append(out.Phrases, streamPhraseOf(l))
			}
			return nil, out, nil
		})
}
