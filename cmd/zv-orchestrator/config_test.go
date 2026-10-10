package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestLoadConfigAllowsParserOnlyMode(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "postgres://example")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.recordWorkerEnabled() {
		t.Fatal("record worker enabled, want disabled")
	}
	if cfg.composeWorkerEnabled() {
		t.Fatal("compose worker enabled, want disabled")
	}
	if cfg.renderWorkerEnabled() {
		t.Fatal("render worker enabled, want disabled")
	}
	if cfg.MediaWorkDir != "" {
		t.Fatalf("MediaWorkDir = %q, want empty default for temp cleanup", cfg.MediaWorkDir)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("HTTPAddr = %q, want loopback default", cfg.HTTPAddr)
	}
}

func TestLoadConfigRejectsLANBindEvenWithMutationToken(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "postgres://example")
	t.Setenv("ZV_HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("ZV_MUTATION_TOKEN", strings.Repeat("a", 64))

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig error = nil, want cleartext non-loopback bind rejected")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("loadConfig error = %q, want loopback requirement", err)
	}
}

func TestLoadConfigRequiresStrongSessionCapability(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "missing"},
		{name: "short", token: "local-token"},
		{name: "uppercase hex", token: strings.Repeat("A", 64)},
		{name: "non hex", token: strings.Repeat("g", 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			t.Setenv("ZV_MUTATION_TOKEN", tt.token)
			_, err := loadConfig()
			if err == nil {
				t.Fatal("loadConfig error = nil, want invalid session capability rejected")
			}
			if strings.Contains(err.Error(), tt.token) && tt.token != "" {
				t.Fatalf("loadConfig error reflected capability: %q", err)
			}
		})
	}
}

func TestClearSubprocessCredentialEnvironmentKeepsLoadedConfigOnlyInMemory(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	t.Setenv(mutationTokenEnvironmentVariable, strings.Repeat("b", 64))
	t.Setenv(firecrawlAPIKeyEnvironmentVariable, "firecrawl-secret")
	t.Setenv(faceitAPIKeyEnvironmentVariable, "faceit-secret")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if err := clearSubprocessCredentialEnvironment(); err != nil {
		t.Fatalf("clearSubprocessCredentialEnvironment error = %v", err)
	}
	if cfg.MutationToken == "" || cfg.FirecrawlAPIKey == "" || cfg.FaceitAPIKey == "" {
		t.Fatal("loaded config lost a credential after environment cleanup")
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, mutationTokenEnvironmentVariable) || strings.EqualFold(name, firecrawlAPIKeyEnvironmentVariable) || strings.EqualFold(name, faceitAPIKeyEnvironmentVariable) {
			t.Fatalf("environment still contains %q after credential cleanup", name)
		}
	}
}

func TestLoadConfigAllowsPartialRecordWorkerConfig(t *testing.T) {
	// Regression: a partially-set record trio must not kill the boot. The
	// desktop app passes only ZV_HLAE_PATH (its provisioned HLAE) and relies on
	// auto-detection for the recorder and CS2; validation used to run before
	// detection and log.Fatal on the incomplete trio, so the whole app failed
	// to start. Incompleteness after detection just leaves the record worker
	// disabled, which capabilities and the startup log already explain.
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "postgres://example")
	t.Setenv("ZV_RECORDER_PATH", "zv-recorder.exe")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v, want partial record config accepted", err)
	}
	if cfg.recordWorkerEnabled() {
		t.Fatal("record worker enabled with a partial trio, want disabled")
	}
}

func TestLoadConfigEnablesMediaWorkers(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "postgres://example")
	toolsDir := t.TempDir()
	fakeTool := func(name string) string {
		t.Helper()
		path := filepath.Join(toolsDir, name)
		if err := os.WriteFile(path, []byte("stub"), 0o700); err != nil {
			t.Fatalf("write fake tool %s: %v", name, err)
		}
		return path
	}
	t.Setenv("ZV_RECORDER_PATH", fakeTool("zv-recorder.exe"))
	t.Setenv("ZV_HLAE_PATH", fakeTool("HLAE.exe"))
	t.Setenv("ZV_CS2_PATH", fakeTool("cs2.exe"))
	t.Setenv("ZV_COMPOSER_PATH", fakeTool("zv-composer.exe"))
	t.Setenv("ZV_EDITOR_PATH", fakeTool("zv-editor.exe"))
	t.Setenv("ZV_FFMPEG_PATH", fakeTool("ffmpeg.exe"))
	t.Setenv("ZV_FFPROBE_PATH", fakeTool("ffprobe.exe"))
	t.Setenv("ZV_RECORD_TIMEOUT", "30m")
	t.Setenv("ZV_COMPOSE_TIMEOUT", "10m")
	t.Setenv("ZV_RENDER_TIMEOUT", "12m")
	t.Setenv("ZV_MEDIA_WORK_DIR", "C:\\zv-work")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if !cfg.recordWorkerEnabled() {
		t.Fatal("record worker disabled, want enabled")
	}
	if !cfg.composeWorkerEnabled() {
		t.Fatal("compose worker disabled, want enabled")
	}
	if !cfg.renderWorkerEnabled() {
		t.Fatal("render worker disabled, want enabled")
	}
	if cfg.RecordTimeout != "30m0s" {
		t.Fatalf("RecordTimeout = %q, want 30m0s", cfg.RecordTimeout)
	}
	if cfg.ComposeTimeout != "10m0s" {
		t.Fatalf("ComposeTimeout = %q, want 10m0s", cfg.ComposeTimeout)
	}
	if cfg.RenderTimeout != "12m0s" {
		t.Fatalf("RenderTimeout = %q, want 12m0s", cfg.RenderTimeout)
	}
	if cfg.MediaWorkDir != "C:\\zv-work" {
		t.Fatalf("MediaWorkDir = %q, want C:\\zv-work", cfg.MediaWorkDir)
	}
}

func TestConfiguredDirectoriesAndMissingToolsNeverEnableWorkers(t *testing.T) {
	dir := t.TempDir()
	cfg := config{
		RecorderPath: dir,
		HLAEPath:     filepath.Join(dir, "missing-hlae.exe"),
		CS2Path:      filepath.Join(dir, "missing-cs2.exe"),
		ComposerPath: dir,
		EditorPath:   dir,
		FFmpegPath:   dir,
		YtdlpPath:    dir,
	}
	if cfg.recordWorkerEnabled() || cfg.composeWorkerEnabled() || cfg.renderWorkerEnabled() ||
		cfg.streamRenderWorkerEnabled() || cfg.streamAcquireWorkerEnabled() {
		t.Fatalf("unusable configured paths enabled a worker: %#v", cfg)
	}
}

func TestExecutableAdmissionResolvesPATHBasenamesAndRejectsInvalidCommands(t *testing.T) {
	toolsDir := t.TempDir()
	toolName := "zv-path-tool"
	if runtime.GOOS == "windows" {
		toolName += ".exe"
	}
	toolPath := filepath.Join(toolsDir, toolName)
	if err := os.WriteFile(toolPath, []byte("stub"), 0o700); err != nil {
		t.Fatalf("write PATH tool: %v", err)
	}
	t.Setenv("PATH", toolsDir)

	cfg := config{
		RecorderPath: toolName,
		HLAEPath:     toolName,
		CS2Path:      toolName,
		ComposerPath: toolName,
		EditorPath:   toolName,
		FFmpegPath:   toolName,
		YtdlpPath:    toolName,
	}
	if !cfg.recordWorkerEnabled() || !cfg.composeWorkerEnabled() || !cfg.renderWorkerEnabled() ||
		!cfg.streamRenderWorkerEnabled() || !cfg.streamAcquireWorkerEnabled() {
		t.Fatalf("PATH basename did not enable workers: %#v", cfg)
	}

	nonExecutable := filepath.Join(toolsDir, "not-executable.txt")
	if err := os.WriteFile(nonExecutable, []byte("stub"), 0o600); err != nil {
		t.Fatalf("write non-executable file: %v", err)
	}
	missingName := "zv-missing-path-tool"
	if runtime.GOOS == "windows" {
		missingName += ".exe"
	}
	for _, tt := range []struct {
		name    string
		command string
	}{
		{name: "missing basename", command: missingName},
		{name: "explicit directory", command: toolsDir},
		{name: "non-executable file", command: nonExecutable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if admittedExecutable(tt.command) {
				t.Fatalf("admittedExecutable(%q) = true, want false", tt.command)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidDuration(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "postgres://example")
	t.Setenv("ZV_RECORD_TIMEOUT", "soon")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig error = nil, want invalid duration")
	}
}

func TestLoadConfigValidatesRecordHUDAtStartup(t *testing.T) {
	for _, hud := range []string{"", "gameplay", "clean", "deathnotices"} {
		t.Run("valid_"+hud, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			t.Setenv("ZV_RECORD_HUD", hud)
			if _, err := loadConfig(); err != nil {
				t.Fatalf("loadConfig(%q) error = %v", hud, err)
			}
		})
	}

	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	t.Setenv("ZV_RECORD_HUD", "death-notices")
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "ZV_RECORD_HUD") {
		t.Fatalf("loadConfig error = %v, want invalid HUD rejected at startup", err)
	}
}

func TestLoadConfigBoundsWorkerConcurrency(t *testing.T) {
	for _, concurrency := range []string{"1", "64"} {
		t.Run("valid_"+concurrency, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			t.Setenv("ZV_WORKER_CONCURRENCY", concurrency)
			cfg, err := loadConfig()
			if err != nil {
				t.Fatalf("loadConfig(%q) error = %v", concurrency, err)
			}
			if got := strconv.Itoa(cfg.WorkerConcurrency); got != concurrency {
				t.Fatalf("WorkerConcurrency = %s, want %s", got, concurrency)
			}
		})
	}

	for _, concurrency := range []string{"0", "65", "999999999999999999999"} {
		t.Run("invalid_"+concurrency, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			t.Setenv("ZV_WORKER_CONCURRENCY", concurrency)
			_, err := loadConfig()
			if err == nil || !strings.Contains(err.Error(), "between 1 and 64") {
				t.Fatalf("loadConfig(%q) error = %v, want bounded concurrency rejection", concurrency, err)
			}
		})
	}
}

func TestClearLegacyCaptionCredentialsEnvironment(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	t.Setenv(legacyGroqAPIKeyVariable, "legacy-team-secret")
	t.Setenv(legacyGroqAPIKeyOverrideVariable, "legacy-override-secret")
	t.Setenv(legacyXAIAPIKeyVariable, "xai-team-secret")
	// On Unix this is a separate variable; on Windows it exercises the native
	// case-insensitive environment. Either way, no casing variant may survive
	// into ffmpeg, HLAE, CS2, or yt-dlp.
	t.Setenv("xai_api_key", "lowercase-team-secret")

	if _, err := loadConfig(); err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if err := clearLegacyCaptionCredentialsEnvironment(); err != nil {
		t.Fatalf("clearLegacyCaptionCredentialsEnvironment error = %v", err)
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		for _, variable := range []string{
			legacyGroqAPIKeyVariable,
			legacyGroqAPIKeyOverrideVariable,
			legacyXAIAPIKeyVariable,
		} {
			if strings.EqualFold(name, variable) {
				t.Fatalf("environment still contains legacy caption credential %q", name)
			}
		}
	}
}

func TestLoadConfigFaceitAPIKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	t.Setenv("FACEIT_API_KEY", "faceit-test-secret")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.FaceitAPIKey != "faceit-test-secret" || !cfg.faceitEnabled() {
		t.Fatalf("faceit config = %q enabled=%v", cfg.FaceitAPIKey, cfg.faceitEnabled())
	}
}

func TestLoadConfigPrefersEnvFaceitKeyOverEmbedded(t *testing.T) {
	previous := embeddedFaceitAPIKey
	embeddedFaceitAPIKey = "embedded-faceit-secret"
	t.Cleanup(func() { embeddedFaceitAPIKey = previous })

	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	t.Setenv("FACEIT_API_KEY", "env-faceit-secret")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.FaceitAPIKey != "env-faceit-secret" {
		t.Fatalf("FaceitAPIKey = %q, want env override", cfg.FaceitAPIKey)
	}
}

func TestLoadConfigUsesEmbeddedFaceitKeyWhenEnvEmpty(t *testing.T) {
	previous := embeddedFaceitAPIKey
	embeddedFaceitAPIKey = "embedded-faceit-secret"
	t.Cleanup(func() { embeddedFaceitAPIKey = previous })

	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.FaceitAPIKey != "embedded-faceit-secret" || !cfg.faceitEnabled() {
		t.Fatalf("faceit embed = %q enabled=%v", cfg.FaceitAPIKey, cfg.faceitEnabled())
	}
}

func TestLoadConfigFirecrawlAPIKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")
	t.Setenv("FIRECRAWL_API_KEY", "fc-test-secret")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.FirecrawlAPIKey != "fc-test-secret" || !cfg.firecrawlEnabled() {
		t.Fatalf("firecrawl config = %q enabled=%v", cfg.FirecrawlAPIKey, cfg.firecrawlEnabled())
	}
}

func TestLoadConfigMusicDirDefaultsUnderDataDir(t *testing.T) {
	// Local Studio never sets ZV_MUSIC_DIR, so an empty value must resolve to the
	// on-disk library the repo ships at <DataDir>/music. Otherwise the songs API
	// returns an empty catalog and the web background-music picker stays blank.
	tests := []struct {
		name     string
		musicDir string
		dataDir  string
		want     string
	}{
		{
			name:     "explicit env wins verbatim",
			musicDir: "C:\\custom\\songs",
			dataDir:  "C:\\zv-data",
			want:     "C:\\custom\\songs",
		},
		{
			name:    "empty env defaults under configured data dir",
			dataDir: "C:\\zv-data",
			want:    filepath.Join("C:\\zv-data", "music"),
		},
		{
			name: "empty env defaults under default data dir",
			want: filepath.Join("./data", "music"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			if tt.dataDir != "" {
				t.Setenv("ZV_DATA_DIR", tt.dataDir)
			}
			if tt.musicDir != "" {
				t.Setenv("ZV_MUSIC_DIR", tt.musicDir)
			}
			cfg, err := loadConfig()
			if err != nil {
				t.Fatalf("loadConfig error = %v", err)
			}
			if got, want := cfg.MusicDir, tt.want; got != want {
				t.Fatalf("MusicDir = %q, want %q", got, want)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ZV_HTTP_ADDR",
		"ZV_DATABASE_URL",
		"ZV_DATA_DIR",
		"ZV_MUSIC_DIR",
		"ZV_RECORD_HUD",
		"ZV_YTDLP_PATH",
		"ZV_WORKER_CONCURRENCY",
		"ZV_MEDIA_WORK_DIR",
		"ZV_RECORDER_PATH",
		"ZV_COMPOSER_PATH",
		"ZV_EDITOR_PATH",
		"ZV_HLAE_PATH",
		"ZV_CS2_PATH",
		"ZV_FFMPEG_PATH",
		"ZV_FFPROBE_PATH",
		"ZV_RECORD_TIMEOUT",
		"ZV_COMPOSE_TIMEOUT",
		"ZV_RENDER_TIMEOUT",
		"ZV_MUTATION_TOKEN",
		"ZV_CAPTURE_LAB_SEED",
		"ZV_CAPTURE_LAB_EVIDENCE_ROOT",
		"XAI_API_KEY",
		"GROQ_API_KEY",
		"ZV_GROQ_API_KEY",
		"FIRECRAWL_API_KEY",
		"FACEIT_API_KEY",
		"ZV_BRIDGE_URL",
		"ZV_BRIDGE_TOKEN",
		"ZV_CLOUD_URL",
		"ZV_CLOUD_WORKER_MIN_FREE_BYTES",
		"ZV_STUDIO_VERSION",
	} {
		t.Setenv(key, "")
	}
	t.Setenv(mutationTokenEnvironmentVariable, strings.Repeat("a", 64))
}

func TestLoadConfigCloudURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr string
	}{
		{name: "unset means the production portal", want: "https://cliphub.gravityroom.app"},
		{name: "off disables the cloud client", value: "off", want: ""},
		{name: "off in any case", value: " OFF ", want: ""},
		{name: "another https portal", value: "https://portal.example/", want: "https://portal.example"},
		{name: "a local portal over http", value: "http://127.0.0.1:3000", want: "http://127.0.0.1:3000"},
		{name: "localhost over http", value: "http://localhost:3000", want: "http://localhost:3000"},
		{name: "cleartext to another host", value: "http://portal.example", wantErr: "https://"},
		{name: "another scheme to loopback", value: "ftp://127.0.0.1", wantErr: "https://"},
		{name: "not a URL", value: "portal", wantErr: "valid absolute URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			t.Setenv("ZV_CLOUD_URL", tt.value)

			cfg, err := loadConfig()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), "ZV_CLOUD_URL") || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("loadConfig error = %v, want ZV_CLOUD_URL rejected mentioning %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig error = %v", err)
			}
			if cfg.CloudURL != tt.want {
				t.Fatalf("CloudURL = %q, want %q", cfg.CloudURL, tt.want)
			}
		})
	}
}

func TestLoadConfigCloudWorkerSettings(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ZV_DATABASE_URL", "memory")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.CloudWorkerMinFreeBytes != 32212254720 {
		t.Fatalf("default CloudWorkerMinFreeBytes = %d, want 30 GiB", cfg.CloudWorkerMinFreeBytes)
	}
	if cfg.BridgeURL != "" || cfg.StudioVersion != "" {
		t.Fatalf("worker mode on by default: bridge=%q studio=%q", cfg.BridgeURL, cfg.StudioVersion)
	}

	t.Setenv("ZV_BRIDGE_URL", "https://portal.example")
	t.Setenv("ZV_BRIDGE_TOKEN", strings.Repeat("b", 64))
	t.Setenv("ZV_CLOUD_WORKER_MIN_FREE_BYTES", "53687091200")
	t.Setenv("ZV_STUDIO_VERSION", " 5.4.4 ")
	cfg, err = loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.BridgeURL != "https://portal.example" || cfg.CloudWorkerMinFreeBytes != 53687091200 || cfg.StudioVersion != "5.4.4" {
		t.Fatalf("config = bridge %q, floor %d, studio %q", cfg.BridgeURL, cfg.CloudWorkerMinFreeBytes, cfg.StudioVersion)
	}

	for _, invalid := range []string{"30GB", "-1", "1.5"} {
		t.Setenv("ZV_CLOUD_WORKER_MIN_FREE_BYTES", invalid)
		if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "ZV_CLOUD_WORKER_MIN_FREE_BYTES") {
			t.Fatalf("loadConfig with floor %q error = %v, want it rejected by name", invalid, err)
		}
	}
}

func TestLoadConfigRejectsAHalfConfiguredOrCleartextBridge(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		token string
	}{
		{name: "URL without token", url: "https://portal.example"},
		{name: "token without URL", token: strings.Repeat("b", 64)},
		{name: "weak token", url: "https://portal.example", token: "secret"},
		{name: "cleartext to another host", url: "http://portal.example", token: strings.Repeat("b", 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("ZV_DATABASE_URL", "memory")
			t.Setenv("ZV_BRIDGE_URL", tt.url)
			t.Setenv("ZV_BRIDGE_TOKEN", tt.token)
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "ZV_BRIDGE_") {
				t.Fatalf("loadConfig error = %v, want the bridge configuration rejected", err)
			}
		})
	}
}
