package tuiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
)

// ReplayVideoSpec opens a Replay video upload, as POST /api/library/matches/{id}/replay-video reads it.
type ReplayVideoSpec struct {
	FPS          int                  `json:"fps"`
	Width        int                  `json:"width"`
	Height       int                  `json:"height"`
	Speed        float64              `json:"speed"`
	IncludeVoice bool                 `json:"include_voice"`
	Segments     []ReplayVideoSegment `json:"segments"`
}

// ReplayVideoSegment is one round's stretch of the video.
type ReplayVideoSegment struct {
	Round    int `json:"round"`
	FromTick int `json:"from_tick"`
	ToTick   int `json:"to_tick"`
	Frames   int `json:"frames"`
}

// ReplayVideoResult is the encoded MP4, waiting at GET /api/library/replay-videos/{video_id}.
type ReplayVideoResult struct {
	VideoID string  `json:"video_id"`
	Bytes   int64   `json:"bytes"`
	Frames  int     `json:"frames"`
	Seconds float64 `json:"seconds"`
	Voice   bool    `json:"voice"`
}

// FrameWriter takes one JPEG frame per call, in order.
type FrameWriter func(write func(jpeg io.WriterTo) error) error

// CreateReplayVideo streams the spec and the frames frames writes, and waits for the encode.
func (c *Client) CreateReplayVideo(ctx context.Context, matchID string, spec ReplayVideoSpec, frames FrameWriter) (ReplayVideoResult, error) {
	path := libraryMatchPath(matchID) + "/replay-video"
	pr, pw := io.Pipe()
	defer pr.Close()
	mw := multipart.NewWriter(pw)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, pr)
	if err != nil {
		_ = pw.Close()
		return ReplayVideoResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	done := make(chan struct{})
	var werr error
	go func() {
		defer close(done)
		defer func() { _ = pw.CloseWithError(werr) }()
		raw, err := json.Marshal(spec)
		if err != nil {
			werr = err
			return
		}
		if werr = mw.WriteField("spec", string(raw)); werr != nil {
			return
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="frame"; filename="frame.jpg"`)
		header.Set("Content-Type", "image/jpeg")
		werr = frames(func(jpeg io.WriterTo) error {
			part, err := mw.CreatePart(header)
			if err != nil {
				return err
			}
			_, err = jpeg.WriteTo(part)
			return err
		})
		if werr == nil {
			werr = mw.Close()
		}
	}()
	var out ReplayVideoResult
	err = c.send(req, http.MethodPost, path, &out)
	_ = pr.Close()
	<-done
	// A frame that could not be drawn explains the broken upload better than the transport does.
	if werr != nil && err != nil && StatusCode(err) == 0 {
		return out, werr
	}
	return out, err
}

// ReplayVideoFile copies an encoded MP4 to dst; Studio drops it once read.
func (c *Client) ReplayVideoFile(ctx context.Context, videoID string, dst io.Writer) error {
	_, err := c.stream(ctx, "/api/library/replay-videos/"+url.PathEscape(videoID), dst)
	return err
}
