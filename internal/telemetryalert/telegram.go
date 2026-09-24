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
		return fmt.Errorf("telegram %s: status %d: %s", method, resp.StatusCode, decoded.Description)
	}
	if result != nil {
		return json.Unmarshal(decoded.Result, result)
	}
	return nil
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// FormatHTML renders an alert for parse_mode=HTML; every value is escaped.
func FormatHTML(a Alert) string {
	var b strings.Builder
	b.WriteString("<b>" + html.EscapeString(a.Header()) + "</b>")
	for _, line := range a.Lines {
		b.WriteString("\n" + html.EscapeString(line))
	}
	if a.Excerpt != "" {
		b.WriteString("\n<i>extracto:</i> " + html.EscapeString(a.Excerpt))
	}
	if a.Link != "" {
		b.WriteString("\n" + html.EscapeString(a.Link))
	}
	return b.String()
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
