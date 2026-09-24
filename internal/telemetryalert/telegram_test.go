package telemetryalert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
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
