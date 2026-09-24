package telemetryalert

import (
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	base := map[string]string{
		"CLIPHUB_ALERT_ADMIN_URL":      "http://127.0.0.1:8121/",
		"CLIPHUB_ALERT_ADMIN_TOKEN":    testAdminToken,
		"CLIPHUB_ALERT_STATE_DIR":      "/var/lib/alerts",
		"CLIPHUB_ALERT_TELEGRAM_TOKEN": testBotToken,
		"CLIPHUB_ALERT_TELEGRAM_CHAT":  "-1001234",
	}
	tests := []struct {
		name, key, value string
		needTelegram     bool
		wantErr          string
	}{
		{name: "valid", needTelegram: true},
		{name: "hostname admin", key: "CLIPHUB_ALERT_ADMIN_URL", value: "http://localhost:8121", wantErr: "numeric loopback"},
		{name: "public admin", key: "CLIPHUB_ALERT_ADMIN_URL", value: "https://[2001:db8::1]:8121", wantErr: "numeric loopback"},
		{name: "no port", key: "CLIPHUB_ALERT_ADMIN_URL", value: "http://127.0.0.1", wantErr: "http(s)"},
		{name: "short token", key: "CLIPHUB_ALERT_ADMIN_TOKEN", value: "short", wantErr: "32 characters"},
		{name: "chat not numeric", key: "CLIPHUB_ALERT_TELEGRAM_CHAT", value: "@channel", wantErr: "numeric chat"},
		{name: "telegram required", key: "CLIPHUB_ALERT_TELEGRAM_TOKEN", value: "", needTelegram: true, wantErr: "TELEGRAM_TOKEN"},
		{name: "telegram optional for bootstrap", key: "CLIPHUB_ALERT_TELEGRAM_TOKEN", value: ""},
		{name: "deadman must be https", key: "CLIPHUB_ALERT_DEADMAN_URL", value: "http://hc.invalid/x", wantErr: "https"},
		{name: "excerpt flag", key: "CLIPHUB_ALERT_INCLUDE_EXCERPT", value: "maybe", wantErr: "boolean"},
		{name: "systemd state directory", key: "CLIPHUB_ALERT_STATE_DIR", value: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"STATE_DIRECTORY": "/var/lib/from-systemd:/other"}
			for k, v := range base {
				env[k] = v
			}
			if tt.key != "" {
				env[tt.key] = tt.value
			}
			cfg, err := LoadConfig(func(name string) string { return env[name] }, tt.needTelegram)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AdminURL != "http://127.0.0.1:8121" || cfg.IncludeExcerpt {
				t.Fatalf("cfg = %+v", cfg)
			}
			if tt.name == "systemd state directory" && cfg.StateDir != "/var/lib/from-systemd" {
				t.Fatalf("state dir = %q", cfg.StateDir)
			}
			if tt.name == "valid" && cfg.TelegramChat != -1001234 {
				t.Fatalf("chat = %d", cfg.TelegramChat)
			}
		})
	}
}
