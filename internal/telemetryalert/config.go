// Package telemetryalert is the oneshot alerter that runs next to the
// telemetry collector. It reads the loopback admin API, keeps issue and alert
// state in its own SQLite file, notifies Telegram, pings a dead-man check and
// renders a static tailnet report. It never writes to the collector.
package telemetryalert

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config comes only from the environment; see docs/telemetry-operations.md.
type Config struct {
	AdminURL       string
	AdminToken     string
	StateDir       string
	TelegramToken  string
	TelegramChat   int64
	DeadmanURL     string
	ReportBase     string
	IncludeExcerpt bool
	// TelegramAPI is only overridden by tests.
	TelegramAPI string
}

const runBudget = 45 * time.Second

// LoadConfig reads CLIPHUB_ALERT_* variables. Telegram credentials are
// optional only when nothing will be sent (dry run or bootstrap).
func LoadConfig(getenv func(string) string, needTelegram bool) (Config, error) {
	cfg := Config{
		AdminURL:      strings.TrimRight(getenv("CLIPHUB_ALERT_ADMIN_URL"), "/"),
		AdminToken:    getenv("CLIPHUB_ALERT_ADMIN_TOKEN"),
		StateDir:      getenv("CLIPHUB_ALERT_STATE_DIR"),
		TelegramToken: getenv("CLIPHUB_ALERT_TELEGRAM_TOKEN"),
		DeadmanURL:    strings.TrimRight(getenv("CLIPHUB_ALERT_DEADMAN_URL"), "/"),
		ReportBase:    strings.TrimRight(getenv("CLIPHUB_ALERT_REPORT_BASE"), "/"),
	}
	if cfg.StateDir == "" {
		// systemd StateDirectory= exports this; the first entry is ours.
		cfg.StateDir, _, _ = strings.Cut(getenv("STATE_DIRECTORY"), ":")
	}
	if err := requireLoopbackURL(cfg.AdminURL); err != nil {
		return Config{}, err
	}
	if len(cfg.AdminToken) < 32 {
		return Config{}, errors.New("CLIPHUB_ALERT_ADMIN_TOKEN must contain at least 32 characters")
	}
	if cfg.StateDir == "" {
		return Config{}, errors.New("CLIPHUB_ALERT_STATE_DIR is required")
	}
	if raw := getenv("CLIPHUB_ALERT_TELEGRAM_CHAT"); raw != "" {
		chat, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || chat == 0 {
			return Config{}, errors.New("CLIPHUB_ALERT_TELEGRAM_CHAT must be a numeric chat id")
		}
		cfg.TelegramChat = chat
	}
	if needTelegram && (cfg.TelegramToken == "" || cfg.TelegramChat == 0) {
		return Config{}, errors.New("CLIPHUB_ALERT_TELEGRAM_TOKEN and CLIPHUB_ALERT_TELEGRAM_CHAT are required")
	}
	for name, value := range map[string]string{"CLIPHUB_ALERT_DEADMAN_URL": cfg.DeadmanURL, "CLIPHUB_ALERT_REPORT_BASE": cfg.ReportBase} {
		if value != "" && !strings.HasPrefix(value, "https://") {
			return Config{}, fmt.Errorf("%s must use https", name)
		}
	}
	if raw := getenv("CLIPHUB_ALERT_INCLUDE_EXCERPT"); raw != "" {
		include, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, errors.New("CLIPHUB_ALERT_INCLUDE_EXCERPT must be a boolean")
		}
		cfg.IncludeExcerpt = include
	}
	return cfg, nil
}

func requireLoopbackURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Port() == "" {
		return errors.New("CLIPHUB_ALERT_ADMIN_URL must be http(s)://<numeric loopback>:<port>")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip == nil || !ip.IsLoopback() {
		return errors.New("CLIPHUB_ALERT_ADMIN_URL must use a numeric loopback host")
	}
	return nil
}
