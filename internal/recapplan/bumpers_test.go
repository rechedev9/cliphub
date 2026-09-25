package recapplan

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
)

func bumperAsset(title string, seconds int64) AssetEvidence {
	return AssetEvidence{Ref: AssetRef{uuid.NewString(), strings.Repeat("e", 64)}, DurationFrames: seconds * 60, HasVideo: true, HasAudio: title == "intro", Title: title, Creator: "ClipHub tests", SourceURL: "local:test-fixture", Permission: "test-only"}
}

func TestBumpersWrapTheProgramAndPlaceTheSponsorAfterRoundTwo(t *testing.T) {
	intro, outro := bumperAsset("intro", 7), bumperAsset("outro", 12)
	sponsor := AssetEvidence{Ref: AssetRef{uuid.NewString(), strings.Repeat("d", 64)}, DurationFrames: 20 * 60, HasAudio: true, HasVideo: true, Title: "Synthetic sponsor", Creator: "ClipHub tests", SourceURL: "local:test-fixture", Permission: "test-only"}
	facts, options := fixtureFacts(), fixtureOptions()
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true, Video: &intro.Ref}, Sponsor: &BumperSlot{Enabled: true, Video: &sponsor.Ref}, Outro: BumperSlot{Enabled: true, Video: &outro.Ref}}
	d, err := Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, []AssetEvidence{intro, outro, sponsor}, "facts")
	if err != nil || len(d.Blockers) > 0 {
		t.Fatalf("plan: %v; blockers: %+v", err, d.Blockers)
	}
	roles := []string{}
	for _, item := range d.Timeline {
		roles = append(roles, item.Role+":"+item.Reason)
	}
	want := []string{"bumper:" + BumperRoleIntro, "round:", "round:", "bumper:" + BumperRoleSponsor, "round:", "bumper:" + BumperRoleOutro}
	if len(roles) != len(want) {
		t.Fatalf("timeline roles %v", roles)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(roles[i], prefix) {
			t.Fatalf("timeline[%d] = %s, want %s…", i, roles[i], prefix)
		}
	}
	first, last := d.Timeline[0], d.Timeline[len(d.Timeline)-1]
	if first.StartFrame != 0 || first.EndFrame != 7*60 || first.SourceRef != intro.Ref.ID {
		t.Fatalf("intro bumper: %+v", first)
	}
	if d.Timeline[1].StartFrame != 7*60 {
		t.Fatalf("gameplay must start after the intro: %+v", d.Timeline[1])
	}
	if last.SourceRef != outro.Ref.ID || last.EndFrame-last.StartFrame != 12*60 || last.StartFrame != d.Timeline[len(d.Timeline)-2].EndFrame {
		t.Fatalf("outro bumper: %+v", last)
	}
	if ad := d.Timeline[3]; ad.SourceRef != sponsor.Ref.ID || ad.StartFrame != d.Timeline[2].EndFrame || ad.EndFrame-ad.StartFrame != 20*60 || d.Timeline[4].StartFrame != ad.EndFrame {
		t.Fatalf("sponsor must play between rounds two and three without consuming gameplay: %+v", d.Timeline)
	}
	if boundaries := d.TransitionBoundaries(); len(boundaries) != 4 {
		t.Fatalf("intro, sponsor in/out and outro cuts: %+v", boundaries)
	}
	refs := options.AssetReferences()
	if len(refs) != 3 || refs[0] != intro.Ref || refs[1] != sponsor.Ref || refs[2] != outro.Ref {
		t.Fatalf("asset references: %+v", refs)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("bumper timeline does not survive document validation: %v", err)
	}
	if !d.Options.HasBumpers() {
		t.Fatal("HasBumpers")
	}
}

func TestSponsorFollowsTheOnlyRoundOfAShortProgram(t *testing.T) {
	sponsor, outro := bumperAsset("sponsor", 5), bumperAsset("outro", 3)
	facts, options := fixtureFacts(), fixtureOptions()
	facts.Rounds = facts.Rounds[:1]
	facts.Rounds[0].RoundEndTick, facts.Rounds[0].NextStartTick = 12800, 0
	options.Bumpers = &BumperOptions{Sponsor: &BumperSlot{Enabled: true, Video: &sponsor.Ref}, Outro: BumperSlot{Enabled: true, Video: &outro.Ref}}
	d, err := Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, []AssetEvidence{sponsor, outro}, "facts")
	if err != nil || len(d.Blockers) > 0 {
		t.Fatalf("plan: %v; blockers: %+v", err, d.Blockers)
	}
	if len(d.Timeline) != 3 || d.Timeline[1].Reason != BumperRoleSponsor || d.Timeline[2].Reason != BumperRoleOutro {
		t.Fatalf("one-round timeline: %+v", d.Timeline)
	}
	warned := 0
	for _, n := range d.Warnings {
		if n.Code == WarnSponsorAfterLastRound {
			warned++
		}
	}
	if warned != 1 {
		t.Fatalf("warnings: %+v", d.Warnings)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("rebuilding the timeline must not repeat the warning: %v", err)
	}

	sponsor.HasVideo = false
	d, err = Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, []AssetEvidence{sponsor, outro}, "facts")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range d.Warnings {
		if n.Code == WarnSponsorAfterLastRound {
			t.Fatalf("a sponsor that is not placed must not warn about its placement: %+v", d.Warnings)
		}
	}
}

func TestBumperBlockersKeepMissingClipsOffTheTimeline(t *testing.T) {
	intro := bumperAsset("intro", 7)
	facts, options := fixtureFacts(), fixtureOptions()
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true}, Sponsor: &BumperSlot{Enabled: true}, Outro: BumperSlot{Enabled: true, Video: &intro.Ref}}
	d, err := Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, b := range d.Blockers {
		if b.Code == ErrAssetMissing {
			messages = append(messages, b.Message)
		}
	}
	if len(messages) != 3 || !strings.Contains(messages[0], "intro video") || !strings.Contains(messages[1], "sponsor video") || !strings.Contains(messages[2], "outro video is missing") {
		t.Fatalf("bumper blockers: %+v", d.Blockers)
	}
	for _, item := range d.Timeline {
		if item.Role == "bumper" {
			t.Fatalf("blocked bumper reached the timeline: %+v", item)
		}
	}

	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true, Video: &intro.Ref}}
	d, err = Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, []AssetEvidence{intro}, "facts")
	if err != nil || len(d.Blockers) > 0 {
		t.Fatalf("plan: %v; blockers: %+v", err, d.Blockers)
	}
	if d.Timeline[0].Role != "bumper" || d.Timeline[len(d.Timeline)-1].Role != "round" {
		t.Fatalf("intro-only timeline: %+v", d.Timeline)
	}
	if !d.HasTransitionSFX() || d.TransitionBoundaries()[0].Frame != d.Timeline[1].StartFrame {
		t.Fatal("the intro must transition into the first round")
	}
}

func TestBumperTransitionsAlwaysWrapDemoWithoutEnablingRoundEffects(t *testing.T) {
	intro, outro := bumperAsset("intro", 1), bumperAsset("outro", 1)
	facts, options := fixtureFacts(), fixtureOptions()
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true, Video: &intro.Ref}, Outro: BumperSlot{Enabled: true, Video: &outro.Ref}}
	for _, mode := range []string{"absent", "off", "on"} {
		t.Run(mode, func(t *testing.T) {
			options.Transitions = nil
			if mode != "absent" {
				o := DynamicTransitions()
				o.Enabled = mode == "on"
				options.Transitions = &o
			}
			d, err := Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, []AssetEvidence{intro, outro}, "facts")
			if err != nil || len(d.Blockers) != 0 {
				t.Fatalf("plan: %v %+v", err, d.Blockers)
			}
			boundaries := d.TransitionBoundaries()
			want := 2
			if mode == "on" {
				want += len(d.Rounds) - 1
			}
			if len(boundaries) != want || boundaries[0].OutgoingIndex != 0 || boundaries[len(boundaries)-1].IncomingIndex != len(d.Timeline)-1 {
				t.Fatalf("%s boundaries: %+v", mode, boundaries)
			}
			if !d.HasTransitionSFX() {
				t.Fatal("bumper SFX missing")
			}
			if options.Transitions != nil && options.Transitions.Enabled != (mode == "on") {
				t.Fatal("mutated approved round preference")
			}
			for i, item := range d.Timeline {
				if item.EndSample-item.StartSample != (item.EndFrame-item.StartFrame)*SamplesPerFrame {
					t.Fatal("changed audio clock")
				}
				if i > 0 && item.StartFrame != d.Timeline[i-1].EndFrame {
					t.Fatal("changed frame coverage")
				}
			}
		})
	}
}

func TestCanonicalNewOptionsKeepsBumpersAndAbsentBumpersStayOffTheWire(t *testing.T) {
	ref := AssetRef{uuid.NewString(), strings.Repeat("e", 64)}
	options := DefaultOptions()
	options.Bumpers = &BumperOptions{Outro: BumperSlot{Enabled: true, Video: &ref}}
	canonical, err := CanonicalNewOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Bumpers == nil || canonical.Bumpers == options.Bumpers || *canonical.Bumpers != *options.Bumpers {
		t.Fatalf("canonical bumpers: %+v", canonical.Bumpers)
	}
	if err := canonical.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"bumpers"`) {
		t.Fatalf("absent bumpers must stay off the wire so approved hashes survive: %s", encoded)
	}
}

// Render history keeps snapshots approved before the sponsor became a bumper
// slot. They must still decode, while a stored draft plan reads as absent so
// the producer plans again instead of failing on a stale hash.
func TestRetiredSponsorFieldsStillDecodeAndOldPlansReadAsAbsent(t *testing.T) {
	d := fixtureDocument(t)
	b, err := json.Marshal(Snapshot{Document: d, Approval: Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: true}})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	wire["document"]["options"].(map[string]any)["sponsor"] = map[string]any{"enabled": true, "video": nil, "placement_policy": "first-two-rounds", "window_start_seconds": 90}
	wire["document"]["sponsor_placement"] = map[string]any{"boundary": "", "start_frame": 0, "duration_frames": 0, "candidates": []any{}}
	legacy, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(legacy, &snapshot); err != nil {
		t.Fatalf("legacy snapshot: %v", err)
	}
	if snapshot.Document.PlanID != d.PlanID || snapshot.Document.Options.HasBumpers() {
		t.Fatalf("legacy snapshot decoded wrongly: %+v", snapshot.Document.Options)
	}

	store := &memStore{}
	id := uuid.New()
	if err := SaveDocument(store, id, d); err != nil {
		t.Fatal(err)
	}
	legacyDocument, err := json.Marshal(wire["document"])
	if err != nil {
		t.Fatal(err)
	}
	planID, _ := uuid.Parse(d.PlanID)
	if err := store.Put(artifacts.FullDemoPlanKey(id, planID), bytes.NewReader(legacyDocument)); err != nil {
		t.Fatal(err)
	}
	if _, found, err := LoadCurrentDocument(store, id); err != nil || found {
		t.Fatalf("legacy plan: found=%v err=%v", found, err)
	}
	if hasRetiredSponsor([]byte(`{"assets":[{"title":"\"sponsor_placement\""}]}`)) {
		t.Fatal("a value that mentions the retired key is not a retired plan")
	}
}
