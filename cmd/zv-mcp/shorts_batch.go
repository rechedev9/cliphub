package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// shortLook is the preset and the look of a Short, shared by generate_short and generate_shorts_batch.
type shortLook struct {
	Preset string
	shortChoices
}

// resolve checks the look before Studio is called; the preset stays unset until catalogPreset.
func (l shortLook) resolve() (tuiclient.GenerateRequest, error) {
	if l.Preset != "" {
		if err := checkArtifactName("preset", l.Preset); err != nil {
			return tuiclient.GenerateRequest{}, err
		}
	}
	edit, err := resolveEdit(l.shortChoices)
	if err != nil {
		return tuiclient.GenerateRequest{}, err
	}
	music, err := resolveMusic(l.shortChoices)
	if err != nil {
		return tuiclient.GenerateRequest{}, err
	}
	return tuiclient.GenerateRequest{Music: music, Edit: edit}, nil
}

// catalogPreset checks the music track against Studio's songs and returns the chosen or default preset.
func (l shortLook) catalogPreset(ctx context.Context, c *tuiclient.Client) (string, error) {
	if l.Music != "" {
		songs, err := c.ListSongs(ctx)
		if err != nil {
			return "", toolError(err)
		}
		if err := checkMusic(l.Music, songs); err != nil {
			return "", err
		}
	}
	presets, err := c.Presets(ctx)
	if err != nil {
		return "", toolError(err)
	}
	return pickPreset(l.Preset, presets)
}

type shortsBatchIn struct {
	JobID    string  `json:"job_id"`
	MinScore float64 `json:"min_score,omitempty" jsonschema:"lowest play score, 0 to 1, the score list_moments gives; 0 or unset uses 0.6 (about a triple kill, or a double with a clutch or a big win-chance swing)"`
	Limit    int     `json:"limit,omitempty" jsonschema:"most Shorts to make, 1 to 10; default 5"`
	Preset   string  `json:"preset,omitempty" jsonschema:"preset name from list_presets; empty uses the default"`
	Approve  string  `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
	shortChoices
}

type shortsBatchOut struct {
	generateOut
	MinScore float64     `json:"min_score"`
	Limit    int         `json:"limit"`
	Plays    []momentOut `json:"plays" jsonschema:"the chosen plays in match order, one Short each; empty when no play reaches min_score"`
}

func registerShortsBatchTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "generate_shorts_batch", Meta: appToolMeta(jobProgressAppURI),
		Description: "Auto-Shorts: one Short per great play of a parsed job, without picking plays by hand. Studio keeps the best play of each round whose score reaches min_score, " +
			"then the limit best of those, captures them all in one CS2 session and renders each as its own Short in one variant (list_videos lists them). " +
			"The look takes the same options as generate_short (list_short_options). Not for death reels or a Full Demo. This opens CS2 full screen and records for minutes, so the user must approve it. " +
			"If the result says needs_approval, show the summary and the plays, ask the user, and call again with the same arguments and approve set to approval_code only if they agree. " +
			"Follow with wait_for_job, passing the returned variant."},
		func(ctx context.Context, req *mcp.CallToolRequest, in shortsBatchIn) (*mcp.CallToolResult, shortsBatchOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, shortsBatchOut{}, err
			}
			look := shortLook{Preset: in.Preset, shortChoices: in.shortChoices}
			body, err := look.resolve()
			if err != nil {
				return nil, shortsBatchOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, shortsBatchOut{}, toolError(err)
			}
			if body.Preset, err = look.catalogPreset(ctx, c); err != nil {
				return nil, shortsBatchOut{}, err
			}
			batch, err := c.GetShortsBatch(ctx, tuiclient.ShortsBatchQuery{JobID: in.JobID, MinScore: in.MinScore, Limit: in.Limit})
			if err != nil {
				return nil, shortsBatchOut{}, toolError(err)
			}
			out := shortsBatchOut{MinScore: batch.MinScore, Limit: batch.Limit, Plays: make([]momentOut, 0, len(batch.Moments))}
			for _, m := range batch.Moments {
				out.Plays = append(out.Plays, describeMoment(m))
			}
			if len(batch.SegmentIDs) == 0 {
				out.Summary = fmt.Sprintf("Ninguna jugada llega a la puntuación %g: baja min_score o elige jugadas con generate_short.", batch.MinScore)
				return nil, out, nil
			}
			plays, err := describePlays(batch.Moments, batch.SegmentIDs)
			if err != nil {
				return nil, shortsBatchOut{}, err
			}
			body.SegmentIDs, body.Edit.SeparateShorts = batch.SegmentIDs, true
			summary := fmt.Sprintf("Grabar %s, uno por jugada (la mejor de cada ronda con puntuación %g o más), con el preset %s: %s. Opciones: %s. CS2 se abrirá a pantalla completa y grabará durante varios minutos.",
				plural(len(batch.SegmentIDs), "Short", "Shorts"), batch.MinScore, body.Preset, plays, describeShortRequest(body))
			ok, err := sessionApprovals.approve(req, approvalSpec{Tool: "generate_shorts_batch", JobID: in.JobID, Request: body, Summary: summary, Given: in.Approve})
			if err != nil {
				return nil, shortsBatchOut{}, err
			}
			if ok.Ask != nil {
				return ok.Ask, shortsBatchOut{}, nil
			}
			if !ok.Approved {
				out.generateOut = generateOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}
				return nil, out, nil
			}
			res, err := c.StartShort(ctx, in.JobID, body)
			if err != nil {
				return nil, shortsBatchOut{}, toolError(err)
			}
			recentCaptures.mark(in.JobID)
			out.generateOut = startedOut(summary, res, "")
			return nil, out, nil
		})
}
