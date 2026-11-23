package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type SubtitleGlossary struct {
	Players  []string `json:"players"`
	Callouts []string `json:"callouts"`
	Lexicon  []string `json:"lexicon"`
}

type DemoSubtitleScope struct {
	JobID      string
	SegmentIDs []string
	Deaths     bool
	Rounds     []int
	PreRoll    int
}

type DemoSubtitleReview struct {
	MatchID  string           `json:"match_id"`
	State    string           `json:"state"`
	Lines    []DemoVoiceLine  `json:"lines"`
	Glossary SubtitleGlossary `json:"glossary"`
}

type StreamSubtitleScope struct {
	JobID   string
	ClipIDs []string
	PlayIDs []string
	Summary bool
}

type StreamSubtitleReview struct {
	StreamID      string            `json:"stream_id"`
	State         string            `json:"state"`
	Untranscribed []StreamStretch   `json:"untranscribed"`
	Lines         []StreamVoiceLine `json:"lines"`
	Glossary      SubtitleGlossary  `json:"glossary"`
}

type PhraseCorrection struct {
	Line string `json:"line"`
	Text string `json:"text"`
}

type PhraseCorrections struct {
	ID    string             `json:"-"`
	Lines []PhraseCorrection `json:"lines"`
}

func reviewQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

func (c *Client) DemoSubtitleReview(ctx context.Context, scope DemoSubtitleScope) (DemoSubtitleReview, error) {
	values := url.Values{}
	if len(scope.SegmentIDs) > 0 {
		values.Set("segments", strings.Join(scope.SegmentIDs, ","))
	}
	if scope.Deaths {
		values.Set("deaths", "1")
	}
	if scope.PreRoll != 0 {
		values.Set("pre_roll_seconds", strconv.Itoa(scope.PreRoll))
	}
	if len(scope.Rounds) > 0 {
		rounds := make([]string, len(scope.Rounds))
		for i, r := range scope.Rounds {
			rounds[i] = strconv.Itoa(r)
		}
		values.Set("rounds", strings.Join(rounds, ","))
	}
	var out DemoSubtitleReview
	err := c.getJSON(ctx, jobPath(scope.JobID)+"/subtitle-review"+reviewQuery(values), &out)
	return out, err
}

func (c *Client) StreamSubtitleReview(ctx context.Context, scope StreamSubtitleScope) (StreamSubtitleReview, error) {
	values := url.Values{}
	if len(scope.ClipIDs) > 0 {
		values.Set("clips", strings.Join(scope.ClipIDs, ","))
	}
	if len(scope.PlayIDs) > 0 {
		values.Set("plays", strings.Join(scope.PlayIDs, ","))
	}
	if scope.Summary {
		values.Set("summary", "1")
	}
	var out StreamSubtitleReview
	err := c.getJSON(ctx, "/api/stream-jobs/"+url.PathEscape(scope.JobID)+"/subtitle-review"+reviewQuery(values), &out)
	return out, err
}

func (c *Client) CorrectDemoPhrases(ctx context.Context, req PhraseCorrections) ([]DemoVoiceLine, error) {
	var out struct {
		Lines []DemoVoiceLine `json:"lines"`
	}
	err := c.doJSON(ctx, http.MethodPost, demoTranscriptPath(req.ID)+"/corrections", req, &out)
	return out.Lines, err
}

func (c *Client) CorrectStreamPhrases(ctx context.Context, req PhraseCorrections) ([]StreamVoiceLine, error) {
	var out struct {
		Lines []StreamVoiceLine `json:"lines"`
	}
	err := c.doJSON(ctx, http.MethodPost, streamTranscriptPath(req.ID)+"/corrections", req, &out)
	return out.Lines, err
}
