package anticheat

import (
	"fmt"
	"regexp"
	"strings"
)

// steamID64Pattern is the 17-digit form of a SteamID64. Validating before
// building a profile URL keeps an unchecked identifier out of an outbound link.
var steamID64Pattern = regexp.MustCompile(`^7656119[0-9]{10}$`)

// Dossier is the evidence pack for one player: a human-readable summary of
// what the analysis measured, the exact ticks a reviewer can seek to, and the
// legitimate channels through which the user can file a report themselves.
//
// It is deliberately not a submission client. ClipHub never sends a report
// anywhere; it prepares the material and points at the official flow.
type Dossier struct {
	SteamID64  string  `json:"steamid64"`
	Name       string  `json:"name"`
	Verdict    Verdict `json:"verdict"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
	// ProfileURL is the player's Steam profile, empty when the SteamID64 is
	// not well formed.
	ProfileURL string `json:"profile_url,omitempty"`
	// Markdown is the full dossier body, ready to be copied into a report or
	// saved next to the demo.
	Markdown string `json:"markdown"`
	// Channels are the ways a user can escalate this themselves.
	Channels []ReportChannel `json:"channels"`
	// Policy is the honest framing that travels with every dossier.
	Policy ReportPolicy `json:"policy"`
}

// ReportChannel is one legitimate place to file a report.
type ReportChannel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// URL is empty for channels that exist only inside the game client.
	URL          string `json:"url,omitempty"`
	Instructions string `json:"instructions"`
	// Effective marks the channels that actually feed an anti-cheat decision,
	// as opposed to the ones that only handle profile or content violations.
	Effective bool `json:"effective"`
}

// ReportPolicy states what this tool will and will not do with a report, and
// why. It is part of the artifact so no downstream surface can drop it.
type ReportPolicy struct {
	Summary  string   `json:"summary"`
	Rules    []string `json:"rules"`
	Rejected string   `json:"rejected"`
}

// reportPolicy is fixed. Mass or automated reporting is refused here rather
// than at the UI layer, so every consumer of the dossier gets the same answer.
func reportPolicy() ReportPolicy {
	return ReportPolicy{
		Summary: "ClipHub prepara el expediente; la denuncia la presentas tú, una sola vez, desde tu propia cuenta.",
		Rules: []string{
			"Los baneos por trampas en CS2 los decide Valve por detección automática y revisión propia, no por número de denuncias recibidas.",
			"Denunciar en masa o de forma coordinada no aumenta la probabilidad de baneo y va contra el Acuerdo de Suscriptor de Steam.",
			"Una denuncia con evidencia concreta (demo, tick, ronda) vale más que muchas denuncias vacías.",
			"Esta herramienta mide anomalías estadísticas: úsala para decidir si merece la pena mirar la demo, no como veredicto.",
		},
		Rejected: "ClipHub no envía denuncias automáticamente ni genera denuncias múltiples contra una cuenta.",
	}
}

// BuildDossier renders the evidence pack for one player of a report.
func BuildDossier(report Report, player PlayerReport) Dossier {
	d := Dossier{
		SteamID64:  player.SteamID64,
		Name:       player.Name,
		Verdict:    player.Verdict,
		Score:      player.Score,
		Confidence: player.Confidence,
		Channels:   reportChannels(player.SteamID64),
		Policy:     reportPolicy(),
	}
	if steamID64Pattern.MatchString(player.SteamID64) {
		d.ProfileURL = "https://steamcommunity.com/profiles/" + player.SteamID64
	}
	d.Markdown = renderDossierMarkdown(report, player, d.ProfileURL)
	return d
}

// reportChannels lists the legitimate escalation paths, most effective first.
func reportChannels(steamID64 string) []ReportChannel {
	channels := []ReportChannel{
		{
			ID:    "cs2_in_game",
			Label: "Denuncia dentro de CS2",
			Instructions: "Es el único canal que alimenta la revisión anti-trampas de Valve. Abre CS2, entra en la partida " +
				"desde tu historial o en el marcador de una partida en curso, selecciona al jugador y usa «Denunciar» " +
				"marcando el motivo de trampas. Se hace una vez por partida y por cuenta.",
			Effective: true,
		},
		{
			ID:    "steam_profile",
			Label: "Denuncia del perfil de Steam",
			URL:   "https://steamcommunity.com/profiles/" + steamID64,
			Instructions: "En el perfil, «Más» → «Denunciar a este usuario». Este canal cubre infracciones del perfil y del " +
				"contenido; no es la vía de los baneos por trampas, pero es la correcta si además hay contenido o " +
				"comportamiento que infringe las normas de Steam.",
			Effective: false,
		},
		{
			ID:    "third_party_platform",
			Label: "Plataforma de terceros",
			Instructions: "Si la demo viene de FACEIT, ESEA u otra plataforma, denuncia además en su soporte adjuntando el " +
				"identificador de la sala o del partido: esas plataformas sí actúan sobre denuncias con evidencia.",
			Effective: true,
		},
	}
	if !steamID64Pattern.MatchString(steamID64) {
		channels[1].URL = ""
	}
	return channels
}

// renderDossierMarkdown writes the human-readable evidence pack.
func renderDossierMarkdown(report Report, player PlayerReport, profileURL string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Expediente CheaterDetect — %s\n\n", displayName(player))
	fmt.Fprintf(&b, "- SteamID64: `%s`\n", player.SteamID64)
	if profileURL != "" {
		fmt.Fprintf(&b, "- Perfil: %s\n", profileURL)
	}
	fmt.Fprintf(&b, "- Veredicto: **%s** (puntuación %.1f/100, confianza %.0f%%)\n",
		VerdictLabel(player.Verdict), player.Score, player.Confidence*100)
	if report.Match.Map != "" {
		fmt.Fprintf(&b, "- Mapa: %s\n", report.Match.Map)
	}
	fmt.Fprintf(&b, "- Rondas analizadas: %d · bajas con arma: %d\n", player.Rounds, player.GunKills)
	if report.Source.DemoPath != "" {
		fmt.Fprintf(&b, "- Demo: `%s`\n", report.Source.DemoPath)
	}
	if report.Source.SHA256 != "" {
		fmt.Fprintf(&b, "- SHA-256 de la demo: `%s`\n", report.Source.SHA256)
	}
	fmt.Fprintf(&b, "- Línea base: `%s` (%s)\n\n", report.Baseline.ID, report.Baseline.Source)

	b.WriteString("## Métricas frente a juego profesional\n\n")
	b.WriteString("| Métrica | Valor | Base pro (media ± σ) | z | Sospecha |\n")
	b.WriteString("|---|---:|---:|---:|---:|\n")
	for _, m := range player.Metrics {
		if !m.Applied {
			fmt.Fprintf(&b, "| %s | %s | %s | — | muestra insuficiente |\n",
				m.Label, formatValue(m.Value, m.Unit), formatBaseline(m.Baseline, m.Unit))
			continue
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %+.2f | %.0f/100 |\n",
			m.Label, formatValue(m.Value, m.Unit), formatBaseline(m.Baseline, m.Unit), m.Z, m.Suspicion)
	}
	b.WriteString("\n")

	b.WriteString("## Momentos revisables\n\n")
	if len(player.Evidence) == 0 {
		b.WriteString("No se ha marcado ningún momento concreto.\n\n")
	} else {
		b.WriteString("Abre la demo y salta a estos ticks para comprobarlo por ti mismo.\n\n")
		for _, e := range player.Evidence {
			fmt.Fprintf(&b, "- Ronda %d, tick %d — %s\n", e.Round, e.Tick, e.Detail)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Qué es y qué no es esto\n\n")
	for _, l := range report.Limitations {
		fmt.Fprintf(&b, "- %s\n", l)
	}
	b.WriteString("\n")

	policy := reportPolicy()
	b.WriteString("## Cómo denunciar\n\n")
	fmt.Fprintf(&b, "%s\n\n", policy.Summary)
	for _, rule := range policy.Rules {
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	fmt.Fprintf(&b, "\n%s\n", policy.Rejected)

	return b.String()
}

func displayName(player PlayerReport) string {
	if strings.TrimSpace(player.Name) == "" {
		return player.SteamID64
	}
	return player.Name
}

func formatValue(v float64, unit string) string {
	switch unit {
	case "%":
		return fmt.Sprintf("%.1f %%", v)
	case "ms", "°/s":
		return fmt.Sprintf("%.0f %s", v, unit)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

func formatBaseline(b MetricBaseline, unit string) string {
	switch unit {
	case "%":
		return fmt.Sprintf("%.1f ± %.1f %%", b.Mean, b.StdDev)
	case "ms", "°/s":
		return fmt.Sprintf("%.0f ± %.0f %s", b.Mean, b.StdDev, unit)
	default:
		return fmt.Sprintf("%.2f ± %.2f", b.Mean, b.StdDev)
	}
}

// VerdictLabel is the Spanish label the CLI and the UI both render, so a band
// never gets described two different ways.
func VerdictLabel(v Verdict) string {
	switch v {
	case VerdictHighlyAnomalous:
		return "muy anómalo"
	case VerdictAnomalous:
		return "anómalo"
	case VerdictInconclusive:
		return "no concluyente"
	case VerdictClean:
		return "sin anomalías"
	default:
		return "datos insuficientes"
	}
}
