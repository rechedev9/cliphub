// Package syncclient connects Studio to a ClipMind server. It sends the local
// library's changes and match files up, and brings the others' changes down,
// so the desktop and the web show the same account.
package syncclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

// ErrNotConnected means no account is linked.
var ErrNotConnected = errors.New("not connected to a ClipMind account")

const batchSize = 500

// Library is the slice of the library store the client reads.
type Library interface {
	ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error)
}

// Files is the artifact storage of the local library.
type Files interface {
	Put(key string, r io.Reader) error
	Open(key string) (io.ReadCloser, error)
	Exists(key string) (bool, error)
	List(key string) ([]string, error)
	Link(srcKey, dstKey string) error
	DeleteTree(key string) error
}

// Config sets up a Client.
type Config struct {
	// StatePath is the file that keeps the link: server, tokens and cursors.
	StatePath string
	DB        *sql.DB
	Library   Library
	Files     Files
	// VoiceFiles holds the team voice audio, one folder per match id; nil means none.
	VoiceFiles Files
	// HTTP is the HTTP client; nil means one with a 2 minute timeout.
	HTTP *http.Client
	// DeviceName labels this session in the account's device list.
	DeviceName string
	Now        func() time.Time
}

// Status is what the UI shows.
type Status struct {
	Connected     bool   `json:"connected"`
	ServerURL     string `json:"server_url,omitempty"`
	Email         string `json:"email,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	LastSyncAt    int64  `json:"last_sync_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	Pending       int    `json:"pending_changes"`
	Syncing       bool   `json:"syncing"`
}

// ConnectInput is a sign-in from Studio.
type ConnectInput struct {
	ServerURL   string
	Email       string
	Password    string
	WorkspaceID string
}

type state struct {
	ServerURL     string `json:"server_url"`
	Email         string `json:"email"`
	DeviceName    string `json:"device_name"`
	RefreshToken  string `json:"refresh_token"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	PushCursor    int64  `json:"push_cursor"`
	PullCursor    int64  `json:"pull_cursor"`
	LastSyncAt    int64  `json:"last_sync_at"`
	LastError     string `json:"last_error"`
}

// Client links one Studio to one workspace.
type Client struct {
	cfg     Config
	syncLog *store.SyncLog
	http    *http.Client

	mu      sync.Mutex
	st      state
	syncing bool
}

// New loads the saved link, if any.
func New(cfg Config) (*Client, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 2 * time.Minute}
	}
	c := &Client{cfg: cfg, syncLog: store.NewSyncLog(cfg.DB), http: cfg.HTTP}
	// #nosec G304 -- StatePath is inside the data directory.
	raw, err := os.ReadFile(cfg.StatePath)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &c.st); err != nil {
			return nil, fmt.Errorf("read sync state: %w", err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}
	return c, nil
}

func (c *Client) save() error {
	raw, err := json.MarshalIndent(c.st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.cfg.StatePath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(c.cfg.StatePath, raw, 0o600)
}

func (c *Client) connected() bool { return c.st.RefreshToken != "" }

// Status reports the link and how many local changes wait to be sent.
func (c *Client) Status(ctx context.Context) Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := Status{
		Connected: c.connected(), ServerURL: c.st.ServerURL, Email: c.st.Email, WorkspaceID: c.st.WorkspaceID,
		WorkspaceName: c.st.WorkspaceName, LastSyncAt: c.st.LastSyncAt, LastError: c.st.LastError, Syncing: c.syncing,
	}
	if s.Connected {
		if n, err := c.syncLog.Pending(ctx, c.st.PushCursor); err == nil {
			s.Pending = n
		}
	}
	return s
}

// validateServerURL refuses to send a password over plain HTTP, except to this machine.
func validateServerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("the server address must look like https://clipmind.example.com")
	}
	host := u.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme == "http" && !local {
		return "", errors.New("the server address must use https")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/"), nil
}

type tokens struct {
	Access      string `json:"access_token"`
	Refresh     string `json:"refresh_token"`
	WorkspaceID string `json:"workspace_id"`
}

type apiError struct {
	Status int
	Code   string
	Msg    string
}

func (e *apiError) Error() string { return fmt.Sprintf("server answered %d: %s", e.Status, e.Msg) }

func (c *Client) do(ctx context.Context, method, path, access string, header http.Header, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.st.ServerURL+path, body)
	if err != nil {
		return err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if strings.HasPrefix(path, "/api/sync/changes") {
		contract, err := c.syncLog.Contract(ctx)
		if err != nil {
			return err
		}
		req.Header.Set(store.SyncContractHeader, contract.Hash)
		req.Header.Set(store.SyncSchemaHeader, strconv.Itoa(contract.Schema))
	}
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
		return &apiError{Status: resp.StatusCode, Code: e.Code, Msg: e.Error}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if w, ok := out.(io.Writer); ok {
		_, err := io.Copy(w, resp.Body)
		return err
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	return dec.Decode(out)
}

func (c *Client) doJSON(ctx context.Context, method, path, access string, in, out any) error {
	var body io.Reader
	header := http.Header{}
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
		header.Set("Content-Type", "application/json")
	}
	return c.do(ctx, method, path, access, header, body, out)
}

// Connect signs in and links this Studio to the account's workspace.
func (c *Client) Connect(ctx context.Context, in ConnectInput) (Status, error) {
	server, err := validateServerURL(in.ServerURL)
	if err != nil {
		return Status{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	device := c.cfg.DeviceName
	if device == "" {
		device = "ClipHub Studio"
	}
	next := state{ServerURL: server, Email: strings.TrimSpace(in.Email), DeviceName: device}
	prev := c.st
	c.st = next
	var t tokens
	if err := c.doJSON(ctx, http.MethodPost, "/auth/login", "", map[string]string{"email": in.Email, "password": in.Password, "device": device}, &t); err != nil {
		c.st = prev
		return Status{}, err
	}
	if in.WorkspaceID != "" && in.WorkspaceID != t.WorkspaceID {
		if err := c.doJSON(ctx, http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": t.Refresh, "workspace_id": in.WorkspaceID}, &t); err != nil {
			c.st = prev
			return Status{}, err
		}
	}
	var me struct {
		Workspace *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspace"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/auth/me", t.Access, nil, &me); err != nil || me.Workspace == nil {
		c.st = prev
		return Status{}, errors.New("the account has no workspace to sync with")
	}
	c.st.RefreshToken = t.Refresh
	c.st.WorkspaceID = me.Workspace.ID
	c.st.WorkspaceName = me.Workspace.Name
	// A new workspace starts from nothing: send everything this library holds.
	if prev.WorkspaceID != c.st.WorkspaceID || prev.ServerURL != c.st.ServerURL {
		if err := c.syncLog.Backfill(ctx); err != nil {
			c.st = prev
			return Status{}, fmt.Errorf("prepare the library for sync: %w", err)
		}
		c.st.PushCursor, c.st.PullCursor = 0, 0
	} else {
		c.st.PushCursor, c.st.PullCursor = prev.PushCursor, prev.PullCursor
	}
	if err := c.save(); err != nil {
		c.st = prev
		return Status{}, err
	}
	return Status{Connected: true, ServerURL: c.st.ServerURL, Email: c.st.Email, WorkspaceID: c.st.WorkspaceID, WorkspaceName: c.st.WorkspaceName}, nil
}

// Disconnect signs the device out and forgets the link. Local data stays.
func (c *Client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected() {
		return nil
	}
	if access, err := c.refreshLocked(ctx); err == nil {
		_ = c.doJSON(ctx, http.MethodPost, "/auth/logout", access, nil, nil)
	}
	c.st = state{}
	if err := os.Remove(c.cfg.StatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// refreshLocked rotates the refresh token and returns a fresh access token.
func (c *Client) refreshLocked(ctx context.Context) (string, error) {
	var t tokens
	err := c.doJSON(ctx, http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": c.st.RefreshToken, "workspace_id": c.st.WorkspaceID}, &t)
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) && (ae.Status == http.StatusBadRequest || ae.Status == http.StatusUnauthorized || ae.Status == http.StatusNotFound) {
			c.st.RefreshToken = ""
			c.st.LastError = "the session ended; sign in again"
			_ = c.save()
		}
		return "", err
	}
	c.st.RefreshToken = t.Refresh
	if err := c.save(); err != nil {
		return "", err
	}
	return t.Access, nil
}

// Sync pulls the others' changes, pushes the local ones, then exchanges match files.
func (c *Client) Sync(ctx context.Context) error {
	c.mu.Lock()
	if !c.connected() {
		c.mu.Unlock()
		return ErrNotConnected
	}
	if c.syncing {
		c.mu.Unlock()
		return nil
	}
	c.syncing = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.syncing = false
		c.mu.Unlock()
	}()

	err := c.syncOnce(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.st.LastError = syncErrorText(err)
	} else {
		c.st.LastError = ""
		c.st.LastSyncAt = c.cfg.Now().UnixMilli()
	}
	_ = c.save()
	return err
}

func (c *Client) syncOnce(ctx context.Context) error {
	c.mu.Lock()
	access, err := c.refreshLocked(ctx)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if err := c.pull(ctx, access); err != nil {
		return fmt.Errorf("receive changes: %w", err)
	}
	if err := c.push(ctx, access); err != nil {
		return fmt.Errorf("send changes: %w", err)
	}
	if err := c.exchangeArtifacts(ctx, access); err != nil {
		return fmt.Errorf("exchange match files: %w", err)
	}
	return nil
}

func (c *Client) pull(ctx context.Context, access string) error {
	for {
		var batch struct {
			Changes []store.SyncChange `json:"changes"`
			Next    json.Number        `json:"next"`
		}
		path := fmt.Sprintf("/api/sync/changes?after=%d&limit=%d", c.st.PullCursor, batchSize)
		if err := c.doJSON(ctx, http.MethodGet, path, access, nil, &batch); err != nil {
			return err
		}
		next, err := batch.Next.Int64()
		if err != nil {
			return err
		}
		if len(batch.Changes) > 0 {
			if err := c.applyPulled(ctx, batch.Changes); err != nil {
				return err
			}
		}
		moved := next > c.st.PullCursor
		c.st.PullCursor = next
		if len(batch.Changes) == 0 && !moved {
			return nil
		}
		if err := c.save(); err != nil {
			return err
		}
		if !moved {
			return nil
		}
	}
}

func (c *Client) push(ctx context.Context, access string) error {
	for {
		batch, err := c.syncLog.Changes(ctx, c.st.PushCursor, batchSize)
		if err != nil {
			return err
		}
		if batch.Next == c.st.PushCursor {
			return nil
		}
		if len(batch.Changes) > 0 {
			if err := c.doJSON(ctx, http.MethodPost, "/api/sync/changes", access, map[string]any{"changes": batch.Changes}, nil); err != nil {
				return err
			}
		}
		c.st.PushCursor = batch.Next
		if err := c.save(); err != nil {
			return err
		}
	}
}

type hubDemo struct {
	ID        string `json:"id"`
	Tactical  bool   `json:"tactical"`
	Positions bool   `json:"positions"`
}

func (c *Client) exchangeArtifacts(ctx context.Context, access string) error {
	var hub struct {
		Demos []hubDemo `json:"demos"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/sync/artifacts", access, nil, &hub); err != nil {
		return err
	}
	onHub := make(map[string]hubDemo, len(hub.Demos))
	for _, d := range hub.Demos {
		onHub[d.ID] = d
	}
	for offset := 0; ; offset += library.MaxListLimit {
		page, err := c.cfg.Library.ListDemos(ctx, library.DemoFilter{Limit: library.MaxListLimit, Offset: offset})
		if err != nil {
			return err
		}
		for _, d := range page {
			if d.State != library.ParseReady {
				continue
			}
			if err := c.exchangeDemo(ctx, access, d, onHub[d.ID.String()]); err != nil {
				// One match must not block the others.
				log.Printf("sync: match files of %s: %v", d.ID, err)
			}
		}
		if len(page) < library.MaxListLimit {
			return nil
		}
	}
}

func (c *Client) exchangeDemo(ctx context.Context, access string, d library.Demo, hub hubDemo) error {
	files := []struct {
		name  string
		key   string
		onHub bool
	}{
		{"tactical.json", artifacts.LibraryTacticalIndexKey(d.ID), hub.Tactical},
		{"positions.zvpos", artifacts.LibraryTacticalPositionsKey(d.ID), hub.Positions},
	}
	for _, f := range files {
		local, err := c.cfg.Files.Exists(f.key)
		if err != nil {
			return err
		}
		path := "/api/sync/artifacts/" + d.ID.String() + "/" + f.name
		switch {
		case local && !f.onHub:
			if err := c.upload(ctx, access, path, f.key); err != nil {
				return err
			}
		case !local && f.onHub:
			if err := c.download(ctx, access, path, f.key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) upload(ctx context.Context, access, path, key string) error {
	src, err := c.cfg.Files.Open(key)
	if err != nil {
		return err
	}
	defer src.Close()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := io.Copy(gz, src); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	header := http.Header{}
	header.Set("Content-Encoding", "gzip")
	header.Set("Content-Type", "application/octet-stream")
	return c.do(ctx, http.MethodPut, path, access, header, &buf, nil)
}

func (c *Client) download(ctx context.Context, access, path, key string) error {
	var buf bytes.Buffer
	if err := c.do(ctx, http.MethodGet, path, access, nil, nil, &buf); err != nil {
		return err
	}
	return c.cfg.Files.Put(key, &buf)
}

// Run syncs every interval while linked, until ctx ends.
func (c *Client) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Sync(ctx); err != nil && !errors.Is(err, ErrNotConnected) {
				log.Printf("sync: %v", err)
			}
		}
	}
}

// syncErrorText is what Ajustes shows; a version mismatch says who has to update.
func syncErrorText(err error) string {
	var api *apiError
	if errors.As(err, &api) {
		switch api.Code {
		case store.CodeSyncClientOutdated:
			return "Actualiza ClipHub Studio: el servidor de ClipMind tiene una versión más nueva y la sincronización está en pausa."
		case store.CodeSyncServerOutdated:
			return "El servidor de ClipMind aún no tiene esta versión de Studio. La sincronización sigue cuando se actualice."
		}
	}
	return err.Error()
}
