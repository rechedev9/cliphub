package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// StorageUsage is GET /api/storage: disk use per job tree.
type StorageUsage struct {
	TotalBytes       int64            `json:"total_bytes"`
	Jobs             []JobStorage     `json:"jobs"`
	CaptureRetention CaptureRetention `json:"capture_retention"`
}

// CaptureRetention is how many rendered Full Demos keep their captures; KeepsAll
// is ZV_KEEP_FULL_DEMO_CAPTURES, which wins over Keep.
type CaptureRetention struct {
	Keep     int  `json:"keep"`
	Max      int  `json:"max"`
	KeepsAll bool `json:"keeps_all,omitempty"`
}

// JobStorage is one job's disk use; Freeable marks kept Full Demo captures a finished render covers.
type JobStorage struct {
	ID           string `json:"id"`
	CaptureBytes int64  `json:"capture_bytes"`
	RenderBytes  int64  `json:"render_bytes"`
	Freeable     bool   `json:"freeable"`
}

// StorageFiles is GET /api/storage/files: Studio's files plus the last PC demo scan.
type StorageFiles struct {
	DataDir     string        `json:"data_dir"`
	Scanning    bool          `json:"scanning"`
	ScannedAt   *time.Time    `json:"scanned_at"`
	Demos       []StorageFile `json:"demos"`
	StudioDemos []StorageFile `json:"studio_demos"`
	Videos      []StorageFile `json:"videos"`
}

// StorageFile is one file on the Almacenamiento screen; Path is absolute.
type StorageFile struct {
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	Kind       string    `json:"kind"`
	JobID      string    `json:"job_id,omitempty"`
}

type freedBytes struct {
	FreedBytes int64 `json:"freed_bytes"`
}

// Storage returns the disk use of every job.
func (c *Client) Storage(ctx context.Context) (StorageUsage, error) {
	var out StorageUsage
	err := c.getJSON(ctx, "/api/storage", &out)
	return out, err
}

// DeleteJobCaptures frees the Full Demo round captures of a job with a finished long video.
func (c *Client) DeleteJobCaptures(ctx context.Context, jobID string) (int64, error) {
	var out freedBytes
	err := c.doJSON(ctx, http.MethodDelete, jobPath(jobID)+"/captures", nil, &out)
	return out.FreedBytes, err
}

// ListStorageFiles lists the files; the first call starts the PC demo scan.
func (c *Client) ListStorageFiles(ctx context.Context) (StorageFiles, error) {
	var out StorageFiles
	err := c.getJSON(ctx, "/api/storage/files", &out)
	return out, err
}

// RescanStorageFiles searches the PC for demos again in the background.
func (c *Client) RescanStorageFiles(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodPost, "/api/storage/files/scan", nil, nil)
}

// DeleteStorageFile removes a file the last list returned, with its render siblings.
func (c *Client) DeleteStorageFile(ctx context.Context, path string) (int64, error) {
	var out freedBytes
	err := c.doJSON(ctx, http.MethodPost, "/api/storage/files/delete", map[string]string{"path": path}, &out)
	return out.FreedBytes, err
}

// RevealStorageFile opens File Explorer on a file the last list returned.
func (c *Client) RevealStorageFile(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/storage/files/reveal", map[string]string{"path": path}, nil)
}

type StorageSpace struct {
	FreeBytes   uint64 `json:"free_bytes"`
	NeededBytes uint64 `json:"needed_bytes"`
	Short       bool   `json:"short"`
}

func (c *Client) StorageSpace(ctx context.Context, captureSeconds, renderSeconds float64) (*StorageSpace, error) {
	q := url.Values{}
	q.Set("capture_seconds", strconv.FormatFloat(captureSeconds, 'f', -1, 64))
	q.Set("render_seconds", strconv.FormatFloat(renderSeconds, 'f', -1, 64))
	var out struct {
		Space *StorageSpace `json:"space"`
	}
	err := c.getJSON(ctx, "/api/storage/space?"+q.Encode(), &out)
	return out.Space, err
}
