package telemetryalert

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Options are the per-invocation switches and test seams of Run.
type Options struct {
	// Bootstrap ingests the retained 30 days and marks every key as known
	// without notifying. It is implied until a bootstrap has caught up.
	Bootstrap bool
	// DryRun prints the alerts and rolls back every state change.
	DryRun   bool
	Now      func() time.Time
	HTTP     *http.Client
	Notifier Notifier
	Out      io.Writer
	Logf     func(string, ...any)
}

// Result summarizes one run.
type Result struct {
	Bootstrap bool
	Alerts    []Alert
	Delivered int
	Failures  []string
}

const (
	normalPages    = 20
	bootstrapPages = 200
	outboxMaxAge   = 24 * time.Hour
	maxExcerpt     = 300
)

// Run executes one alerter pass under a state-dir lock and a 45 s budget.
func Run(ctx context.Context, cfg Config, opts Options) (Result, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: 10 * time.Second}
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Logf == nil {
		opts.Logf = log.Printf
	}
	if opts.Notifier == nil {
		if opts.DryRun {
			opts.Notifier = writerNotifier{opts.Out}
		} else if cfg.TelegramToken != "" {
			opts.Notifier = Telegram{API: cfg.TelegramAPI, Token: cfg.TelegramToken, ChatID: cfg.TelegramChat, Client: opts.HTTP}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, runBudget)
	defer cancel()
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		return Result{}, err
	}
	unlock, err := lockStateDir(cfg.StateDir)
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	db, err := openState(ctx, filepath.Join(cfg.StateDir, "alert.db"))
	if err != nil {
		return Result{}, err
	}
	defer db.Close()
	e := &engine{cfg: cfg, opts: opts, now: opts.Now().UTC(), db: db, src: source{cfg.AdminURL, cfg.AdminToken, opts.HTTP}}
	if err := e.collect(ctx); err != nil {
		return e.result, err
	}
	if opts.DryRun {
		for _, alert := range e.result.Alerts {
			if err := opts.Notifier.Send(ctx, alert); err != nil {
				return e.result, err
			}
		}
		return e.result, nil
	}
	e.deliver(ctx)
	if err := renderReport(ctx, state{db}, cfg, e.health, e.now); err != nil {
		e.fail("report", err)
	}
	if err := PingDeadman(ctx, opts.HTTP, cfg.DeadmanURL, e.result.Failures); err != nil {
		opts.Logf("telemetry-alert stage=deadman class=failed error=%v", err)
	}
	return e.result, nil
}

type engine struct {
	cfg       Config
	opts      Options
	now       time.Time
	db        *sql.DB
	st        state
	src       source
	bootstrap bool
	latest    string // newest release seen; Resolver uses it
	health    *Health
	result    Result
}

// fail records a label-only reason for the dead-man /fail ping.
func (e *engine) fail(label string, err error) {
	e.opts.Logf("telemetry-alert stage=%s class=failed error=%v", label, err)
	e.result.Failures = append(e.result.Failures, label)
}

func (e *engine) collect(ctx context.Context) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	e.st = state{tx}
	flag, err := e.st.cursor(ctx, "bootstrapped")
	if err != nil {
		return err
	}
	e.bootstrap = e.opts.Bootstrap || flag != "1" && !e.opts.DryRun
	e.result.Bootstrap = e.bootstrap
	if e.opts.Bootstrap {
		for _, name := range []string{"errors", "logs"} {
			if err := e.st.setCursor(ctx, name, ""); err != nil {
				return err
			}
		}
	}
	if e.latest, err = e.st.cursor(ctx, "latest_release"); err != nil {
		return err
	}
	if !e.bootstrap && !e.opts.DryRun {
		if err := e.handleCallbacks(ctx); err != nil {
			return err
		}
	}

	if health, err := e.src.fetchHealth(ctx); err != nil {
		e.fail("healthz", err)
	} else {
		e.health = &health
		if health.DBOK != nil && !*health.DBOK {
			e.result.Failures = append(e.result.Failures, "db_ok=false")
		}
	}
	pages := normalPages
	if e.bootstrap {
		pages = bootstrapPages
	}
	errorCursor, err := e.st.cursor(ctx, "errors")
	if err != nil {
		return err
	}
	events, nextErrors, eventsDone, err := e.src.fetchErrors(ctx, errorCursor, pages)
	switch {
	case errors.Is(err, errNoEventsSource):
		e.opts.Logf("telemetry-alert stage=errors class=unavailable detail=collector_without_v1_errors")
		eventsDone = true
	case err != nil:
		e.fail("errors", err)
	}
	logCursorText, err := e.st.cursor(ctx, "logs")
	if err != nil {
		return err
	}
	logCursor, _ := strconv.ParseInt(logCursorText, 10, 64)
	logs, nextLogs, logsDone, err := e.src.fetchLogs(ctx, logCursor, pages)
	if err != nil {
		e.fail("logs", err)
	}

	records := make([]Record, 0, len(events)+len(logs))
	for _, event := range events {
		records = append(records, recordFromEvent(event))
	}
	for _, record := range logs {
		records = append(records, recordFromLog(record))
	}
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Stream != b.Stream {
			return a.Stream == "event"
		}
		return a.Cursor < b.Cursor
	})
	for _, record := range records {
		if err := e.process(ctx, record); err != nil {
			return fmt.Errorf("process record: %w", err)
		}
	}
	for name, value := range map[string]string{"errors": nextErrors, "logs": strconv.FormatInt(nextLogs, 10), "latest_release": e.latest} {
		if err := e.st.setCursor(ctx, name, value); err != nil {
			return err
		}
	}
	if e.health != nil {
		if err := e.checkHealth(ctx); err != nil {
			return err
		}
	}
	if !e.bootstrap {
		if err := e.repeats(ctx); err != nil {
			return err
		}
		if err := e.digest(ctx); err != nil {
			return err
		}
	} else if eventsDone && logsDone {
		if err := e.finishBootstrap(ctx); err != nil {
			return err
		}
	}
	if err := e.st.prune(ctx, e.now); err != nil {
		return err
	}
	if e.opts.DryRun {
		return nil // the deferred rollback discards every change
	}
	for _, alert := range e.result.Alerts {
		encoded, err := json.Marshal(alert)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO outbox(created_at,alert) VALUES(?,?)", ms(e.now), string(encoded)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (e *engine) process(ctx context.Context, r Record) error {
	alias, err := e.st.alias(ctx, r.Install, r.At)
	if err != nil {
		return err
	}
	if r.Release != "" && compareReleases(r.Release, e.latest) > 0 {
		e.latest = r.Release
	}
	if r.Stream == "log" {
		if handled, err := e.processLog(ctx, r, alias); err != nil || !handled {
			return err
		}
	}
	labels, crash, ok := classify(r)
	if !ok {
		return nil
	}
	occ, err := e.occurrence(ctx, r, labels, crash, alias)
	if err != nil {
		return err
	}
	if r.Event == "attempt.finished" && r.Operation != "" {
		return e.platformBreak(ctx, r, alias, occ)
	}
	return nil
}

// processLog stores job/session context and runs the log-only rules. It
// returns false when the record was already processed (replay).
func (e *engine) processLog(ctx context.Context, r Record, alias int) (bool, error) {
	firstLine, _, _ := strings.Cut(r.Message, "\n")
	switch r.Event {
	case "attempt.toolchain", "render.profile", "delivery.quality":
		if r.Job == "" {
			return true, nil
		}
		column := map[string]string{"attempt.toolchain": "toolchain", "render.profile": "profile", "delivery.quality": "quality"}[r.Event]
		if err := e.st.setJob(ctx, r, column, firstLine); err != nil {
			return false, err
		}
		if r.Event == "delivery.quality" {
			jc, err := e.st.jobContext(ctx, r.Job, r.Session)
			if err != nil {
				return false, err
			}
			if a, ok := evalQuality(parseKV(firstLine), jc, alias, r.Release, r.Job); ok {
				return true, e.emitIgnore(ctx, a)
			}
		}
	case "device.context":
		if r.Session != "" {
			return true, e.st.setSession(ctx, r, firstLine)
		}
	case "attempt.heartbeat":
		if r.Job != "" {
			return true, e.st.setHeartbeat(ctx, r)
		}
	case "attempt.finished":
		if r.Operation == "" || r.Outcome == "" {
			return true, nil
		}
		return e.st.insertAttempt(ctx, r)
	case "user.report":
		result, err := e.st.q.ExecContext(ctx, "INSERT OR IGNORE INTO reports(id,at,support_code,job_id,category,release) VALUES(?,?,?,?,?,?)",
			r.ID, ms(r.At), r.Install, r.Job, parseKV(firstLine)["category"], r.Release)
		if err != nil {
			return false, err
		}
		if n, _ := result.RowsAffected(); n == 0 || r.Job == "" {
			return true, nil
		}
		jc, err := e.st.jobContext(ctx, r.Job, r.Session)
		if err != nil {
			return false, err
		}
		return true, e.emitIgnore(ctx, evalUserReport(parseKV(firstLine)["category"], jc, alias, r.Release, r.Job))
	case "delivery.gap", "delivery.health":
		if a, ok := evalDeliveryLoss(r, alias); ok {
			return true, e.emitIgnore(ctx, a)
		}
	}
	return true, nil
}

func (e *engine) occurrence(ctx context.Context, r Record, labels Labels, crash bool, alias int) (Occurrence, error) {
	occ := newOccurrence(r, labels, crash)
	occ.Alias = alias
	if duplicate, err := e.st.duplicateInJob(ctx, r.Job, r.At); err != nil || duplicate {
		return occ, err
	}
	if inserted, err := e.st.insertOccurrence(ctx, occ, r); err != nil || !inserted {
		return occ, err
	}
	jc, err := e.st.jobContext(ctx, r.Job, r.Session)
	if err != nil {
		return occ, err
	}
	occ.Context = jc
	prior, err := e.st.issue(ctx, occ.Key)
	if err != nil {
		return occ, err
	}
	alerts, regressed := evalIssue(occ, prior, e.now)
	if prior == nil {
		err = e.st.insertIssue(ctx, occ, true, e.now)
	} else {
		err = e.st.seeIssue(ctx, occ, regressed, e.now)
	}
	if err != nil {
		return occ, err
	}
	for _, a := range alerts {
		if err := e.emitIgnore(ctx, a); err != nil {
			return occ, err
		}
	}
	return occ, nil
}

func (e *engine) platformBreak(ctx context.Context, r Record, alias int, occ Occurrence) error {
	recent, err := e.st.attempts(ctx, "a.support_code=? AND a.operation=? AND a.release=?", r.Install, r.Operation, r.Release)
	if err != nil {
		return err
	}
	good, err := e.st.attempts(ctx, "a.support_code=? AND a.operation=? AND a.outcome='ok'", r.Install, r.Operation)
	if err != nil {
		return err
	}
	in := PlatformInput{Alias: alias, Operation: r.Operation, Release: r.Release, Recent: recent, Latest: occ}
	if len(good) > 0 {
		in.LastGood = &good[0]
	}
	if a, ok := evalPlatformBreak(in); ok {
		return e.emitIgnore(ctx, a)
	}
	return nil
}

func (e *engine) emitIgnore(ctx context.Context, a Alert) error {
	_, err := e.emit(ctx, a)
	return err
}

// emit applies the rule's dedup window and the flood guard, then queues the
// alert. It enforces the excerpt policy for every notifier.
func (e *engine) emit(ctx context.Context, a Alert) (bool, error) {
	rule := ruleByName(a.Rule)
	last, err := e.st.lastSent(ctx, a.Rule, a.DedupKey)
	if err != nil {
		return false, err
	}
	if !last.IsZero() && (rule.Cooldown == 0 || e.now.Sub(last) < rule.Cooldown) {
		return false, nil
	}
	if e.bootstrap {
		// Priority 0 keeps bootstrap history out of the flood guard.
		return true, e.st.recordSent(ctx, a.Rule, a.DedupKey, 0, e.now)
	}
	if !e.cfg.IncludeExcerpt {
		a.Excerpt = ""
	} else if utf8.RuneCountInString(a.Excerpt) > maxExcerpt {
		a.Excerpt = string([]rune(a.Excerpt)[:maxExcerpt]) + "…"
	}
	a.Link = e.link(a.IssueKey)
	if a.Priority == 1 && a.Rule != RuleFlood {
		count, err := e.st.countSent(ctx, 1, e.now.Add(-time.Hour), RuleFlood)
		if err != nil {
			return false, err
		}
		if count >= floodLimit {
			flood := newAlert(RuleFlood, "flood", fmt.Sprintf("más de %d alertas P1 en 60 min", floodLimit), "las siguientes de esta hora llegan en silencio")
			if _, err := e.emit(ctx, flood); err != nil {
				return false, err
			}
			a.Silent = true
		}
	}
	if err := e.st.recordSent(ctx, a.Rule, a.DedupKey, a.Priority, e.now); err != nil {
		return false, err
	}
	e.result.Alerts = append(e.result.Alerts, a)
	return true, nil
}

func (e *engine) link(issueKey string) string {
	switch {
	case e.cfg.ReportBase == "":
		return ""
	case issueKey != "":
		return e.cfg.ReportBase + "/issue/" + issueKey + ".html"
	default:
		return e.cfg.ReportBase + "/index.html"
	}
}

func (e *engine) repeats(ctx context.Context) error {
	open, err := e.st.issues(ctx, "status='open' AND acked=0 AND last_seen_at>notified_through")
	if err != nil {
		return err
	}
	for _, issue := range open {
		perAlias, latest, err := e.st.pendingRepeats(ctx, issue)
		if err != nil {
			return err
		}
		lastRepeat, err := e.st.lastSent(ctx, RuleRepeat, issue.Key)
		if err != nil {
			return err
		}
		a, ok := evalRepeat(issue, perAlias, lastRepeat, e.now)
		if !ok {
			continue
		}
		if sent, err := e.emit(ctx, a); err != nil {
			return err
		} else if sent {
			if err := e.st.markNotified(ctx, issue.Key, e.now, latest); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *engine) digest(ctx context.Context) error {
	day := digestDay(e.now)
	if day == "" {
		return nil
	}
	if done, err := e.st.cursor(ctx, "digest_day"); err != nil || done == day {
		return err
	}
	data, err := e.st.loadDigest(ctx, e.now, day)
	if err != nil {
		return err
	}
	if err := e.emitIgnore(ctx, buildDigest(data)); err != nil {
		return err
	}
	return e.st.setCursor(ctx, "digest_day", day)
}

// checkHealth diffs the collector's in-memory rejection counters against the
// previous run. A restart (new started_at) resets the counters, so every
// current value is new; the very first snapshot is only a baseline.
func (e *engine) checkHealth(ctx context.Context) error {
	previousJSON, err := e.st.cursor(ctx, "health_rejections")
	if err != nil {
		return err
	}
	previousStart, err := e.st.cursor(ctx, "health_started_at")
	if err != nil {
		return err
	}
	previous := map[string]int64{}
	switch {
	case previousJSON == "":
		previous = e.health.Rejections
	case previousStart == e.health.StartedAt:
		_ = json.Unmarshal([]byte(previousJSON), &previous)
	}
	for _, a := range evalHealth(*e.health, previous) {
		if err := e.emitIgnore(ctx, a); err != nil {
			return err
		}
	}
	encoded, _ := json.Marshal(e.health.Rejections)
	if err := e.st.setCursor(ctx, "health_rejections", string(encoded)); err != nil {
		return err
	}
	return e.st.setCursor(ctx, "health_started_at", e.health.StartedAt)
}

func (e *engine) finishBootstrap(ctx context.Context) error {
	if _, err := e.st.q.ExecContext(ctx, "UPDATE issues SET notified_through=last_seen_at, notified_at=?", ms(e.now)); err != nil {
		return err
	}
	if day := digestDay(e.now); day != "" {
		if err := e.st.setCursor(ctx, "digest_day", day); err != nil {
			return err
		}
	}
	e.opts.Logf("telemetry-alert stage=bootstrap class=complete")
	return e.st.setCursor(ctx, "bootstrapped", "1")
}

// handleCallbacks applies Ack / Resolver / Silenciar 24h presses from the
// configured chat only.
func (e *engine) handleCallbacks(ctx context.Context) error {
	source, ok := e.opts.Notifier.(callbackSource)
	if !ok {
		return nil
	}
	offsetText, err := e.st.cursor(ctx, "tg_offset")
	if err != nil {
		return err
	}
	offset, _ := strconv.ParseInt(offsetText, 10, 64)
	callbacks, next, err := source.Callbacks(ctx, offset)
	if err != nil {
		e.fail("telegram_updates", err)
		return nil
	}
	for _, callback := range callbacks {
		if callback.ChatID != e.cfg.TelegramChat {
			continue
		}
		reply, err := e.applyCallback(ctx, callback.Data)
		if err != nil {
			return err
		}
		if err := source.Answer(ctx, callback.ID, reply); err != nil {
			e.opts.Logf("telemetry-alert stage=telegram_answer class=failed error=%v", err)
		}
	}
	return e.st.setCursor(ctx, "tg_offset", strconv.FormatInt(next, 10))
}

func (e *engine) applyCallback(ctx context.Context, data string) (string, error) {
	action, key, _ := strings.Cut(data, ":")
	issue, err := e.st.issue(ctx, key)
	if err != nil || issue == nil {
		return "Issue desconocida", err
	}
	switch action {
	case "ack":
		_, err = e.st.q.ExecContext(ctx, "UPDATE issues SET acked=1 WHERE key=?", key)
		return "Ack: sin repeticiones hasta que se reabra", err
	case "res":
		release := e.latest
		if release == "" {
			release = issue.LastRelease
		}
		_, err = e.st.q.ExecContext(ctx, "UPDATE issues SET status='resolved', resolved_in_release=?, acked=0 WHERE key=?", release, key)
		return "Resuelta en " + release, err
	case "sil":
		_, err = e.st.q.ExecContext(ctx, "UPDATE issues SET silenced_until=? WHERE key=?", ms(e.now.Add(24*time.Hour)), key)
		return "Silenciada 24 h", err
	}
	return "Acción desconocida", nil
}

// deliver sends queued alerts in order and stops at the first failure so the
// rest wait for the next run. Alerts older than a day are dropped.
func (e *engine) deliver(ctx context.Context) {
	rows, err := e.db.QueryContext(ctx, "SELECT id, created_at, alert FROM outbox ORDER BY id LIMIT 100")
	if err != nil {
		e.fail("outbox", err)
		return
	}
	type queued struct {
		id, created int64
		alert       Alert
	}
	var pending []queued
	for rows.Next() {
		var q queued
		var encoded string
		if err := rows.Scan(&q.id, &q.created, &encoded); err == nil && json.Unmarshal([]byte(encoded), &q.alert) == nil {
			pending = append(pending, q)
		}
	}
	rows.Close()
	for _, q := range pending {
		if e.now.Sub(fromMS(q.created)) > outboxMaxAge {
			e.opts.Logf("telemetry-alert stage=outbox class=expired rule=%s", q.alert.Rule)
		} else if e.opts.Notifier == nil {
			e.fail("notifier", errors.New("no notifier configured"))
			return
		} else if err := e.opts.Notifier.Send(ctx, q.alert); err != nil {
			e.fail("telegram", err)
			return
		} else {
			e.result.Delivered++
		}
		if _, err := e.db.ExecContext(ctx, "DELETE FROM outbox WHERE id=?", q.id); err != nil {
			e.fail("outbox", err)
			return
		}
	}
}
