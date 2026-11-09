package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/botreplay"
	"github.com/rechedev9/cliphub/internal/practice"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	codeInvalidBotReplay = "invalid_bot_replay"
	codeNoPositions      = "no_positions"
)

type botReplayRequest struct {
	Side           string `json:"side"`
	ReplaceSteamID string `json:"replace_steamid"`
}

type botReplayKeys struct {
	Restart string `json:"restart"`
	Next    string `json:"next"`
	Start   string `json:"start"`
}

type botReplayResponse struct {
	Launched       bool               `json:"launched"`
	AlreadyRunning bool               `json:"already_running"`
	CfgPath        string             `json:"cfg_path"`
	ExecCommand    string             `json:"exec_command"`
	Map            string             `json:"map"`
	Round          int                `json:"round"`
	Side           string             `json:"side"`
	BotSide        string             `json:"bot_side"`
	Keys           botReplayKeys      `json:"keys"`
	Bots           []botreplay.Bot    `json:"bots"`
	Starts         []botreplay.Start  `json:"starts"`
	Current        int                `json:"current"`
	Moments        []botreplay.Moment `json:"moments"`
	Lineups        []botreplay.Lineup `json:"lineups"`
	Limits         []string           `json:"limits"`
}

// LaunchLibraryBotReplay handles POST
// /api/library/matches/{id}/tactical/rounds/{round}/bot-replay: it writes the
// practice cfg in which bots stand where the other side stood, and opens CS2.
func (h *Handlers) LaunchLibraryBotReplay(w http.ResponseWriter, r *http.Request) {
	if h.practiceGame == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codePracticeUnavailable, "practice is not available")
		return
	}
	var in botReplayRequest
	if err := decodeSingleJSONBody(w, r, &in, true); err != nil {
		writeCodedError(w, http.StatusBadRequest, codeInvalidBotReplay, "invalid JSON body")
		return
	}
	m, doc, round, ok := h.libraryRound(w, r)
	if !ok {
		return
	}
	frames, err := h.allRoundFrames(m.DemoID, doc, round.Number)
	if err != nil {
		internalError(w, "read round positions", err)
		return
	}
	plan, err := botreplay.Build(botreplay.Options{
		Map: m.Map, Doc: doc, Round: round, Frames: frames,
		Side: tacticalplan.Side(in.Side), ReplaceSteamID: in.ReplaceSteamID,
	})
	switch {
	case errors.Is(err, botreplay.ErrNoPositions):
		writeCodedError(w, http.StatusConflict, codeNoPositions, "the round has no saved positions; analyse the demo again")
		return
	case err != nil:
		writeCodedError(w, http.StatusBadRequest, codeInvalidBotReplay, err.Error())
		return
	}
	result, err := practice.StartCfg(h.practiceGame, plan.Map, botreplay.RenderCfg(plan))
	if errors.Is(err, practice.ErrCS2NotFound) {
		writeCodedError(w, http.StatusServiceUnavailable, codeCS2NotFound, "CS2 is not installed")
		return
	}
	if err != nil {
		internalError(w, "launch bot replay", err)
		return
	}
	writeJSON(w, http.StatusOK, botReplayResponse{
		Launched: result.Launched, AlreadyRunning: result.AlreadyRunning, CfgPath: result.CfgPath,
		ExecCommand: practice.ExecCommand, Map: plan.Map, Round: plan.Round,
		Side: string(plan.UserSide), BotSide: string(plan.BotSide),
		Keys: botReplayKeys{Restart: botreplay.RestartKey, Next: botreplay.NextKey, Start: botreplay.StartKey},
		Bots: plan.Bots, Starts: nonNil(plan.Starts), Current: plan.Current,
		Moments: plan.Moments, Lineups: nonNil(plan.Lineups), Limits: botreplay.Limits,
	})
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// allRoundFrames reads every sampled frame of one round; a match without a
// blob has none.
func (h *Handlers) allRoundFrames(demoID uuid.UUID, doc tacticalplan.Document, round int) ([]tacticalplan.Frame, error) {
	offset, ok := doc.Positions.Offset(round)
	if !ok || offset.FrameCount == 0 {
		return nil, nil
	}
	rc, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoID))
	if err != nil {
		return nil, nil
	}
	defer rc.Close()
	blob, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}
	header, err := tacticalplan.DecodeHeader(blob)
	if err != nil {
		return nil, err
	}
	return tacticalplan.DecodeFrames(blob, offset.ByteOffset, offset.FrameCount, header)
}
