package streamclips

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func scriptLines(script, prefix string) []string {
	var out []string
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, prefix) {
			out = append(out, line)
		}
	}
	return out
}

func spoken(startMS int64, words ...string) []library.DemoVoiceWord {
	out := make([]library.DemoVoiceWord, 0, len(words))
	for i, text := range words {
		at := startMS + int64(i)*400
		out = append(out, library.DemoVoiceWord{Text: text, StartMS: at, EndMS: at + 300})
	}
	return out
}

var (
	fraseTiro    = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	fraseCreo    = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	fraseOculta  = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	fraseFuera   = uuid.MustParse("00000000-0000-0000-0000-000000000004")
	fraseLocura  = uuid.MustParse("00000000-0000-0000-0000-000000000005")
	fraseLarga   = uuid.MustParse("00000000-0000-0000-0000-000000000007")
	streamFrases = []VoiceLine{
		{ID: fraseTiro, StartMS: 61_000, EndMS: 63_000, Text: "vaya tiro le he dado, tío", Words: []library.DemoVoiceWord{
			{Text: "vaya", StartMS: 61_000, EndMS: 61_300}, {Text: "tiro", StartMS: 61_400, EndMS: 61_800}, {Text: "le", StartMS: 62_000, EndMS: 62_100},
			{Text: "he", StartMS: 62_100, EndMS: 62_200}, {Text: "dado,", StartMS: 62_200, EndMS: 62_600}, {Text: "tío", StartMS: 62_700, EndMS: 63_000},
		}},
		{ID: fraseCreo, StartMS: 68_500, EndMS: 71_000, Text: "no me lo creo", Corrected: true, OriginalText: "no me lo cree"},
		{ID: fraseOculta, StartMS: 104_000, EndMS: 105_000, Text: "esto no sale", Words: spoken(104_000, "esto", "no", "sale")},
		{ID: fraseFuera, StartMS: 106_000, EndMS: 107_000, Text: "ni esto", Discarded: true, Words: spoken(106_000, "ni", "esto")},
		{ID: fraseLocura, StartMS: 102_000, EndMS: 104_000, Text: "¡Qué locura!", Words: []library.DemoVoiceWord{
			{Text: "¡Qué", StartMS: 102_000, EndMS: 102_800}, {Text: "locura!", StartMS: 103_000, EndMS: 104_000},
		}},
		{ID: fraseLarga, StartMS: 108_000, EndMS: 109_600, Text: "los contraterroristas, cuidado", Words: spoken(108_000, "los", "contraterroristas,", "cuidado")},
	}
)

func TestAShortDrawsTheSpokenWordsOfItsMomentos(t *testing.T) {
	plan := DefaultEditPlan()
	plan.Clips = []ClipRange{
		{ID: "a", StartSeconds: 60, EndSeconds: 70},
		{ID: "b", StartSeconds: 100, EndSeconds: 110, Edit: &ClipEdit{Speed: 2}},
	}
	plan.Groups = []ClipGroup{{ID: "short-1", ClipIDs: []string{"a", "b"}}}
	track := SubtitleTrack{Lines: streamFrases, Hidden: map[string][]uuid.UUID{"short-1": {fraseOculta}}}

	got, err := NewSubtitleScripts(plan.SubtitleVideos(), track)
	if err != nil {
		t.Fatal(err)
	}
	style := "Style: Stream,Montserrat ExtraBold,88,&H00FFFFFF,&H00FFFFFF,&H00000000,&H96000000,0,0,0,0,100,100,0,0,1,7,2,2,120,192,740,1"
	want := map[string][]string{
		"a": {
			`Dialogue: 0,0:00:01.00,0:00:01.40,Stream,,0,0,0,,{\c&H00E6FF&}VAYA{\c&HFFFFFF&} TIRO\NLE HE`,
			`Dialogue: 0,0:00:01.40,0:00:02.00,Stream,,0,0,0,,VAYA {\c&H00E6FF&}TIRO{\c&HFFFFFF&}\NLE HE`,
			`Dialogue: 0,0:00:02.00,0:00:02.10,Stream,,0,0,0,,VAYA TIRO\N{\c&H00E6FF&}LE{\c&HFFFFFF&} HE`,
			`Dialogue: 0,0:00:02.10,0:00:02.20,Stream,,0,0,0,,VAYA TIRO\NLE {\c&H00E6FF&}HE{\c&HFFFFFF&}`,
			`Dialogue: 0,0:00:02.20,0:00:02.70,Stream,,0,0,0,,{\c&H00E6FF&}DADO{\c&HFFFFFF&} TÍO`,
			`Dialogue: 0,0:00:02.70,0:00:03.35,Stream,,0,0,0,,DADO {\c&H00E6FF&}TÍO{\c&HFFFFFF&}`,
			`Dialogue: 0,0:00:08.50,0:00:09.00,Stream,,0,0,0,,{\c&H00E6FF&}NO{\c&HFFFFFF&} ME LO CREO`,
			`Dialogue: 0,0:00:09.00,0:00:09.50,Stream,,0,0,0,,NO {\c&H00E6FF&}ME{\c&HFFFFFF&} LO CREO`,
			`Dialogue: 0,0:00:09.50,0:00:10.00,Stream,,0,0,0,,NO ME {\c&H00E6FF&}LO{\c&HFFFFFF&} CREO`,
		},
		"b": {
			`Dialogue: 0,0:00:01.00,0:00:01.50,Stream,,0,0,0,,{\c&H00E6FF&}¡QUÉ{\c&HFFFFFF&} LOCURA!`,
			`Dialogue: 0,0:00:01.50,0:00:02.35,Stream,,0,0,0,,¡QUÉ {\c&H00E6FF&}LOCURA!{\c&HFFFFFF&}`,
			`Dialogue: 0,0:00:04.00,0:00:04.20,Stream,,0,0,0,,{\c&H00E6FF&}LOS{\c&HFFFFFF&}\N{\fscx82\fscy82}CONTRATERRORISTAS{\fscx100\fscy100}`,
			`Dialogue: 0,0:00:04.20,0:00:04.40,Stream,,0,0,0,,LOS\N{\c&H00E6FF&}{\fscx82\fscy82}CONTRATERRORISTAS{\fscx100\fscy100}{\c&HFFFFFF&}`,
			`Dialogue: 0,0:00:04.40,0:00:04.90,Stream,,0,0,0,,{\c&H00E6FF&}CUIDADO{\c&HFFFFFF&}`,
		},
	}
	for clip, dialogues := range want {
		script := got.ByClip[clip]
		if styles := scriptLines(script, "Style:"); len(styles) != 1 || styles[0] != style {
			t.Errorf("momento %s style = %q, want %q", clip, styles, style)
		}
		if lines := scriptLines(script, "Dialogue:"); !slices.Equal(lines, dialogues) {
			t.Errorf("momento %s draws\n%s\nwant\n%s", clip, strings.Join(lines, "\n"), strings.Join(dialogues, "\n"))
		}
	}
	if !strings.Contains(got.ByClip["a"], "PlayResX: 1080\nPlayResY: 1920\n") {
		t.Errorf("the script of a Short is not on the 1080x1920 frame:\n%s", got.ByClip["a"])
	}
}

func TestAResumenDrawsEachFraseAsOneSubtitulo(t *testing.T) {
	video := SubtitleVideo{ID: SummaryVideo, Width: 1920, Height: 1080, Clips: []ClipRange{
		{ID: "play-001", StartSeconds: 60, EndSeconds: 70},
		{ID: "play-002", StartSeconds: 100, EndSeconds: 108},
	}}
	lines := append([]VoiceLine{{ID: uuid.MustParse("00000000-0000-0000-0000-000000000006"), StartMS: 107_500, EndMS: 108_400, Text: "a por ellos"}}, streamFrases...)
	track := SubtitleTrack{Lines: lines, Hidden: map[string][]uuid.UUID{"short-1": {fraseOculta}, SummaryVideo: {fraseLocura, fraseLarga}}}

	got, err := NewSubtitleScripts([]SubtitleVideo{video}, track)
	if err != nil {
		t.Fatal(err)
	}
	style := "Style: Subtitle,Montserrat ExtraBold,46,&H00FFFFFF,&H00FFFFFF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,3,1,2,230,230,150,1"
	want := map[string][]string{
		"play-001": {
			"Dialogue: 0,0:00:01.00,0:00:03.00,Subtitle,,0,0,0,,Vaya tiro le he dado, tío",
			"Dialogue: 0,0:00:08.50,0:00:10.00,Subtitle,,0,0,0,,No me lo creo",
		},
		"play-002": {
			"Dialogue: 0,0:00:04.00,0:00:05.20,Subtitle,,0,0,0,,Esto no sale",
			"Dialogue: 0,0:00:07.50,0:00:08.00,Subtitle,,0,0,0,,A por ellos",
		},
	}
	for clip, dialogues := range want {
		script := got.ByClip[clip]
		if styles := scriptLines(script, "Style:"); len(styles) != 1 || styles[0] != style {
			t.Errorf("jugada %s style = %q, want %q", clip, styles, style)
		}
		if lines := scriptLines(script, "Dialogue:"); !slices.Equal(lines, dialogues) {
			t.Errorf("jugada %s draws\n%s\nwant\n%s", clip, strings.Join(lines, "\n"), strings.Join(dialogues, "\n"))
		}
	}
}

func TestSubtitleFingerprintFollowsWhatAVideoDraws(t *testing.T) {
	videos := []SubtitleVideo{{ID: "a", Width: 1080, Height: 1920, Clips: []ClipRange{{ID: "a", StartSeconds: 60, EndSeconds: 70}}}}
	scripts := func(track SubtitleTrack, videos []SubtitleVideo) SubtitleScripts {
		t.Helper()
		got, err := NewSubtitleScripts(videos, track)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	drawn := scripts(SubtitleTrack{Lines: streamFrases}, videos)
	if drawn.Fingerprint == "" || drawn.Fingerprint != scripts(SubtitleTrack{Lines: streamFrases}, videos).Fingerprint {
		t.Fatalf("fingerprint = %q; want one that repeats for the same subtítulos", drawn.Fingerprint)
	}
	corrected := slices.Clone(streamFrases)
	corrected[0].Text = "vaya tiro le he dado, tía"
	if scripts(SubtitleTrack{Lines: corrected}, videos).Fingerprint == drawn.Fingerprint {
		t.Error("a corrected frase kept the fingerprint")
	}
	hidden := SubtitleTrack{Lines: streamFrases, Hidden: map[string][]uuid.UUID{"a": {fraseTiro}}}
	if scripts(hidden, videos).Fingerprint == drawn.Fingerprint {
		t.Error("a hidden frase kept the fingerprint")
	}
	silent := 0.0
	muted := []SubtitleVideo{{ID: "a", Width: 1080, Height: 1920, Clips: []ClipRange{{ID: "a", StartSeconds: 60, EndSeconds: 70, Edit: &ClipEdit{SourceVolume: &silent}}}}}
	for name, got := range map[string]SubtitleScripts{"no frases": scripts(SubtitleTrack{}, videos), "a muted momento": scripts(SubtitleTrack{Lines: streamFrases}, muted)} {
		if got.Fingerprint != "" || len(got.ByClip) != 0 {
			t.Errorf("%s: scripts = %+v; want nothing to draw", name, got)
		}
	}
}

func TestBuildFFmpegArgsDrawsSubtitlesOnTheEditedClock(t *testing.T) {
	plan := DefaultEditPlan()
	plan.FaceCropReviewed = true
	clip := ClipRange{ID: "a", StartSeconds: 60, EndSeconds: 70, Edit: &ClipEdit{Speed: 2, FadeInSeconds: 0.5}}
	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "in.mp4", OutputPath: "out.mp4", SubtitlesFilter: "ass=filename=a.ass:fontsdir=fonts"}, plan, clip)
	if err != nil {
		t.Fatal(err)
	}
	graph := args[slices.Index(args, "-filter_complex")+1]
	if !strings.Contains(graph, "setpts=PTS/2.000000,ass=filename=a.ass:fontsdir=fonts,fade=") {
		t.Fatalf("filter graph = %s; want the subtitles after the speed change and under the fade", graph)
	}
	plain, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: "in.mp4", OutputPath: "out.mp4"}, plan, clip)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(plain, " "), "ass=") {
		t.Fatalf("a clip without subtitles draws them: %v", plain)
	}
}
