package recapplan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func bumperAsset(title string, seconds int64) AssetEvidence {
	return AssetEvidence{Ref: AssetRef{uuid.NewString(), strings.Repeat("e", 64)}, DurationFrames: seconds * 60, HasVideo: true, HasAudio: title == "intro", Title: title, Creator: "ClipHub tests", SourceURL: "local:test-fixture", Permission: "test-only"}
}

func TestBumpersWrapTheProgramAroundGameplayAndSponsor(t *testing.T) {
	intro, outro := bumperAsset("intro", 7), bumperAsset("outro", 12)
	sponsor := AssetEvidence{Ref: AssetRef{uuid.NewString(), strings.Repeat("d", 64)}, DurationFrames: 20 * 60, HasAudio: true, HasVideo: true, Title: "Synthetic sponsor", Creator: "ClipHub tests", SourceURL: "local:test-fixture", Permission: "test-only"}
	facts, options := fixtureFacts(), fixtureOptions()
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true, Video: &intro.Ref}, Outro: BumperSlot{Enabled: true, Video: &outro.Ref}}
	options.Sponsor.Enabled, options.Sponsor.Video = true, &sponsor.Ref
	d, err := Plan(facts, options, VoiceEvidence{Availability: "no_packets"}, []AssetEvidence{intro, outro, sponsor}, "facts")
	if err != nil || len(d.Blockers) > 0 {
		t.Fatalf("plan: %v; blockers: %+v", err, d.Blockers)
	}
	roles := []string{}
	for _, item := range d.Timeline {
		roles = append(roles, item.Role+":"+item.Reason)
	}
	want := []string{"bumper:" + BumperRoleIntro, "round:", "round:", "sponsor:approved-sponsor-placement", "round:", "bumper:" + BumperRoleOutro}
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
	// The sponsor window is measured in program frames, so the intro shifts
	// the accepted round boundary by its own duration.
	if d.SponsorPlacement.StartFrame != d.Timeline[3].StartFrame || d.SponsorPlacement.Boundary != "round-002" {
		t.Fatalf("sponsor placement: %+v", d.SponsorPlacement)
	}
	refs := options.AssetReferences()
	if len(refs) != 3 || refs[0] != sponsor.Ref || refs[1] != intro.Ref || refs[2] != outro.Ref {
		t.Fatalf("asset references: %+v", refs)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("bumper timeline does not survive document validation: %v", err)
	}
	if !d.Options.HasBumpers() {
		t.Fatal("HasBumpers")
	}
}

func TestBumperBlockersAndManualSponsorNeverSplitsTheIntro(t *testing.T) {
	intro := bumperAsset("intro", 7)
	facts, options := fixtureFacts(), fixtureOptions()
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true}, Outro: BumperSlot{Enabled: true, Video: &intro.Ref}}
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
	if len(messages) != 2 || !strings.Contains(messages[0], "intro video") || !strings.Contains(messages[1], "outro video is missing") {
		t.Fatalf("bumper blockers: %+v", d.Blockers)
	}
	for _, item := range d.Timeline {
		if item.Role == "bumper" {
			t.Fatalf("blocked bumper reached the timeline: %+v", item)
		}
	}

	frame := int64(60)
	options.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true, Video: &intro.Ref}}
	if _, _, found := resolveSponsor(SponsorOptions{PlacementPolicy: "manual-frame", ManualStartFrame: &frame, AllowSplitRound: true}, nil, 7*60, 20*60); found {
		t.Fatal("manual sponsor frame inside the intro bumper was accepted")
	}
	options.Sponsor.Enabled = false
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
