package streamclips

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

func TestFindBannerFontPrefersEmbeddedMontserrat(t *testing.T) {
	got := FindBannerFont()
	if filepath.Base(got) != mediafont.FileName {
		t.Fatalf("FindBannerFont = %q, want embedded %s", got, mediafont.FileName)
	}
}

func TestEditPlanValidationRejectsOutOfBoundsCrop(t *testing.T) {
	plan := DefaultEditPlan()
	plan.FaceCrop = CropRect{X: 0.8, Y: 0, Width: 0.3, Height: 0.2}

	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "within the source frame") {
		t.Fatalf("Validate error = %v, want source frame bounds error", err)
	}
}

func TestEditPlanValidationRejectsInvalidClipRange(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 12, EndSeconds: 10}}

	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "greater than start_seconds") {
		t.Fatalf("Validate error = %v, want range error", err)
	}
}

func TestEditPlanValidateForSourceDurationRejectsOverrun(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 20}}

	err := plan.ValidateForSourceDuration(15.15)
	if err == nil || !strings.Contains(err.Error(), "exceeds source duration 15.150") {
		t.Fatalf("ValidateForSourceDuration error = %v, want source-duration overrun", err)
	}

	plan.Clips[0].EndSeconds = 15.15
	if err := plan.ValidateForSourceDuration(15.15); err != nil {
		t.Fatalf("ValidateForSourceDuration exact bound error = %v", err)
	}
}

func TestEditPlanValidateForRenderRequiresClipAndCurrentSchema(t *testing.T) {
	plan := DefaultEditPlan()
	if err := plan.ValidateForRender(20); err == nil || !strings.Contains(err.Error(), "has no clips") {
		t.Fatalf("ValidateForRender empty error = %v, want no clips", err)
	}
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 10}}
	if err := plan.ValidateForRender(20); err != nil {
		t.Fatalf("ValidateForRender error = %v", err)
	}
	plan.SchemaVersion = "999.0"
	if err := plan.ValidateForRender(20); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("ValidateForRender future schema error = %v", err)
	}
}

func TestMigrateLegacySourceDurationOnlyFitsHistoricalTwentySecondDefault(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Clips = []ClipRange{
		{ID: "legacy", StartSeconds: 0, EndSeconds: 20},
		{ID: "outside", StartSeconds: 18, EndSeconds: 20},
	}

	got, changed := MigrateLegacySourceDuration(plan, 15.15)
	if !changed {
		t.Fatal("MigrateLegacySourceDuration changed = false, want true")
	}
	if len(got.Clips) != 1 || got.Clips[0].EndSeconds != 15.15 {
		t.Fatalf("migrated clips = %+v, want one clip ending at 15.15", got.Clips)
	}
	if plan.Clips[0].EndSeconds != 20 {
		t.Fatalf("caller mutated: %+v", plan.Clips[0])
	}
	if err := got.ValidateForSourceDuration(15.15); err != nil {
		t.Fatalf("migrated plan validation error = %v", err)
	}

	custom := DefaultEditPlan()
	custom.Clips = []ClipRange{{ID: "custom", StartSeconds: 0, EndSeconds: 19}}
	unchanged, changed := MigrateLegacySourceDuration(custom, 15.15)
	if changed || unchanged.Clips[0].EndSeconds != 19 {
		t.Fatalf("custom overrun was migrated: changed=%v plan=%+v", changed, unchanged)
	}
	if err := unchanged.ValidateForSourceDuration(15.15); err == nil {
		t.Fatal("custom overrun validation succeeded, want strict rejection")
	}
}

func TestBuildFFmpegArgsCreatesVerticalStackCommand(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 1.5, EndSeconds: 4.25}}

	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 1.500000000",
		"-t 2.750000000",
		"scale=1080:768",
		"scale=1080:1152",
		"vstack=inputs=2",
		"fps=60",
		"-map 0:a?",
		"-crf 18",
		"-movflags +faststart",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if got := args[len(args)-1]; got != "out.mp4" {
		t.Fatalf("last arg = %q, want output path", got)
	}
	if strings.Contains(joined, "drawtext=") {
		t.Fatalf("empty streamer nick must not add a banner: %s", joined)
	}
}

func TestBuildFFmpegArgsPreservesNativeFrameBoundaryPrecision(t *testing.T) {
	plan := DefaultEditPlan()
	start := float64(1001) / 30000
	duration := float64(30030) / 30000
	plan.Clips = []ClipRange{{ID: "clip-ntsc", StartSeconds: start, EndSeconds: start + duration}}

	args, err := BuildFFmpegArgs(
		FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"},
		plan,
		plan.Clips[0],
	)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-ss 0.033366667", "-t 1.001000000"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing native timestamp %q: %s", want, joined)
		}
	}
}

func TestBuildFFmpegArgsOldStreamerBannerPlanUsesCurrentLayoutDefault(t *testing.T) {
	var banner StreamerBannerPlan
	if err := json.Unmarshal([]byte(`{"nick":"zacketizorcs2"}`), &banner); err != nil {
		t.Fatalf("Unmarshal old streamer banner plan: %v", err)
	}
	plan := DefaultEditPlan()
	plan.StreamerBanner = banner
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}}

	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: `C:\Windows\Fonts\arialbd.ttf`,
	}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"vstack=inputs=2[content]",
		"color=c=0x9146ff:s=1080x96:r=60:d=5.000",
		"drawbox=x=0:y=0:w=116:h=96:color=0x5b1ba9:t=fill",
		`fontfile='C\:/Windows/Fonts/arialbd.ttf'`,
		"text='zacketizorcs2'",
		"fontsize=52",
		"overlay=x='0':y=670:eval=frame:eof_action=pass:shortest=0",
		"fps=60",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestBuildFFmpegArgsOverlaysKeyDropBanner(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Variant = VariantStreamerFullframeNoCam
	start, end := 0.5, 2.5
	plan.KeyDropBanner = KeyDropBannerPlan{Style: "operator", Code: "TESTCODE", StartSeconds: &start, EndSeconds: &end}
	clip := ClipRange{ID: "c1", StartSeconds: 0, EndSeconds: 4}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:       "in.mp4",
		OutputPath:       "out.mp4",
		// Plate is pre-composited with the live code; filter only scales/overlays it.
		KeyDropImagePath: "/cache/keydrop-banner.png",
		SourceHasAudio:   true,
	}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "keydrop-banner.png") {
		t.Fatalf("args missing keydrop plate input: %s", joined)
	}
	if !strings.Contains(joined, "keydropped") {
		t.Fatalf("args missing keydrop output label: %s", joined)
	}
	if !strings.Contains(joined, "between(t\\,0.500000\\,2.500000)") {
		t.Fatalf("args missing keydrop visibility window: %s", joined)
	}
	// Code is burned into the plate PNG before FFmpeg; the filtergraph must not
	// re-draw it (that path could ignore a changed plan code).
	if strings.Contains(joined, "drawtext=") && strings.Contains(joined, "TESTCODE") {
		t.Fatalf("args re-draw keydrop code in filtergraph: %s", joined)
	}
}

func TestBuildFFmpegArgsPositionsStaticStreamerBanner(t *testing.T) {
	positionY := 0.5
	plan := DefaultEditPlan()
	plan.StreamerBanner = StreamerBannerPlan{Nick: "zacketizorcs2", PositionY: &positionY}
	clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}

	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: "font.ttf",
	}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "overlay=x='0':y=912:eval=frame") {
		t.Fatalf("args missing static banner at top pixel 912: %s", joined)
	}
}

func TestBuildFFmpegArgsDefaultsFullFrameBannerToTwentyPercent(t *testing.T) {
	plan := EditPlan{
		Variant:        VariantStreamerFullframeNoCam,
		GameplayCrop:   CropRect{X: 0, Y: 0, Width: 1, Height: 1},
		StreamerBanner: StreamerBannerPlan{Nick: "zacketizorcs2"},
	}
	clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}

	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: "font.ttf",
	}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "overlay=x='0':y=336:eval=frame") {
		t.Fatalf("args missing full-frame banner centered at 20%%: %s", joined)
	}
}

func TestBuildFFmpegArgsLandscapeUsesCompactLowerThird(t *testing.T) {
	layout, ok := VariantByName(VariantStreamerLandscape16x9)
	if !ok {
		t.Fatal("landscape layout is not registered")
	}
	plan := EditPlan{
		Variant:        layout.Name,
		GameplayCrop:   layout.DefaultGameplayCrop,
		StreamerBanner: StreamerBannerPlan{Nick: "zacketizorcs2"},
	}
	clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: "font.ttf",
	}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"scale=1920:1080:force_original_aspect_ratio=decrease:flags=lanczos+accurate_rnd+full_chroma_int,pad=1920:1080:(ow-iw)/2:(oh-ih)/2:color=black[content]",
		"color=c=0x111319:s=520x64:r=60:d=5.000",
		"text='@zacketizorcs2'",
		"overlay=x='32':y=983:eval=frame",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if unwanted := "color=c=0x9146ff:s=1920x96"; strings.Contains(joined, unwanted) {
		t.Fatalf("landscape args contain the vertical full-width banner %q: %s", unwanted, joined)
	}
}

func TestFFmpegDrawtextTextEscapesFilterMetacharacters(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "option delimiters", value: "o'clock:50%,[x]", want: `o\'clock\:50\%\,\[x\]`},
		{name: "backslash", value: `a\b`, want: `a\\b`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ffmpegDrawtextText(tt.value); got != tt.want {
				t.Fatalf("ffmpegDrawtextText(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestBuildFFmpegArgsDefaultsLegacyBannerToStackSeam(t *testing.T) {
	layout, ok := VariantByName(VariantStreamerVerticalStack)
	if !ok {
		t.Fatal("legacy layout is not registered")
	}
	plan := DefaultEditPlan()
	plan.Variant = layout.Name
	plan.FaceCrop = layout.DefaultFaceCrop
	plan.GameplayCrop = layout.DefaultGameplayCrop
	plan.StreamerBanner = StreamerBannerPlan{Nick: "zacketizorcs2"}
	plan.Clips = []ClipRange{{ID: "one", StartSeconds: 0, EndSeconds: 5}}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: "font.ttf",
	}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "overlay=x='0':y=472:eval=frame") {
		t.Fatalf("args missing legacy banner centered at the 520px seam: %s", joined)
	}
}

func TestBuildFFmpegArgsAnimatesStreamerBanner(t *testing.T) {
	tests := []struct {
		name     string
		duration float64
		wantX    string
	}{
		{
			name:     "normal clip uses fixed phase",
			duration: 5,
			wantX:    `overlay=x='if(lt(t\,0.350000)\,-w*(1-t/0.350000)\,if(lt(t\,4.650000)\,0\,-w*(t-4.650000)/0.350000))'`,
		},
		{
			name:     "short clip uses half duration",
			duration: 0.6,
			wantX:    `overlay=x='if(lt(t\,0.300000)\,-w*(1-t/0.300000)\,if(lt(t\,0.300000)\,0\,-w*(t-0.300000)/0.300000))'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DefaultEditPlan()
			plan.StreamerBanner = StreamerBannerPlan{Nick: "zacketizorcs2", SlideEnabled: true}
			clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: tt.duration}

			args, err := BuildFFmpegArgs(FFmpegInputs{
				SourcePath:     "source.mp4",
				OutputPath:     "out.mp4",
				BannerFontPath: "font.ttf",
			}, plan, clip)
			if err != nil {
				t.Fatalf("BuildFFmpegArgs error = %v", err)
			}
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, tt.wantX) {
				t.Fatalf("args missing animation expression %q: %s", tt.wantX, joined)
			}
		})
	}
}

func TestEditPlanValidatesStreamerBannerPosition(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{name: "lower boundary", value: 0.025},
		{name: "upper boundary", value: 0.975},
		{name: "below boundary", value: 0.024999, wantErr: true},
		{name: "above boundary", value: 0.975001, wantErr: true},
		{name: "nan", value: math.NaN(), wantErr: true},
		{name: "positive infinity", value: math.Inf(1), wantErr: true},
		{name: "negative infinity", value: math.Inf(-1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DefaultEditPlan()
			plan.StreamerBanner.PositionY = &tt.value
			err := plan.Validate()
			if tt.wantErr && (err == nil || !strings.Contains(err.Error(), "position_y")) {
				t.Fatalf("Validate error = %v, want position_y error", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate error = %v, want nil", err)
			}
		})
	}
}

func TestBuildFFmpegArgsEmptyNickIgnoresBannerRenderingFields(t *testing.T) {
	positionY := 0.75
	plan := DefaultEditPlan()
	plan.StreamerBanner = StreamerBannerPlan{PositionY: &positionY, SlideEnabled: true}
	clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}

	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, unwanted := range []string{"color=c=0x9146ff", "drawtext=", "overlay="} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("empty streamer nick must not add %q: %s", unwanted, joined)
		}
	}
}

func TestBuildFFmpegArgsRejectsBannerWithoutFont(t *testing.T) {
	plan := DefaultEditPlan()
	plan.StreamerBanner = StreamerBannerPlan{Nick: "zacketizorcs2"}
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}}

	_, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, plan.Clips[0])
	if err == nil || !strings.Contains(err.Error(), "font path is required") {
		t.Fatalf("BuildFFmpegArgs error = %v, want banner font error", err)
	}
}

func TestBuildFFmpegArgsLegacyVariantUsesHighQualityScaling(t *testing.T) {
	plan := EditPlan{
		Variant:      VariantStreamerVerticalStack,
		FaceCrop:     CropRect{X: 0, Y: 0, Width: 1, Height: 0.35},
		GameplayCrop: CropRect{X: 0, Y: 0.35, Width: 1, Height: 0.65},
	}
	clip := ClipRange{ID: "clip-001", StartSeconds: 1.5, EndSeconds: 4.25}

	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 1.500000000",
		"-t 2.750000000",
		"split=2[facein][gamein]",
		"scale=1080:520:force_original_aspect_ratio=increase:flags=lanczos+accurate_rnd+full_chroma_int,crop=1080:520[face]",
		"scale=1080:1400:force_original_aspect_ratio=increase:flags=lanczos+accurate_rnd+full_chroma_int,crop=1080:1400[game]",
		"vstack=inputs=2",
		"fps=60",
		"-map 0:a?",
		"-crf 18",
		"-movflags +faststart",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestBuildFFmpegArgsFullframeNoCamCommand(t *testing.T) {
	plan := EditPlan{
		Variant:      VariantStreamerFullframeNoCam,
		GameplayCrop: CropRect{X: 0, Y: 0, Width: 1, Height: 1},
	}
	clip := ClipRange{ID: "clip-001", StartSeconds: 1.5, EndSeconds: 4.25}

	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "split=2") || strings.Contains(joined, "vstack") {
		t.Fatalf("fullframe-nocam args must not split or stack: %s", joined)
	}
	for _, want := range []string{
		"scale=1080:1920:force_original_aspect_ratio=increase:flags=lanczos+accurate_rnd+full_chroma_int,crop=1080:1920",
		"fps=60",
		"-map 0:a?",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestBuildFFmpegArgsLandscape16x9PreservesFullFrame(t *testing.T) {
	plan := EditPlan{
		Variant:      VariantStreamerLandscape16x9,
		GameplayCrop: CropRect{X: 0, Y: 0, Width: 1, Height: 1},
	}
	clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}
	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "vstack") {
		t.Fatalf("landscape args must preserve the source frame without stacking: %s", joined)
	}
	if want := "scale=1920:1080:force_original_aspect_ratio=decrease:flags=lanczos+accurate_rnd+full_chroma_int,pad=1920:1080:(ow-iw)/2:(oh-ih)/2:color=black"; !strings.Contains(joined, want) {
		t.Fatalf("args missing %q: %s", want, joined)
	}
	if strings.Contains(joined, "force_original_aspect_ratio=increase,crop=1920:1080") {
		t.Fatalf("landscape args crop non-16:9 sources: %s", joined)
	}
}

func TestBuildFFmpegArgsMixesMusicUnderOriginalAudio(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Music = MusicPlan{Key: "concrete-teeth", Volume: 0.3}
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}}

	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		MusicPath:      "music/concrete-teeth.mp3",
		SourceHasAudio: true,
	}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-stream_loop -1 -i music/concrete-teeth.mp3",
		"[1:a]volume=0.300000[bgm]",
		"[0:a][bgm]amix=inputs=2:duration=first:dropout_transition=0:normalize=0[a]",
		"-map [a]",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "-map 0:a?") {
		t.Fatalf("music mix must replace the passthrough audio map: %s", joined)
	}
	if strings.Contains(joined, "-shortest") {
		t.Fatalf("amix duration=first already bounds the mix; -shortest is for silent sources only: %s", joined)
	}
}

func TestBuildFFmpegArgsMusicOnSilentSourceUsesMusicAlone(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Music = MusicPlan{Key: "concrete-teeth"}
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}}

	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		MusicPath:      "music/concrete-teeth.mp3",
		SourceHasAudio: false,
	}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	// Default volume applies when the plan does not set one.
	for _, want := range []string{"[1:a]volume=0.250000[a]", "-map [a]", "-shortest"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "amix") {
		t.Fatalf("silent source must not amix a missing stream: %s", joined)
	}
}

func TestBuildFFmpegArgsGradeInsertsEqFilter(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Effects = EffectsPlan{Grade: true}
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}}

	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "eq=contrast=1.05:saturation=1.15,fps=60") {
		t.Fatalf("args missing grade filter before fps: %s", joined)
	}
}

func TestEditPlanValidationRejectsBadMusic(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Music = MusicPlan{Key: "../escape"}
	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "invalid music key") {
		t.Fatalf("Validate error = %v, want invalid music key", err)
	}

	plan.Music = MusicPlan{Key: "concrete-teeth", Volume: 1.5}
	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "music volume") {
		t.Fatalf("Validate error = %v, want music volume error", err)
	}
}

func TestKickMarkGlyphUsesOfficialGrid(t *testing.T) {
	if len(kickMarkRows) != 9 {
		t.Fatalf("kickMarkRows rows = %d, want 9", len(kickMarkRows))
	}
	for i, row := range kickMarkRows {
		if len(row) != 8 {
			t.Fatalf("kickMarkRows[%d] = %q (len %d), want 8", i, row, len(row))
		}
	}

	glyph := kickMarkGlyph(kickBannerColor)
	tests := []struct {
		name string
		want string
	}{
		{name: "top stem", want: "drawbox=x=26:y=12:w=24:h=8:color=0x53fc18:t=fill"},
		{name: "top arm", want: "drawbox=x=66:y=12:w=24:h=8:color=0x53fc18:t=fill"},
		{name: "row1 arm", want: "drawbox=x=58:y=20:w=32:h=8:color=0x53fc18:t=fill"},
		{name: "full bar", want: "drawbox=x=26:y=28:w=64:h=8:color=0x53fc18:t=fill"},
		{name: "waist", want: "drawbox=x=26:y=44:w=48:h=8:color=0x53fc18:t=fill"},
		{name: "bottom arm", want: "drawbox=x=66:y=76:w=24:h=8:color=0x53fc18:t=fill"},
		{name: "bottom stem", want: "drawbox=x=26:y=76:w=24:h=8:color=0x53fc18:t=fill"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(glyph, tt.want) {
				t.Fatalf("glyph missing %s %q:\n%s", tt.name, tt.want, glyph)
			}
		})
	}
	for _, old := range []string{
		"drawbox=x=38:y=22:w=12:h=52",
		"drawbox=x=60:y=28:w=14:h=12",
		"drawbox=x=60:y=58:w=14:h=16",
	} {
		if strings.Contains(glyph, old) {
			t.Fatalf("glyph still has broken Kick boxes %q:\n%s", old, glyph)
		}
	}
}

func TestBuildFFmpegArgsUsesKickBannerPalette(t *testing.T) {
	plan := DefaultEditPlan()
	plan.StreamerBanner = StreamerBannerPlan{Nick: "aimagia", Platform: StreamerBannerPlatformKick}
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}}

	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: "font.ttf",
	}, plan, plan.Clips[0])
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"color=c=0x53fc18:s=1080x96:r=60:d=5.000",
		"drawbox=x=0:y=0:w=116:h=96:color=0x0d0d0d:t=fill",
		"drawbox=x=26:y=12:w=24:h=8:color=0x53fc18:t=fill",
		"drawbox=x=66:y=12:w=24:h=8:color=0x53fc18:t=fill",
		"drawbox=x=26:y=44:w=48:h=8:color=0x53fc18:t=fill",
		"drawbox=x=66:y=76:w=24:h=8:color=0x53fc18:t=fill",
		"fontcolor=black",
		"text='aimagia'",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "color=c=0x9146ff") {
		t.Fatalf("kick banner still uses twitch purple: %s", joined)
	}
}

func TestBuildFFmpegArgsLandscapeKickUsesGreenAccent(t *testing.T) {
	layout, ok := VariantByName(VariantStreamerLandscape16x9)
	if !ok {
		t.Fatal("landscape layout is not registered")
	}
	plan := EditPlan{
		Variant:        layout.Name,
		GameplayCrop:   layout.DefaultGameplayCrop,
		StreamerBanner: StreamerBannerPlan{Nick: "aimagia", Platform: StreamerBannerPlatformKick},
	}
	clip := ClipRange{ID: "clip-001", StartSeconds: 0, EndSeconds: 5}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath:     "source.mp4",
		OutputPath:     "out.mp4",
		BannerFontPath: "font.ttf",
	}, plan, clip)
	if err != nil {
		t.Fatalf("BuildFFmpegArgs error = %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "drawbox=x=0:y=0:w=8:h=64:color=0x53fc18:t=fill") {
		t.Fatalf("landscape kick args missing green accent: %s", joined)
	}
}

func TestEditPlanValidatesStreamerBannerPlatform(t *testing.T) {
	tests := []struct {
		platform string
		wantErr  bool
	}{
		{platform: ""},
		{platform: StreamerBannerPlatformTwitch},
		{platform: StreamerBannerPlatformKick},
		{platform: "Kick"},
		{platform: "youtube", wantErr: true},
		{platform: "kick-clip", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			plan := DefaultEditPlan()
			plan.StreamerBanner = StreamerBannerPlan{Nick: "aimagia", Platform: tt.platform}
			err := plan.Validate()
			if tt.wantErr && (err == nil || !strings.Contains(err.Error(), "platform")) {
				t.Fatalf("Validate error = %v, want platform error", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate error = %v, want nil", err)
			}
		})
	}
}

func TestEditPlanNormalizesAndValidatesStreamerBannerNick(t *testing.T) {
	plan := DefaultEditPlan()
	plan.StreamerBanner = StreamerBannerPlan{Nick: "  zacketizorcs2  "}
	plan = NormalizeEditPlan(plan)
	if plan.StreamerBanner.Nick != "zacketizorcs2" {
		t.Fatalf("normalized nick = %q, want %q", plan.StreamerBanner.Nick, "zacketizorcs2")
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate error = %v", err)
	}

	for _, nick := range []string{"nick with spaces", "@streamer", strings.Repeat("a", 26)} {
		plan.StreamerBanner.Nick = nick
		if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "streamer banner nick") {
			t.Fatalf("Validate nick %q error = %v, want streamer banner nick error", nick, err)
		}
	}
}
func TestBuildFFmpegArgsRejectsUnknownVariant(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Variant = "other"
	plan.Clips = []ClipRange{{ID: "clip-001", StartSeconds: 1.5, EndSeconds: 4.25}}

	_, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "source.mp4", OutputPath: "out.mp4"}, plan, plan.Clips[0])
	if err == nil || !strings.Contains(err.Error(), "unsupported stream render variant") {
		t.Fatalf("BuildFFmpegArgs error = %v, want unsupported variant error", err)
	}
}
