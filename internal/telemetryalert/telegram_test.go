package telemetryalert

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const testBotToken = "123456:test-bot-token"

type telegramCall struct {
	method string
	body   map[string]any
}

// fakeTelegram is an httptest Bot API: it records every call and serves
// queued updates to getUpdates.
type fakeTelegram struct {
	mu      sync.Mutex
	calls   []telegramCall
	updates []map[string]any
	server  *httptest.Server
	// reject, when set, refuses a sendMessage text with an HTTP status and a
	// Bot API description; status 0 accepts it. Refused calls are not in calls.
	reject   func(text string) (int, string)
	rejected []string
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	f := &fakeTelegram{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/bot" + testBotToken + "/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Not Found"}`))
			return
		}
		method := strings.TrimPrefix(r.URL.Path, prefix)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		if text, _ := body["text"].(string); method == "sendMessage" && f.reject != nil {
			if status, description := f.reject(text); status != 0 {
				f.rejected = append(f.rejected, text)
				f.mu.Unlock()
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": status, "description": description})
				return
			}
		}
		f.calls = append(f.calls, telegramCall{method, body})
		var result any = true
		switch method {
		case "getUpdates":
			offset := int64(body["offset"].(float64))
			var pending []map[string]any
			for _, update := range f.updates {
				if int64(update["update_id"].(int)) >= offset {
					pending = append(pending, update)
				}
			}
			result = pending
			if pending == nil {
				result = []any{}
			}
		case "sendMessage":
			result = map[string]any{"message_id": len(f.calls)}
		}
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeTelegram) client() Telegram {
	return Telegram{API: f.server.URL, Token: testBotToken, ChatID: 42, Client: f.server.Client()}
}

func (f *fakeTelegram) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, call := range f.calls {
		out = append(out, call.method)
	}
	return out
}

func (f *fakeTelegram) messages() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, call := range f.calls {
		if call.method == "sendMessage" {
			out = append(out, call.body)
		}
	}
	return out
}

func TestTelegramSendFormatsAudibleAndSilentMessages(t *testing.T) {
	tg := newFakeTelegram(t)
	client := tg.client()
	p1 := newAlert(RuleNewIssue, "k", "render:variant", "<b>code</b>", "3.0.0", "#1")
	p1.IssueKey = "0123456789abcdef"
	p1.Lines = []string{"pista: a & b"}
	if err := client.Send(context.Background(), p1); err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), newAlert(RuleRepeat, "k", "x")); err != nil {
		t.Fatal(err)
	}
	messages := tg.messages()
	if len(messages) != 2 {
		t.Fatalf("messages = %d", len(messages))
	}
	audible, silent := messages[0], messages[1]
	if audible["parse_mode"] != "HTML" || audible["disable_notification"] != false || audible["chat_id"] != float64(42) {
		t.Fatalf("P1 body = %v", audible)
	}
	text := audible["text"].(string)
	if !strings.Contains(text, "&lt;b&gt;code&lt;/b&gt;") || !strings.Contains(text, "pista: a &amp; b") || !strings.HasPrefix(text, "<b>[P1] Nuevo") {
		t.Fatalf("P1 text not escaped: %q", text)
	}
	buttons, _ := json.Marshal(audible["reply_markup"])
	for _, want := range []string{`"ack:0123456789abcdef"`, `"res:0123456789abcdef"`, `"sil:0123456789abcdef"`, "Silenciar 24h", "Resolver"} {
		if !strings.Contains(string(buttons), want) {
			t.Errorf("buttons %s lack %s", buttons, want)
		}
	}
	if silent["disable_notification"] != true || silent["reply_markup"] != nil {
		t.Fatalf("P2 body = %v", silent)
	}
}

func TestTelegramErrorsNeverCarryTheToken(t *testing.T) {
	client := Telegram{API: "http://127.0.0.1:1", Token: testBotToken, ChatID: 42, Client: http.DefaultClient}
	err := client.Send(context.Background(), newAlert(RuleRepeat, "k", "x"))
	if err == nil || strings.Contains(err.Error(), "test-bot-token") {
		t.Fatalf("error = %v", err)
	}
}

// TestTelegramButtonsResolveAndRegress drives Resolver through getUpdates:
// only the configured chat is obeyed, the issue is resolved in the newest
// release seen, and a newer release then pages as a regression.
func TestTelegramButtonsResolveAndRegress(t *testing.T) {
	h := newHarness(t, "hlae.json")
	tg := newFakeTelegram(t)
	h.notifier = tg.client()
	h.run("2026-09-22T00:00:00Z")
	h.run("2026-09-23T13:40:00Z")
	key := IssueKey(Labels{"orchestrator", "pipeline.error", "worker", "record:demo"}, "hlae_hook_incompatible")
	callback := func(id int, chat int64, data string) map[string]any {
		return map[string]any{"update_id": id, "callback_query": map[string]any{
			"id": "cb" + data, "data": data, "message": map[string]any{"chat": map[string]any{"id": chat}},
		}}
	}
	tg.mu.Lock()
	tg.updates = []map[string]any{callback(7, 99, "ack:"+key), callback(8, 42, "res:"+key)}
	tg.mu.Unlock()
	h.run("2026-09-23T13:45:00Z")
	var answers []string
	var lastOffset float64
	for _, call := range tg.calls {
		switch call.method {
		case "answerCallbackQuery":
			answers = append(answers, call.body["text"].(string))
		case "getUpdates":
			lastOffset = call.body["offset"].(float64)
		}
	}
	if len(answers) != 1 || answers[0] != "Resuelta en 4.0.1" {
		t.Fatalf("answers = %v", answers)
	}
	if n := countRows(t, h, "SELECT COUNT(*) FROM issues WHERE status='resolved' AND resolved_in_release='4.0.1' AND acked=0"); n != 1 {
		t.Fatalf("resolved issues = %d", n)
	}
	h.run("2026-09-23T13:46:00Z")
	if lastOffset = lastGetUpdatesOffset(tg); lastOffset != 9 {
		t.Fatalf("getUpdates offset = %v, want 9", lastOffset)
	}

	h.admin.add(fixtureFile{Events: []ErrorEvent{{
		ReceivedAt: mustTime(t, "2026-09-24T10:00:00Z"), SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000024",
		Release: "4.0.2", Component: "orchestrator", Name: "pipeline.error", Stage: "worker", Class: "record:demo",
		JobID: "4ae10001-0000-4000-8000-000000000010", Message: "failure_code=hlae_hook_incompatible substage=hook_start; exited 6",
	}}})
	before := len(tg.messages())
	h.run("2026-09-24T10:01:00Z")
	var regressed bool
	for _, message := range tg.messages()[before:] {
		if strings.HasPrefix(message["text"].(string), "<b>[P1] Regresión · record:demo/hook_start · hlae_hook_incompatible · 4.0.2") {
			regressed = true
		}
	}
	if !regressed {
		t.Fatalf("no regression page in %v", tg.messages()[before:])
	}
}

func lastGetUpdatesOffset(tg *fakeTelegram) float64 {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	var offset float64
	for _, call := range tg.calls {
		if call.method == "getUpdates" {
			offset = call.body["offset"].(float64)
		}
	}
	return offset
}

// TestTelegramPayloadPrivacy replays every incident through the Bot API fake
// and asserts that no support code, redaction token or message text reaches
// Telegram unless the excerpt flag is on.
func TestTelegramPayloadPrivacy(t *testing.T) {
	forbidden := []string{
		"CH-", "[path]", "after three masters", "true peak", "Result too large", "Parsed_loudnorm",
		"native error dialog", "AfxHookSource2", "terminó inesperadamente", "Task attempt completed",
		"5e551011", "rtx_3060", "AMD_Ryzen", "v581.57",
	}
	runs := []string{
		"2026-09-01T00:00:00Z", "2026-09-07T13:11:00Z", "2026-09-07T14:12:00Z", "2026-09-07T20:15:00Z",
		"2026-09-10T14:01:00Z", "2026-09-10T14:31:00Z", "2026-09-10T16:03:00Z", "2026-09-10T20:13:00Z",
		"2026-09-11T07:05:00Z", "2026-09-11T07:31:00Z", "2026-09-22T20:00:00Z", "2026-09-23T13:36:00Z",
		"2026-09-23T13:40:00Z", "2026-09-23T13:52:00Z", "2026-09-23T17:42:00Z", "2026-09-24T09:05:00Z",
	}
	for _, include := range []bool{false, true} {
		h := newHarness(t, "loudnorm.json", "hlae.json", "faceit.json", "black-capture.json", "shutdown-kill.json")
		h.cfg.IncludeExcerpt = include
		tg := newFakeTelegram(t)
		h.notifier = tg.client()
		for _, at := range runs {
			h.run(at)
		}
		var all strings.Builder
		for _, message := range tg.messages() {
			encoded, _ := json.Marshal(message)
			all.Write(encoded)
			all.WriteString("\n")
		}
		payloads := all.String()
		if len(tg.messages()) < 8 {
			t.Fatalf("only %d messages replayed", len(tg.messages()))
		}
		if !include {
			for _, needle := range forbidden {
				if strings.Contains(payloads, needle) {
					t.Errorf("payload contains %q:\n%s", needle, payloads)
				}
			}
		} else if !strings.Contains(payloads, "after three masters") {
			t.Errorf("excerpt flag on but no excerpt in payloads")
		}
	}
}

func (f *fakeTelegram) setReject(reject func(text string) (int, string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reject = reject
}

// queueFailures makes the next run page one new issue per failure code.
func queueFailures(t *testing.T, h *harness, at string, codes ...string) {
	t.Helper()
	var events []ErrorEvent
	for i, code := range codes {
		events = append(events, ErrorEvent{ReceivedAt: mustTime(t, at).Add(time.Duration(i) * time.Second),
			SupportCode: "CH-1111-2222-3333-4444-5555", SessionID: "5e551011-0000-4000-8000-000000000083", Release: "5.2.1",
			Component: "orchestrator", Name: "pipeline.error", Stage: "worker", Class: "render:variant",
			Message: "failure_code=" + code + " substage=test; boom"})
	}
	h.admin.add(fixtureFile{Events: events})
}

// One alert Telegram refuses for good (400) must not hold back the ones
// queued after it; rate limits and channel-wide failures (a revoked token)
// keep the queue for the next run.
func TestOutboxDeadLettersPermanentRejectionsAndDeliversTheRest(t *testing.T) {
	h := newHarness(t)
	tg := newFakeTelegram(t)
	h.notifier = tg.client()
	var logLines []string
	captureLog := func(o *Options) {
		o.Logf = func(format string, args ...any) { logLines = append(logLines, fmt.Sprintf(format, args...)) }
	}
	h.run("2026-09-20T09:00:00Z") // bootstrap

	queueFailures(t, h, "2026-09-20T10:00:00Z", "synthetic_a", "synthetic_b", "synthetic_c")
	tg.setReject(func(text string) (int, string) {
		if strings.Contains(text, "synthetic_b") {
			return http.StatusBadRequest, "Bad Request: message is too long"
		}
		return 0, ""
	})
	result := h.run("2026-09-20T10:01:00Z", captureLog)
	if result.Delivered != 2 || result.DeadLettered != 1 || !slices.Equal(result.Failures, []string{"dead_letter=1"}) {
		t.Fatalf("result: delivered=%d dead=%d failures=%v", result.Delivered, result.DeadLettered, result.Failures)
	}
	var sent []string
	for _, message := range tg.messages() {
		sent = append(sent, message["text"].(string))
	}
	if len(sent) != 2 || !strings.Contains(sent[0], "synthetic_a") || !strings.Contains(sent[1], "synthetic_c") {
		t.Fatalf("delivered %q", sent)
	}
	if n := countRows(t, h, "SELECT COUNT(*) FROM outbox"); n != 0 {
		t.Fatalf("outbox keeps %d rows", n)
	}
	if !slices.Contains(logLines, "telemetry-alert stage=outbox class=dead_letter rule=new_issue status=400") ||
		strings.Contains(strings.Join(logLines, "\n"), "too long") {
		t.Fatalf("dead-letter log lines: %q", logLines)
	}

	queueFailures(t, h, "2026-09-20T10:05:00Z", "synthetic_d", "synthetic_e")
	for _, refusal := range []struct {
		status      int
		description string
	}{{http.StatusTooManyRequests, "Too Many Requests: retry after 5"}, {http.StatusUnauthorized, "Unauthorized"}} {
		tg.setReject(func(string) (int, string) { return refusal.status, refusal.description })
		result = h.run("2026-09-20T10:06:00Z")
		if result.Delivered != 0 || result.DeadLettered != 0 || !slices.Equal(result.Failures, []string{"telegram"}) {
			t.Fatalf("status %d: delivered=%d dead=%d failures=%v", refusal.status, result.Delivered, result.DeadLettered, result.Failures)
		}
		if n := countRows(t, h, "SELECT COUNT(*) FROM outbox"); n != 2 {
			t.Fatalf("status %d: outbox keeps %d rows, want 2", refusal.status, n)
		}
	}
	tg.setReject(nil)
	if result = h.run("2026-09-20T10:07:00Z"); result.Delivered != 2 || len(result.Failures) != 0 {
		t.Fatalf("after recovery: delivered=%d failures=%v", result.Delivered, result.Failures)
	}
}

var validEntity = regexp.MustCompile(`&(amp|lt|gt|#34|#39);`)

// Telegram refuses texts over 4096 characters with a 400 that would
// dead-letter the alert. The digest has no natural bound (one line per
// release and operation), so every text is cut to whole lines.
func TestTelegramTextNeverExceedsTheLimit(t *testing.T) {
	tg := newFakeTelegram(t)
	tg.setReject(func(text string) (int, string) {
		if utf16Len(text) > telegramTextLimit {
			return http.StatusBadRequest, "Bad Request: message is too long"
		}
		return 0, ""
	})
	data := digestData{Day: "2026-09-24"}
	for i := range 400 {
		data.Attempts = append(data.Attempts, releaseOpCount{Release: fmt.Sprintf("5.%d.0", i), Operation: "render:variant", OK: i, Error: 1})
	}
	digest := buildDigest(data)
	digest.Link = "https://report.invalid/alerts/index.html"
	if err := tg.client().Send(context.Background(), digest); err != nil {
		t.Fatal(err)
	}
	text := tg.messages()[0]["text"].(string)
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] != digest.Link {
		t.Fatalf("link dropped: %q", lines[len(lines)-1])
	}
	var dropped int
	if _, err := fmt.Sscanf(lines[len(lines)-2], "… %d líneas más", &dropped); err != nil {
		t.Fatalf("no dropped-lines marker: %q", lines[len(lines)-2])
	}
	kept := lines[1 : len(lines)-2]
	if len(kept)+dropped != len(digest.Lines) || len(kept) < 50 {
		t.Fatalf("kept %d + dropped %d != %d lines", len(kept), dropped, len(digest.Lines))
	}
	for i, line := range kept {
		if line != html.EscapeString(digest.Lines[i]) {
			t.Fatalf("line %d cut: %q", i, line)
		}
	}

	// Lines are measured escaped and dropped whole, so no entity is cut.
	noisy := newAlert(RuleRepeat, "k", "x")
	for range 400 {
		noisy.Lines = append(noisy.Lines, strings.Repeat(`&<"`, 7))
	}
	text = FormatHTML(noisy)
	if utf16Len(text) > telegramTextLimit || strings.Contains(validEntity.ReplaceAllString(text, ""), "&") {
		t.Fatalf("noisy text: %d units, broken entity: %t", utf16Len(text), strings.Contains(validEntity.ReplaceAllString(text, ""), "&"))
	}
	if short := FormatHTML(newAlert(RuleRepeat, "k", "x")); strings.Contains(short, "más") {
		t.Fatalf("short alert gained a marker: %q", short)
	}
}
