package httpapi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/storage"
)

// rosterCacheMaxEntries bounds the roster cache; the cap is far above the 100
// jobs a list can return, so a full reset is the whole eviction policy.
const rosterCacheMaxEntries = 1024

// rosterMatch and rosterPlayer mirror the roster artifact the scan worker
// writes (parser.RosterResult) field for field, so the list can decode it
// without importing the demo parser into the API.
type rosterMatch struct {
	Map        string `json:"map"`
	ScoreCT    int    `json:"score_ct"`
	ScoreT     int    `json:"score_t"`
	Rounds     int    `json:"rounds"`
	ClanNameCT string `json:"clan_name_ct,omitempty"`
	ClanNameT  string `json:"clan_name_t,omitempty"`
}

type rosterPlayer struct {
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name"`
	Team      string  `json:"team"`
	Kills     int     `json:"kills"`
	Deaths    int     `json:"deaths"`
	Assists   int     `json:"assists"`
	Headshots int     `json:"headshots"`
	MVPs      int     `json:"mvps"`
	Rounds    int     `json:"rounds"`
	ADR       float64 `json:"adr"`
	HSPct     float64 `json:"hs_pct"`
	KAST      float64 `json:"kast"`
	Rating    float64 `json:"rating"`
	Rounds2K  int     `json:"rounds_2k,omitempty"`
	Rounds3K  int     `json:"rounds_3k,omitempty"`
	Rounds4K  int     `json:"rounds_4k,omitempty"`
	Rounds5K  int     `json:"rounds_5k,omitempty"`
}

type rosterArtifact struct {
	Players []rosterPlayer `json:"players"`
	Match   rosterMatch    `json:"match"`
}

// jobSummary is the part of a job's roster the Partidas list needs: the
// match line and the target player's scoreboard row. Studio used to fetch
// the whole roster artifact once per listed job on every poll; the list now
// carries it inline so one request feeds the whole page.
type jobSummary struct {
	Match  rosterMatch   `json:"match"`
	Target *rosterPlayer `json:"target,omitempty"`
}

// jobListItem is one GET /api/jobs row: the job document plus its summary.
type jobListItem struct {
	job.Job
	Summary *jobSummary `json:"summary,omitempty"`
}

// rosterCacheEntry is one memoized answer about a job's roster artifact.
// found=false is a real answer ("this job has no roster"), not a placeholder:
// without it every roster-less job in the list paid a failed storage open on
// every poll tick, forever.
type rosterCacheEntry struct {
	roster rosterArtifact
	found  bool
}

// rosterSummaryCache memoizes the roster answer for a job id, present or
// absent. A roster is written once by the scan worker and never rewritten, so
// a positive entry only goes stale when the job is deleted, which evicts it.
//
// A negative entry is bounded by the scan lifecycle instead. The only writer
// of the artifact is workers.ParserWorker.HandleScanRoster, which runs while
// the job is queued/scanning and publishes the artifact before moving the job
// to scanned; a job created with a target skips the scan and never gets one at
// all. So once a job's status is past queued/scanning, "absent" is permanent:
// the scan either already published (and we would have read it), failed, or
// was never enqueued. rosterMayStillAppear encodes that, and it gates both the
// write and the read of a negative entry, so even a status that somehow moved
// back into the scan window re-opens the artifact instead of being served a
// stale absent. The zero value is ready.
type rosterSummaryCache struct {
	mu      sync.Mutex
	rosters map[uuid.UUID]rosterCacheEntry
}

// rosterMayStillAppear reports whether the roster scan could still publish an
// artifact for a job in this status, which is exactly when a miss must not be
// remembered.
func rosterMayStillAppear(s job.Status) bool {
	switch s {
	case job.StatusQueued, job.StatusScanning:
		return true
	default:
		return false
	}
}

func (c *rosterSummaryCache) get(id uuid.UUID) (rosterCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.rosters[id]
	return entry, ok
}

func (c *rosterSummaryCache) put(id uuid.UUID, entry rosterCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rosters == nil || len(c.rosters) >= rosterCacheMaxEntries {
		c.rosters = make(map[uuid.UUID]rosterCacheEntry)
	}
	c.rosters[id] = entry
}

// evict drops the job's memoized answer, present or absent, so the next read
// goes back to the store. It is the single invalidation path for both kinds.
func (c *rosterSummaryCache) evict(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rosters, id)
}

// loadRoster returns the job's roster from the cache or the artifact store.
// A missing artifact (scan still running, or a job created with a target and
// never scanned) is ok=false, never an error; only a corrupt or unreadable
// artifact is. A miss is memoized only once the job is past the scan window,
// so a roster that is still on its way is re-checked on the next read.
func (c *rosterSummaryCache) loadRoster(store storage.Storage, j job.Job) (rosterArtifact, bool, error) {
	settled := !rosterMayStillAppear(j.Status)
	if entry, ok := c.get(j.ID); ok && (entry.found || settled) {
		return entry.roster, entry.found, nil
	}
	rc, err := store.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			if settled {
				c.put(j.ID, rosterCacheEntry{})
			}
			return rosterArtifact{}, false, nil
		}
		return rosterArtifact{}, false, fmt.Errorf("open roster for job %s: %w", j.ID, err)
	}
	defer rc.Close()
	var roster rosterArtifact
	if err := json.NewDecoder(rc).Decode(&roster); err != nil {
		return rosterArtifact{}, false, fmt.Errorf("decode roster for job %s: %w", j.ID, err)
	}
	c.put(j.ID, rosterCacheEntry{roster: roster, found: true})
	return roster, true, nil
}

// summarize pairs a job with its roster summary. A job without a readable
// roster lists without one, exactly as the client used to tolerate a 409.
func (c *rosterSummaryCache) summarize(store storage.Storage, j job.Job) jobListItem {
	item := jobListItem{Job: j}
	roster, ok, err := c.loadRoster(store, j)
	if err != nil || !ok {
		return item
	}
	summary := &jobSummary{Match: roster.Match}
	if j.TargetSteamID != "" {
		for i := range roster.Players {
			if roster.Players[i].SteamID64 == j.TargetSteamID {
				target := roster.Players[i]
				summary.Target = &target
				break
			}
		}
	}
	item.Summary = summary
	return item
}

func (c *rosterSummaryCache) summarizeAll(store storage.Storage, jobs []job.Job) []jobListItem {
	items := make([]jobListItem, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, c.summarize(store, j))
	}
	return items
}
