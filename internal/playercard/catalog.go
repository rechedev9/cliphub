// Package playercard draws the player cards of a 9:16 Short: the templates,
// the facts of the match they show and the HTML the overlay renderer draws.
package playercard

import "slices"

// Template is one card design: its id, the name the UI shows and the facts it needs.
type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Needs       []string `json:"needs"`
}

// The facts a template can need. A fact the demo lacks is left out of the card.
const (
	NeedRating = "rating"
	NeedRole   = "role"
	NeedStats  = "stats"
	NeedImage  = "image"
	NeedTeam   = "team"
	NeedResult = "result"
)

// Templates is the one catalog of card designs; the UI, the editor and MCP read it.
var Templates = []Template{
	{ID: "carta", Name: "Carta", Description: "La valoración en grande, el rol y seis estadísticas de la partida.", Needs: []string{NeedRating, NeedRole, NeedStats, NeedImage}},
	{ID: "ficha", Name: "Ficha", Description: "La foto en grande, el nombre, el equipo y tres datos de la partida.", Needs: []string{NeedImage, NeedTeam, NeedStats}},
	{ID: "mvp", Name: "MVP", Description: "El resultado de la partida y la mejor estadística del jugador.", Needs: []string{NeedResult, NeedStats, NeedImage}},
}

// Lookup returns the catalog template with this id.
func Lookup(id string) (Template, bool) {
	i := slices.IndexFunc(Templates, func(t Template) bool { return t.ID == id })
	if i < 0 {
		return Template{}, false
	}
	return Templates[i], true
}

// IDs lists the template ids in catalog order.
func IDs() []string {
	ids := make([]string, 0, len(Templates))
	for _, t := range Templates {
		ids = append(ids, t.ID)
	}
	return ids
}
