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
			assertDemoVoiceMutedBeforeRecord(t, plan, js)
			assertDemoVoiceRestoredInSoftQuit(t, js)
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
	assertDemoVoiceRestoredInSoftQuit(t, js)
}

func assertDemoVoiceMutedBeforeRecord(t *testing.T, plan RecordingPlan, js string) {
	t.Helper()
	for _, want := range []string{
		`"voice_enable 0"`,
		`"tv_listen_voice_indices 0"`,
		`"tv_listen_voice_indices_h 0"`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated JS missing mute %q", want)
		}
	}
	if strings.Contains(js, `"key": "voice-restore-01"`) {
		t.Fatal("voice restore is still on the tick schedule; failCapture and demo-EOF skip that schedule")
	}
	schedule, _, _ := buildRuntimeSchedule(plan)
	muteTick, firstRecord := 0, 0
	for _, item := range schedule {
		for _, cmd := range item.Commands {
			switch cmd {
			case "voice_enable 0":
				muteTick = item.Tick
			case "voice_enable 1":
				t.Fatalf("voice restore %q is still scheduled at tick %d", cmd, item.Tick)
			case "mirv_streams record start":
				if firstRecord == 0 || item.Tick < firstRecord {
					firstRecord = item.Tick
				}
			}
		}
	}
	if muteTick == 0 || firstRecord == 0 {
		t.Fatalf("mute=%d record-start=%d", muteTick, firstRecord)
	}
	if muteTick >= firstRecord {
		t.Fatalf("demo voice muted at tick %d, after record-start %d", muteTick, firstRecord)
	}
}

func assertDemoVoiceRestoredInSoftQuit(t *testing.T, js string) {
	t.Helper()
	idx := strings.Index(js, "const beginSoftQuit = () => {")
	if idx < 0 {
		t.Fatal("beginSoftQuit definition missing")
	}
	rest := js[idx:]
	end := strings.Index(rest, "};\n")
	if end < 0 {
		t.Fatal("beginSoftQuit body unterminated")
	}
	body := rest[:end]
	restore := strings.Index(body, `mirv.exec("voice_enable 1")`)
	mask := strings.Index(body, `mirv.exec("tv_listen_voice_indices -1")`)
	maskH := strings.Index(body, `mirv.exec("tv_listen_voice_indices_h -1")`)
	disc := strings.Index(body, `mirv.exec("disconnect")`)
	if restore < 0 || mask < 0 || maskH < 0 {
		t.Fatalf("beginSoftQuit missing voice restore:\n%s", body)
	}
	if disc < 0 {
		t.Fatal("beginSoftQuit does not disconnect")
	}
	if restore > disc || mask > disc || maskH > disc {
		t.Fatalf("voice restore must run before disconnect:\n%s", body)
	}
}
