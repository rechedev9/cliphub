package subtitles

const (
	zoneFrameWidth, zoneFrameHeight = 1080, 1920

	shortsLeft, shortsTop, shortsRight, shortsBottom = 48, 288, 192, 672
	tiktokLeft, tiktokTop, tiktokRight, tiktokBottom = 120, 240, 120, 660
	tiktokButtonsRight                               = 180
	reelsLeft, reelsTop, reelsRight, reelsBottom     = 65, 269, 65, 672

	portraitFontSize    = 46
	portraitBlockBottom = 1180

	landscapeFrameWidth, landscapeFrameHeight = 1920, 1080
	landscapeFontSize                         = 46
	landscapeSideMargin                       = 230
	landscapeBottomMargin                     = 150
)

type Zone struct {
	Left, Top, Right, Bottom int
}

type Placement struct {
	FontSize int
	Box      Zone
}

func PortraitSafeZone(width, height int) Zone {
	return Zone{
		Left:   scale(max(shortsLeft, tiktokLeft, reelsLeft), width, zoneFrameWidth),
		Top:    scale(max(shortsTop, tiktokTop, reelsTop), height, zoneFrameHeight),
		Right:  width - scale(max(shortsRight, tiktokRight, tiktokButtonsRight, reelsRight), width, zoneFrameWidth),
		Bottom: height - scale(max(shortsBottom, tiktokBottom, reelsBottom), height, zoneFrameHeight),
	}
}

func PlacementOf(width, height int) Placement {
	side := scale(landscapeSideMargin, width, landscapeFrameWidth)
	p := Placement{FontSize: scale(landscapeFontSize, height, landscapeFrameHeight), Box: Zone{Left: side, Right: width - side, Bottom: height - scale(landscapeBottomMargin, height, landscapeFrameHeight)}}
	if height > width {
		zone := PortraitSafeZone(width, height)
		p = Placement{FontSize: scale(portraitFontSize, width, zoneFrameWidth), Box: Zone{Left: zone.Left, Right: zone.Right, Bottom: scale(portraitBlockBottom, height, zoneFrameHeight)}}
	}
	p.Box.Top = p.Box.Bottom - FormatOf(width, height).AtOnce*2*p.FontSize
	return p
}
