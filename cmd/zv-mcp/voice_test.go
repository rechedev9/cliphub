package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testVoiceJSON = `{"match_id":"` + testMatchID + `","present":true,"duration_ms":3725000,"speech_ms":900000,` +
	`"tracks":[{"track":1,"label":"ts3","file_name":"ts3.wav","channel":-1,"duration_ms":3725000,"source":"upload"}],` +
	`"sync":{"state":"unsynced","method":"round_start","offset_ms":-4000,"auto_offset_ms":-4000,"confidence":0,"note":"no fit"},` +
	`"transcript":{"state":"none"},"transcriber":{"state":"not_installed"}}`

const testCommsJSON = `{"match_id":"` + testMatchID + `","sync":{"state":"manual","offset_ms":1000},"transcript":{"state":"ready"},` +
	`"rounds":[{"round":3,"speakers":[{"track":1,"label":"IGL","freezetime_ms":4000,"midround_ms":6000}]}],` +
	`"totals":[{"track":1,"label":"IGL","freezetime_ms":4000,"midround_ms":6000}],` +
	`"lines":[{"track":1,"label":"IGL","round":3,"round_ms":65000,"phase":"midround","tick":9000,"text":"rotamos a B"}]}`

func voiceStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/matches/{id}/voice", testVoiceJSON)
	rec.handle(mux, "POST /api/library/matches/{id}/voice", testVoiceJSON)
	rec.handle(mux, "PATCH /api/library/matches/{id}/voice", testVoiceJSON)
	rec.handle(mux, "DELETE /api/library/matches/{id}/voice", ``)
	rec.handle(mux, "GET /api/library/matches/{id}/voice/comms", testCommsJSON)
	rec.handle(mux, "GET /api/library/matches/{id}/voice/demo", `{"match_id":"`+testMatchID+`","state":"none","tracks":[]}`)
	rec.handle(mux, "POST /api/library/matches/{id}/voice/demo", `{"match_id":"`+testMatchID+`","state":"extracting","tracks":[]}`)
	rec.handle(mux, "GET /api/library/voice/transcriber", `{"state":"not_installed","version":"1.9.5","model":"ggml-small-q5_1.bin","download_bytes":199014127}`)
	rec.handle(mux, "POST /api/library/voice/transcriber/install", `{"state":"installing","version":"1.9.5","model":"ggml-small-q5_1.bin","download_bytes":199014127}`)
	rec.handle(mux, "GET /api/library/voice/search", `{"hits":[{"match_id":"`+testMatchID+`","map":"de_mirage","played_at":"2026-11-01T18:00:00Z","teams":["FUT","OG"],`+
		`"track":1,"label":"IGL","round":3,"round_ms":65000,"phase":"midround","tick":9000,"text":"rotamos a B"},`+
		`{"match_id":"`+testMatchID+`","map":"de_mirage","played_at":"2026-11-01T18:00:00Z","teams":[],"round":4,"phase":"freezetime","tick":9500,"text":"rotamos ya"}],"truncated":false}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerVoiceTools(server, s) })
	return cs, rec
}

func TestVoiceToolsImportSyncAndDelete(t *testing.T) {
	cs, rec := voiceStudio(t)
	audio := filepath.Join(t.TempDir(), "ts3.wav")
	if err := os.WriteFile(audio, []byte("RIFF"), 0o600); err != nil {
		t.Fatal(err)
	}
	var v voiceOut
	callTool(t, cs, "import_voice", map[string]any{"match_id": testMatchID, "audio_path": audio}, &v)
	if !strings.Contains(rec.get("POST /api/library/matches/{id}/voice"), `name="audio"; filename="ts3.wav"`) {
		t.Errorf("import body = %s", rec.get("POST /api/library/matches/{id}/voice"))
	}
	if v.Sync.Label != "La voz aún no está sincronizada" || v.Duration != "1:02:05" || v.Sync.OffsetMS != -4000 {
		t.Fatalf("import = %+v", v)
	}

	callTool(t, cs, "sync_voice", map[string]any{"match_id": testMatchID, "offset_ms": -2500}, &v)
	if got := rec.get("PATCH /api/library/matches/{id}/voice"); !strings.HasSuffix(got, `{"offset_ms":-2500}`) {
		t.Errorf("sync body = %s", got)
	}
	callTool(t, cs, "sync_voice", map[string]any{"match_id": testMatchID, "auto": true}, &v)
	if got := rec.get("PATCH /api/library/matches/{id}/voice"); !strings.HasSuffix(got, `{"auto":true}`) {
		t.Errorf("auto body = %s", got)
	}
	toolFails(t, cs, "sync_voice", map[string]any{"match_id": testMatchID}, "offset_ms, or auto")

	var del deleteOut
	callTool(t, cs, "delete_voice", map[string]any{"match_id": testMatchID}, &del)
	if !del.NeedsApproval || rec.get("DELETE /api/library/matches/{id}/voice") != "" || !strings.Contains(del.Summary, "1 pista") {
		t.Fatalf("delete without approval = %+v", del)
	}
	callTool(t, cs, "delete_voice", map[string]any{"match_id": testMatchID, "approve": del.ApprovalCode}, &del)
	if !del.Deleted || rec.get("DELETE /api/library/matches/{id}/voice") == "" {
		t.Fatalf("approved delete = %+v", del)
	}
}

func TestVoiceCommsToolsRoundTalkAndSearch(t *testing.T) {
	cs, rec := voiceStudio(t)
	var round roundCommsOut
	callTool(t, cs, "get_round_comms", map[string]any{"match_id": testMatchID, "round": 3}, &round)
	if got := rec.get("GET /api/library/matches/{id}/voice/comms"); !strings.Contains(got, "round=3") {
		t.Errorf("comms query = %s", got)
	}
	if len(round.Speakers) != 1 || round.Speakers[0].TotalMS != 10000 || len(round.Lines) != 1 {
		t.Fatalf("round comms = %+v", round)
	}
	if l := round.Lines[0]; l.RoundClock != "1:05" || l.StudioPath != "/replay?r="+testMatchID+":3:9000" {
		t.Fatalf("line = %+v", l)
	}

	var talk matchTalkOut
	callTool(t, cs, "match_talk_time", map[string]any{"match_id": testMatchID}, &talk)
	if len(talk.Totals) != 1 || len(talk.Rounds) != 1 || talk.Sync != "Sincronizada a mano" {
		t.Fatalf("talk = %+v", talk)
	}

	var found searchCommsOut
	callTool(t, cs, "search_comms", map[string]any{"text": " rotamos ", "team_id": testTeamID, "from": "2026-11-01", "period": "midround"}, &found)
	got := rec.get("GET /api/library/voice/search")
	if !strings.Contains(got, "q=rotamos") || !strings.Contains(got, "team_id="+testTeamID) || !strings.Contains(got, "from=2026-11-01T00%3A00%3A00Z") || !strings.Contains(got, "phase=midround") || !strings.Contains(got, "limit=50") {
		t.Errorf("search query = %s", got)
	}
	// The server filters by phase; the tool keeps what it returns.
	if len(found.Hits) != 2 || found.Hits[0].Round != 3 || found.Hits[0].PlayedAt != "2026-11-01" {
		t.Fatalf("search = %+v", found)
	}
	toolFails(t, cs, "search_comms", map[string]any{"text": "eco", "period": "lunch"}, "period must be")
	toolFails(t, cs, "search_comms", map[string]any{"text": ""}, "text must be")
}

func TestInstallTranscriberWaitsForApproval(t *testing.T) {
	cs, rec := voiceStudio(t)
	var out transcriberOut
	callTool(t, cs, "install_transcriber", map[string]any{}, &out)
	if !out.NeedsApproval || out.ApprovalCode == "" || !strings.Contains(out.Summary, "189 MB") || rec.get("POST /api/library/voice/transcriber/install") != "" {
		t.Fatalf("install without approval = %+v", out)
	}
	callTool(t, cs, "install_transcriber", map[string]any{"approve": out.ApprovalCode}, &out)
	if out.State != "installing" || rec.get("POST /api/library/voice/transcriber/install") == "" {
		t.Fatalf("approved install = %+v", out)
	}
}
func TestDemoVoiceReadsAndStartsTheExtraction(t *testing.T) {
	cs, rec := voiceStudio(t)
	var out demoVoiceOut
	callTool(t, cs, "demo_voice", map[string]any{"match_id": testMatchID}, &out)
	if out.State != "none" || rec.get("POST /api/library/matches/{id}/voice/demo") != "" {
		t.Fatalf("read only = %+v", out)
	}
	callTool(t, cs, "demo_voice", map[string]any{"match_id": testMatchID, "extract": true}, &out)
	if out.State != "extracting" || rec.get("POST /api/library/matches/{id}/voice/demo") == "" {
		t.Fatalf("extract = %+v", out)
	}
}
