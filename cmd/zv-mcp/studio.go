package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// agentSession is the document Studio writes to <userData>/agent-session.json
// after boot. Without the file the MCP falls back to ORCHESTRATOR_URL and
// ZV_MUTATION_TOKEN, which is how a bare orchestrator is driven in development.
type agentSession struct {
	Version         int    `json:"version"`
	Enabled         bool   `json:"enabled"`
	OrchestratorURL string `json:"orchestratorUrl"`
	Token           string `json:"token"`
}

var (
	errStudioClosed = errors.New("ClipHub Studio is not running. Ask the user to open ClipHub Studio and try again")
	errAgentOff     = errors.New(`ClipHub Studio is open but assistant access is off. Ask the user to turn on "Permitir que Claude use Studio" in Studio, Ajustes`)
	tokenPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	loopbackPattern = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]{1,5}$`)
)

type studio struct {
	sessionPath string
}

func newStudio() *studio {
	dir := os.Getenv("CLIPHUB_STUDIO_USER_DATA")
	if dir == "" {
		dir = filepath.Join(os.Getenv("APPDATA"), "cliphub-studio")
	}
	return &studio{sessionPath: filepath.Join(dir, "agent-session.json")}
}

// client resolves the orchestrator on every call, so a Studio restart (new
// port or token) heals without restarting the MCP server.
func (s *studio) client(ctx context.Context) (*tuiclient.Client, error) {
	cfg, err := s.config()
	if err != nil {
		return nil, err
	}
	c := tuiclient.New(cfg)
	if !c.Healthy(ctx) {
		return nil, errStudioClosed
	}
	return c, nil
}

func (s *studio) config() (tuiclient.Config, error) {
	raw, err := os.ReadFile(s.sessionPath)
	if errors.Is(err, os.ErrNotExist) {
		if os.Getenv("ORCHESTRATOR_URL") != "" {
			return tuiclient.Config{}, nil
		}
		return tuiclient.Config{}, errStudioClosed
	}
	if err != nil {
		return tuiclient.Config{}, fmt.Errorf("read Studio session: %w", err)
	}
	sess, err := parseAgentSession(raw)
	if err != nil {
		return tuiclient.Config{}, err
	}
	return tuiclient.Config{BaseURL: sess.OrchestratorURL, Token: sess.Token}, nil
}

func parseAgentSession(raw []byte) (agentSession, error) {
	var sess agentSession
	if err := json.Unmarshal(raw, &sess); err != nil || sess.Version != 1 {
		return agentSession{}, errStudioClosed
	}
	if !sess.Enabled {
		return agentSession{}, errAgentOff
	}
	if !loopbackPattern.MatchString(sess.OrchestratorURL) || !tokenPattern.MatchString(sess.Token) {
		return agentSession{}, errors.New("the Studio session file is invalid; ask the user to restart ClipHub Studio")
	}
	return sess, nil
}

// toolError turns an orchestrator failure into a message the model can act on,
// without leaking request details such as the token header.
func toolError(err error) error {
	if err == nil || errors.Is(err, errStudioClosed) || errors.Is(err, errAgentOff) {
		return err
	}
	var apiErr *tuiclient.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return errors.New("ClipHub Studio restarted; call the tool again")
		case http.StatusNotFound:
			return fmt.Errorf("not found: %s", apiErr.Message)
		}
		return fmt.Errorf("Studio refused the request (%d): %s", apiErr.StatusCode, apiErr.Message)
	}
	if strings.Contains(err.Error(), "connect") {
		return errStudioClosed
	}
	return err
}
