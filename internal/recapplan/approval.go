package recapplan

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// ResolveApproval accepts only a server-persisted document and verifies its
// dependencies again at execution. A client-supplied timeline is never trusted.
func ResolveApproval(ctx context.Context, store storage.Storage, id uuid.UUID, demoKey, target, ffmpeg string, proposed Snapshot) (Snapshot, error) {
	if err := proposed.Validate(); err != nil {
		return Snapshot{}, err
	}
	planID, err := uuid.Parse(proposed.Document.PlanID)
	if err != nil {
		return Snapshot{}, err
	}
	d, found, err := LoadDocument(store, id, planID)
	if err != nil {
		return Snapshot{}, err
	}
	if !found || d.PlanHash != proposed.Approval.PlanHash {
		return Snapshot{}, &Error{ErrPlanStale, "Approved plan is missing or differs from the saved document; plan and approve again"}
	}
	snapshot := Snapshot{Document: d, Approval: proposed.Approval}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	facts, found, err := LoadFacts(store, id)
	if err != nil {
		return Snapshot{}, err
	}
	factsHash, hashErr := HashValue(facts)
	if !found || hashErr != nil || factsHash != d.Input.FactsHash || facts.TargetSteamID64 != target || facts.DemoSHA256 != d.Input.DemoSHA256 || d.Input.FactsRef != artifacts.FullDemoFactsKey(id) {
		return Snapshot{}, &Error{ErrPlanStale, "Demo facts or player changed; plan and approve again"}
	}
	if err := mediaassets.VerifyContent(ctx, store, demoKey, d.Input.DemoSHA256, 8<<30); err != nil {
		return Snapshot{}, &Error{ErrPlanStale, "Demo content is unavailable or changed: " + err.Error()}
	}
	for _, asset := range d.Assets {
		assetID, err := uuid.Parse(asset.Ref.ID)
		if err != nil {
			return Snapshot{}, err
		}
		if err := mediaassets.VerifyContent(ctx, store, mediaassets.MediaKey(assetID), asset.Ref.SHA256, 8<<30); err != nil {
			return Snapshot{}, &Error{ErrAssetMissing, "Asset " + asset.Ref.ID + ": " + err.Error()}
		}
		p, found, err := mediaassets.LoadProvenance(store, assetID)
		if err != nil || !found || p.AssetSHA256 != asset.Ref.SHA256 || p.Title != asset.Title || p.Creator != asset.Creator || p.SourceURL != asset.SourceURL || p.Permission != asset.Permission || p.Attribution != asset.Attribution {
			return Snapshot{}, &Error{ErrPlanStale, "Asset provenance changed; plan and approve again"}
		}
	}
	voice := d.Voice
	if d.Options.Audio.Voice.Enabled || d.Options.Editorial.KeepFreezeVoice {
		extraction, err := voicecomms.EnsureStored(ctx, store, id, demoKey, d.Input.DemoSHA256, target, ffmpeg)
		if err != nil {
			return Snapshot{}, &Error{ErrVoiceDecode, err.Error()}
		}
		voice = VoiceFromExtraction(extraction)
	}
	replanned, err := Plan(facts, d.Options, voice, d.Assets, artifacts.FullDemoFactsKey(id))
	if err != nil {
		return Snapshot{}, err
	}
	if replanned.PlanHash != d.PlanHash {
		return Snapshot{}, &Error{ErrPlanStale, "Resolved voice, assets or planner output changed; approve the updated plan"}
	}
	return snapshot, nil
}

func VoiceFromExtraction(stored voicecomms.StoredExtraction) VoiceEvidence {
	r := stored.Result
	v := VoiceEvidence{Availability: r.Availability, ExtractorVersion: r.ExtractorVersion, ClockKind: r.ClockKind,
		IndexRef: stored.IndexKey, IndexHash: stored.IndexHash, SelectedPackets: r.SelectedPackets, ExcludedPackets: r.ExcludedPackets, Activity: []TickRange{}}
	for _, a := range r.Activity {
		v.Activity = append(v.Activity, TickRange{Start: a.StartTick, End: a.EndTick})
	}
	return v
}

// AssetReferences preserves playlist order while resolving each immutable file once.
func (o Options) AssetReferences() []AssetRef {
	refs := []AssetRef{}
	if o.Audio.Music.Enabled {
		refs = append(refs, o.Audio.Music.Assets...)
	}
	if o.Sponsor.Enabled {
		if o.Sponsor.Video != nil {
			refs = append(refs, *o.Sponsor.Video)
		}
		if o.Sponsor.AudioPolicy == "replace-narration" && o.Sponsor.Narration != nil {
			refs = append(refs, *o.Sponsor.Narration)
		}
	}
	unique := []AssetRef{}
	for _, ref := range refs {
		if !slices.Contains(unique, ref) {
			unique = append(unique, ref)
		}
	}
	return unique
}

func AssetFromMedia(a mediaassets.Asset, p mediaassets.Provenance) (AssetEvidence, error) {
	if err := a.Validate(); err != nil {
		return AssetEvidence{}, err
	}
	if err := p.Validate(); err != nil {
		return AssetEvidence{}, err
	}
	if a.SHA256 != p.AssetSHA256 || a.Probe.DurationSeconds <= 0 || a.Probe.DurationSeconds > 43200 {
		return AssetEvidence{}, fmt.Errorf("asset duration or provenance is invalid")
	}
	frames := int64(a.Probe.DurationSeconds*OutputFPS + 0.5)
	if frames < 1 || frames > 43200*OutputFPS {
		return AssetEvidence{}, fmt.Errorf("asset duration is outside supported bounds")
	}
	return AssetEvidence{Ref: AssetRef{ID: a.ID.String(), SHA256: a.SHA256}, DurationFrames: frames,
		HasVideo: a.Probe.VideoCodec != "" && a.Probe.Width > 0 && a.Probe.Height > 0, HasAudio: a.Probe.HasAudio,
		Title: p.Title, Creator: p.Creator, SourceURL: p.SourceURL, Permission: p.Permission, Attribution: p.Attribution}, nil
}
