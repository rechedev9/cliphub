package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// A series is a best of 3 or best of 5: one job per map for the same player,
// kept in the order the maps were played. Nothing is stored; the order is the
// order of the ids the assistant passes.
const (
	minSeriesMaps = 2
	maxSeriesMaps = 5
	orderPlay     = "play"
	orderBest     = "best"
)

type createSeriesIn struct {
	DemoIDs []string `json:"demo_ids" jsonschema:"library demo ids of the series in the order the maps were played, map 1 first; 2 to 5 demos"`
	SteamID string   `json:"steamid" jsonschema:"SteamID64 of the player from get_match; it is looked up in every map, since the name can change between maps"`
}

type seriesMapOut struct {
	MapNumber int    `json:"map_number"`
	Map       string `json:"map"`
	DemoID    string `json:"demo_id,omitempty"`
	MatchID   string `json:"match_id,omitempty"`
	Player    string `json:"player" jsonschema:"untrusted player name in this map"`
	JobID     string `json:"job_id"`
	Status    string `json:"status,omitempty"`
	Running   bool   `json:"running,omitempty"`
	Moments   int    `json:"moments,omitempty"`
}

type createSeriesOut struct {
	Maps   []seriesMapOut `json:"maps"`
	JobIDs []string       `json:"job_ids" jsonschema:"pass to list_series_moments in this order once wait_for_job reports each one parsed"`
}

type seriesMomentsIn struct {
	JobIDs []string `json:"job_ids" jsonschema:"job ids from create_series_jobs, map 1 first"`
	Order  string   `json:"order,omitempty" jsonschema:"play (default): map 1 to the last map, each in round order; best: highest score first across every map"`
	Limit  int      `json:"limit,omitempty" jsonschema:"how many plays to return; default all of them"`
}

type seriesMomentOut struct {
	MapNumber int    `json:"map_number"`
	Map       string `json:"map"`
	JobID     string `json:"job_id" jsonschema:"the job of this map: generate_short takes this job_id with segment ids of this map only"`
	momentOut
}

type seriesMomentsOut struct {
	Maps    []seriesMapOut    `json:"maps"`
	Moments []seriesMomentOut `json:"moments"`
	Total   int               `json:"total" jsonschema:"plays in the whole series"`
}

// seriesMap is one parsed map of a series with its scored plays.
type seriesMap struct {
	Number  int
	JobID   string
	Moments []tuiclient.Moment
}

func registerSeriesTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "create_series_jobs",
		Description: "Start a series (best of 3 or best of 5) for one player: give the imported demos of every map in the order they were played and the player's SteamID64. " +
			"Checks that the player is in every map, matching by SteamID, then creates or reuses one job per map. Follow with wait_for_job on each job until \"parsed\", then list_series_moments."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createSeriesIn) (*mcp.CallToolResult, createSeriesOut, error) {
			if err := checkSeriesIDs("demo_ids", in.DemoIDs); err != nil {
				return nil, createSeriesOut{}, err
			}
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, createSeriesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, createSeriesOut{}, toolError(err)
			}
			// Check every map before creating any job, so a wrong demo leaves nothing behind.
			maps := make([]seriesMapOut, 0, len(in.DemoIDs))
			for i, demoID := range in.DemoIDs {
				m, err := seriesMatch(ctx, c, demoID, i+1)
				if err != nil {
					return nil, createSeriesOut{}, err
				}
				p, ok := seriesPlayer(m, in.SteamID)
				if !ok {
					return nil, createSeriesOut{}, fmt.Errorf("SteamID %s did not play map %d (%s); check that the demos are the same series and in play order", in.SteamID, i+1, cleanName(m.Map))
				}
				maps = append(maps, seriesMapOut{MapNumber: i + 1, Map: cleanName(m.Map), DemoID: demoID, MatchID: m.ID, Player: cleanName(p.Name)})
			}
			out := createSeriesOut{Maps: maps, JobIDs: make([]string, 0, len(maps))}
			for i := range out.Maps {
				res, err := c.CreatePlayerJob(ctx, out.Maps[i].DemoID, in.SteamID)
				if err != nil {
					return nil, createSeriesOut{}, fmt.Errorf("map %d: %w", i+1, toolError(err))
				}
				out.Maps[i].JobID, out.Maps[i].Status, out.Maps[i].Running = res.JobID, cleanName(res.Status), workerOwnsJob(res.Status)
				out.JobIDs = append(out.JobIDs, res.JobID)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_series_moments", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the plays of a whole series, one job per map from create_series_jobs. By default the plays come in play order: map 1 round by round, then map 2, and so on. " +
			"Each play carries map_number, map and the job_id of its map. generate_short records one map at a time: call it once per map with that map's job_id and segment ids; a single Short that mixes maps is not supported."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in seriesMomentsIn) (*mcp.CallToolResult, seriesMomentsOut, error) {
			if err := checkSeriesIDs("job_ids", in.JobIDs); err != nil {
				return nil, seriesMomentsOut{}, err
			}
			order := in.Order
			if order == "" {
				order = orderPlay
			}
			if order != orderPlay && order != orderBest {
				return nil, seriesMomentsOut{}, fmt.Errorf("order must be %q or %q", orderPlay, orderBest)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, seriesMomentsOut{}, toolError(err)
			}
			jobs := make([]tuiclient.Job, 0, len(in.JobIDs))
			for i, id := range in.JobIDs {
				job, err := c.GetJob(ctx, id)
				if err != nil {
					return nil, seriesMomentsOut{}, fmt.Errorf("map %d: %w", i+1, toolError(err))
				}
				jobs = append(jobs, job)
			}
			if err := checkSeriesJobs(jobs); err != nil {
				return nil, seriesMomentsOut{}, err
			}
			out := seriesMomentsOut{Maps: make([]seriesMapOut, 0, len(jobs))}
			maps := make([]seriesMap, 0, len(jobs))
			for i, job := range jobs {
				doc, err := c.GetMoments(ctx, job.ID)
				if err != nil {
					return nil, seriesMomentsOut{}, fmt.Errorf("map %d is not ready (%s); wait_for_job until parsed: %w", i+1, cleanName(job.Status), toolError(err))
				}
				maps = append(maps, seriesMap{Number: i + 1, JobID: job.ID, Moments: doc.Moments})
				out.Maps = append(out.Maps, describeSeriesMap(i+1, job, len(doc.Moments)))
			}
			out.Moments = concatSeries(maps, order)
			out.Total = len(out.Moments)
			if in.Limit > 0 && in.Limit < len(out.Moments) {
				out.Moments = out.Moments[:in.Limit]
			}
			return nil, out, nil
		})
}

// seriesMatch reads the analysed match of one map's demo.
func seriesMatch(ctx context.Context, c *tuiclient.Client, demoID string, number int) (tuiclient.LibraryMatch, error) {
	d, err := c.GetLibraryDemo(ctx, demoID)
	if err != nil {
		return tuiclient.LibraryMatch{}, fmt.Errorf("map %d: %w", number, toolError(err))
	}
	if d.State != "ready" || d.MatchID == nil {
		return tuiclient.LibraryMatch{}, fmt.Errorf("map %d is %s, not ready; wait_for_demo first", number, cleanName(d.State))
	}
	m, err := c.GetLibraryMatch(ctx, *d.MatchID)
	if err != nil {
		return tuiclient.LibraryMatch{}, fmt.Errorf("map %d: %w", number, toolError(err))
	}
	return m, nil
}

// seriesPlayer finds the player in one map by SteamID: names change between
// maps and two players can share one, so the name is never the key.
func seriesPlayer(m tuiclient.LibraryMatch, steamID string) (tuiclient.LibraryMatchPlayer, bool) {
	for _, p := range m.Players {
		if p.SteamID64 == steamID {
			return p, true
		}
	}
	return tuiclient.LibraryMatchPlayer{}, false
}

// checkSeriesJobs refuses jobs of different players or the same demo twice.
func checkSeriesJobs(jobs []tuiclient.Job) error {
	if len(jobs) == 0 {
		return errors.New("a series needs at least one job")
	}
	player := jobs[0].TargetSteamID
	demos := make(map[string]int, len(jobs))
	for i, job := range jobs {
		if job.TargetSteamID != player {
			return fmt.Errorf("map %d follows another player (SteamID %s, map 1 follows %s); every map of a series needs a job for the same SteamID", i+1, cleanName(job.TargetSteamID), cleanName(player))
		}
		if first, ok := demos[job.DemoSHA256]; ok && job.DemoSHA256 != "" {
			return fmt.Errorf("maps %d and %d are the same demo", first, i+1)
		}
		demos[job.DemoSHA256] = i + 1
	}
	return nil
}

func describeSeriesMap(number int, job tuiclient.Job, moments int) seriesMapOut {
	out := seriesMapOut{MapNumber: number, JobID: job.ID, Status: cleanName(job.Status), Moments: moments}
	if job.KillPlan != nil {
		out.Map, out.Player = cleanName(job.KillPlan.Demo.Map), cleanName(job.KillPlan.Target.NameInDemo)
	}
	return out
}

// concatSeries joins the plays of every map. Play order is map, round, then
// tick; best order is score, with play order breaking ties.
func concatSeries(maps []seriesMap, order string) []seriesMomentOut {
	type play struct {
		mapNumber int
		jobID     string
		m         tuiclient.Moment
	}
	var plays []play
	for _, sm := range maps {
		for _, m := range sm.Moments {
			plays = append(plays, play{mapNumber: sm.Number, jobID: sm.JobID, m: m})
		}
	}
	slices.SortStableFunc(plays, func(a, b play) int {
		if order == orderBest && a.m.Score != b.m.Score {
			return cmp.Compare(b.m.Score, a.m.Score)
		}
		return cmp.Or(cmp.Compare(a.mapNumber, b.mapNumber), cmp.Compare(a.m.Round, b.m.Round),
			cmp.Compare(a.m.TickStart, b.m.TickStart), cmp.Compare(a.m.SegmentID, b.m.SegmentID))
	})
	out := make([]seriesMomentOut, 0, len(plays))
	for _, p := range plays {
		out = append(out, seriesMomentOut{MapNumber: p.mapNumber, Map: cleanName(p.m.Map), JobID: p.jobID, momentOut: describeMoment(p.m)})
	}
	return out
}

// checkSeriesIDs accepts 2 to 5 distinct ids, in the order the maps were played.
func checkSeriesIDs(field string, ids []string) error {
	if len(ids) < minSeriesMaps || len(ids) > maxSeriesMaps {
		return fmt.Errorf("%s takes %d to %d ids, one per map in play order; for a single map use the one-demo tools", field, minSeriesMaps, maxSeriesMaps)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if err := checkUUID(field, id); err != nil {
			return err
		}
		if seen[id] {
			return fmt.Errorf("%s lists %s twice", field, id)
		}
		seen[id] = true
	}
	return nil
}
