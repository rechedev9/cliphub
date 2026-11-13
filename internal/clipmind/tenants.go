package clipmind

import (
	"container/list"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
)

// tenant is one workspace's library: its own SQLite file, its own artifact
// folder and its own orchestrator handlers. Nothing is shared between tenants.
type tenant struct {
	id      string
	repos   *store.Repositories
	files   *storage.Local
	handler http.Handler
	elem    *list.Element
	refs    int
	closing bool
	stop    context.CancelFunc
}

// tenants opens workspaces lazily and keeps the most recently used ones. A
// tenant in use is never closed: eviction waits for its last request.
type tenants struct {
	root   string
	token  string
	max    int
	shared []httpapi.Option

	mu    sync.Mutex
	open  map[string]*tenant
	order *list.List
}

func newTenants(root string, max int, shared []httpapi.Option) (*tenants, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("workspaces folder: %w", err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	if max <= 0 {
		max = 64
	}
	return &tenants{root: root, token: hex.EncodeToString(raw), max: max, shared: shared, open: map[string]*tenant{}, order: list.New()}, nil
}

// workspaceDir is the folder of one workspace; the id is a UUID from the catalog.
func (t *tenants) workspaceDir(id string) string { return filepath.Join(t.root, id) }

// acquire returns the tenant with a reference the caller releases when its request ends.
func (t *tenants) acquire(id string) (*tenant, error) {
	if id == "" || id != filepath.Base(id) {
		return nil, errors.New("invalid workspace id")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	tn, ok := t.open[id]
	if !ok {
		var err error
		if tn, err = t.openLocked(id); err != nil {
			return nil, err
		}
	}
	tn.refs++
	t.order.MoveToFront(tn.elem)
	t.evictLocked()
	return tn, nil
}

func (t *tenants) release(tn *tenant) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tn.refs--
	if tn.closing && tn.refs == 0 {
		_ = tn.repos.Close()
	}
	t.evictLocked()
}

func (t *tenants) openLocked(id string) (*tenant, error) {
	dir := t.workspaceDir(id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("workspace folder: %w", err)
	}
	repos, err := store.OpenSQLite(filepath.Join(dir, "jobs.db"))
	if err != nil {
		return nil, fmt.Errorf("open workspace %s: %w", id, err)
	}
	files, err := storage.NewLocal(filepath.Join(dir, "data"))
	if err != nil {
		_ = repos.Close()
		return nil, fmt.Errorf("workspace storage: %w", err)
	}
	opts := append([]httpapi.Option{
		httpapi.WithMutationToken(t.token),
		httpapi.WithRequireReadAuth(true),
		httpapi.WithRateLimit(0, 0),
		httpapi.WithUploadConcurrency(2),
		httpapi.WithLibrary(repos.Library),
		httpapi.WithLibraryUploads(),
		httpapi.WithSyncHub(store.NewSyncLog(repos.DB())),
	}, t.shared...)
	h := httpapi.NewHandlers(repos.Jobs, files, noQueue{}, opts...)
	ctx, stop := context.WithCancel(context.Background())
	h.StartProFollowSync(ctx)
	tn := &tenant{id: id, repos: repos, files: files, handler: httpapi.Routes(h), stop: stop}
	tn.elem = t.order.PushFront(tn)
	t.open[id] = tn
	return tn, nil
}

func (t *tenants) evictLocked() {
	for el := t.order.Back(); el != nil && len(t.open) > t.max; {
		prev := el.Prev()
		tn := el.Value.(*tenant)
		if tn.refs == 0 {
			t.dropLocked(tn)
		}
		el = prev
	}
}

func (t *tenants) dropLocked(tn *tenant) {
	delete(t.open, tn.id)
	t.order.Remove(tn.elem)
	tn.stop()
	tn.closing = true
	if tn.refs == 0 {
		_ = tn.repos.Close()
	}
}

// remove closes a workspace and deletes its folder. It waits for nothing: the
// workspace no longer exists in the catalog, so no new request can reach it.
func (t *tenants) remove(id string) error {
	if id == "" || id != filepath.Base(id) {
		return errors.New("invalid workspace id")
	}
	t.mu.Lock()
	if tn, ok := t.open[id]; ok {
		t.dropLocked(tn)
	}
	t.mu.Unlock()
	return os.RemoveAll(t.workspaceDir(id))
}

func (t *tenants) closeAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, tn := range t.open {
		t.dropLocked(tn)
	}
}

// noQueue stands in for the job queue: the web edition runs no jobs.
type noQueue struct{}

var errNoQueue = errors.New("jobs are not available in this edition")

func (noQueue) Enqueue(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
	return nil, errNoQueue
}

func (noQueue) EnqueueWithTransition(*asynq.Task, func(error) error, ...asynq.Option) (*asynq.TaskInfo, error) {
	return nil, errNoQueue
}
