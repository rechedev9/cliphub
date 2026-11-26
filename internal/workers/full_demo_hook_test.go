package workers

import (
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoRenderHashAndDeadlineFollowTheHook(t *testing.T) {
	twoRounds := func(f *recapplan.Facts) {
		f.EndTick = 4000
		f.Rounds[0].NextStartTick = 1200
		f.Rounds[0].Kills = []killplan.Kill{{Tick: 800}, {Tick: 850}}
		f.Rounds = append(f.Rounds, recapplan.RoundFacts{ID: "round-002", Number: 2, StartTick: 1200, FreezeEndTick: 1500, RoundEndTick: 3000, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}})
	}
	planOf := func(t *testing.T, hook bool) (string, recapplan.Snapshot) {
		t.Helper()
		result, _, _ := fullDemoPublicationFixture(t, "clip", func(f *recapplan.Facts, o *recapplan.Options) {
			twoRounds(f)
			if hook {
				o.Hook = &recapplan.HookOptions{Enabled: true}
			}
		})
		snapshot := recapplan.Snapshot{Document: *result.Plan.FullDemo, Approval: recapplan.Approval{PlanHash: result.Plan.FullDemo.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
		hash, err := fullDemoRenderFingerprint(result, "gameplay-pov-60", snapshot)
		if err != nil {
			t.Fatal(err)
		}
		return hash, snapshot
	}
	plainHash, plain := planOf(t, false)
	hookHash, hooked := planOf(t, true)
	if hooked.Document.Hook == nil || plain.Document.Hook != nil {
		t.Fatalf("fixture hooks = %+v / %+v", plain.Document.Hook, hooked.Document.Hook)
	}
	if plainHash == hookHash {
		t.Fatal("the hook must give the render its own identity")
	}
	a, errA := plain.Document.CaptureHash()
	b, errB := hooked.Document.CaptureHash()
	if errA != nil || errB != nil || a != b {
		t.Fatal("the hook must reuse the capture of the plan without it")
	}
	config := RenderWorkerConfig{Timeout: "1s"}
	if with, without := config.renderTimeout(&hooked, recording.RecordingPlan{}), config.renderTimeout(&plain, recording.RecordingPlan{}); with <= without {
		t.Fatalf("the render deadline must cover the hook: %s <= %s", with, without)
	}
}
