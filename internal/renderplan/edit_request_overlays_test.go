package renderplan

import "testing"

func TestEditRequestShortOverlayRules(t *testing.T) {
	short := func(edit EditRequest) EditRequest {
		edit.Format, edit.KillEffect, edit.Transition = FormatShort9x16, KillEffectClean, TransitionCut
		return NormalizeEditRequest(edit)
	}
	if err := short(EditRequest{VictimReplay: true, VictimReplaySpeed: 0.25, PlayBadges: true, WinProbability: true, StatsCard: true}).Validate(); err != nil {
		t.Fatalf("a Short with every overlay was rejected: %v", err)
	}
	if err := short(EditRequest{VictimReplay: true, VictimReplaySpeed: 0.75}).Validate(); err == nil {
		t.Fatal("victim replay speed 0.75 was accepted")
	}
	landscape := short(EditRequest{StatsCard: true})
	landscape.Format = FormatLandscape16x9
	if err := landscape.Validate(); err == nil {
		t.Fatal("a landscape stats card was accepted; the card is drawn for 9:16 only")
	}
	if got := short(EditRequest{VictimReplay: true, VictimReplaySpeed: DefaultVictimReplaySpeed}).VictimReplaySpeed; got != 0 {
		t.Fatalf("the default speed normalized to %v, want unset", got)
	}
	if got := short(EditRequest{VictimReplaySpeed: 1}).VictimReplaySpeed; got != 0 {
		t.Fatalf("a speed without a replay normalized to %v, want unset", got)
	}
	if death := short(EditRequest{DeathReel: true, PlayBadges: true, WinProbability: true}); death.PlayBadges || death.WinProbability {
		t.Fatal("a death reel kept the play badges or the win chance; its clips are not kill segments")
	}
}
