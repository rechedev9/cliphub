package recording

import (
	"strings"
	"testing"
)

func TestFullDemoVoiceCaptureMutesBeforeRecord(t *testing.T) {
	tests := []struct {
		name string
		hud  HUDMode
	}{
		{name: "gameplay HUD is the full-demo capture", hud: HUDModeGameplay},
		{name: "empty HUD defaults to gameplay", hud: ""},
		{name: "clean shorts HUD still mutes demo voice", hud: HUDModeClean},
		{name: "deathnotices shorts HUD still mutes demo voice", hud: HUDModeDeathnotices},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := testPlan()
			plan.Stream.HUDMode = tt.hud
			js, err := GenerateHLAEJavaScript(plan)
			if err != nil {
				t.Fatalf("GenerateHLAEJavaScript: %v", err)
			}
			for _, want := range []string{
				`"voice_enable 0"`,
				`"tv_listen_voice_indices 0"`,
				`"tv_listen_voice_indices_h 0"`,
				`"voice_enable 1"`,
				`"tv_listen_voice_indices -1"`,
				`"tv_listen_voice_indices_h -1"`,
				`"key": "voice-restore-01"`,
			} {
				if !strings.Contains(js, want) {
					t.Fatalf("generated JS missing %q", want)
				}
			}
			schedule, _, _ := buildRuntimeSchedule(plan)
			muteTick, restoreTick, firstRecord, lastRecord := 0, 0, 0, 0
			for _, item := range schedule {
				for _, cmd := range item.Commands {
					switch cmd {
					case "voice_enable 0":
						muteTick = item.Tick
					case "voice_enable 1":
						restoreTick = item.Tick
					case "mirv_streams record start":
						if firstRecord == 0 || item.Tick < firstRecord {
							firstRecord = item.Tick
						}
					case "mirv_streams record end":
						if item.Tick > lastRecord {
							lastRecord = item.Tick
						}
					}
				}
			}
			if muteTick == 0 || restoreTick == 0 || firstRecord == 0 || lastRecord == 0 {
				t.Fatalf("mute=%d restore=%d record=%d..%d", muteTick, restoreTick, firstRecord, lastRecord)
			}
			if muteTick >= firstRecord {
				t.Fatalf("demo voice muted at tick %d, after record-start %d", muteTick, firstRecord)
			}
			if restoreTick <= lastRecord {
				t.Fatalf("demo voice restored at tick %d, before record-end %d", restoreTick, lastRecord)
			}
		})
	}
}

func TestFullDemoVoiceCaptureMuteAndRestoreCvars(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "mute",
			got:  voiceMuteCommands(),
			want: []string{"voice_enable 0", "tv_listen_voice_indices 0", "tv_listen_voice_indices_h 0"},
		},
		{
			name: "restore",
			got:  voiceRestoreCommands(),
			want: []string{"voice_enable 1", "tv_listen_voice_indices -1", "tv_listen_voice_indices_h -1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%q)", len(tt.got), len(tt.want), tt.got)
			}
			for i := range tt.want {
				if tt.got[i] != tt.want[i] {
					t.Fatalf("cmd[%d] = %q, want %q", i, tt.got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFullDemoVoiceCaptureRestoresWhenGameplayHasNoHUDCleanup(t *testing.T) {
	plan := testPlan()
	plan.Stream.HUDMode = HUDModeGameplay
	plan.Stream.PortraitSafeKillfeed = false
	if cmds := hudCleanupCommands(plan.Stream); len(cmds) != 0 {
		t.Fatalf("gameplay HUD cleanup = %q, want empty so voice restore cannot piggyback on it", cmds)
	}
	js, err := GenerateHLAEJavaScript(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js, `"key": "voice-restore-01"`) {
		t.Fatal("gameplay capture dropped voice restore because HUD cleanup is empty")
	}
}
