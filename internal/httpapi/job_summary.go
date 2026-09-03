package httpapi

import (
	"encoding/json"
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

// rosterSummaryCache memoizes decoded roster artifacts by job id. A roster is
// written once by the scan worker and never rewritten, so an entry only goes
// stale when the job is deleted, which evicts it. The zero value is ready.
type rosterSummaryCache struct {
	mu      sync.Mutex
	rosters map[uuid.UUID]rosterArtifact
}

func (c *rosterSummaryCache) get(id uuid.UUID) (rosterArtifact, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	roster, ok := c.rosters[id]
	return roster, ok
}

func (c *rosterSummaryCache) put(id uuid.UUID, roster rosterArtifact) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rosters == nil || len(c.rosters) >= rosterCacheMaxEntries {
		c.rosters = make(map[uuid.UUID]rosterArtifact)
	}
	c.rosters[id] = roster
}

func (c *rosterSummaryCache) evict(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rosters, id)
}

// loadRoster returns the job's roster from the cache or the artifact store.
// A missing artifact (scan still running, or a job created with a target and
// never scanned) is ok=false, never an error; only a corrupt or unreadable
// artifact is.
func (c *rosterSummaryCache) loadRoster(store storage.Storage, id uuid.UUID) (rosterArtifact, bool, error) {
	if roster, ok := c.get(id); ok {
		return roster, true, nil
	}
	rc, err := store.Open(artifacts.RosterKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return rosterArtifact{}, false, nil
		}
		return rosterArtifact{}, false, err
	}
	defer rc.Close()
	var roster rosterArtifact
	if err := json.NewDecoder(rc).Decode(&roster); err != nil {
		return rosterArtifact{}, false, err
	}
	c.put(id, roster)
	return roster, true, nil
}

// summarize pairs a job with its roster summary. A job without a readable
// roster lists without one, exactly as the client used to tolerate a 409.
func (c *rosterSummaryCache) summarize(store storage.Storage, j job.Job) jobListItem {
	item := jobListItem{Job: j}
	roster, ok, err := c.loadRoster(store, j.ID)
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
