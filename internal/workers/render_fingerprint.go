package workers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/playercard"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tasks"
)

type renderMusicInput struct {
	Key        string   `json:"key,omitempty"`
	Available  bool     `json:"available"`
	Volume     float64  `json:"volume,omitempty"`
	GameVolume *float64 `json:"game_volume,omitempty"`
	SHA256     string   `json:"sha256,omitempty"`
}

type renderFingerprintInput struct {
	SchemaVersion string                    `json:"schema_version"`
	Variant       string                    `json:"variant"`
	Edit          renderplan.EditRequest    `json:"edit"`
	Music         renderMusicInput          `json:"music"`
	Recording     recording.RecordingResult `json:"recording"`
	KillPlan      killplan.Plan             `json:"kill_plan"`
}

func renderInputFingerprint(result recording.RecordingResult, plan *killplan.Plan, variant, musicKey, musicPath string, effectiveMusicVolume float64, gameVolume *float64, edit renderplan.EditRequest) (string, error) {
	if edit.FullDemo != nil {
		return fullDemoRenderFingerprint(result, variant, *edit.FullDemo)
	}
	music := renderMusicInput{Key: musicKey, Available: musicPath != "", Volume: effectiveMusicVolume, GameVolume: gameVolume}
	if musicPath != "" {
		// #nosec G304 -- musicPath is the worker-local materialization of a validated durable music artifact.
		f, err := os.Open(musicPath)
		if err != nil {
			return "", fmt.Errorf("open music: %w", err)
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", fmt.Errorf("hash music: %w", copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close music: %w", closeErr)
		}
		music.SHA256 = fmt.Sprintf("%x", h.Sum(nil))
	}
	doc := renderFingerprintInput{
		SchemaVersion: "1.0",
		Variant:       variant,
		Edit:          edit,
		Music:         music,
		Recording:     result,
		KillPlan:      *plan,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum), nil
}

// renderExtras are what a render reads beside its edit and capture: the subtitle track and the player
// card. The render reads them once, so the video and its fingerprint see the same ones.
type renderExtras struct {
	Subtitles *subtitles.Track
	Card      *playercard.Facts
	Warnings  []string
}

// readRenderExtras reads the subtitles and the player card of a render; one it cannot read is left out
// of the video instead of failing it.
func (w *RenderWorker) readRenderExtras(ctx context.Context, j job.Job, edit renderplan.EditRequest) renderExtras {
	return renderExtras{Subtitles: w.readSubtitles(ctx, j, edit), Card: w.shortPlayerCard(ctx, j, edit)}
}

func (w *RenderWorker) awaitRenderExtras(ctx context.Context, j job.Job, edit renderplan.EditRequest) renderExtras {
	track, warning := w.awaitSubtitles(ctx, j, edit)
	extras := renderExtras{Subtitles: track, Card: w.shortPlayerCard(ctx, j, edit)}
	if warning != "" {
		extras.Warnings = []string{warning}
	}
	return extras
}

type renderExtrasInput struct {
	job         job.Job
	edit        renderplan.EditRequest
	fingerprint string
}

func (w *RenderWorker) fingerprintRenderExtras(ctx context.Context, in renderExtrasInput) (renderExtras, string, error) {
	extras := w.awaitRenderExtras(ctx, in.job, in.edit)
	fingerprint, err := extras.fingerprint(in.fingerprint)
	return extras, fingerprint, err
}

func (e renderExtras) renderWarnings(result editor.Result) []string {
	result.Warnings = append(slices.Clone(result.Warnings), e.Warnings...)
	return renderplan.CompleteRenderWarnings(result)
}

// fingerprint adds the extras to the fingerprint of the render's edit and capture, so a corrected phrase
// or a new card image renders again. Without extras it stays the same, which keeps older caches valid.
func (e renderExtras) fingerprint(base string) (string, error) {
	if e.Subtitles == nil && e.Card == nil {
		return base, nil
	}
	type card struct {
		Card        playercard.Card `json:"card"`
		ImageSHA256 string          `json:"image_sha256,omitempty"`
		ImageSource string          `json:"image_source,omitempty"`
	}
	doc := struct {
		Base      string           `json:"base"`
		Subtitles *subtitles.Track `json:"subtitles,omitempty"`
		Card      *card            `json:"card,omitempty"`
	}{Base: base, Subtitles: e.Subtitles}
	if e.Card != nil {
		doc.Card = &card{Card: e.Card.Card, ImageSource: e.Card.ImageSource}
		if len(e.Card.Image) > 0 {
			doc.Card.ImageSHA256 = fmt.Sprintf("%x", sha256.Sum256(e.Card.Image))
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil
}

// shortPlayerCard reads the player a one-player 9:16 Short's cards show; a failed read renders the
// Short without them.
func (w *RenderWorker) shortPlayerCard(ctx context.Context, j job.Job, edit renderplan.EditRequest) *playercard.Facts {
	if edit.OpeningCard == "" && edit.EndCard == "" && edit.CoverCard == "" {
		return nil
	}
	if edit.Format != renderplan.FormatShort9x16 || edit.FullDemo != nil || edit.MatchRecap || playercard.Unavailable(j) != "" {
		return nil
	}
	facts, err := playercard.Gather(ctx, w.playerCardSources(), j.ID, playercard.Target(j))
	if err != nil {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("player cards left out: %w", err))
		return nil
	}
	return &facts
}
