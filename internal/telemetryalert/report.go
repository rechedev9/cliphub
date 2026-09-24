package telemetryalert

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// The report is static HTML in <state>/www, served tailnet-only by
// `tailscale serve`. It is the only place that shows support codes and
// filtered messages; it has no JavaScript and a CSP that blocks everything
// except inline styles.

//go:embed report.html.tmpl
var reportTemplateText string

var reportTemplate = template.Must(template.New("report").Parse(reportTemplateText))

type keyCount struct {
	Key   string
	Count int64
}

type reportHealth struct {
	Available                  bool
	Status, Version, Started   string
	DBOK                       string
	DBBad                      bool
	LastEvents, LastLogs       string
	Rejections                 []keyCount
	EventsStorage, LogsStorage string
}

type reportIssue struct {
	Key, Label, Code, Labels  string
	Status, StatusClass       string
	FirstRelease, LastRelease string
	FirstSeen, LastSeen       string
	Count, Installs           int
	Signature                 string
	Link                      string
}

type reportOccurrence struct {
	At, Alias, SupportCode, Release, Label, Key, Job, Message string
}

type reportUserReport struct {
	At, Alias, SupportCode, Category, Release, Command string
}

type reportJob struct {
	Job, Release, Command string
}

type overviewPage struct {
	Generated  string
	Health     reportHealth
	Issues     []reportIssue
	ReleaseOps []releaseOpCount
	Errors     []reportOccurrence
	Reports    []reportUserReport
	Changes    []string
	CS2Builds  []string
}

type issuePage struct {
	Generated string
	Issue     reportIssue
	Operation string
	Releases  []keyCount
	Timeline  []reportOccurrence
	Jobs      []reportJob
	LastGood  *reportJob
}

func localTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.In(madrid).Format("2006-01-02 15:04")
}

func debugCommand(job string) string {
	return "node scripts/telemetry-debug.mjs --job " + job
}

func toReportIssue(i Issue) reportIssue {
	o := Occurrence{Labels: i.Labels, Code: i.Code, Substage: i.Substage}
	headline := o.headline("")
	status, class := "abierta", "open"
	switch {
	case i.Regressed():
		status, class = "regresión (resuelta en "+i.ResolvedIn+")", "regressed"
	case i.Status == "resolved":
		status, class = "resuelta en "+i.ResolvedIn, "resolved"
	case i.Status == "ignored":
		status, class = "ignorada", "ignored"
	case i.Acked:
		status = "abierta, ack"
	}
	return reportIssue{Key: i.Key, Label: headline[0], Code: headline[1], Labels: i.Labels.String(), Status: status, StatusClass: class,
		FirstRelease: i.FirstRelease, LastRelease: i.LastRelease, FirstSeen: localTime(i.FirstSeen), LastSeen: localTime(i.LastSeen),
		Count: i.Count, Installs: i.Installs, Signature: i.Signature, Link: "issue/" + i.Key + ".html"}
}

func buildHealth(h *Health) reportHealth {
	if h == nil {
		return reportHealth{}
	}
	out := reportHealth{Available: true, Status: h.Status, Version: h.Version, Started: h.StartedAt, DBOK: "desconocido",
		LastEvents: "—", LastLogs: "—", EventsStorage: "—", LogsStorage: "—"}
	if h.DBOK != nil {
		out.DBOK, out.DBBad = map[bool]string{true: "sí", false: "no"}[*h.DBOK], !*h.DBOK
	}
	if value := h.LastReceivedAt["events"]; value != nil {
		out.LastEvents = *value
	}
	if value := h.LastReceivedAt["logs"]; value != nil {
		out.LastLogs = *value
	}
	if h.EventsStorageRatio != nil {
		out.EventsStorage = fmt.Sprintf("%.0f%%", *h.EventsStorageRatio*100)
	}
	if h.LogsStorageRatio != nil {
		out.LogsStorage = fmt.Sprintf("%.0f%%", *h.LogsStorageRatio*100)
	}
	for key, count := range h.Rejections {
		out.Rejections = append(out.Rejections, keyCount{key, count})
	}
	sort.Slice(out.Rejections, func(i, j int) bool { return out.Rejections[i].Key < out.Rejections[j].Key })
	return out
}

// occurrencesForReport lists counted occurrences; paired copies stay out.
func (s state) occurrencesForReport(ctx context.Context, where string, args ...any) ([]reportOccurrence, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT o.at, COALESCE(a.alias,0), o.support_code, o.release, i.labels, i.substage, o.key, o.job_id, o.message
		FROM occurrences o JOIN issues i ON i.key=o.key LEFT JOIN install_alias a ON a.support_code=o.support_code
		WHERE o.dup=0 AND `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []reportOccurrence
	for rows.Next() {
		var row reportOccurrence
		var at int64
		var alias int
		var labels, substage string
		if err := rows.Scan(&at, &alias, &row.SupportCode, &row.Release, &labels, &substage, &row.Key, &row.Job, &row.Message); err != nil {
			return nil, err
		}
		row.At, row.Alias = localTime(fromMS(at)), aliasLabel(alias)
		row.Label = parseLabels(labels).Short()
		if substage != "" {
			row.Label += "/" + substage
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func loadOverview(ctx context.Context, s state, h *Health, now time.Time) (overviewPage, []Issue, error) {
	page := overviewPage{Generated: localTime(now), Health: buildHealth(h)}
	issues, err := s.issues(ctx, "1=1 ORDER BY status='open' DESC, last_seen_at DESC")
	if err != nil {
		return page, nil, err
	}
	for _, issue := range issues {
		page.Issues = append(page.Issues, toReportIssue(issue))
	}
	rows, err := s.q.QueryContext(ctx, `SELECT release, operation, SUM(CASE WHEN outcome='ok' THEN count ELSE 0 END),
		SUM(CASE WHEN outcome='error' THEN count ELSE 0 END) FROM daily GROUP BY release, operation`)
	if err != nil {
		return page, nil, err
	}
	for rows.Next() {
		var row releaseOpCount
		if err := rows.Scan(&row.Release, &row.Operation, &row.OK, &row.Error); err != nil {
			rows.Close()
			return page, nil, err
		}
		page.ReleaseOps = append(page.ReleaseOps, row)
	}
	rows.Close()
	sortReleaseOps(page.ReleaseOps)
	if page.Errors, err = s.occurrencesForReport(ctx, "1=1 ORDER BY o.at DESC LIMIT 50"); err != nil {
		return page, nil, err
	}
	reportRows, err := s.q.QueryContext(ctx, `SELECT r.at, COALESCE(a.alias,0), r.support_code, r.category, r.release, r.job_id
		FROM reports r LEFT JOIN install_alias a ON a.support_code=r.support_code ORDER BY r.at DESC LIMIT 20`)
	if err != nil {
		return page, nil, err
	}
	for reportRows.Next() {
		var row reportUserReport
		var at int64
		var alias int
		var job string
		if err := reportRows.Scan(&at, &alias, &row.SupportCode, &row.Category, &row.Release, &job); err != nil {
			reportRows.Close()
			return page, nil, err
		}
		row.At, row.Alias, row.Command = localTime(fromMS(at)), aliasLabel(alias), debugCommand(job)
		page.Reports = append(page.Reports, row)
	}
	reportRows.Close()
	profiles, err := s.profileRows(ctx)
	if err != nil {
		return page, nil, err
	}
	page.Changes = profileChanges(profiles, time.Time{})
	page.CS2Builds, err = s.cs2Builds(ctx, now.Add(-retention))
	return page, issues, err
}

func loadIssuePage(ctx context.Context, s state, issue Issue, now time.Time) (issuePage, error) {
	page := issuePage{Generated: localTime(now), Issue: toReportIssue(issue), Operation: taskOperation(issue.Labels.Name, issue.Labels.Class)}
	var err error
	if page.Timeline, err = s.occurrencesForReport(ctx, "o.key=? ORDER BY o.at DESC LIMIT 200", issue.Key); err != nil {
		return page, err
	}
	releases := map[string]int64{}
	seen := map[string]bool{}
	for _, row := range page.Timeline {
		releases[row.Release]++
		if row.Job != "" && !seen[row.Job] {
			seen[row.Job] = true
			page.Jobs = append(page.Jobs, reportJob{Job: row.Job, Release: row.Release, Command: debugCommand(row.Job)})
		}
	}
	for release, count := range releases {
		page.Releases = append(page.Releases, keyCount{release, count})
	}
	sort.Slice(page.Releases, func(i, j int) bool { return compareReleases(page.Releases[i].Key, page.Releases[j].Key) > 0 })
	if page.Operation != "" {
		good, err := s.lastGood(ctx, "a.operation=?", page.Operation)
		if err != nil {
			return page, err
		}
		if good != nil {
			page.LastGood = &reportJob{Job: good.Job, Release: good.Release + " · " + localTime(good.At), Command: debugCommand(good.Job)}
		}
	}
	return page, nil
}

// reportVersion identifies the page template; a new binary with a changed
// template re-renders every issue page once.
var reportVersion = func() string {
	sum := sha256.Sum256([]byte(reportTemplateText))
	return hex.EncodeToString(sum[:8])
}()

// renderReport writes index.html on every run and an issue page only when
// the issue changed since its last render (the dirty flag), when the page is
// missing or when the template changed. Every write is atomic.
func renderReport(ctx context.Context, s state, cfg Config, h *Health, now time.Time) error {
	dir := filepath.Join(cfg.StateDir, "www")
	if err := os.MkdirAll(filepath.Join(dir, "issue"), 0o750); err != nil {
		return err
	}
	overview, issues, err := loadOverview(ctx, s, h, now)
	if err != nil {
		return err
	}
	if err := writePage(filepath.Join(dir, "index.html"), "index", overview); err != nil {
		return err
	}
	rendered, err := s.cursor(ctx, "report_version")
	if err != nil {
		return err
	}
	for _, issue := range issues {
		path := filepath.Join(dir, "issue", issue.Key+".html")
		if !issue.Dirty && rendered == reportVersion {
			if _, err := os.Stat(path); err == nil {
				continue
			}
		}
		page, err := loadIssuePage(ctx, s, issue, now)
		if err != nil {
			return err
		}
		if err := writePage(path, "issue", page); err != nil {
			return err
		}
		if _, err := s.q.ExecContext(ctx, "UPDATE issues SET dirty=0 WHERE key=?", issue.Key); err != nil {
			return err
		}
	}
	if rendered == reportVersion {
		return nil
	}
	return s.setCursor(ctx, "report_version", reportVersion)
}

func writePage(path, name string, data any) error {
	var buf bytes.Buffer
	if err := reportTemplate.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
