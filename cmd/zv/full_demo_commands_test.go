package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

const fullDemoCLIJobID = "e16560b4-6ee2-48a6-8e40-15e5d712856d"

func fullDemoCLIFixture(t *testing.T) (recapplan.Snapshot, string, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "web", "lib", "full-demo-plan.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot recapplan.Snapshot
	if err := json.Unmarshal(b, &snapshot); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plan, options := filepath.Join(dir, "plan.json"), filepath.Join(dir, "options.json")
	if err := writeJSONArtifact(plan, snapshot.Document); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONArtifact(options, snapshot.Document.Options); err != nil {
		t.Fatal(err)
	}
	return snapshot, plan, options
}

func fullDemoSampleArgs(t *testing.T, name string) []string {
	t.Helper()
	if name == "full-demo-defaults" {
		return []string{"--format", "json"}
	}
	snapshot, plan, options := fullDemoCLIFixture(t)
	switch name {
	case "full-demo-import":
		return []string{"--demo", filepath.Join(repoRoot(t), "testdata", "agent-demo.fixture"), "--steamid", snapshot.Document.Input.TargetSteamID64, "--dry-run"}
	case "full-demo-asset":
		media := filepath.Join(filepath.Dir(plan), "asset.wav")
		writeFile(t, media, "upload bytes for transport validation only")
		b, err := os.ReadFile(media)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(b)
		provenance := filepath.Join(filepath.Dir(plan), "provenance.json")
		if err := writeJSONArtifact(provenance, mediaassets.Provenance{SchemaVersion: "1.0", AssetSHA256: hex.EncodeToString(digest[:]), Title: "Test asset", Creator: "Test", SourceURL: "local:test", Permission: "Owned synthetic test material"}); err != nil {
			t.Fatal(err)
		}
		return []string{"--input", media, "--provenance", provenance, "--dry-run"}
	case "full-demo-plan":
		return []string{"--job", fullDemoCLIJobID, "--options", options, "--out", filepath.Join(filepath.Dir(plan), "planned.json"), "--dry-run"}
	case "full-demo-inspect":
		return []string{"--plan", plan}
	case "full-demo-execute":
		return []string{"--job", fullDemoCLIJobID, "--plan", plan, "--approve", snapshot.Document.PlanHash, "--allow-safe-tail-trim=true", "--dry-run"}
	default:
		t.Fatalf("unknown Full Demo workflow %s", name)
		return nil
	}
}

func TestFullDemoCLIPlanningAndApprovedAdmission(t *testing.T) {
	snapshot, plan, options := fullDemoCLIFixture(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-ClipHub-Token") != "test-local-session" {
			t.Error("missing session token")
		}
		switch r.URL.Path {
		case "/api/jobs/" + fullDemoCLIJobID + "/full-demo/plan":
			if r.Method != http.MethodPost {
				t.Errorf("method = %s", r.Method)
			}
			var request struct {
				Options recapplan.Options `json:"options"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(request.Options, snapshot.Document.Options) {
				t.Error("options changed across CLI transport")
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(snapshot.Document)
		case "/api/jobs/" + fullDemoCLIJobID + "/generate":
			var request struct {
				Preset     string                 `json:"preset"`
				SegmentIDs []string               `json:"segment_ids"`
				Edit       renderplan.EditRequest `json:"edit"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Preset != "gameplay-pov-60" || request.SegmentIDs == nil || len(request.SegmentIDs) != 0 {
				t.Error("request changed pipeline or selected legacy segments")
			}
			if err := request.Edit.Validate(); err != nil {
				t.Error(err)
			}
			if request.Edit.FullDemo == nil || !reflect.DeepEqual(request.Edit.FullDemo.Document, snapshot.Document) || request.Edit.FullDemo.Approval.PlanHash != snapshot.Document.PlanHash {
				t.Error("approval/document changed")
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": fullDemoCLIJobID, "status": "queued", "variant": "gameplay-pov-60"})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("ZV_MUTATION_TOKEN", "test-local-session")
	for _, command := range []string{"plan", "execute"} {
		for _, dry := range []bool{true, false} {
			t.Run(command+"/dry="+map[bool]string{true: "true", false: "false"}[dry], func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "result.json")
				args := []string{command, "--job", fullDemoCLIJobID, "--url", server.URL, "--format", "json"}
				if command == "plan" {
					args = append(args, "--options", options, "--out", output)
				} else {
					args = append(args, "--plan", plan, "--approve", snapshot.Document.PlanHash, "--allow-safe-tail-trim=true")
				}
				if dry {
					args = append(args, "--dry-run")
				}
				before := calls.Load()
				var stdout, stderr bytes.Buffer
				if code := runFullDemo(args, &stdout, &stderr); code != 0 {
					t.Fatalf("code %d: %s %s", code, stdout.String(), stderr.String())
				}
				if dry {
					if calls.Load() != before {
						t.Fatal("dry-run contacted the server")
					}
					if _, err := os.Stat(output); !os.IsNotExist(err) {
						t.Fatal("dry-run wrote an artifact")
					}
				} else if calls.Load() != before+1 {
					t.Fatal("expected exactly one request")
				}
				if !dry && command == "plan" {
					d, err := recapplan.ReadDocumentFile(output)
					if err != nil || d.PlanHash != snapshot.Document.PlanHash {
						t.Fatalf("saved plan differs: %v", err)
					}
				}
			})
		}
	}
}

func TestFullDemoCLIBoundaryRejectsAmbiguityBeforeHTTP(t *testing.T) {
	snapshot, plan, options := fullDemoCLIFixture(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing approval", []string{"execute", "--job", fullDemoCLIJobID, "--plan", plan, "--allow-safe-tail-trim=true"}},
		{"different approval", []string{"execute", "--job", fullDemoCLIJobID, "--plan", plan, "--approve", strings.Repeat("0", 64), "--allow-safe-tail-trim=true"}},
		{"missing safety choice", []string{"execute", "--job", fullDemoCLIJobID, "--plan", plan, "--approve", snapshot.Document.PlanHash}},
		{"different safety choice", []string{"execute", "--job", fullDemoCLIJobID, "--plan", plan, "--approve", snapshot.Document.PlanHash, "--allow-safe-tail-trim=false"}},
		{"invalid job", []string{"plan", "--job", "../other", "--options", options, "--out", "unused.json"}},
		{"overwrite input", []string{"plan", "--job", fullDemoCLIJobID, "--options", options, "--out", options}},
		{"ambiguous inspection", []string{"inspect", "--plan", plan, "--job", fullDemoCLIJobID}},
		{"unknown option", []string{"defaults", "--voice-volume", "0"}},
		{"duplicate option", []string{"defaults", "--format", "json", "--format", "text"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runFullDemo(tc.args, &stdout, &stderr); code != exitInvalidArgs {
				t.Fatalf("code=%d: %s %s", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestFullDemoCLIRejectsRemoteOriginsAndRedirects(t *testing.T) {
	for _, origin := range []string{"https://example.com", "http://example.com", "file:///test", "http://secret@127.0.0.1", "http://127.0.0.1/api", "http://127.0.0.1?token=secret"} {
		t.Run(origin, func(t *testing.T) {
			if _, err := fullDemoCLIClient(origin); err == nil {
				t.Fatal("unsafe origin accepted")
			}
		})
	}
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinationCalls.Add(1); _, _ = io.WriteString(w, `{}`) }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client, err := fullDemoCLIClient(source.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetFullDemoPlan(t.Context(), fullDemoCLIJobID); err == nil {
		t.Fatal("redirect accepted")
	}
	if destinationCalls.Load() != 0 {
		t.Fatal("redirect destination received the request")
	}
}
