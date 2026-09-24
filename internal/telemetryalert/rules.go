package telemetryalert

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Rule names, as in the alert rule table of the observability design.
const (
	RuleNewIssue      = "new_issue"
	RuleRegressed     = "regressed"
	RulePlatformBreak = "platform_break"
	RuleCrash         = "crash"
	RuleUserReport    = "user_report"
	RuleQuality       = "output_quality"
	RuleChannelHealth = "channel_health"
	RuleRepeat        = "repeat"
	RuleDeliveryLoss  = "delivery_loss"
	RuleDigest        = "digest"
	RuleFlood         = "flood_guard"
)

// Rule is one row of the declarative rule table. Cooldown 0 means once per
// dedup key. Dead-man A/B live in healthchecks.io, not here.
type Rule struct {
	Name     string
	Priority int // 1 audible, 2 silent
	Cooldown time.Duration
	Title    string
}

var ruleTable = []Rule{
	{RuleNewIssue, 1, 0, "Nuevo"},                     // key: issue
	{RuleRegressed, 1, 0, "Regresión"},                // key: issue + release
	{RulePlatformBreak, 1, 0, "Plataforma rota"},      // key: install + operation + release
	{RuleCrash, 1, 6 * time.Hour, "Crash"},            // key: install + issue
	{RuleUserReport, 1, 10 * time.Minute, "Reporte"},  // key: job
	{RuleQuality, 1, 6 * time.Hour, "Calidad"},        // key: install + preset
	{RuleChannelHealth, 1, 24 * time.Hour, "Canal"},   // key: channel:status:code
	{RuleRepeat, 2, time.Hour, "Repite"},              // key: issue (6 h between repeats, see evalRepeat)
	{RuleDeliveryLoss, 2, 24 * time.Hour, "Entrega"},  // key: install
	{RuleDigest, 2, 0, "Resumen diario"},              // key: Madrid date
	{RuleFlood, 1, time.Hour, "Avalancha de alertas"}, // key: constant
}

func ruleByName(name string) Rule {
	for _, rule := range ruleTable {
		if rule.Name == name {
			return rule
		}
	}
	panic("unknown rule " + name)
}

const (
	floodLimit      = 10
	repeatFirstGap  = time.Hour
	repeatGap       = 6 * time.Hour
	stuckJobAfter   = 3 * time.Hour
	storageHighMark = 0.8
)

// Alert is what a rule produces. Headline and Lines hold only enums, labels,
// releases, counts and aliases; Excerpt is filtered message text that the
// engine drops unless CLIPHUB_ALERT_INCLUDE_EXCERPT is set.
type Alert struct {
	Rule     string   `json:"rule"`
	Priority int      `json:"priority"`
	Silent   bool     `json:"silent,omitempty"`
	DedupKey string   `json:"dedup_key"`
	IssueKey string   `json:"issue_key,omitempty"`
	Headline []string `json:"headline"`
	Lines    []string `json:"lines,omitempty"`
	Excerpt  string   `json:"excerpt,omitempty"`
	Link     string   `json:"link,omitempty"`
}

// Header is the first line: "[P1] Nuevo · render:variant · …".
func (a Alert) Header() string {
	return strings.Join(append([]string{fmt.Sprintf("[P%d] %s", a.Priority, ruleByName(a.Rule).Title)}, a.Headline...), " · ")
}

// Text is the plain rendering used by dry runs and non-HTML notifiers.
func (a Alert) Text() string {
	lines := append([]string{a.Header()}, a.Lines...)
	if a.Excerpt != "" {
		lines = append(lines, "extracto: "+a.Excerpt)
	}
	if a.Link != "" {
		lines = append(lines, a.Link)
	}
	return strings.Join(lines, "\n")
}

func newAlert(rule, dedupKey string, headline ...string) Alert {
	return Alert{Rule: rule, Priority: ruleByName(rule).Priority, DedupKey: dedupKey, Headline: headline}
}

// Record is one error event or log record, normalized for the rules.
type Record struct {
	Stream     string // "event" or "log"
	ID         string
	Cursor     int64
	At         time.Time // collector receipt time; every window uses it
	Install    string    // support code; never leaves the VPS
	Session    string
	Job        string
	Release    string
	Labels     Labels
	Event      string
	Level      string
	Operation  string
	Outcome    string
	Message    string
	DurationMS int64
	ExitCode   *int64
	Lost       int64
}

func recordFromEvent(e ErrorEvent) Record {
	return Record{Stream: "event", ID: e.ID, At: e.ReceivedAt.UTC(), Install: e.SupportCode, Session: e.SessionID, Job: e.JobID,
		Release: e.Release, Labels: Labels{e.Component, e.Name, e.Stage, e.Class}, Event: e.Name, Level: "error", Message: e.Message,
		Operation: taskOperation(e.Name, e.Class)}
}

func recordFromLog(l LogRecord) Record {
	return Record{Stream: "log", ID: l.ID, Cursor: l.Cursor, At: l.ReceivedAt.UTC(), Install: l.SupportCode, Session: l.SessionID,
		Job: l.JobID, Release: l.Release, Labels: Labels{Component: l.Source, Name: l.Event}, Event: l.Event, Level: l.Level,
		Operation: l.Operation, Outcome: l.Outcome, Message: l.Message, DurationMS: l.DurationMS, ExitCode: l.ExitCode, Lost: l.LostRecords}
}

func taskOperation(name, class string) string {
	if name == "pipeline.error" && strings.Contains(class, ":") {
		return class
	}
	return ""
}

// Crash signals (P1 real crash). desktop.boot_failed and renderer.process_gone
// are crash-like events that share the shutdown-kill exclusion.
var crashEvents = map[string]bool{
	"desktop.backend_crashed": true, "process.uncaught_exception": true, "process.gone": true,
	"session.crashed_previous": true, "runtime.fatal_previous": true, "http.panicked": true,
	"desktop.boot_failed": true, "renderer.process_gone": true,
}

var crashGoneReasons = map[string]bool{"crashed": true, "oom": true, "launch-failed": true}

// shutdownKill matches processes killed at Windows logoff or shutdown:
// exit_class=shutdown_kill, today's 'código 1073807364' (0x40010004) boot
// failures and taskkill's 3221225794 (0xC0000142).
func shutdownKill(message string) bool {
	return parseKV(message)["exit_class"] == "shutdown_kill" ||
		strings.Contains(message, "1073807364") || strings.Contains(message, "3221225794")
}

// classify decides whether a record is an issue occurrence and which labels
// key it. The log copy of a task failure uses the pipeline.error labels so it
// lands on the same issue as the error event.
func classify(r Record) (labels Labels, crash, ok bool) {
	if crashEvents[r.Event] {
		if shutdownKill(r.Message) || r.Level != "error" && r.Stream == "log" {
			return Labels{}, false, false
		}
		if r.Event == "process.gone" && !crashGoneReasons[parseKV(r.Message)["reason"]] {
			return Labels{}, false, false
		}
		// renderer.process_gone carries no reason, so it pages as a new issue
		// but not through the crash rule.
		return r.Labels, r.Event != "renderer.process_gone", true
	}
	if r.Stream == "event" {
		return r.Labels, false, true
	}
	if r.Event == "attempt.finished" && r.Outcome == "error" || r.Event == "pipeline.error" && r.Level == "error" {
		return Labels{Component: "orchestrator", Name: "pipeline.error", Stage: "worker", Class: r.Operation}, false, true
	}
	return Labels{}, false, false
}

// JobContext is the enum context joined from attempt.toolchain,
// render.profile, delivery.quality and the session's device.context.
type JobContext struct {
	Operation string
	Toolchain map[string]string
	Profile   map[string]string
	Quality   map[string]string
	Device    map[string]string
}

var safeValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.:+-]{1,40}$`)

// safe keeps enum-like values only, so a free-form field can never carry
// message text to Telegram.
func safe(value string) string {
	if safeValuePattern.MatchString(value) {
		return value
	}
	return "?"
}

func kvLine(values map[string]string, keys ...string) string {
	var parts []string
	for _, key := range keys {
		if value, ok := values[key]; ok && value != "" {
			parts = append(parts, key+"="+safe(value))
		}
	}
	return strings.Join(parts, " ")
}

// machineLine is the toolchain + GPU vendor line of an alert.
func (c JobContext) machineLine() string {
	line := kvLine(c.Toolchain, "cs2_build", "hlae", "encoder")
	if vendor := c.Device["gpu_vendor"]; vendor != "" {
		line = strings.TrimSpace(line + " gpu_vendor=" + safe(vendor))
	}
	return line
}

func jobPrefix(job string) string {
	if len(job) < 8 {
		return ""
	}
	return "job " + job[:8] + "…"
}

func aliasLabel(alias int) string {
	if alias < 1 {
		return "#?"
	}
	return "#" + strconv.Itoa(alias)
}

// Occurrence is one classified error on its way to an issue.
type Occurrence struct {
	Key, Code, Substage, Signature string
	Labels                         Labels
	Release                        string
	At                             time.Time
	Alias                          int
	Job                            string
	Crash                          bool
	Message                        string
	Context                        JobContext
}

func newOccurrence(r Record, labels Labels, crash bool) Occurrence {
	code, substage := ParseFailureCode(r.Message)
	part := code
	signature := Signature(failureCodePattern.ReplaceAllString(strings.TrimSpace(r.Message), ""))
	if part == "" {
		part = "unclassified:" + signature
	}
	return Occurrence{Key: IssueKey(labels, part), Code: code, Substage: substage, Signature: signature, Labels: labels,
		Release: r.Release, At: r.At, Job: r.Job, Crash: crash, Message: r.Message}
}

func (o Occurrence) headline(count string) []string {
	label := o.Labels.Short()
	if o.Substage != "" {
		label += "/" + o.Substage
	}
	code := o.Code
	if code == "" {
		code = "sin código"
	}
	out := []string{label, code, o.Release, aliasLabel(o.Alias)}
	if count != "" {
		out = append(out, count)
	}
	return out
}

func (o Occurrence) detailLines() []string {
	var lines []string
	if h, ok := lookupHint(o.Code, o.Message); ok {
		lines = append(lines, h.line())
	}
	if prefix := jobPrefix(o.Job); prefix != "" {
		lines = append(lines, prefix)
	}
	if machine := o.Context.machineLine(); machine != "" {
		lines = append(lines, machine)
	}
	return append(lines, "clave "+o.Key)
}

// evalIssue implements P1 new issue, P1 regressed and P1 real crash for one
// occurrence given the issue as it was before it.
func evalIssue(o Occurrence, prior *Issue, now time.Time) (alerts []Alert, regressed bool) {
	withDetail := func(a Alert) Alert {
		a.IssueKey, a.Lines, a.Excerpt = o.Key, o.detailLines(), o.Message
		return a
	}
	switch {
	case prior == nil:
		return []Alert{withDetail(newAlert(RuleNewIssue, o.Key, o.headline("×1")...))}, false
	case prior.Status == "ignored":
		return nil, false
	case prior.Status == "resolved":
		if compareReleases(o.Release, prior.ResolvedIn) <= 0 {
			return nil, false
		}
		a := withDetail(newAlert(RuleRegressed, o.Key+"|"+o.Release, o.headline("resuelto en "+prior.ResolvedIn)...))
		return []Alert{a}, true
	case o.Crash && !now.Before(prior.SilencedUntil):
		return []Alert{withDetail(newAlert(RuleCrash, aliasLabel(o.Alias)+"|"+o.Key, o.headline(fmt.Sprintf("×%d", prior.Count+1))...))}, false
	}
	return nil, false
}

// PlatformInput is one install's attempts of one operation on its current
// release, newest first, plus the last good attempt on any release.
type PlatformInput struct {
	Alias     int
	Operation string
	Release   string
	Recent    []attemptRow
	LastGood  *attemptRow
	Latest    Occurrence
}

// evalPlatformBreak fires when the last 2 attempts failed and none succeeded
// on the current release (catches the HLAE 4.0.1 2/2 break).
func evalPlatformBreak(in PlatformInput) (Alert, bool) {
	var decided []attemptRow
	for _, row := range in.Recent {
		if row.Outcome == "ok" {
			return Alert{}, false
		}
		if row.Outcome == "error" {
			decided = append(decided, row)
		}
	}
	if len(decided) < 2 {
		return Alert{}, false
	}
	firstBad := decided[len(decided)-1]
	a := newAlert(RulePlatformBreak, fmt.Sprintf("%s|%s|%s", aliasLabel(in.Alias), in.Operation, in.Release),
		in.Operation, in.Release, aliasLabel(in.Alias), fmt.Sprintf("%d/%d fallos, 0 ok", len(decided), len(decided)))
	if in.LastGood != nil {
		a.Lines = append(a.Lines, strings.TrimSpace("último bueno: "+in.LastGood.Release+" "+kvLine(in.LastGood.Toolchain, "cs2_build", "hlae")))
	} else {
		a.Lines = append(a.Lines, "último bueno: ninguno en 30 días")
	}
	a.Lines = append(a.Lines, strings.TrimSpace("primer fallo: "+kvLine(firstBad.Toolchain, "cs2_build", "hlae")))
	if h, ok := lookupHint(in.Latest.Code, in.Latest.Message); ok {
		a.Lines = append(a.Lines, h.line())
	}
	if prefix := jobPrefix(in.Latest.Job); prefix != "" {
		a.Lines = append(a.Lines, prefix)
	}
	a.IssueKey = in.Latest.Key
	return a, true
}

// Full Demo floors. Every Full Demo delivery uses the gameplay-pov-60 preset
// (internal/editor/preset.go PresetGameplayPOV60); other presets get floors
// once real renders have been measured.
var fullDemoPresets = map[string]bool{"gameplay-pov-60": true}

func isFullDemoPreset(preset string) bool {
	return fullDemoPresets[strings.ToLower(preset)]
}

// evalQuality implements P1 output quality over one delivery.quality record.
func evalQuality(quality map[string]string, c JobContext, alias int, release, job string) (Alert, bool) {
	preset := quality["preset"]
	if !isFullDemoPreset(preset) {
		return Alert{}, false
	}
	number := func(key string) (float64, bool) {
		value, err := strconv.ParseFloat(quality[key], 64)
		return value, err == nil
	}
	var faults []string
	height, _ := number("height")
	fps, _ := number("fps")
	if bitrate, ok := number("bitrate_kbps"); ok && height >= 1080 && fps >= 59 && bitrate < 10000 {
		faults = append(faults, fmt.Sprintf("bitrate %.0f kb/s < 10000", bitrate))
	}
	if yavg, ok := number("yavg_mean"); ok && yavg < 25 {
		faults = append(faults, fmt.Sprintf("YAVG %.1f < 25", yavg))
	}
	if black, ok := number("black_ratio"); ok && black > 0.5 {
		faults = append(faults, fmt.Sprintf("negro %.2f > 0.5", black))
	}
	if len(faults) == 0 {
		return Alert{}, false
	}
	a := newAlert(RuleQuality, aliasLabel(alias)+"|"+safe(preset), safe(preset), release, aliasLabel(alias))
	a.Lines = append(a.Lines, strings.Join(faults, " · "))
	if machine := c.machineLine(); machine != "" {
		a.Lines = append(a.Lines, machine)
	}
	a.Lines = append(a.Lines, qualityHint.line())
	if prefix := jobPrefix(job); prefix != "" {
		a.Lines = append(a.Lines, prefix)
	}
	return a, true
}

var reportCategories = map[string]bool{"black_video": true, "wrong_overlay": true, "audio": true, "cuts": true, "other": true}

// evalUserReport implements P1 user report with the job's profile and quality.
func evalUserReport(category string, c JobContext, alias int, release, job string) Alert {
	if !reportCategories[category] {
		category = "other"
	}
	a := newAlert(RuleUserReport, job, category, release, aliasLabel(alias))
	if c.Operation != "" {
		a.Headline = append(a.Headline, safe(c.Operation))
	}
	if prefix := jobPrefix(job); prefix != "" {
		a.Lines = append(a.Lines, prefix)
	}
	if line := kvLine(c.Profile, "source_kind", "overlay_source", "hud", "fps", "resolution", "encoder", "aac_path"); line != "" {
		a.Lines = append(a.Lines, "render.profile: "+line)
	}
	if line := kvLine(c.Quality, "preset", "bitrate_kbps", "yavg_mean", "black_ratio"); line != "" {
		a.Lines = append(a.Lines, "delivery.quality: "+line)
	}
	if machine := c.machineLine(); machine != "" {
		a.Lines = append(a.Lines, machine)
	}
	if h, ok := reportHints[category]; ok {
		a.Lines = append(a.Lines, h.line())
	}
	return a
}

var rejectionStatuses = map[string]bool{"400": true, "401": true, "413": true, "415": true, "422": true, "429": true, "507": true}

// evalHealth implements P1 channel health from the admin healthz and the
// rejection counters seen by the previous run.
func evalHealth(h Health, previous map[string]int64) []Alert {
	var alerts []Alert
	if h.DBOK != nil && !*h.DBOK {
		alerts = append(alerts, newAlert(RuleChannelHealth, "db", "db_ok=false", "el colector no puede leer su base de datos"))
	}
	keys := make([]string, 0, len(h.Rejections))
	for key := range h.Rejections {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 || !rejectionStatuses[parts[1]] {
			continue
		}
		delta := h.Rejections[key]
		if before, ok := previous[key]; ok && before <= delta {
			delta -= before
		}
		if delta > 0 {
			alerts = append(alerts, newAlert(RuleChannelHealth, key, "rechazos "+safe(key), fmt.Sprintf("+%d desde la última ejecución", delta)))
		}
	}
	for _, ratio := range []struct {
		name  string
		value *float64
	}{{"events", h.EventsStorageRatio}, {"logs", h.LogsStorageRatio}} {
		if ratio.value != nil && *ratio.value >= storageHighMark {
			alerts = append(alerts, newAlert(RuleChannelHealth, "storage:"+ratio.name, "almacenamiento "+ratio.name, fmt.Sprintf("%.0f%% del límite", *ratio.value*100)))
		}
	}
	return alerts
}

// evalDeliveryLoss implements P2 delivery loss.
func evalDeliveryLoss(r Record, alias int) (Alert, bool) {
	kv := parseKV(r.Message)
	var detail string
	switch r.Event {
	case "delivery.gap":
		detail = fmt.Sprintf("delivery.gap lost=%d", max(r.Lost, 1))
	case "delivery.health":
		dropped, _ := strconv.ParseInt(kv["dropped_records"], 10, 64)
		rejected, _ := strconv.ParseInt(kv["rejected_records"], 10, 64)
		if dropped <= 0 && rejected <= 0 {
			return Alert{}, false
		}
		detail = fmt.Sprintf("dropped_records=%d rejected_records=%d", dropped, rejected)
	default:
		return Alert{}, false
	}
	return newAlert(RuleDeliveryLoss, aliasLabel(alias), aliasLabel(alias), r.Release, detail), true
}

// evalRepeat implements P2 repeat of an open issue: the first batch waits an
// hour after the issue's last message, later batches six hours.
func evalRepeat(i Issue, perAlias map[int]int, lastRepeat, now time.Time) (Alert, bool) {
	total := 0
	for _, count := range perAlias {
		total += count
	}
	if total == 0 || i.Status != "open" || i.Acked || now.Before(i.SilencedUntil) {
		return Alert{}, false
	}
	gap := repeatFirstGap
	if !lastRepeat.IsZero() && !lastRepeat.Before(i.NotifiedAt) {
		gap = repeatGap
	}
	if now.Sub(i.NotifiedAt) < gap {
		return Alert{}, false
	}
	o := Occurrence{Labels: i.Labels, Code: i.Code, Substage: i.Substage, Release: i.LastRelease}
	headline := o.headline(fmt.Sprintf("×%d", total))
	aliases := make([]int, 0, len(perAlias))
	for alias := range perAlias {
		aliases = append(aliases, alias)
	}
	slices.Sort(aliases)
	var per []string
	for _, alias := range aliases {
		per = append(per, fmt.Sprintf("%s ×%d", aliasLabel(alias), perAlias[alias]))
	}
	a := newAlert(RuleRepeat, i.Key, append(headline[:3], headline[4:]...)...)
	a.IssueKey = i.Key
	a.Lines = []string{"instalaciones: " + strings.Join(per, ", "), "clave " + i.Key}
	return a, true
}

// compareReleases orders x.y.z releases numerically.
func compareReleases(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}
