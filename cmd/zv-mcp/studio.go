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
	"time"

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
	// WebOrigin serves the Studio UI's public assets, such as the HUD previews.
	WebOrigin string `json:"webOrigin,omitempty"`
}

// uploadTimeout bounds one whole HTTP exchange of an import: a demo can be
// several hundred MiB, which the default 30 s client would cut off.
const (
	uploadTimeout = 20 * time.Minute
	healthTimeout = 10 * time.Second
)

var (
	errStudioClosed = errors.New("ClipHub Studio is not running. Ask the user to open ClipHub Studio and try again")
	errAgentOff     = errors.New(`ClipHub Studio is open but assistant access is off. Ask the user to turn on "Permitir que un asistente use Studio" in Studio, Ajustes, Asistentes`)
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
	return s.clientWith(ctx, nil)
}

// uploadClient is client with a timeout long enough for a demo upload.
func (s *studio) uploadClient(ctx context.Context) (*tuiclient.Client, error) {
	return s.clientWith(ctx, &http.Client{Timeout: uploadTimeout})
}

func (s *studio) clientWith(ctx context.Context, hc *http.Client) (*tuiclient.Client, error) {
	cfg, err := s.config()
	if err != nil {
		return nil, err
	}
	cfg.HTTPClient = hc
	c := tuiclient.New(cfg)
	probe, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	if !c.Healthy(probe) {
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

// webOrigin returns the Studio UI origin, or CLIPHUB_WEB_ORIGIN when a bare
// orchestrator is driven without the session file.
func (s *studio) webOrigin() (string, error) {
	raw, err := os.ReadFile(s.sessionPath)
	if errors.Is(err, os.ErrNotExist) {
		if origin := os.Getenv("CLIPHUB_WEB_ORIGIN"); origin != "" {
			return origin, nil
		}
		return "", errStudioClosed
	}
	if err != nil {
		return "", fmt.Errorf("read Studio session: %w", err)
	}
	sess, err := parseAgentSession(raw)
	if err != nil {
		return "", err
	}
	if sess.WebOrigin == "" {
		return "", errors.New("this Studio version does not publish its UI address; ask the user to update ClipHub Studio")
	}
	return sess.WebOrigin, nil
}

func parseAgentSession(raw []byte) (agentSession, error) {
	var sess agentSession
	if err := json.Unmarshal(raw, &sess); err != nil || sess.Version != 1 {
		return agentSession{}, errStudioClosed
	}
	if !sess.Enabled {
		return agentSession{}, errAgentOff
	}
	if !loopbackPattern.MatchString(sess.OrchestratorURL) || !tokenPattern.MatchString(sess.Token) ||
		(sess.WebOrigin != "" && !loopbackPattern.MatchString(sess.WebOrigin)) {
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
			return fmt.Errorf("not found: %s", cleanText(apiErr.Message, maxErrorRunes))
		}
		return fmt.Errorf("Studio refused the request (%d): %s", apiErr.StatusCode, cleanText(apiErr.Message, maxErrorRunes))
	}
	if strings.Contains(err.Error(), "connect") {
		return errStudioClosed
	}
	return err
}
