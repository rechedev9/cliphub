package vodplays

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/image/draw"
)

// The sprite of a notice is rebuilt from many frames: the notice stays still
// while the game behind it moves, so a pixel that never changes is HUD and how
// much it changes against the bare background gives its opacity.
const (
	noticeSamplesPerSecond = 30.0
	// A notice can be on screen for about eight seconds before the clip starts.
	noticeLookbackSeconds = 8.0
	noticePad             = 3
	noticeMatchSlack      = 4
	noticeMaxGapSeconds   = 2.5
	minNoticeFrames       = 8
	// The pad rows above and below a notice are bare game; their change over
	// time, column by column, is what a fully transparent pixel would show.
	referenceSmoothing   = 8
	noiseFloorPercentile = 0.02
)

type NoticeOptions struct {
	FFmpeg       string
	Input        string
	StartSeconds float64
	EndSeconds   float64
	// WorkDir receives one PNG per notice; it must exist.
	WorkDir string
}

// NoticeSprite is one highlighted kill-feed notice rebuilt as an RGBA image
// in 1080p feed pixels, with the source seconds it was on screen.
type NoticeSprite struct {
	Path             string
	AppearSeconds    float64
	DisappearSeconds float64
	Width, Height    int
	// Spots are where the notice sat on the feed, one per row it held.
	Spots []NoticeSpot
}

// NoticeSpot is one feed rectangle a notice held, with the source seconds.
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
	size       map[image.Point]int
	crops      []noticeCrop
}

type noticeCrop struct {
	size   image.Point
	pixels []byte
}

// ExtractNotices rebuilds every highlighted notice seen during the range.
func ExtractNotices(ctx context.Context, opts NoticeOptions) ([]NoticeSprite, error) {
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
				tr = &noticeTrack{minX: ring.Bounds.Min.X, maxX: ring.Bounds.Max.X, first: s.Seconds, size: map[image.Point]int{}}
				tracks = append(tracks, tr)
			}
			tr.addSpot(ring.Bounds, s.Seconds)
			tr.last = s.Seconds
			tr.size[ring.Bounds.Size()]++
			tr.crops = append(tr.crops, cropNotice(s.Frame, ring.Bounds))
		}
	})
	if err != nil {
		return nil, err
	}
	var sprites []NoticeSprite
	for _, tr := range tracks {
		end := tr.last + 1/noticeSamplesPerSecond
		if end <= opts.StartSeconds || tr.first >= opts.EndSeconds {
			continue
		}
		img, ok := rebuildNotice(tr)
		if !ok {
			continue
		}
		path := filepath.Join(opts.WorkDir, fmt.Sprintf("notice-%03d.png", len(sprites)+1))
		if err := writePNG(path, img); err != nil {
			return nil, err
		}
		sprites = append(sprites, NoticeSprite{
			Path: path, AppearSeconds: tr.first, DisappearSeconds: end,
			Width: img.Bounds().Dx(), Height: img.Bounds().Dy(),
			Spots: tr.spots,
		})
	}
	sort.Slice(sprites, func(a, b int) bool { return sprites[a].AppearSeconds < sprites[b].AppearSeconds })
	return sprites, nil
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

func cropNotice(frame []byte, b image.Rectangle) noticeCrop {
	r := b.Inset(-noticePad).Intersect(image.Rect(0, 0, FeedWidth, FeedHeight))
	return noticeCrop{size: b.Size(), pixels: copyRegion(frame, r)}
}

func copyRegion(frame []byte, r image.Rectangle) []byte {
	out := make([]byte, 0, r.Dx()*r.Dy()*3)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := (y*FeedWidth + r.Min.X) * 3
		out = append(out, frame[i:i+r.Dx()*3]...)
	}
	return out
}

// rebuildNotice solves each pixel as observed = a*F + (1-a)*background over
// the frames where the notice had its usual size. The background of a column
// is read from the bare pad rows right above and below it.
func rebuildNotice(tr *noticeTrack) (*image.NRGBA, bool) {
	var size image.Point
	best := 0
	for s, n := range tr.size {
		if n > best {
			size, best = s, n
		}
	}
	w, h := size.X+2*noticePad, size.Y+2*noticePad
	var frames [][]byte
	for _, c := range tr.crops {
		if c.size == size && len(c.pixels) == w*h*3 {
			frames = append(frames, c.pixels)
		}
	}
	if len(frames) < minNoticeFrames {
		return nil, false
	}
	n := float64(len(frames))
	sd := make([]float64, w*h)
	mean := make([]float64, w*h*3)
	for p := 0; p < w*h; p++ {
		total := 0.0
		for ch := 0; ch < 3; ch++ {
			sum, sq := 0.0, 0.0
			for _, f := range frames {
				v := float64(f[p*3+ch])
				sum += v
				sq += v * v
			}
			m := sum / n
			mean[p*3+ch] = m
			total += math.Sqrt(math.Max(sq/n-m*m, 0))
		}
		sd[p] = total / 3
	}
	refSD, refMean := columnReference(sd, mean, w, h)
	sorted := append([]float64(nil), sd...)
	sort.Float64s(sorted)
	floor := sorted[int(float64(len(sorted))*noiseFloorPercentile)]
	alpha := make([]float64, w*h)
	notice := image.Rect(noticePad, noticePad, w-noticePad, h-noticePad)
	plate := notice.Inset(noticePad)
	var plateAlpha []float64
	for y := notice.Min.Y; y < notice.Max.Y; y++ {
		for x := notice.Min.X; x < notice.Max.X; x++ {
			a := clamp01(1 - math.Max(sd[y*w+x]-floor, 0)/math.Max(refSD[x]-floor, 1))
			alpha[y*w+x] = a
			if (image.Point{X: x, Y: y}).In(plate) {
				plateAlpha = append(plateAlpha, a)
			}
		}
	}
	// The plate covers its whole inside, so nothing there is more transparent
	// than the plate; a lower value is compression noise at a sharp edge.
	sort.Float64s(plateAlpha)
	floorAlpha := plateAlpha[len(plateAlpha)/2]
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := notice.Min.Y; y < notice.Max.Y; y++ {
		for x := notice.Min.X; x < notice.Max.X; x++ {
			p := y*w + x
			a := alpha[p]
			if (image.Point{X: x, Y: y}).In(plate) {
				a = math.Max(a, floorAlpha)
			}
			if a < 0.02 {
				continue
			}
			var rgb [3]uint8
			for ch := 0; ch < 3; ch++ {
				rgb[ch] = uint8(math.Round(clampByte((mean[p*3+ch] - (1-a)*refMean[x*3+ch]) / a)))
			}
			img.SetNRGBA(x, y, color.NRGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: uint8(math.Round(a * 255))})
		}
	}
	return img, true
}

// columnReference averages the outermost pad rows of each column, smoothed
// across nearby columns, into the change and colour of bare background.
func columnReference(sd, mean []float64, w, h int) ([]float64, []float64) {
	rawSD := make([]float64, w)
	rawMean := make([]float64, w*3)
	for x := 0; x < w; x++ {
		rawSD[x] = (sd[x] + sd[(h-1)*w+x]) / 2
		for ch := 0; ch < 3; ch++ {
			rawMean[x*3+ch] = (mean[x*3+ch] + mean[((h-1)*w+x)*3+ch]) / 2
		}
	}
	refSD := make([]float64, w)
	refMean := make([]float64, w*3)
	for x := 0; x < w; x++ {
		lo, hi := max(0, x-referenceSmoothing), min(w-1, x+referenceSmoothing)
		count := float64(hi - lo + 1)
		for k := lo; k <= hi; k++ {
			refSD[x] += rawSD[k] / count
			for ch := 0; ch < 3; ch++ {
				refMean[x*3+ch] += rawMean[k*3+ch] / count
			}
		}
	}
	return refSD, refMean
}

// ScaleNotice resamples a sprite to the output size. CatmullRom works on
// premultiplied colour, so the clear pad never bleeds a dark fringe.
func ScaleNotice(sprite NoticeSprite, scale float64, path string) (NoticeSprite, error) {
	f, err := os.Open(sprite.Path)
	if err != nil {
		return NoticeSprite{}, err
	}
	src, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return NoticeSprite{}, err
	}
	w := max(1, int(math.Round(float64(sprite.Width)*scale)))
	h := max(1, int(math.Round(float64(sprite.Height)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	if err := writePNG(path, dst); err != nil {
		return NoticeSprite{}, err
	}
	sprite.Path, sprite.Width, sprite.Height = path, w, h
	return sprite, nil
}

func clamp01(v float64) float64   { return math.Min(math.Max(v, 0), 1) }
func clampByte(v float64) float64 { return math.Min(math.Max(v, 0), 255) }

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
