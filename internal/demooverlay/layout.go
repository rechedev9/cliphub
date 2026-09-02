package demooverlay

// Layout is the 1920x1080 overlay geometry. Tests lock these so the intro
// keeps a native-HUD center channel and the outro stays full-frame.
type Layout struct {
	Width  int
	Height int
	Intro  IntroLayout
	Outro  OutroLayout
}

type IntroLayout struct {
	PanelWidth  int
	PanelHeight int
	PanelTop    int
	LeftPanelX  int
	RightPanelX int
	// LeftPanelCropX and RightPanelCropX are the measured panel origins in the
	// scale-to-cover 1920x1080 plate/chrome artwork. Overlay/text use LeftPanelX
	// and RightPanelX so the pair stays centered in frame.
	LeftPanelCropX  int
	RightPanelCropX int
	CenterGap       int
	RowHeight       int
	MaxPlayers      int
	HeaderH         int
	CardInset       int
	AvatarSize      int
	AvatarXOff      int
	AvatarYOff      int
	NameSize        int
	StatSize        int
	LabelSize       int
	BadgeSize       int
}

type OutroLayout struct {
	Margin    int
	HeaderH   int
	RowHeight int
	FullFrame bool
	ColGap    int
	NameWidth int
	StatWidth int
	HeaderY   int
	Row0      int
}

func DefaultLayout() Layout {
	const (
		panelW     = 563
		panelCropL = 42
		panelCropR = 1308
		top        = 28
		height     = 1020
		centerGap  = 703
	)
	margin := (FrameWidth - 2*panelW - centerGap) / 2
	leftX := margin
	rightX := margin + panelW + centerGap
	return Layout{
		Width:  FrameWidth,
		Height: FrameHeight,
		Intro: IntroLayout{
			PanelWidth:      panelW,
			PanelHeight:     height,
			PanelTop:        top,
			LeftPanelX:      leftX,
			RightPanelX:     rightX,
			LeftPanelCropX:  panelCropL,
			RightPanelCropX: panelCropR,
			CenterGap:       centerGap,
			RowHeight:       196,
			MaxPlayers:      5,
			HeaderH:         36,
			CardInset:       88,
			AvatarSize:      64,
			AvatarXOff:      16,
			AvatarYOff:      16,
			NameSize:        20,
			StatSize:        14,
			LabelSize:       10,
			BadgeSize:       26,
		},
		Outro: OutroLayout{
			Margin:    270,
			HeaderH:   60,
			RowHeight: 118,
			FullFrame: true,
			ColGap:    760,
			NameWidth: 240,
			StatWidth: 88,
			HeaderY:   155,
			Row0:      220,
		},
	}
}

func faceitIntroLayout() IntroLayout {
	spec := defaultFaceitLayout.Intro
	p := spec.Panel
	return IntroLayout{
		PanelWidth:      p.Width,
		PanelHeight:     p.Height,
		PanelTop:        p.Top,
		LeftPanelX:      p.LeftX,
		RightPanelX:     p.RightX,
		LeftPanelCropX:  p.LeftX,
		RightPanelCropX: p.RightX,
		CenterGap:       p.CenterGap,
		RowHeight:       p.RowHeight,
		MaxPlayers:      p.MaxPlayers,
		HeaderH:         p.HeaderHeight,
		CardInset:       spec.Name.X,
		AvatarSize:      spec.Avatar.Width,
		AvatarXOff:      spec.Avatar.X,
		AvatarYOff:      spec.Avatar.Y,
		NameSize:        spec.Name.FontSize,
		StatSize:        spec.Stats.ValueSize,
		LabelSize:       spec.Stats.LabelSize,
		BadgeSize:       spec.Level.Rect.Width,
	}
}

func faceitOutroLayout() OutroLayout {
	spec := defaultFaceitLayout.Outro
	statWidth := 0
	if len(spec.Columns) > 0 {
		statWidth = spec.Columns[0].Width
	}
	return OutroLayout{
		Margin:    spec.Margin,
		HeaderH:   spec.RowY - spec.HeaderY,
		RowHeight: spec.RowHeight,
		FullFrame: true,
		ColGap:    0,
		NameWidth: spec.NameWidth,
		StatWidth: statWidth,
		HeaderY:   spec.HeaderY,
		Row0:      spec.RowY,
	}
}

func (l Layout) NativeHUDVisible() bool {
	return l.Intro.CenterGap >= 700
}
