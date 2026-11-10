package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// openRequestFile is the request Studio's window watches in its profile (desktop/src/studio-raise.ts).
	openRequestFile    = "open-request.json"
	openRequestVersion = 2
	maxStudioPath      = 16 * 1024
	maxOpenAckBytes    = 4 * 1024
)

// openWait is how long open_in_studio waits for the window's answer, a page load included.
var openWait = 8 * time.Second

type openInStudioIn struct {
	StudioPath string `json:"studio_path,omitempty" jsonschema:"a studio_path another tool returned, such as /replay?...; set this or replay_url"`
	ReplayURL  string `json:"replay_url,omitempty" jsonschema:"a full Studio address (http://127.0.0.1:<port>/...) another tool or the user gave; set this or studio_path"`
}

type openInStudioOut struct {
	Opened bool   `json:"opened" jsonschema:"true once the Studio window opened the page and came to the front"`
	URL    string `json:"url" jsonschema:"the full Studio address of the page, for the user to open by hand when opened is false"`
	Note   string `json:"note,omitempty"`
}

type openAck struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Reason  string `json:"reason"`
}

// studioPathOf turns the input into a page path on the Studio UI origin.
func (in openInStudioIn) studioPathOf(origin string) (string, error) {
	if (in.StudioPath == "") == (in.ReplayURL == "") {
		return "", errors.New("set exactly one of studio_path or replay_url")
	}
	p := in.StudioPath
	if in.ReplayURL != "" {
		u, err := url.Parse(in.ReplayURL)
		if err != nil || u.Scheme+"://"+u.Host != origin {
			return "", fmt.Errorf("replay_url must be a page of this Studio (%s); pass the studio_path instead", origin)
		}
		p = u.RequestURI()
		if u.Fragment != "" {
			p += "#" + u.Fragment
		}
	}
	if !isStudioPagePath(p) {
		return "", errors.New("studio_path must be a Studio page path that starts with one /, as other tools return it; API routes and files are refused")
	}
	return p, nil
}

// isStudioPagePath keeps app pages only, as the desktop window does: no API route, Next asset or file.
func isStudioPagePath(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "\\\r\n") || len(p) > maxStudioPath {
		return false
	}
	pathname, _, _ := strings.Cut(p, "?")
	pathname, _, _ = strings.Cut(pathname, "#")
	if pathname == "/api" || strings.HasPrefix(pathname, "/api/") || strings.HasPrefix(pathname, "/_next/") {
		return false
	}
	return !strings.Contains(path.Base(pathname), ".")
}

func registerOpenInStudioTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "open_in_studio",
		Description: "Bring the ClipHub Studio window to the front on a page: a studio_path from another tool (Replay, Preparar, Estadísticas, " +
			"a report or one of its round lists) or a full Studio address; only app pages, never API routes or files. The window routes there " +
			"in the app when Studio is open and the assistant may use it, and opened says whether it did. When it does not answer (loading, " +
			"an older Studio, the page failed), opened is false with the full http://127.0.0.1:<port> address for the user to open, " +
			"since the orchestrator has no handle on the window itself."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in openInStudioIn) (*mcp.CallToolResult, openInStudioOut, error) {
			origin, err := s.webOrigin()
			if err != nil {
				return nil, openInStudioOut{}, err
			}
			p, err := in.studioPathOf(origin)
			if err != nil {
				return nil, openInStudioOut{}, err
			}
			out := openInStudioOut{URL: origin + p}
			if _, statErr := os.Stat(s.sessionPath); statErr != nil {
				out.Note = "no Studio window runs for this orchestrator; give the user the url"
				return nil, out, nil
			}
			ack, err := s.requestOpen(ctx, p)
			if err != nil {
				return nil, out, err
			}
			out.Opened = ack != nil && ack.OK
			switch {
			case ack == nil:
				out.Note = "the Studio window did not answer (still loading, or this Studio version cannot be driven); give the user the url"
			case !ack.OK:
				out.Note = "the Studio window refused the page: " + cleanText(ack.Reason, maxErrorRunes) + "; give the user the url"
			}
			return nil, out, nil
		})
}

// requestOpen hands the path to the window through the profile and waits for its ack; nil when none came.
func (s *studio) requestOpen(ctx context.Context, p string) (*openAck, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(raw[:])
	dir := filepath.Dir(s.sessionPath)
	file, ackFile := filepath.Join(dir, openRequestFile), filepath.Join(dir, "open-ack-"+id+".json")
	body, err := json.Marshal(map[string]any{"version": openRequestVersion, "id": id, "path": p})
	if err != nil {
		return nil, err
	}
	tmp := filepath.Join(dir, "open-request-"+id+".tmp")
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return nil, fmt.Errorf("write the open request: %w", err)
	}
	if err := os.Rename(tmp, file); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("write the open request: %w", err)
	}
	defer func() { _ = os.Remove(ackFile) }()
	deadline := time.Now().Add(openWait)
	for time.Now().Before(deadline) {
		if ack, ok := readOpenAck(ackFile, id); ok {
			return &ack, nil
		}
		select {
		case <-ctx.Done():
			withdrawOpen(file, id)
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if ack, ok := readOpenAck(ackFile, id); ok {
		return &ack, nil
	}
	withdrawOpen(file, id)
	return nil, nil
}

func readOpenAck(file, id string) (openAck, bool) {
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxOpenAckBytes {
		return openAck{}, false
	}
	raw, err := os.ReadFile(file)
	var ack openAck
	if err != nil || json.Unmarshal(raw, &ack) != nil || ack.ID != id || ack.Version != openRequestVersion {
		return openAck{}, false
	}
	return ack, true
}

// withdrawOpen removes the request only while it is still this call's, so a newer call keeps its own.
func withdrawOpen(file, id string) {
	raw, err := os.ReadFile(file)
	var req struct {
		ID string `json:"id"`
	}
	if err == nil && json.Unmarshal(raw, &req) == nil && req.ID == id {
		_ = os.Remove(file)
	}
}
