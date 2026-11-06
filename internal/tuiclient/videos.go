package tuiclient

import (
	"context"
	"net/url"
	"path"
	"strings"
)

// RenderVideos is GET /api/jobs/{id}/renders/{variant}: the render state plus the
// names of the finished MP4s on disk.
type RenderVideos struct {
	RenderVariantState
	Videos []string `json:"videos"`
}

// GetRenderVideos returns one render variant with its video names, or a 404
// APIError when the variant was never requested.
func (c *Client) GetRenderVideos(ctx context.Context, id, variant string) (RenderVideos, error) {
	var out RenderVideos
	err := c.getJSON(ctx, renderPath(id, variant), &out)
	return out, err
}

// VideoDurations maps a video name (the file stem GetRenderVideos lists) to its
// length in seconds, read from the variant's publish pack. A variant without a
// pack yet gives an empty map.
func (c *Client) VideoDurations(ctx context.Context, id, variant string) (map[string]float64, error) {
	var pack struct {
		Items []struct {
			Video           string  `json:"video"`
			DurationSeconds float64 `json:"duration_seconds"`
		} `json:"items"`
	}
	if err := c.getJSON(ctx, renderPath(id, variant)+"/pack", &pack); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(pack.Items))
	for _, item := range pack.Items {
		// Names are file stems of the publish path, not segment ids.
		name := strings.TrimSuffix(path.Base(strings.ReplaceAll(item.Video, `\`, "/")), ".mp4")
		if item.DurationSeconds > 0 && name != "" && name != "." {
			out[name] = item.DurationSeconds
		}
	}
	return out, nil
}

// PublishAssistant is the suggested upload copy for one finished video.
type PublishAssistant struct {
	Metadata struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	} `json:"metadata"`
	Recommendations []PublishRecommendation `json:"recommendations"`
	Keywords        []string                `json:"keywords"`
	Schedule        PublishSchedule         `json:"schedule"`
	Trends          PublishTrends           `json:"trends"`
}

// PublishRecommendation is one alternative copy built from a title template.
type PublishRecommendation struct {
	Template    string   `json:"template,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Score       float64  `json:"score"`
	Rationale   string   `json:"rationale"`
}

// PublishSchedule suggests when to publish, per day, in the user's time zone.
type PublishSchedule struct {
	TimeZone string               `json:"time_zone"`
	Days     []PublishScheduleDay `json:"days"`
	Caveat   string               `json:"caveat"`
}

type PublishScheduleDay struct {
	Date    string                `json:"date"`
	Weekday string                `json:"weekday"`
	Slots   []PublishScheduleSlot `json:"slots"`
}

type PublishScheduleSlot struct {
	LocalTime  string  `json:"local_time"`
	Confidence float64 `json:"confidence"`
	Score      float64 `json:"score"`
	Rationale  string  `json:"rationale"`
}

// PublishTrends are the YouTube search terms trending for CS2, when Studio could fetch them.
type PublishTrends struct {
	Available bool     `json:"available"`
	Terms     []string `json:"terms"`
	Reason    string   `json:"reason,omitempty"`
}

// GetPublishAssistant returns the suggested title, description and tags of a
// finished video. It answers 409 while the render is not ready for publication.
func (c *Client) GetPublishAssistant(ctx context.Context, id, variant, name string) (PublishAssistant, error) {
	var out PublishAssistant
	err := c.getJSON(ctx, renderPath(id, variant)+"/videos/"+url.PathEscape(name)+"/publish-assistant", &out)
	return out, err
}

func renderPath(id, variant string) string {
	return "/api/jobs/" + url.PathEscape(id) + "/renders/" + url.PathEscape(variant)
}
