package vodplays

import (
	"context"
	"image"
	"math"
)

// A notice is tracked across frames so the render knows every row it held and
// for how long; a track seen in too few frames is scenery that looked like one.
const (
	noticeSamplesPerSecond = 30.0
	// A notice can be on screen for about eight seconds before the clip starts.
	noticeLookbackSeconds = 8.0
	noticePad             = 3
	noticeMatchSlack      = 4
	noticeMaxGapSeconds   = 2.5
	minNoticeFrames       = 8
)

type NoticeOptions struct {
	FFmpeg       string
	Input        string
	StartSeconds float64
	EndSeconds   float64
}

// NoticeSpot is one feed rectangle a highlighted notice held, with the source seconds.
type NoticeSpot struct {
	Area     image.Rectangle
	From, To float64
}

// SourceArea is Area as fractions of the source frame: x, y, width, height.
func (s NoticeSpot) SourceArea() (float64, float64, float64, float64) {
	fx := func(px int) float64 { return feedLeft + float64(px)/FeedWidth*feedSpan }
	fy := func(py int) float64 { return float64(py) / FeedHeight * feedDepth }
	return fx(s.Area.Min.X), fy(s.Area.Min.Y), fx(s.Area.Max.X) - fx(s.Area.Min.X), fy(s.Area.Max.Y) - fy(s.Area.Min.Y)
}

type noticeTrack struct {
	minX, maxX int
	spots      []NoticeSpot
	first      float64
	last       float64
	// sizes counts frames by ring size: a notice keeps one size, scenery does not.
	sizes map[image.Point]int
}

func (tr *noticeTrack) steadyFrames() int {
	best := 0
	for _, n := range tr.sizes {
		best = max(best, n)
	}
	return best
}

// FindNotices returns every row a highlighted notice held while the range
// was on screen, in source seconds, ordered by when it appeared.
func FindNotices(ctx context.Context, opts NoticeOptions) ([]NoticeSpot, error) {
	start := math.Max(0, opts.StartSeconds-noticeLookbackSeconds)
	var tracks []*noticeTrack
	err := Scan(ctx, ScanOptions{
		FFmpeg:           opts.FFmpeg,
		Input:            opts.Input,
		SamplesPerSecond: noticeSamplesPerSecond,
		StartSeconds:     start,
		DurationSeconds:  opts.EndSeconds - start,
	}, func(s Sample) {
		for _, ring := range s.Rings {
			tr := matchNoticeTrack(tracks, ring.Bounds, s.Seconds)
			if tr == nil {
				tr = &noticeTrack{minX: ring.Bounds.Min.X, maxX: ring.Bounds.Max.X, first: s.Seconds, sizes: map[image.Point]int{}}
				tracks = append(tracks, tr)
			}
			tr.addSpot(ring.Bounds, s.Seconds)
			tr.last = s.Seconds
			tr.sizes[ring.Bounds.Size()]++
		}
	})
	if err != nil {
		return nil, err
	}
	var spots []NoticeSpot
	for _, tr := range tracks {
		end := tr.last + 1/noticeSamplesPerSecond
		if tr.steadyFrames() < minNoticeFrames || end <= opts.StartSeconds || tr.first >= opts.EndSeconds {
			continue
		}
		spots = append(spots, tr.spots...)
	}
	return spots, nil
}

// addSpot extends the current row of the track, or starts a new one when the
// notice moved to another row, so a mask never covers a row it left.
func (tr *noticeTrack) addSpot(b image.Rectangle, at float64) {
	area := b.Inset(-noticePad).Intersect(image.Rect(0, 0, FeedWidth, FeedHeight))
	end := at + 1/noticeSamplesPerSecond
	if n := len(tr.spots); n > 0 && absInt(tr.spots[n-1].Area.Min.Y-area.Min.Y) <= noticeMatchSlack {
		tr.spots[n-1].Area = tr.spots[n-1].Area.Union(area)
		tr.spots[n-1].To = end
		return
	}
	tr.spots = append(tr.spots, NoticeSpot{Area: area, From: at, To: end})
}

// matchNoticeTrack finds the notice a ring continues. A notice keeps its
// width and right edge while it slides between rows, so x is what matches.
func matchNoticeTrack(tracks []*noticeTrack, b image.Rectangle, at float64) *noticeTrack {
	for i := len(tracks) - 1; i >= 0; i-- {
		tr := tracks[i]
		if at-tr.last > noticeMaxGapSeconds {
			continue
		}
		if absInt(tr.minX-b.Min.X) <= noticeMatchSlack && absInt(tr.maxX-b.Max.X) <= noticeMatchSlack {
			return tr
		}
	}
	return nil
}
