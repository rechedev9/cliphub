package streamclips

import "testing"

func TestEditPlanFingerprintTracksRenderAffectingContent(t *testing.T) {
	base := DefaultEditPlan()
	base.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 1, EndSeconds: 2}}
	first, err := EditPlanFingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := EditPlanFingerprint(base); err != nil || got != first {
		t.Fatalf("fingerprint = %q, %v; want deterministic %q", got, err, first)
	}

	// Every field the renderer reads must move the fingerprint, so a stale
	// render can never be mistaken for the current plan.
	tests := []struct {
		name   string
		mutate func(*EditPlan)
	}{
		{name: "clip range", mutate: func(p *EditPlan) { p.Clips[0].EndSeconds = 3 }},
		{name: "clip edit", mutate: func(p *EditPlan) { p.Clips[0].Edit = &ClipEdit{Speed: 2} }},
		{name: "text overlay", mutate: func(p *EditPlan) {
			p.Clips[0].Edit = &ClipEdit{TextOverlays: []TextOverlay{{Text: "hola", PositionY: 0.5}}}
		}},
		{name: "gameplay crop", mutate: func(p *EditPlan) { p.GameplayCrop = CropRect{X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5} }},
		{name: "layout variant", mutate: func(p *EditPlan) { p.Variant = VariantStreamerFullframeNoCam }},
		{name: "streamer banner", mutate: func(p *EditPlan) { p.StreamerBanner = StreamerBannerPlan{Nick: "zacketizorcs2"} }},
		{name: "music", mutate: func(p *EditPlan) { p.Music = MusicPlan{Key: "concrete-teeth"} }},
		{name: "effects grade", mutate: func(p *EditPlan) { p.Effects = EffectsPlan{Grade: true} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := base
			plan.Clips = append([]ClipRange(nil), base.Clips...)
			tt.mutate(&plan)
			got, err := EditPlanFingerprint(plan)
			if err != nil {
				t.Fatal(err)
			}
			if got == first {
				t.Fatalf("fingerprint unchanged after changing %s", tt.name)
			}
		})
	}
}
