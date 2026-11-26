package recapplan

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func hookOptions() Options {
	o := fixtureOptions()
	o.Hook = &HookOptions{Enabled: true}
	return o
}

func hookFacts(kills map[int][]int) Facts {
	facts := fixtureFacts()
	for i := range facts.Rounds {
		for _, tick := range kills[facts.Rounds[i].Number] {
			facts.Rounds[i].Kills = append(facts.Rounds[i].Kills, killplan.Kill{Tick: tick, Weapon: "ak47"})
		}
	}
	return facts
}

func hookPlan(t *testing.T, facts Facts, options Options, assets ...AssetEvidence) Document {
	t.Helper()
	d, err := Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, assets, "facts")
	if err != nil || len(d.Blockers) > 0 {
		t.Fatalf("plan: %v; blockers: %+v", err, d.Blockers)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("planned document is invalid: %v", err)
	}
	return d
}

func hookItems(d Document) []TimelineItem {
	var items []TimelineItem
	for _, item := range d.Timeline {
		if item.IsHook() {
			items = append(items, item)
		}
	}
	return items
}

func TestHookIsOffByDefaultAndLeavesSavedPlansAlone(t *testing.T) {
	if DefaultOptions().Hook != nil {
		t.Fatal("the opening hook must be off by default")
	}
	facts := hookFacts(map[int][]int{2: {12000, 12100, 12300}})
	plain := hookPlan(t, facts, fixtureOptions())
	raw, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"hook"`) || len(hookItems(plain)) != 0 {
		t.Fatalf("a plan without the hook changed its wire shape or timeline: %s", raw)
	}
	var restored Document
	if err := decodeStrict(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if hash, err := restored.Hash(); err != nil || hash != plain.PlanHash {
		t.Fatalf("a saved plan changed its hash: %s != %s (%v)", hash, plain.PlanHash, err)
	}
	canonical, err := CanonicalNewOptions(DefaultOptions())
	if err != nil || canonical.Hook != nil {
		t.Fatalf("canonical defaults carry a hook: %+v %v", canonical.Hook, err)
	}
	current := DefaultOptions()
	current.Hook = &HookOptions{}
	if canonical, err = CanonicalNewOptions(current); err != nil || canonical.Hook != nil {
		t.Fatalf("a hook that is off must leave the canonical options: %+v %v", canonical.Hook, err)
	}
	current.Hook = &HookOptions{Enabled: true}
	if canonical, err = CanonicalNewOptions(current); err != nil || canonical.Hook == nil || !canonical.Hook.Enabled {
		t.Fatalf("the canonical options dropped the hook: %+v %v", canonical.Hook, err)
	}
	if err := canonical.ValidateCurrentFullDemoPolicy(); err != nil {
		t.Fatalf("a plan with the hook must be admitted: %v", err)
	}

	chosen := hookPlan(t, facts, hookOptions())
	if chosen.PlanHash == plain.PlanHash {
		t.Fatal("the hook must change the plan hash")
	}
	a, errA := plain.CaptureHash()
	b, errB := chosen.CaptureHash()
	if errA != nil || errB != nil || a != b {
		t.Fatal("the hook must not change the capture")
	}
	encoded, err := json.Marshal(chosen)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Document
	if err := decodeStrict(encoded, &decoded); err != nil || decoded.Hook == nil || *decoded.Hook != *chosen.Hook {
		t.Fatalf("hook round trip: %+v %v", decoded.Hook, err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("a decoded plan with the hook is invalid: %v", err)
	}
	again := hookPlan(t, facts, hookOptions())
	if again.PlanHash != chosen.PlanHash {
		t.Fatal("planning the same hook twice gave two hashes")
	}
}

func TestHookTeasesTheBestPlayBeforeTheFirstRound(t *testing.T) {
	d := hookPlan(t, hookFacts(map[int][]int{1: {5000}, 2: {12000, 12100, 12300}}), hookOptions())
	if d.Hook == nil || *d.Hook != (Hook{RoundID: "round-002", FirstKillTick: 12000, LastKillTick: 12300, Kills: 3}) {
		t.Fatalf("resolved hook = %+v, want the triple kill of round 2", d.Hook)
	}
	if len(d.Rounds) != 3 || len(d.Timeline) != 4 || len(hookItems(d)) != 1 {
		t.Fatalf("the hook must add one item and no round: %d rounds, %+v", len(d.Rounds), d.Timeline)
	}
	hook, first := d.Timeline[0], d.Timeline[1]
	round := d.Rounds[1]
	if !hook.IsHook() || hook.SourceRef != "round-002" || hook.SourceStartTick != round.RequestedStartTick || hook.SourceEndTick != round.EffectiveEndTick {
		t.Fatalf("hook item = %+v", hook)
	}
	if hook.StartFrame != 0 || hook.EndFrame != HookMinSeconds*OutputFPS || hook.SourceOffsetFrames != 810 {
		t.Fatalf("hook window = %+v, want 8 s ending 1.5 s after the last kill", hook)
	}
	if first.SourceRef != "round-001" || first.IsHook() || first.StartFrame != hook.EndFrame || first.SourceOffsetFrames != 0 {
		t.Fatalf("the match must start with round 1 after the hook: %+v", first)
	}
	if again := d.Timeline[2]; again.SourceRef != "round-002" || again.IsHook() || again.SourceOffsetFrames != 0 {
		t.Fatalf("the play must be seen again in its round: %+v", again)
	}
	for _, n := range d.Warnings {
		if n.Code == WarnHookSkipped {
			t.Fatalf("a planned hook warned: %+v", n)
		}
	}
}

func TestHookWindowStaysInsideItsRoundAndItsLimits(t *testing.T) {
	for _, tc := range []struct {
		name           string
		kills          []int
		offset, frames int64
	}{
		{"extends backwards to the minimum", []int{5000}, 1230, 480},
		{"clamps at the round start and extends forwards", []int{2600}, 0, 480},
		{"keeps the end of a long play", []int{3000, 3700, 4400, 5100, 5800}, 990, 1200},
		{"keeps a play between the limits", []int{4000, 4700}, 840, 690},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := hookPlan(t, hookFacts(map[int][]int{1: tc.kills}), hookOptions())
			items := hookItems(d)
			if len(items) != 1 || items[0].SourceRef != "round-001" || items[0].SourceOffsetFrames != tc.offset || items[0].EndFrame-items[0].StartFrame != tc.frames {
				t.Fatalf("hook = %+v, want offset %d and %d frames", items, tc.offset, tc.frames)
			}
			total, err := TickFrames(d.Rounds[0].EffectiveEndTick-d.Rounds[0].RequestedStartTick, d.Clock.TickRate)
			if err != nil || items[0].SourceOffsetFrames+tc.frames > total {
				t.Fatalf("the hook reads past its round: %+v of %d (%v)", items[0], total, err)
			}
		})
	}
}

func TestHookPrefersTheBiggerPlayAndBreaksTiesByMatchOrder(t *testing.T) {
	d := hookPlan(t, hookFacts(map[int][]int{1: {4000, 6000}, 2: {12000}, 25: {17500, 17600}}), hookOptions())
	if d.Hook == nil || d.Hook.RoundID != "round-025" || d.Hook.Kills != 2 {
		t.Fatalf("hook = %+v, want the fast double of round 25 over two lone kills 20 s apart", d.Hook)
	}
	tied := hookPlan(t, hookFacts(map[int][]int{1: {5000}, 2: {12000}}), hookOptions())
	if tied.Hook == nil || tied.Hook.RoundID != "round-001" {
		t.Fatalf("equal plays must keep the earliest: %+v", tied.Hook)
	}
}

func TestHookIsSkippedWithAWarningNeverABlocker(t *testing.T) {
	skipped := func(t *testing.T, d Document) {
		t.Helper()
		warned := 0
		for _, n := range d.Warnings {
			if n.Code == WarnHookSkipped {
				warned++
			}
		}
		if d.Hook != nil || len(hookItems(d)) != 0 || warned != 1 || len(d.Blockers) != 0 {
			t.Fatalf("a hook without a play must be skipped with one warning: hook %+v, warnings %+v, blockers %+v", d.Hook, d.Warnings, d.Blockers)
		}
	}
	skipped(t, hookPlan(t, fixtureFacts(), hookOptions()))
	single := hookOptions()
	single.Rounds = []int{2}
	skipped(t, hookPlan(t, hookFacts(map[int][]int{2: {12000, 12100}}), single))
	selected := hookOptions()
	selected.Rounds = []int{1, 25}
	skipped(t, hookPlan(t, hookFacts(map[int][]int{2: {12000, 12100}}), selected))
	afterDeath := hookPlan(t, hookFacts(map[int][]int{2: {14100}}), hookOptions())
	skipped(t, afterDeath)
}

func TestHookFollowsARoundSelection(t *testing.T) {
	options := hookOptions()
	options.Rounds = []int{1, 25}
	d := hookPlan(t, hookFacts(map[int][]int{1: {5000}, 2: {12000, 12100, 12300}}), options)
	if d.Hook == nil || d.Hook.RoundID != "round-001" || len(d.Timeline) != 3 {
		t.Fatalf("the hook must come from a round in the video: %+v %+v", d.Hook, d.Timeline)
	}
}

func TestHookSitsAfterTheIntroAndLeavesTheSponsorAfterRoundTwo(t *testing.T) {
	intro, sponsor, outro := bumperAsset("intro", 7), bumperAsset("sponsor", 20), bumperAsset("outro", 12)
	options := hookOptions()
	transitions := DynamicTransitions()
	options.Transitions = &transitions
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true, Video: &intro.Ref}, Sponsor: &BumperSlot{Enabled: true, Video: &sponsor.Ref}, Outro: BumperSlot{Enabled: true, Video: &outro.Ref}}
	d := hookPlan(t, hookFacts(map[int][]int{1: {5000, 5100}}), options, intro, sponsor, outro)
	var got []string
	for _, item := range d.Timeline {
		got = append(got, item.Role+":"+item.SourceRef+":"+item.Reason)
	}
	want := []string{"bumper:" + intro.Ref.ID, "round:round-001:hook", "round:round-001:", "round:round-002:", "bumper:" + sponsor.Ref.ID, "round:round-025:", "bumper:" + outro.Ref.ID}
	if len(got) != len(want) {
		t.Fatalf("timeline = %v", got)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Fatalf("timeline[%d] = %s, want %s…", i, got[i], want[i])
		}
	}
	if d.Timeline[1].StartFrame != 7*OutputFPS {
		t.Fatalf("the hook must follow the intro clip: %+v", d.Timeline[1])
	}
	boundaries := d.TransitionBoundaries()
	if len(boundaries) != 6 || boundaries[1].OutgoingIndex != 1 || boundaries[1].IncomingIndex != 2 {
		t.Fatalf("the cut from the hook into its own round needs the round effect: %+v", boundaries)
	}
	for _, n := range d.Warnings {
		if n.Code == WarnSponsorAfterLastRound {
			t.Fatal("the hook must not count as a round for the sponsor")
		}
	}
}

func TestHookSurvivesACertifiedTailTrim(t *testing.T) {
	d := hookPlan(t, hookFacts(map[int][]int{25: {18950}}), hookOptions())
	planned := hookItems(d)
	if len(planned) != 1 || planned[0].SourceRef != "round-025" || planned[0].SourceOffsetFrames != 900 || planned[0].EndFrame != 480 {
		t.Fatalf("planned hook = %+v", planned)
	}
	snapshot := Snapshot{Document: d, Approval: Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now()}}
	effective, err := ApplyCertifiedEnds(snapshot, map[string]int{"round-001": 8700, "round-002": 14300, "round-025": 19001})
	if err != nil {
		t.Fatal(err)
	}
	trimmed := hookItems(effective)
	round := effective.Rounds[2]
	total, err := TickFrames(round.EffectiveEndTick-round.RequestedStartTick, effective.Clock.TickRate)
	if err != nil || round.EffectiveEndTick != 19001 || len(trimmed) != 1 {
		t.Fatalf("effective round = %+v, hook = %+v (%v)", round, trimmed, err)
	}
	hook := trimmed[0]
	frames := hook.EndFrame - hook.StartFrame
	if frames != HookMinSeconds*OutputFPS || hook.SourceOffsetFrames+frames != total || hook.SourceEndTick != 19001 {
		t.Fatalf("the trimmed hook must end with its round and keep 8 s: %+v of %d frames", hook, total)
	}
	kill, err := TickFrames(18950-round.RequestedStartTick, effective.Clock.TickRate)
	if err != nil || kill < hook.SourceOffsetFrames || kill >= hook.SourceOffsetFrames+frames {
		t.Fatalf("the trimmed hook lost its kill: frame %d outside %+v", kill, hook)
	}
	if err := ValidateTimeline(effective.Timeline); err != nil {
		t.Fatal(err)
	}
	if hashed, err := effective.Hash(); err != nil || hashed != effective.PlanHash || effective.PlanHash == d.PlanHash {
		t.Fatalf("the effective document must carry its own hash: %v", err)
	}
	if d.Rounds[2].EffectiveEndTick != 19200 || hookItems(d)[0] != planned[0] {
		t.Fatal("the approved document was mutated")
	}
}

func TestHookMustMatchItsDocument(t *testing.T) {
	d := hookPlan(t, hookFacts(map[int][]int{2: {12000, 12100}}), hookOptions())
	rehash := func(d Document) Document {
		d.PlanHash, _ = d.Hash()
		return d
	}
	moved := d
	hook := *d.Hook
	hook.RoundID = "round-099"
	moved.Hook = &hook
	if err := rehash(moved).Validate(); err == nil {
		t.Fatal("a hook from a round outside the video was accepted")
	}
	late := d
	hook = *d.Hook
	hook.LastKillTick = 14200
	late.Hook = &hook
	if err := rehash(late).Validate(); err == nil {
		t.Fatal("a hook past the certified play was accepted")
	}
	unasked := d
	unasked.Options.Hook = nil
	if err := rehash(unasked).Validate(); err == nil {
		t.Fatal("a hook without its option was accepted")
	}
	dropped := d
	dropped.Hook = nil
	if err := rehash(dropped).Validate(); err == nil {
		t.Fatal("a timeline with a hook that the document does not resolve was accepted")
	}
}
