package demooverlay

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/rechedev9/cliphub/internal/htmlstill"
)

const (
	ShortSubscribeDone      = "SUBSCRIBED"
	ShortSubscribeLikeFirst = 1
	ShortSubscribeLikeLast  = shortSubscribeLikeFrames

	ShortSubscribeFollowFirst = ShortSubscribeLikeLast + 1
	ShortSubscribeBellFirst   = ShortSubscribeFollowFirst + shortSubscribePressFrames
	ShortSubscribeFollowLast  = ShortSubscribeBellFirst + subscribeBellFrames

	shortSubscribeLikeFrames  = 8
	shortSubscribePressFrames = 4
)

var (
	shortLikeScales    = [shortSubscribeLikeFrames]float64{0.88, 1.18, 1.36, 1.3, 1.16, 1.06, 1, 1}
	shortPressScales   = [shortSubscribePressFrames]float64{0.95, 0.92, 0.97, 1}
	shortRippleScales  = []float64{0.35, 0.6, 0.85, 1.05, 1.2}
	shortRippleOpacity = []float64{0.35, 0.3, 0.22, 0.12, 0.05}
)

//go:embed short_subscribe.html
var shortSubscribeAssets embed.FS

type shortSubscribeFrame struct {
	Liked, Subscribed bool
	Thumb, Sub, Bell  template.CSS
	LikeRipple        template.CSS
	SubRipple         template.CSS
}

func ShortSubscribeSpriteFor(size htmlstill.Size) SubscribeSprite {
	if size.Height > size.Width {
		return SubscribeSprite{CellTop: 1293, CellHeight: 150, Frames: ShortSubscribeFollowLast}
	}
	return SubscribeSprite{CellTop: SubscribeBottomEdge - 48 - 55, CellHeight: 110, Frames: ShortSubscribeFollowLast}
}

func shortSubscribeFrames() []shortSubscribeFrame {
	rest := shortSubscribeFrame{Thumb: scaleCSS(1), Sub: scaleCSS(1), Bell: "transform:rotate(0deg)"}
	frames := []shortSubscribeFrame{rest}
	for i, scale := range shortLikeScales {
		frame := rest
		frame.Liked, frame.Thumb, frame.LikeRipple = i > 0, scaleCSS(scale), shortRipple(i)
		frames = append(frames, frame)
	}
	liked := frames[len(frames)-1]
	for i, scale := range shortPressScales {
		frame := liked
		frame.Subscribed, frame.Sub, frame.SubRipple = i >= 2, scaleCSS(scale), shortRipple(i)
		frames = append(frames, frame)
	}
	subscribed := frames[len(frames)-1]
	for _, swing := range subscribeBellSwings(subscribeBellFrames)[1:] {
		frame := subscribed
		frame.Bell = swing
		frames = append(frames, frame)
	}
	return append(frames, subscribed)
}

func scaleCSS(scale float64) template.CSS {
	return template.CSS(fmt.Sprintf("transform:scale(%.2f)", scale))
}

func shortRipple(i int) template.CSS {
	if i >= len(shortRippleScales) {
		return ""
	}
	return template.CSS(fmt.Sprintf("transform:translate(-50%%,-50%%) scale(%.2f);opacity:%.2f", shortRippleScales[i], shortRippleOpacity[i]))
}

func ShortSubscribeHTML(card SubscribeCard, size htmlstill.Size) ([][]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	logo, err := subscribeLogoURL(card.LogoPath)
	if err != nil {
		return nil, err
	}
	layout := "landscape"
	if size.Height > size.Width {
		layout = "portrait"
	}
	sprite := ShortSubscribeSpriteFor(size)
	frames, rows := shortSubscribeFrames(), sprite.rowsPerPage(size)
	if rows < 1 {
		return nil, fmt.Errorf("like and subscribe sprite cell of %d px does not fit a %dx%d page", sprite.CellHeight, size.Width, size.Height)
	}
	var pages [][]byte
	for first := 0; first < len(frames); first += rows {
		page, err := htmlPage(shortSubscribeAssets, "short_subscribe.html", struct {
			pageFonts
			Label, Done, Layout       string
			Width, Height, CellHeight int
			Frames                    []shortSubscribeFrame
			Logo                      template.URL
			ChannelName               string
		}{fonts, SubscribeLabel, ShortSubscribeDone, layout, size.Width, size.Height, sprite.CellHeight, frames[first:min(first+rows, len(frames))], logo, card.ChannelName})
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func RenderShortSubscribeSprite(card SubscribeCard, size htmlstill.Size, outPath string) (SubscribeSprite, error) {
	pages, err := ShortSubscribeHTML(card, size)
	if err != nil {
		return SubscribeSprite{}, err
	}
	sprite := ShortSubscribeSpriteFor(size)
	return sprite, renderSpriteSheet(pages, sprite, size, outPath)
}
