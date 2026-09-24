package telemetryalert

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// retention mirrors the collector's 30-day policy for everything that can
// hold a support code or filtered message text.
const retention = 30 * 24 * time.Hour

var schema = []string{
	"PRAGMA journal_mode=WAL",
	"PRAGMA busy_timeout=5000",
	`CREATE TABLE IF NOT EXISTS cursors (name TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS issues (
		key TEXT PRIMARY KEY, labels TEXT NOT NULL, code TEXT NOT NULL, substage TEXT NOT NULL,
		signature TEXT NOT NULL, first_seen_at INTEGER NOT NULL, first_release TEXT NOT NULL,
		last_seen_at INTEGER NOT NULL, last_release TEXT NOT NULL,
		count_30d INTEGER NOT NULL DEFAULT 0, installs INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved','ignored')),
		resolved_in_release TEXT NOT NULL DEFAULT '', sample_message TEXT,
		crash INTEGER NOT NULL DEFAULT 0, acked INTEGER NOT NULL DEFAULT 0,
		silenced_until INTEGER NOT NULL DEFAULT 0, notified_at INTEGER NOT NULL DEFAULT 0,
		notified_through INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS occurrences (
		id TEXT PRIMARY KEY, key TEXT NOT NULL, at INTEGER NOT NULL, support_code TEXT NOT NULL,
		session_id TEXT NOT NULL, job_id TEXT NOT NULL, release TEXT NOT NULL,
		crash INTEGER NOT NULL, message TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS occurrences_key_at ON occurrences(key, at)`,
	`CREATE INDEX IF NOT EXISTS occurrences_at ON occurrences(at)`,
	`CREATE TABLE IF NOT EXISTS daily (
		day TEXT NOT NULL, release TEXT NOT NULL, operation TEXT NOT NULL, outcome TEXT NOT NULL,
		count INTEGER NOT NULL, PRIMARY KEY (day, release, operation, outcome))`,
	`CREATE TABLE IF NOT EXISTS attempts (
		id TEXT PRIMARY KEY, at INTEGER NOT NULL, support_code TEXT NOT NULL, session_id TEXT NOT NULL,
		job_id TEXT NOT NULL, operation TEXT NOT NULL, release TEXT NOT NULL, outcome TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS attempts_install ON attempts(support_code, operation, release, at)`,
	`CREATE TABLE IF NOT EXISTS jobs (
		job_id TEXT PRIMARY KEY, support_code TEXT NOT NULL, session_id TEXT NOT NULL,
		operation TEXT NOT NULL DEFAULT '', release TEXT NOT NULL,
		toolchain TEXT NOT NULL DEFAULT '', profile TEXT NOT NULL DEFAULT '', profile_at INTEGER NOT NULL DEFAULT 0,
		quality TEXT NOT NULL DEFAULT '', heartbeat_at INTEGER NOT NULL DEFAULT 0,
		heartbeat_ms INTEGER NOT NULL DEFAULT 0, finished_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY, support_code TEXT NOT NULL, device TEXT NOT NULL, at INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS reports (
		id TEXT PRIMARY KEY, at INTEGER NOT NULL, support_code TEXT NOT NULL, job_id TEXT NOT NULL,
		category TEXT NOT NULL, release TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS sent (
		id INTEGER PRIMARY KEY, rule TEXT NOT NULL, dedup_key TEXT NOT NULL,
		priority INTEGER NOT NULL, at INTEGER NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS sent_rule_key ON sent(rule, dedup_key, at)`,
	`CREATE TABLE IF NOT EXISTS install_alias (
		support_code TEXT PRIMARY KEY, alias INTEGER NOT NULL UNIQUE,
		first_seen_at INTEGER NOT NULL, last_seen_at INTEGER NOT NULL)`,
	// Alerts are queued here inside the run transaction and delivered after
	// commit, so a Telegram outage delays an alert instead of losing it.
	`CREATE TABLE IF NOT EXISTS outbox (id INTEGER PRIMARY KEY, created_at INTEGER NOT NULL, alert TEXT NOT NULL)`,
}

func openState(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range schema {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize alert state: %w", err)
		}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

type querier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type state struct{ q querier }

func ms(t time.Time) int64 { return t.UnixMilli() }

func fromMS(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func (s state) cursor(ctx context.Context, name string) (string, error) {
	var value string
	err := s.q.QueryRowContext(ctx, "SELECT value FROM cursors WHERE name=?", name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s state) setCursor(ctx context.Context, name, value string) error {
	_, err := s.q.ExecContext(ctx, "INSERT INTO cursors(name,value) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET value=excluded.value", name, value)
	return err
}

// alias returns the install's stable "#n" number, assigning the next one on
// first sight. Numbers are never reused, even after pruning.
func (s state) alias(ctx context.Context, supportCode string, at time.Time) (int, error) {
	if supportCode == "" {
		return 0, nil
	}
	var alias int
	err := s.q.QueryRowContext(ctx, "SELECT alias FROM install_alias WHERE support_code=?", supportCode).Scan(&alias)
	if err == nil {
		_, err = s.q.ExecContext(ctx, "UPDATE install_alias SET last_seen_at=MAX(last_seen_at,?), first_seen_at=MIN(first_seen_at,?) WHERE support_code=?", ms(at), ms(at), supportCode)
		return alias, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	next, err := s.cursor(ctx, "next_alias")
	if err != nil {
		return 0, err
	}
	alias, _ = strconv.Atoi(next)
	if alias < 1 {
		alias = 1
	}
	if _, err = s.q.ExecContext(ctx, "INSERT INTO install_alias(support_code,alias,first_seen_at,last_seen_at) VALUES(?,?,?,?)", supportCode, alias, ms(at), ms(at)); err != nil {
		return 0, err
	}
	return alias, s.setCursor(ctx, "next_alias", strconv.Itoa(alias+1))
}

// touchJob creates the job row on first sight and keeps its identity current.
func (s state) touchJob(ctx context.Context, r Record) error {
	_, err := s.q.ExecContext(ctx, `INSERT INTO jobs(job_id,support_code,session_id,operation,release,updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(job_id) DO UPDATE SET updated_at=MAX(jobs.updated_at,excluded.updated_at),
		operation=CASE WHEN excluded.operation<>'' THEN excluded.operation ELSE jobs.operation END,
		release=excluded.release, session_id=excluded.session_id`,
		r.Job, r.Install, r.Session, r.Operation, r.Release, ms(r.At))
	return err
}

// setJob stores one context column (toolchain, profile, quality).
func (s state) setJob(ctx context.Context, r Record, column, value string) error {
	if err := s.touchJob(ctx, r); err != nil {
		return err
	}
	extra := ""
	if column == "profile" {
		extra = ", profile_at=?"
	}
	args := []any{value}
	if extra != "" {
		args = append(args, ms(r.At))
	}
	args = append(args, r.Job)
	_, err := s.q.ExecContext(ctx, "UPDATE jobs SET "+column+"=?"+extra+" WHERE job_id=?", args...)
	return err
}

func (s state) setHeartbeat(ctx context.Context, r Record) error {
	if err := s.touchJob(ctx, r); err != nil {
		return err
	}
	_, err := s.q.ExecContext(ctx, "UPDATE jobs SET heartbeat_at=?, heartbeat_ms=MAX(heartbeat_ms,?) WHERE job_id=?", ms(r.At), r.DurationMS, r.Job)
	return err
}

func (s state) setSession(ctx context.Context, r Record, device string) error {
	_, err := s.q.ExecContext(ctx, `INSERT INTO sessions(session_id,support_code,device,at) VALUES(?,?,?,?)
		ON CONFLICT(session_id) DO UPDATE SET device=excluded.device, at=excluded.at`, r.Session, r.Install, device, ms(r.At))
	return err
}

// jobContext joins the job's toolchain/profile/quality and its session's
// device context; every value is parsed from key=value enums.
func (s state) jobContext(ctx context.Context, job, session string) (JobContext, error) {
	out := JobContext{}
	if job != "" {
		var toolchain, profile, quality, jobSession string
		err := s.q.QueryRowContext(ctx, "SELECT operation,toolchain,profile,quality,session_id FROM jobs WHERE job_id=?", job).
			Scan(&out.Operation, &toolchain, &profile, &quality, &jobSession)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return out, err
		}
		out.Toolchain, out.Profile, out.Quality = parseKV(toolchain), parseKV(profile), parseKV(quality)
		if session == "" {
			session = jobSession
		}
	}
	if session != "" {
		var device string
		err := s.q.QueryRowContext(ctx, "SELECT device FROM sessions WHERE session_id=?", session).Scan(&device)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return out, err
		}
		out.Device = parseKV(device)
	}
	return out, nil
}

// insertAttempt stores an attempt.finished once and counts it in daily.
func (s state) insertAttempt(ctx context.Context, r Record) (bool, error) {
	result, err := s.q.ExecContext(ctx, "INSERT OR IGNORE INTO attempts(id,at,support_code,session_id,job_id,operation,release,outcome) VALUES(?,?,?,?,?,?,?,?)",
		r.ID, ms(r.At), r.Install, r.Session, r.Job, r.Operation, r.Release, r.Outcome)
	if err != nil {
		return false, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return false, nil
	}
	_, err = s.q.ExecContext(ctx, `INSERT INTO daily(day,release,operation,outcome,count) VALUES(?,?,?,?,1)
		ON CONFLICT(day,release,operation,outcome) DO UPDATE SET count=count+1`,
		r.At.UTC().Format("2006-01-02"), r.Release, r.Operation, r.Outcome)
	if err == nil && r.Job != "" {
		if err = s.touchJob(ctx, r); err == nil {
			_, err = s.q.ExecContext(ctx, "UPDATE jobs SET finished_at=? WHERE job_id=?", ms(r.At), r.Job)
		}
	}
	return true, err
}

type attemptRow struct {
	At        time.Time
	Release   string
	Job       string
	Outcome   string
	Toolchain map[string]string
}

func (s state) attempts(ctx context.Context, where string, args ...any) ([]attemptRow, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT a.at,a.release,a.job_id,a.outcome,COALESCE(j.toolchain,'')
		FROM attempts a LEFT JOIN jobs j ON j.job_id=a.job_id WHERE `+where+` ORDER BY a.at DESC, a.rowid DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []attemptRow
	for rows.Next() {
		var row attemptRow
		var at int64
		var toolchain string
		if err := rows.Scan(&at, &row.Release, &row.Job, &row.Outcome, &toolchain); err != nil {
			return nil, err
		}
		row.At, row.Toolchain = fromMS(at), parseKV(toolchain)
		out = append(out, row)
	}
	return out, rows.Err()
}

// Issue is one row of the issues table.
type Issue struct {
	Key, Code, Substage, Signature  string
	Labels                          Labels
	FirstSeen, LastSeen             time.Time
	FirstRelease, LastRelease       string
	Count, Installs                 int
	Status, ResolvedIn              string
	SampleMessage                   string
	Crash, Acked                    bool
	SilencedUntil, NotifiedAt, Thru time.Time
}

// Regressed is true for an open issue that was resolved before.
func (i Issue) Regressed() bool { return i.Status == "open" && i.ResolvedIn != "" }

const issueColumns = `key,labels,code,substage,signature,first_seen_at,first_release,last_seen_at,last_release,
	count_30d,installs,status,resolved_in_release,COALESCE(sample_message,''),crash,acked,silenced_until,notified_at,notified_through`

func scanIssue(scan func(...any) error) (Issue, error) {
	var i Issue
	var labels string
	var first, last, silenced, notified, through int64
	err := scan(&i.Key, &labels, &i.Code, &i.Substage, &i.Signature, &first, &i.FirstRelease, &last, &i.LastRelease,
		&i.Count, &i.Installs, &i.Status, &i.ResolvedIn, &i.SampleMessage, &i.Crash, &i.Acked, &silenced, &notified, &through)
	i.Labels, i.FirstSeen, i.LastSeen = parseLabels(labels), fromMS(first), fromMS(last)
	i.SilencedUntil, i.NotifiedAt, i.Thru = fromMS(silenced), fromMS(notified), fromMS(through)
	return i, err
}

func (s state) issue(ctx context.Context, key string) (*Issue, error) {
	i, err := scanIssue(s.q.QueryRowContext(ctx, "SELECT "+issueColumns+" FROM issues WHERE key=?", key).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s state) issues(ctx context.Context, where string, args ...any) ([]Issue, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT "+issueColumns+" FROM issues WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		i, err := scanIssue(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s state) insertIssue(ctx context.Context, o Occurrence, notified bool, now time.Time) error {
	var notifiedAt, through int64
	if notified {
		notifiedAt, through = ms(now), ms(o.At)
	}
	_, err := s.q.ExecContext(ctx, `INSERT INTO issues(key,labels,code,substage,signature,first_seen_at,first_release,
		last_seen_at,last_release,count_30d,installs,sample_message,crash,notified_at,notified_through)
		VALUES(?,?,?,?,?,?,?,?,?,1,1,?,?,?,?)`,
		o.Key, o.Labels.String(), o.Code, o.Substage, o.Signature, ms(o.At), o.Release, ms(o.At), o.Release,
		o.Message, o.Crash, notifiedAt, through)
	return err
}

// seeIssue records a later occurrence; regressed reopens the issue.
func (s state) seeIssue(ctx context.Context, o Occurrence, regressed bool, now time.Time) error {
	_, err := s.q.ExecContext(ctx, `UPDATE issues SET last_seen_at=MAX(last_seen_at,?),
		last_release=CASE WHEN ?>=last_seen_at THEN ? ELSE last_release END, sample_message=?, count_30d=count_30d+1 WHERE key=?`,
		ms(o.At), ms(o.At), o.Release, o.Message, o.Key)
	if err == nil && regressed {
		_, err = s.q.ExecContext(ctx, "UPDATE issues SET status='open', acked=0, silenced_until=0, notified_at=?, notified_through=? WHERE key=?", ms(now), ms(o.At), o.Key)
	}
	return err
}

// duplicateInJob merges the event and log copies of one job failure.
func (s state) duplicateInJob(ctx context.Context, key, job string, at time.Time) (bool, error) {
	if job == "" {
		return false, nil
	}
	var found int
	err := s.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM occurrences WHERE key=? AND job_id=? AND at BETWEEN ? AND ?",
		key, job, ms(at.Add(-10*time.Minute)), ms(at.Add(10*time.Minute))).Scan(&found)
	return found > 0, err
}

func (s state) insertOccurrence(ctx context.Context, o Occurrence, r Record) (bool, error) {
	result, err := s.q.ExecContext(ctx, "INSERT OR IGNORE INTO occurrences(id,key,at,support_code,session_id,job_id,release,crash,message) VALUES(?,?,?,?,?,?,?,?,?)",
		r.ID, o.Key, ms(o.At), r.Install, r.Session, r.Job, r.Release, o.Crash, r.Message)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n == 1, nil
}

// pendingRepeats counts non-crash occurrences after the issue's last message,
// per install alias.
func (s state) pendingRepeats(ctx context.Context, i Issue) (map[int]int, time.Time, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT COALESCE(a.alias,0), COUNT(*), MAX(o.at) FROM occurrences o
		LEFT JOIN install_alias a ON a.support_code=o.support_code
		WHERE o.key=? AND o.at>? AND o.crash=0 GROUP BY 1`, i.Key, ms(i.Thru))
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	counts := map[int]int{}
	var latest int64
	for rows.Next() {
		var alias, count int
		var at int64
		if err := rows.Scan(&alias, &count, &at); err != nil {
			return nil, time.Time{}, err
		}
		counts[alias] = count
		latest = max(latest, at)
	}
	return counts, fromMS(latest), rows.Err()
}

func (s state) markNotified(ctx context.Context, key string, now, through time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE issues SET notified_at=?, notified_through=MAX(notified_through,?) WHERE key=?", ms(now), ms(through), key)
	return err
}

func (s state) lastSent(ctx context.Context, rule, key string) (time.Time, error) {
	var at sql.NullInt64
	err := s.q.QueryRowContext(ctx, "SELECT MAX(at) FROM sent WHERE rule=? AND dedup_key=?", rule, key).Scan(&at)
	return fromMS(at.Int64), err
}

func (s state) countSent(ctx context.Context, priority int, since time.Time, exceptRule string) (int, error) {
	var n int
	err := s.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM sent WHERE priority=? AND at>=? AND rule<>?", priority, ms(since), exceptRule).Scan(&n)
	return n, err
}

func (s state) recordSent(ctx context.Context, rule, key string, priority int, now time.Time) error {
	_, err := s.q.ExecContext(ctx, "INSERT INTO sent(rule,dedup_key,priority,at) VALUES(?,?,?,?)", rule, key, priority, ms(now))
	return err
}

// prune applies the 30-day policy. Issue rows stay as labels, key, releases
// and counts; their sample message is dropped.
func (s state) prune(ctx context.Context, now time.Time) error {
	cutoff := ms(now.Add(-retention))
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{"DELETE FROM occurrences WHERE at<?", []any{cutoff}},
		{"DELETE FROM attempts WHERE at<?", []any{cutoff}},
		{"DELETE FROM jobs WHERE updated_at<?", []any{cutoff}},
		{"DELETE FROM sessions WHERE at<?", []any{cutoff}},
		{"DELETE FROM reports WHERE at<?", []any{cutoff}},
		{"DELETE FROM sent WHERE at<?", []any{cutoff}},
		{"DELETE FROM install_alias WHERE last_seen_at<?", []any{cutoff}},
		{"DELETE FROM daily WHERE day<?", []any{now.Add(-retention).UTC().Format("2006-01-02")}},
		{"UPDATE issues SET sample_message=NULL WHERE last_seen_at<?", []any{cutoff}},
		{`UPDATE issues SET count_30d=(SELECT COUNT(*) FROM occurrences o WHERE o.key=issues.key),
			installs=(SELECT COUNT(DISTINCT support_code) FROM occurrences o WHERE o.key=issues.key)`, nil},
	} {
		if _, err := s.q.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return fmt.Errorf("prune: %w", err)
		}
	}
	return nil
}

// parseKV reads the first line of a "key=value key=value" record message.
func parseKV(message string) map[string]string {
	line, _, _ := strings.Cut(message, "\n")
	out := map[string]string{}
	for _, field := range strings.Fields(line) {
		if key, value, ok := strings.Cut(field, "="); ok && key != "" {
			out[key] = value
		}
	}
	return out
}
