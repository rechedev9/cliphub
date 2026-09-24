package telemetryalert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf16"
)

const defaultTelegramAPI = "https://api.telegram.org"

// Telegram sends HTML messages to one chat. P2 alerts and flood-guarded P1s
// use disable_notification; issue alerts carry Ack / Resolver / Silenciar 24h.
type Telegram struct {
	API    string
	Token  string
	ChatID int64
	Client *http.Client
}

type telegramResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func (t Telegram) call(ctx context.Context, method string, body any, result any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	api := t.API
	if api == "" {
		api = defaultTelegramAPI
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api+"/bot"+t.Token+"/"+method, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.Client.Do(req)
	if err != nil {
		// The URL embeds the bot token; never surface it in logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer resp.Body.Close()
	var decoded telegramResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&decoded); err != nil {
		return fmt.Errorf("telegram %s: status %d", method, resp.StatusCode)
	}
	if !decoded.OK {
		err := fmt.Errorf("telegram %s: status %d: %s", method, resp.StatusCode, decoded.Description)
		if permanentTelegramStatus(resp.StatusCode) {
			return &PermanentError{Status: resp.StatusCode, Err: err}
		}
		return err
	}
	if result != nil {
		return json.Unmarshal(decoded.Result, result)
	}
	return nil
}

// permanentTelegramStatus reports a 4xx that rejects this message for good
// (400: too long, unparsable HTML). 429 is a rate limit. 401 and 404 mean a
// bad or revoked bot token and 403 a bot the chat blocked or removed: those
// fail every message alike until the operator fixes the channel, so they stay
// transient and the outbox keeps its alerts, as for a token rotation.
func permanentTelegramStatus(status int) bool {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests:
		return false
	}
	return status >= 400 && status < 500
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// telegramTextLimit is the Bot API cap on a message text. It is checked on
// the HTML source in UTF-16 units, which is never shorter than the parsed
// text Telegram counts.
const telegramTextLimit = 4096

// FormatHTML renders an alert for parse_mode=HTML; every value is escaped.
// A text over telegramTextLimit drops trailing body lines behind a
// "… N líneas más" line and keeps the header, excerpt and link. Only whole
// escaped lines are dropped, so an HTML entity or tag is never cut.
func FormatHTML(a Alert) string {
	header := "<b>" + html.EscapeString(a.Header()) + "</b>"
	body := make([]string, len(a.Lines))
	for i, line := range a.Lines {
		body[i] = html.EscapeString(line)
	}
	var tail []string
	if a.Excerpt != "" {
		tail = append(tail, "<i>extracto:</i> "+html.EscapeString(a.Excerpt))
	}
	if a.Link != "" {
		tail = append(tail, html.EscapeString(a.Link))
	}
	compose := func(header string, kept int, tail []string) string {
		parts := append([]string{header}, body[:kept]...)
		if dropped := len(body) - kept; dropped > 0 {
			parts = append(parts, moreLines(dropped))
		}
		return strings.Join(append(parts, tail...), "\n")
	}
	// prefix[k] is the size of the first k body lines, newline included.
	prefix := make([]int, len(body)+1)
	for i, line := range body {
		prefix[i+1] = prefix[i] + utf16Len(line) + 1
	}
	for {
		fixed := utf16Len(header)
		for _, line := range tail {
			fixed += utf16Len(line) + 1
		}
		for kept := len(body); kept >= 0; kept-- {
			size := fixed + prefix[kept]
			if dropped := len(body) - kept; dropped > 0 {
				size += utf16Len(moreLines(dropped)) + 1
			}
			if size <= telegramTextLimit {
				return compose(header, kept, tail)
			}
		}
		if len(tail) == 0 {
			break
		}
		tail = tail[:len(tail)-1]
	}
	// Only an absurd header gets here: cut it at a rune boundary.
	header = "<b>" + escapeWithin(a.Header(), telegramTextLimit-utf16Len("<b></b>\n"+moreLines(len(body)))) + "</b>"
	return compose(header, 0, nil)
}

func moreLines(n int) string {
	if n == 1 {
		return "… 1 línea más"
	}
	return fmt.Sprintf("… %d líneas más", n)
}

// escapeWithin escapes s rune by rune and stops, with an ellipsis, before the
// escaped text exceeds budget UTF-16 units.
func escapeWithin(s string, budget int) string {
	var b strings.Builder
	used := 0
	for _, r := range s {
		escaped := html.EscapeString(string(r))
		if used+utf16Len(escaped)+1 > budget {
			return b.String() + "…"
		}
		b.WriteString(escaped)
		used += utf16Len(escaped)
	}
	return b.String()
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

func (t Telegram) Send(ctx context.Context, a Alert) error {
	body := map[string]any{
		"chat_id":                  t.ChatID,
		"text":                     FormatHTML(a),
		"parse_mode":               "HTML",
		"disable_notification":     a.Priority == 2 || a.Silent,
		"disable_web_page_preview": true,
	}
	if a.IssueKey != "" {
		body["reply_markup"] = map[string]any{"inline_keyboard": [][]inlineButton{{
			{"Ack", "ack:" + a.IssueKey}, {"Resolver", "res:" + a.IssueKey}, {"Silenciar 24h", "sil:" + a.IssueKey},
		}}}
	}
	return t.call(ctx, "sendMessage", body, nil)
}

// Callbacks reads button presses after offset without long polling.
func (t Telegram) Callbacks(ctx context.Context, offset int64) ([]Callback, int64, error) {
	var updates []struct {
		UpdateID      int64 `json:"update_id"`
		CallbackQuery *struct {
			ID      string `json:"id"`
			Data    string `json:"data"`
			Message *struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"callback_query"`
	}
	body := map[string]any{"offset": offset, "timeout": 0, "allowed_updates": []string{"callback_query"}}
	if err := t.call(ctx, "getUpdates", body, &updates); err != nil {
		return nil, offset, err
	}
	var out []Callback
	next := offset
	for _, update := range updates {
		next = max(next, update.UpdateID+1)
		if q := update.CallbackQuery; q != nil && q.Message != nil {
			out = append(out, Callback{ID: q.ID, ChatID: q.Message.Chat.ID, Data: q.Data})
		}
	}
	return out, next, nil
}

func (t Telegram) Answer(ctx context.Context, callbackID, text string) error {
	return t.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callbackID, "text": text}, nil)
}
