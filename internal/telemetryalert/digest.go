package telemetryalert

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // Europe/Madrid must resolve on minimal hosts and in tests.
)

var madrid = mustLocation("Europe/Madrid")

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}

const digestHour = 9

// digestDay returns the Madrid date whose digest is due at now, or "".
func digestDay(now time.Time) string {
	local := now.In(madrid)
	if local.Hour() < digestHour {
		return ""
	}
	return local.Format(time.DateOnly)
}

type releaseOpCount struct {
	Release, Operation string
	OK, Error          int
}

type profileRow struct {
	Release string
	At      time.Time
	Profile map[string]string
}

type stuckJob struct {
	Alias     int
	Operation string
	Job       string
	Hours     float64
}

type digestData struct {
	Day         string
	Attempts    []releaseOpCount
	Open        []Issue
	NewInstalls []int
	Changes     []string
	CS2Builds   []string
	Stuck       []stuckJob
}

func (s state) attemptCounts(ctx context.Context, since time.Time) ([]releaseOpCount, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT release, operation, SUM(outcome='ok'), SUM(outcome='error') FROM attempts
		WHERE at>=? GROUP BY release, operation`, ms(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []releaseOpCount
	for rows.Next() {
		var row releaseOpCount
		if err := rows.Scan(&row.Release, &row.Operation, &row.OK, &row.Error); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	sortReleaseOps(out)
	return out, rows.Err()
}

func sortReleaseOps(rows []releaseOpCount) {
	sort.SliceStable(rows, func(i, j int) bool {
		if c := compareReleases(rows[i].Release, rows[j].Release); c != 0 {
			return c > 0
		}
		return rows[i].Operation < rows[j].Operation
	})
}

func (s state) profileRows(ctx context.Context) ([]profileRow, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT release, profile_at, profile FROM jobs WHERE profile<>'' ORDER BY profile_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []profileRow
	for rows.Next() {
		var row profileRow
		var at int64
		var profile string
		if err := rows.Scan(&row.Release, &at, &profile); err != nil {
			return nil, err
		}
		row.At, row.Profile = fromMS(at), parseKV(profile)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s state) cs2Builds(ctx context.Context, since time.Time) ([]string, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT toolchain FROM jobs WHERE toolchain<>'' AND updated_at>=?", ms(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var builds []string
	for rows.Next() {
		var toolchain string
		if err := rows.Scan(&toolchain); err != nil {
			return nil, err
		}
		if build := parseKV(toolchain)["cs2_build"]; build != "" && !slices.Contains(builds, safe(build)) {
			builds = append(builds, safe(build))
		}
	}
	slices.Sort(builds)
	return builds, rows.Err()
}

func (s state) stuckJobs(ctx context.Context, since time.Time) ([]stuckJob, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT COALESCE(a.alias,0), j.operation, j.job_id, j.heartbeat_ms FROM jobs j
		LEFT JOIN install_alias a ON a.support_code=j.support_code
		WHERE j.heartbeat_at>=? AND j.heartbeat_ms>? AND j.finished_at<j.heartbeat_at ORDER BY j.heartbeat_at`,
		ms(since), stuckJobAfter.Milliseconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stuckJob
	for rows.Next() {
		var job stuckJob
		var heartbeat int64
		if err := rows.Scan(&job.Alias, &job.Operation, &job.Job, &heartbeat); err != nil {
			return nil, err
		}
		job.Hours = float64(heartbeat) / float64(time.Hour.Milliseconds())
		out = append(out, job)
	}
	return out, rows.Err()
}

func (s state) newInstalls(ctx context.Context, since time.Time) ([]int, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT alias FROM install_alias WHERE first_seen_at>=? ORDER BY alias", ms(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var alias int
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		out = append(out, alias)
	}
	return out, rows.Err()
}

func (s state) loadDigest(ctx context.Context, now time.Time, day string) (digestData, error) {
	since := now.Add(-24 * time.Hour)
	d := digestData{Day: day}
	var err error
	if d.Attempts, err = s.attemptCounts(ctx, since); err != nil {
		return d, err
	}
	if d.Open, err = s.issues(ctx, "status='open' ORDER BY last_seen_at DESC"); err != nil {
		return d, err
	}
	if d.NewInstalls, err = s.newInstalls(ctx, since); err != nil {
		return d, err
	}
	profiles, err := s.profileRows(ctx)
	if err != nil {
		return d, err
	}
	d.Changes = profileChanges(profiles, since)
	if d.CS2Builds, err = s.cs2Builds(ctx, since); err != nil {
		return d, err
	}
	d.Stuck, err = s.stuckJobs(ctx, since)
	return d, err
}

var profileFields = []string{"overlay_source", "hud", "encoder", "aac_path", "resolution", "fps"}

// profileChanges compares, per source_kind, the render.profile values of the
// newest release with the previous release that rendered that source. It is
// how a silent format change (FACEIT 3.0.1) reaches the digest without an
// error. Only sources rendered since `since` are reported.
func profileChanges(rows []profileRow, since time.Time) []string {
	bySource := map[string][]profileRow{}
	for _, row := range rows {
		kind := row.Profile["source_kind"]
		if kind != "" {
			bySource[kind] = append(bySource[kind], row)
		}
	}
	kinds := make([]string, 0, len(bySource))
	for kind := range bySource {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	var lines []string
	for _, kind := range kinds {
		group := bySource[kind]
		releases := map[string]time.Time{}
		for _, row := range group {
			if row.At.After(releases[row.Release]) {
				releases[row.Release] = row.At
			}
		}
		ordered := make([]string, 0, len(releases))
		for release := range releases {
			ordered = append(ordered, release)
		}
		sort.Slice(ordered, func(i, j int) bool { return compareReleases(ordered[i], ordered[j]) > 0 })
		if len(ordered) < 2 || releases[ordered[0]].Before(since) {
			continue
		}
		latest, previous := ordered[0], ordered[1]
		for _, field := range profileFields {
			now, before := valuesIn(group, latest, field), valuesIn(group, previous, field)
			if slices.Equal(now, before) || len(now) == 0 {
				continue
			}
			line := fmt.Sprintf("render.profile source_kind=%s ahora %s=%s (%s)", safe(kind), field, strings.Join(now, ","), latest)
			for _, gone := range before {
				if slices.Contains(now, gone) {
					continue
				}
				if last, ok := lastWith(group, field, gone); ok {
					line += fmt.Sprintf("; último con %s=%s: %s %s", field, gone, last.Release, last.At.In(madrid).Format(time.DateOnly))
				}
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func valuesIn(rows []profileRow, release, field string) []string {
	var values []string
	for _, row := range rows {
		if value := row.Profile[field]; row.Release == release && value != "" && !slices.Contains(values, safe(value)) {
			values = append(values, safe(value))
		}
	}
	slices.Sort(values)
	return values
}

func lastWith(rows []profileRow, field, value string) (profileRow, bool) {
	for i := len(rows) - 1; i >= 0; i-- {
		if safe(rows[i].Profile[field]) == value {
			return rows[i], true
		}
	}
	return profileRow{}, false
}

// buildDigest renders the daily digest (silent) from the loaded data.
func buildDigest(d digestData) Alert {
	a := newAlert(RuleDigest, d.Day, d.Day)
	if len(d.Attempts) == 0 {
		a.Lines = append(a.Lines, "intentos 24 h: ninguno")
	} else {
		a.Lines = append(a.Lines, "intentos 24 h:")
		for _, row := range d.Attempts {
			a.Lines = append(a.Lines, fmt.Sprintf("%s %s: %d ok / %d error", safe(row.Operation), row.Release, row.OK, row.Error))
		}
	}
	a.Lines = append(a.Lines, fmt.Sprintf("issues abiertas: %d", len(d.Open)))
	for i, issue := range d.Open {
		if i == 5 {
			a.Lines = append(a.Lines, fmt.Sprintf("… y %d más", len(d.Open)-5))
			break
		}
		o := Occurrence{Labels: issue.Labels, Code: issue.Code, Substage: issue.Substage, Release: issue.LastRelease}
		headline := o.headline("")
		a.Lines = append(a.Lines, fmt.Sprintf("· %s %s ×%d (%s)", headline[0], headline[1], issue.Count, issue.LastRelease))
	}
	if len(d.NewInstalls) > 0 {
		var aliases []string
		for _, alias := range d.NewInstalls {
			aliases = append(aliases, aliasLabel(alias))
		}
		a.Lines = append(a.Lines, "instalaciones nuevas: "+strings.Join(aliases, ", "))
	}
	a.Lines = append(a.Lines, d.Changes...)
	if len(d.CS2Builds) > 0 {
		a.Lines = append(a.Lines, "cs2_build vistos: "+strings.Join(d.CS2Builds, ", "))
	}
	for _, job := range d.Stuck {
		a.Lines = append(a.Lines, fmt.Sprintf("job >3 h sin attempt.finished: %s %s %s (%.1f h)", aliasLabel(job.Alias), safe(job.Operation), jobPrefix(job.Job), job.Hours))
	}
	return a
}
