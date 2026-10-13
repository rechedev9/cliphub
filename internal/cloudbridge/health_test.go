package cloudbridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCS2PatchVersionComesFromSteamInf(t *testing.T) {
	root := t.TempDir()
	cs2 := filepath.Join(root, "Counter-Strike Global Offensive", "game", "bin", "win64", "cs2.exe")
	writeFile(t, cs2, "")
	steamInf := filepath.Join(root, "Counter-Strike Global Offensive", "game", "csgo", "steam.inf")

	if got := cs2PatchVersion(cs2); got != "" {
		t.Fatalf("version without steam.inf = %q, want empty rather than a guess", got)
	}
	writeFile(t, steamInf, "ClientVersion=2000700\r\nServerVersion=2000700\r\nPatchVersion=1.41.8.5\r\nProductName=cs2\r\nappID=730\r\n")
	if got := cs2PatchVersion(cs2); got != "1.41.8.5" {
		t.Fatalf("version = %q, want 1.41.8.5", got)
	}
	writeFile(t, steamInf, "PatchVersion=<script>\n")
	if got := cs2PatchVersion(cs2); got != "" {
		t.Fatalf("version from a malformed steam.inf = %q, want empty", got)
	}
	if got := cs2PatchVersion(""); got != "" {
		t.Fatalf("version without a CS2 path = %q, want empty", got)
	}
}

func TestHLAEVersionPrefersTheInstallMarker(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name   string
		dir    string
		marker string
		want   string
	}{
		{name: "Studio install with marker", dir: filepath.Join(root, "a", "tools", "hlae", "2.192.6"), marker: `{"version":"2.192.6","sourceSha256":"abc"}`, want: "2.192.6"},
		{name: "interim Studio build", dir: filepath.Join(root, "b", "tools", "hlae", "2.192.2-cliphub.1"), marker: `{"version":"2.192.2-cliphub.1"}`, want: "2.192.2-cliphub.1"},
		{name: "Studio install without marker", dir: filepath.Join(root, "c", "tools", "hlae", "2.192.5"), want: "2.192.5"},
		{name: "manual install on C", dir: filepath.Join(root, "HLAE-2.192.4"), want: "2.192.4"},
		{name: "unversioned folder", dir: filepath.Join(root, "HLAE"), want: ""},
		{name: "marker that is not a version", dir: filepath.Join(root, "d", "portable"), marker: `{"version":"latest"}`, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exe := filepath.Join(tc.dir, "HLAE.exe")
			writeFile(t, exe, "")
			if tc.marker != "" {
				writeFile(t, filepath.Join(tc.dir, ".cliphub-install.json"), tc.marker)
			}
			if got := hlaeVersion(exe); got != tc.want {
				t.Fatalf("hlaeVersion = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSystemMachineChecksSteamLessOftenWhileCapturing(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	checks := 0
	steamUp := true
	machine := NewSystemMachine(MachineConfig{DataDir: t.TempDir(), RecordEnabled: true, Kinds: []string{kindShort}})
	machine.now = func() time.Time { return now }
	machine.running = func(_ context.Context, image string) (bool, error) {
		if image != steamImage {
			t.Fatalf("unexpected process check for %q", image)
		}
		checks++
		return steamUp, nil
	}
	ctx := context.Background()

	first := machine.Health(ctx, false)
	if !first.SteamRunning || !first.RecordEnabled || checks != 1 {
		t.Fatalf("first health = %+v after %d checks, want Steam running after one check", first, checks)
	}
	if first.DiskTotalBytes == 0 || first.DiskFreeBytes > first.DiskTotalBytes {
		t.Fatalf("disk = %d free of %d, want the real volume of the data dir", first.DiskFreeBytes, first.DiskTotalBytes)
	}

	// During a capture the last answer is reused for a minute, so the
	// heartbeat does not shell out next to CS2 every 15 seconds.
	steamUp = false
	now = now.Add(45 * time.Second)
	if health := machine.Health(ctx, true); !health.SteamRunning || checks != 1 {
		t.Fatalf("health while capturing = %+v after %d checks, want the cached answer", health, checks)
	}
	// Idle, the same age is stale and Steam's exit is noticed.
	if health := machine.Health(ctx, false); health.SteamRunning || checks != 2 {
		t.Fatalf("idle health = %+v after %d checks, want a fresh check that sees Steam gone", health, checks)
	}

	// A process listing that fails is not evidence that Steam runs.
	now = now.Add(time.Minute)
	machine.running = func(context.Context, string) (bool, error) { return true, errors.New("tasklist failed") }
	if health := machine.Health(ctx, false); health.SteamRunning {
		t.Fatal("Steam reported running although the process listing failed")
	}
}

// The portal compares this value with the planSchema a Studio writes into a
// job spec, which is the schema version of the plan it parsed.
func TestHealthReportsTheKillPlanSchemaThisWorkerParsesWith(t *testing.T) {
	machine := NewSystemMachine(MachineConfig{DataDir: t.TempDir()})
	machine.running = func(context.Context, string) (bool, error) { return false, nil }
	health := machine.Health(context.Background(), false)
	if health.PlanSchema == "" || health.PlanSchema != killplan.SchemaVersion {
		t.Fatalf("planSchema = %q, want the kill plan schema %q", health.PlanSchema, killplan.SchemaVersion)
	}
	plan, err := json.Marshal(killplan.NewPlan())
	if err != nil {
		t.Fatal(err)
	}
	var written struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(plan, &written); err != nil {
		t.Fatal(err)
	}
	if written.SchemaVersion != health.PlanSchema {
		t.Fatalf("a plan written by this build carries schema %q, the heartbeat says %q", written.SchemaVersion, health.PlanSchema)
	}
	wire, err := json.Marshal(health)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), `"planSchema":"`+killplan.SchemaVersion+`"`) {
		t.Fatalf("heartbeat health = %s, want it to carry planSchema", wire)
	}
}
