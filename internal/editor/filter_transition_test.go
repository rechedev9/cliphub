package editor

import (
	"strings"
	"testing"
)

func TestTransitionFlashFilters(t *testing.T) {
	tests := []struct {
		name      string
		short     ShortEdit
		want      []string
		forbidden []string
	}{
		{
			name: "first clip tail only",
			short: ShortEdit{
				Transition:       TransitionFlash,
				SegmentOrdinal:   1,
				SegmentTotal:     3,
				DurationSeconds:  6,
			},
			want:      []string{"fade=t=out:st=5.900:d=0.100:color=white"},
			forbidden: []string{"fade=t=in:"},
		},
		{
			name: "middle clip head and tail",
			short: ShortEdit{
				Transition:       TransitionFlash,
				SegmentOrdinal:   2,
				SegmentTotal:     3,
				DurationSeconds:  6,
			},
			want: []string{
				"fade=t=in:st=0:d=0.100:color=white",
				"fade=t=out:st=5.900:d=0.100:color=white",
			},
		},
		{
			name: "last clip head only",
			short: ShortEdit{
				Transition:       TransitionFlash,
				SegmentOrdinal:   3,
				SegmentTotal:     3,
				DurationSeconds:  6,
			},
			want:      []string{"fade=t=in:st=0:d=0.100:color=white"},
			forbidden: []string{"fade=t=out:"},
		},
		{
			name: "single clip no fades",
			short: ShortEdit{
				Transition:       TransitionFlash,
				SegmentOrdinal:   1,
				SegmentTotal:     1,
				DurationSeconds:  6,
			},
			forbidden: []string{"fade=t=in:", "fade=t=out:"},
		},
		{
			name: "short clip clamps tail start",
			short: ShortEdit{
				Transition:       TransitionFlash,
				SegmentOrdinal:   1,
				SegmentTotal:     2,
				DurationSeconds:  0.05,
			},
			want: []string{"fade=t=out:st=0.000:d=0.050:color=white"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(appendTransitionFlashFilters(nil, tc.short), ",")
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("filter missing %q:\n%s", want, got)
				}
			}
			for _, forbid := range tc.forbidden {
				if strings.Contains(got, forbid) {
					t.Fatalf("filter must not contain %q:\n%s", forbid, got)
				}
			}
		})
	}
}

func TestVideoFilterTransitionFlashInGraph(t *testing.T) {
	short := ShortEdit{
		Preset:           PresetViral60Clean,
		OutputFormat:     OutputFormatShort9x16,
		Transition:       TransitionFlash,
		SegmentOrdinal:   2,
		SegmentTotal:     3,
		DurationSeconds:  5,
		OutputFPS:        60,
	}
	got := VideoFilter(short)
	for _, want := range []string{"fade=t=in:st=0:d=0.100:color=white", "fade=t=out:st=4.900:d=0.100:color=white"} {
		if !strings.Contains(got, want) {
			t.Fatalf("VideoFilter missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "drawbox=x=0:y=0:w=iw:h=ih:color=0xffffff") {
		t.Fatalf("transition flash must not use drawbox white flash:\n%s", got)
	}
}

func TestIntroSlideXIncludesHoldAndSlideOut(t *testing.T) {
	got := introSlideX(5.0, 0.4, 13.7, 14.0, -563, 0)
	if !strings.Contains(got, "1-pow(1-(t-5.000)/0.400") {
		t.Fatalf("missing slide-in ease-out:\n%s", got)
	}
	if !strings.Contains(got, "pow((t-13.700)/0.300") {
		t.Fatalf("missing slide-out ease-in:\n%s", got)
	}
	if !strings.Contains(got, "if(lt(t\\,13.700)\\,0\\,") {
		t.Fatalf("missing hold position:\n%s", got)
	}
}
