package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestComboPlayNotesCountOneRound(t *testing.T) {
	pb := tuiclient.TeamPlaybook{Side: "CT", Combo: tuiclient.PlaybookCombo{Without: []string{"site:A"}},
		Rounds: []tuiclient.PlaybookRound{{MatchID: testMatchID, Round: 4, Pattern: "stack", Site: "B", Won: true}}}
	p := comboPlay(pb, testTeamID, "Sin A")
	if p.Notes != "Combinación del playbook: sin site:A. Gana el 100 % de 1 ronda." || p.Kind != "stack" || len(p.Examples) != 1 {
		t.Errorf("play = %+v", p)
	}
}

const testComboPlaybook = `{"team_name":"Vitality","skipped":[],"map":"de_mirage","side":"T","matches":1,"round_count":3,"win_rate":{"pct":66.7},
	"framed":3,"categories":[],"tempo":{},"sites":[],"zones":[],
	"tactics":[{"key":"utility:smoke@Window","kind":"utility","value":"Window","grenade":"smoke","label":"Ventana","rounds":3,"share":{"pct":100},"win_rate":{"pct":66.7},"refs":[]},
	{"key":"awp:TopofMid","kind":"awp","value":"TopofMid","label":"Top of Mid","rounds":1,"share":{"pct":33.3},"win_rate":{"pct":0},"refs":[]}],
	"combo":{"with":["utility:smoke@Window"],"without":["site:A"]},
	"rounds":[{"match_id":"` + testMatchID + `","round":3,"tick":300,"pattern":"execute","site":"B","won":true},
	{"match_id":"` + testMatchID + `","round":5,"tick":500,"pattern":"split","site":"B","won":false},
	{"match_id":"` + testMatchID + `","round":8,"tick":800,"pattern":"execute","site":"mid","won":true}]}`

func comboStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/teams/{id}/playbook", testComboPlaybook)
	rec.handle(mux, "POST /api/library/plays", testPlayJSON)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) {
		registerTeamReportTools(server, s)
		registerPlaybookComboTools(server, s)
	})
	return cs, rec
}

func TestTeamPlaybookCombinesTactics(t *testing.T) {
	cs, rec := comboStudio(t)
	var pb teamPlaybookOut
	callTool(t, cs, "team_playbook", map[string]any{"team_id": testTeamID, "map": "de_mirage", "side": "T",
		"with": []string{"utility:smoke@Window"}, "without": []string{"site:A"}}, &pb)
	got := rec.get("GET /api/library/teams/{id}/playbook")
	if !strings.Contains(got, "with=utility%3Asmoke%40Window") || !strings.Contains(got, "without=site%3AA") {
		t.Fatalf("playbook request = %s, want the combination", got)
	}
	if len(pb.Tactics) != 2 || pb.Tactics[0].Key != "utility:smoke@Window" || pb.Tactics[0].Name != "smoke at Ventana" || pb.Tactics[1].Name != "awp at Top of Mid" {
		t.Fatalf("tactics = %+v", pb.Tactics)
	}
	if len(pb.With) != 1 || len(pb.Without) != 1 {
		t.Errorf("combo = %v / %v", pb.With, pb.Without)
	}
	toolFails(t, cs, "team_playbook", map[string]any{"team_id": testTeamID, "map": "de_mirage", "side": "T", "with": []string{"weapon:awp"}},
		"not a tactic key")
}

func TestSavePlaybookComboPostsThePlay(t *testing.T) {
	cs, rec := comboStudio(t)
	var p playDetailOut
	callTool(t, cs, "save_playbook_combo", map[string]any{"team_id": testTeamID, "map": "de_mirage", "side": "T",
		"with": []string{"utility:smoke@Window"}, "without": []string{"site:A"}, "title": "Humo en Ventana"}, &p)
	posted := rec.get("POST /api/library/plays")
	_, body, _ := strings.Cut(posted, " {")
	var play struct {
		Title, Map, Side, Kind, Site, Origin, Notes string
		Examples                                    []struct{ Round int }
	}
	if err := json.Unmarshal([]byte("{"+body), &play); err != nil {
		t.Fatalf("posted %s: %v", posted, err)
	}
	if play.Origin != "playbook" || play.Kind != "execute" || play.Site != "B" || play.Title != "Humo en Ventana" || len(play.Examples) != 3 {
		t.Errorf("posted play = %+v", play)
	}
	if play.Notes != "Combinación del playbook: con utility:smoke@Window; sin site:A. Gana el 67 % de 3 rondas." {
		t.Errorf("notes = %q", play.Notes)
	}
	toolFails(t, cs, "save_playbook_combo", map[string]any{"team_id": testTeamID, "map": "de_mirage", "side": "T", "title": "x"},
		"at least one tactic key")
}
