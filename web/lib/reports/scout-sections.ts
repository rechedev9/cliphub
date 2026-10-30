import type { TacticalHistogram, TacticalRate, TacticalSide } from '../api/tactical.ts';
import type {
  ReportSide,
  ScoutEconomyContextName,
  ScoutEvidence,
  ScoutMatchSummary,
  ScoutPatternRow,
  ScoutPlaceRow,
  ScoutPlayerCount,
  ScoutReport,
  ScoutSkipReason,
  ScoutUtilityRow,
} from '../api/scout.ts';
import { matchHref } from '../clips/routes.ts';
import { prettyMapName } from '../format.ts';
import { roundEvidenceHref } from '../library/rounds.ts';
import { buyLabel, eventKindLabel, patternLabel, siteLabel } from '../tactical-labels.ts';

/**
 * The scouting report as plain sections of tables. The page and the exported
 * HTML both render this model, so the file a coach receives says exactly what
 * the screen said. Every row that counts rounds carries its example rounds.
 */

export type ReportTone = 'win' | 'loss' | 'ct' | 't';

export type ReportCell = {
  readonly text: string;
  readonly numeric?: boolean;
  /** Below the reliable sample: shown, but marked as a small sample. */
  readonly lowSample?: boolean;
  readonly tone?: ReportTone;
  /** A link to a record, such as the match a summary row describes. */
  readonly href?: string;
  /** The example rounds behind the row, each a link to its round and tick. */
  readonly evidence?: readonly ScoutEvidence[];
};

export type ReportTable = {
  readonly caption: string;
  readonly columns: readonly string[];
  readonly rows: readonly (readonly ReportCell[])[];
  /** What an empty table means, so a gap reads as a finding and not as a bug. */
  readonly empty: string;
};

export type ReportSection = {
  readonly id: string;
  readonly title: string;
  readonly intro: string;
  readonly facts: readonly { readonly label: string; readonly value: string }[];
  readonly tables: readonly ReportTable[];
};

export const REPORT_SIDE_LABEL: Record<ReportSide, string> = { '': 'Ambos lados', CT: 'CT', T: 'T' };

export const ECONOMY_CONTEXT_LABEL: Record<ScoutEconomyContextName, string> = {
  after_pistol_loss: 'Tras perder la pistola',
  after_loss: 'Tras perder otra ronda',
};

export const SKIP_REASON_LABEL: Record<ScoutSkipReason, string> = {
  not_found: 'La partida ya no está en la biblioteca.',
  team_not_in_match: 'El equipo no jugó esta partida.',
  analysis_missing: 'La partida aún no tiene análisis táctico.',
  over_match_limit: 'Supera el máximo de partidas de un informe.',
};

/** The grenade kinds an execute is summed over, with the plural the mean reads in. */
const UTILITY_MEAN_ORDER = [
  ['smoke', 'humos'],
  ['flash', 'flashes'],
  ['molotov', 'molotovs'],
  ['he', 'HE'],
] as const;

const decimal = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 1 });

/** "43 % (10/23)", or "Sin datos" for an empty denominator. */
export function rateText(rate: TacticalRate): string {
  if (rate.total <= 0) return 'Sin datos';
  return `${Math.round((rate.count / rate.total) * 100)} % (${rate.count}/${rate.total})`;
}

/** Seconds from freeze end as a round clock: 42.3 reads "0:42". */
export function secondsText(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '';
  const whole = Math.round(seconds);
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, '0')}`;
}

/** The median of a timing histogram, or "Sin datos" without samples. */
export function medianText(histogram: TacticalHistogram): string {
  return histogram.samples > 0 ? `${secondsText(histogram.median)} (n=${histogram.samples})` : 'Sin datos';
}

const dateFormat = new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Europe/Madrid' });

/** "12 oct 2026", or an empty string for an unreadable date. */
export function dateText(iso: string): string {
  const at = Date.parse(iso);
  return Number.isNaN(at) ? '' : dateFormat.format(at);
}

/** The path that opens one example round on its tick in the match replay. */
export function evidencePath(evidence: ScoutEvidence): string {
  return roundEvidenceHref(matchHref(evidence.match_id), evidence.round, evidence.tick);
}

/** The short label of an example round: its map and round number. */
export function evidenceLabel(evidence: ScoutEvidence, matches: ReadonlyMap<string, ScoutMatchSummary>): string {
  const match = matches.get(evidence.match_id);
  return match ? `${prettyMapName(match.map)} R${evidence.round}` : `R${evidence.round}`;
}

/** The long label of an example round: which match, which round, which tick. */
export function evidenceTitle(evidence: ScoutEvidence, matches: ReadonlyMap<string, ScoutMatchSummary>): string {
  const match = matches.get(evidence.match_id);
  const where = match ? `${prettyMapName(match.map)}, ${dateText(match.played_at)}, contra ${match.opponent || 'rival sin nombre'}` : 'Partida';
  return `${where}. Ronda ${evidence.round}, tick ${evidence.tick}`;
}

export function matchIndex(report: Pick<ScoutReport, 'matches'>): ReadonlyMap<string, ScoutMatchSummary> {
  return new Map(report.matches.map((match) => [match.match_id, match]));
}

function scoreTone(match: ScoutMatchSummary): ReportTone | undefined {
  if (match.score > match.opponent_score) return 'win';
  if (match.score < match.opponent_score) return 'loss';
  return undefined;
}

function sideTone(side: TacticalSide): ReportTone {
  return side === 'CT' ? 'ct' : 't';
}

const text = (value: string): ReportCell => ({ text: value });
const count = (value: number): ReportCell => ({ text: String(value), numeric: true });
const rate = (value: TacticalRate): ReportCell => ({ text: rateText(value), numeric: true, lowSample: value.total > 0 && !value.reliable });
const side = (value: TacticalSide): ReportCell => ({ text: value, tone: sideTone(value) });
const examples = (value: readonly ScoutEvidence[]): ReportCell => ({ text: value.length === 0 ? 'Sin ejemplos' : '', evidence: value });

function players(list: readonly ScoutPlayerCount[]): string {
  return list.map((player) => `${player.name || player.steamid64} (${player.count})`).join(', ');
}

function calloutText(callout: string): string {
  return callout === '' ? 'Sin zona' : callout;
}

function patternTable(caption: string, rows: readonly ScoutPatternRow[], empty: string): ReportTable {
  return {
    caption,
    columns: ['Lado', 'Forma', 'Sitio', 'Rondas', '% del lado', '% ganadas', 'Rondas de ejemplo'],
    rows: rows.map((row) => [
      side(row.side),
      text(patternLabel(row.pattern)),
      text(siteLabel(row.site)),
      count(row.rounds),
      rate(row.share),
      rate(row.win_rate),
      examples(row.examples),
    ]),
    empty,
  };
}

function utilityTable(caption: string, rows: readonly ScoutUtilityRow[], empty: string): ReportTable {
  return {
    caption,
    columns: ['Granada', 'Zona donde cae', 'Rondas con ella', 'Granadas', 'Segundo (mediana)', 'Lanzadores', 'Rondas de ejemplo'],
    rows: rows.map((row) => [
      text(eventKindLabel(row.kind)),
      text(calloutText(row.callout)),
      rate(row.share),
      count(row.count),
      { text: secondsText(row.median_seconds), numeric: true },
      text(players(row.throwers)),
      examples(row.examples),
    ]),
    empty,
  };
}

function placeTable(caption: string, rows: readonly ScoutPlaceRow[], empty: string): ReportTable {
  return {
    caption,
    columns: ['Zona', 'Rondas', '% de los duelos', 'Duelo ganado', 'Rondas de ejemplo'],
    rows: rows.map((row) => [text(row.place), count(row.rounds), rate(row.share), rate(row.won), examples(row.examples)]),
    empty,
  };
}

function summarySection(report: ScoutReport): ReportSection {
  const t = report.tendencies;
  const skipped = report.skipped.map((entry) => [
    { text: entry.match_id, href: matchHref(entry.match_id) },
    text(SKIP_REASON_LABEL[entry.reason]),
  ]);
  return {
    id: 'resumen',
    title: 'Resumen',
    intro: `Lo que se leyó de ${report.team_name}: sus partidas, su balance por mapa y lo que no se pudo leer.`,
    facts: [
      { label: 'Partidas', value: String(report.matches.length) },
      { label: 'Rondas leídas', value: String(t.round_count) },
      { label: 'Rondas ganadas', value: rateText({ count: t.wins, total: t.round_count, pct: t.round_count > 0 ? (t.wins / t.round_count) * 100 : 0, reliable: true }) },
      { label: 'Aperturas ganadas', value: rateText(t.openings.won) },
    ],
    tables: [
      {
        caption: 'Partidas leídas',
        columns: ['Fecha', 'Mapa', 'Rival', 'Marcador', 'Rondas leídas', 'Rondas ganadas'],
        rows: report.matches.map((match) => [
          { text: dateText(match.played_at), href: matchHref(match.match_id) },
          text(prettyMapName(match.map)),
          text(match.opponent || 'Rival sin nombre'),
          {
            text: `${match.score}-${match.opponent_score}`,
            numeric: true,
            tone: scoreTone(match),
          },
          count(match.rounds),
          count(match.wins),
        ]),
        empty: 'Ninguna partida cumple el filtro.',
      },
      {
        caption: 'Por mapa',
        columns: ['Mapa', 'Partidas', 'Ganadas', 'Rondas', '% rondas', '% rondas CT', '% rondas T'],
        rows: report.maps.map((map) => [
          text(prettyMapName(map.map)),
          count(map.matches),
          count(map.match_wins),
          count(map.rounds),
          rate(map.win_rate),
          rate(map.ct_win_rate),
          rate(map.t_win_rate),
        ]),
        empty: 'Sin mapas.',
      },
      ...(skipped.length > 0
        ? [{ caption: 'Partidas que no se pudieron leer', columns: ['Partida', 'Motivo'], rows: skipped, empty: '' }]
        : []),
    ],
  };
}

function pistolSection(report: ScoutReport): ReportSection {
  return {
    id: 'pistolas',
    title: 'Pistolas',
    intro: 'Las rondas con patrones más fijos: qué forma juegan y dónde se decide la ronda, por lado.',
    facts: report.pistols.sides.map((entry) => ({
      label: `Pistolas ${entry.side}`,
      value: `${entry.rounds} jugadas, ${rateText(entry.win_rate)} ganadas`,
    })),
    tables: [patternTable('Pistolas por forma y sitio', report.pistols.rows, 'Ninguna ronda de pistola en las partidas leídas.')],
  };
}

function defaultsSection(report: ScoutReport): ReportSection {
  const d = report.t_defaults;
  const early = report.thresholds.early_utility_seconds;
  return {
    id: 'defaults-t',
    title: 'Defaults T',
    intro: `Rondas de T con compra completa (sin pistolas). Granadas que caen en los primeros ${early} s y dónde se pelea el primer duelo.`,
    facts: [
      { label: 'Rondas', value: String(d.rounds) },
      { label: 'Ganadas', value: rateText(d.win_rate) },
      { label: 'Primer contacto (mediana)', value: medianText(d.first_contact) },
    ],
    tables: [
      patternTable('Formas de la ronda', d.patterns, 'Sin rondas de T con compra completa.'),
      utilityTable(`Granadas en los primeros ${early} s`, d.utility, 'Sin granadas tempranas registradas.'),
      placeTable('Dónde se pelea el primer duelo', d.opening_places, 'Sin duelos de apertura con zona.'),
    ],
  };
}

function setupsSection(report: ScoutReport): ReportSection {
  const s = report.ct_setups;
  const at = report.thresholds.setup_seconds;
  return {
    id: 'setups-ct',
    title: 'Setups CT',
    intro: `Rondas de CT con compra completa. Dónde está cada jugador a los ${at} s y qué granadas usan al empezar.`,
    facts: [
      { label: 'Rondas', value: String(s.rounds) },
      { label: 'Ganadas', value: rateText(s.win_rate) },
      { label: `Rondas con posiciones a los ${at} s`, value: String(s.sampled) },
    ],
    tables: [
      patternTable('Formas de la ronda', s.patterns, 'Sin rondas de CT con compra completa.'),
      {
        caption: `Posición de cada jugador a los ${at} s`,
        columns: ['Jugador', 'Rondas leídas', 'Zonas', 'Rondas de ejemplo'],
        rows: s.positions.map((row) => [
          text(row.name || row.steamid64),
          count(row.rounds),
          text(row.spots.map((spot) => `${spot.callout} ${rateText(spot.share)}`).join(', ')),
          examples(row.spots[0]?.examples ?? []),
        ]),
        empty: 'Sin posiciones: las partidas no tienen el fichero de posiciones o no hay rondas de CT con compra completa.',
      },
      utilityTable(`Granadas en los primeros ${report.thresholds.early_utility_seconds} s`, s.utility, 'Sin granadas tempranas registradas.'),
    ],
  };
}

function executesSection(report: ScoutReport): ReportSection {
  const window = report.thresholds.execute_window_seconds;
  const tables: ReportTable[] = [
    {
      caption: 'Ejecuciones por sitio',
      columns: ['Sitio', 'Rondas', '% de las ejecuciones', '% ganadas', 'Plantadas', 'Plantada (mediana)', 'Primer contacto (mediana)', 'Granadas de media', 'Rondas de ejemplo'],
      rows: report.executes.map((site) => [
        text(siteLabel(site.site)),
        count(site.rounds),
        rate(site.share),
        rate(site.win_rate),
        rate(site.plants),
        text(medianText(site.plant_time)),
        text(medianText(site.first_contact)),
        text(UTILITY_MEAN_ORDER.map(([kind, noun]) => `${decimal.format(site.mean_utility[kind] ?? 0)} ${noun}`).join(', ')),
        examples(site.examples),
      ]),
      empty: 'Sin ejecuciones ni splits a A o B en las partidas leídas.',
    },
    ...report.executes.map((site) =>
      utilityTable(`Granadas de la ejecución a ${siteLabel(site.site)}`, site.utility, 'Sin granadas en la ventana de la ejecución.'),
    ),
  ];
  return {
    id: 'ejecuciones',
    title: 'Ejecuciones',
    intro: `Rondas de T clasificadas como ejecución o split a un sitio. Las granadas cuentan si caen en los ${window} s antes de la plantada, o del primer contacto si no hubo plantada.`,
    facts: [],
    tables,
  };
}

function economySection(report: ScoutReport): ReportSection {
  return {
    id: 'economia',
    title: 'Economía tras perder',
    intro: 'Qué compran en la ronda siguiente a una derrota dentro de la misma parte: si fuerzan o salvan, y cómo les va.',
    facts: [],
    tables: [
      {
        caption: 'Rondas tras una derrota',
        columns: ['Contexto', 'Lado', 'Rondas', '% ganadas', 'Rondas de ejemplo'],
        rows: report.economy.contexts.map((row) => [
          text(ECONOMY_CONTEXT_LABEL[row.context]),
          side(row.side),
          count(row.rounds),
          rate(row.win_rate),
          examples(row.examples),
        ]),
        empty: 'Ninguna derrota seguida de otra ronda en la misma parte.',
      },
      {
        caption: 'Compra tras una derrota',
        columns: ['Contexto', 'Lado', 'Compra', 'Rondas', '% del contexto', '% ganadas', 'Rondas de ejemplo'],
        rows: report.economy.rows.map((row) => [
          text(ECONOMY_CONTEXT_LABEL[row.context]),
          side(row.side),
          text(buyLabel(row.buy)),
          count(row.rounds),
          rate(row.share),
          rate(row.win_rate),
          examples(row.examples),
        ]),
        empty: 'Sin compras que leer.',
      },
    ],
  };
}

function awpSection(report: ScoutReport): ReportSection {
  return {
    id: 'awp',
    title: 'AWP',
    intro: 'Desde dónde consiguen sus bajas con el AWP, por lado, y quién lo lleva.',
    facts: [],
    tables: [
      {
        caption: 'AWP por lado',
        columns: ['Lado', 'Rondas', 'Rondas con baja de AWP', 'Bajas', 'Aperturas'],
        rows: report.awp.sides.map((row) => [side(row.side), count(row.rounds), rate(row.kill_rounds), count(row.kills), count(row.opening_kills)]),
        empty: 'Sin rondas.',
      },
      {
        caption: 'Bajas de AWP por zona',
        columns: ['Lado', 'Zona', 'Bajas', '% del lado', 'Aperturas', 'Jugadores', 'Rondas de ejemplo'],
        rows: report.awp.rows.map((row) => [
          side(row.side),
          text(calloutText(row.place)),
          count(row.kills),
          rate(row.share),
          count(row.opening_kills),
          text(players(row.players)),
          examples(row.examples),
        ]),
        empty: 'Ninguna baja con AWP en las partidas leídas.',
      },
    ],
  };
}

function openingsSection(report: ScoutReport): ReportSection {
  return {
    id: 'aperturas',
    title: 'Aperturas',
    intro: 'Quién pelea el primer duelo de la ronda, cuántos gana, si le tradean al perderlo y dónde.',
    facts: [],
    tables: [
      {
        caption: 'Duelos de apertura por jugador',
        columns: ['Jugador', 'Lado', 'Rondas', 'Duelos de apertura', 'Ganados', 'Tradeado al perder', 'Zonas', 'Rondas de ejemplo'],
        rows: report.openings.map((row) => [
          text(row.name || row.steamid64),
          side(row.side),
          count(row.rounds),
          rate(row.attempts),
          rate(row.won),
          rate(row.traded_on_loss),
          text(row.places.slice(0, 3).map((place) => `${place.place} (${place.count})`).join(', ')),
          examples(row.examples),
        ]),
        empty: 'Sin duelos de apertura.',
      },
    ],
  };
}

/** Every section of a scouting report, in the order an analyst reads a rival. */
export function scoutSections(report: ScoutReport): ReportSection[] {
  return [
    summarySection(report),
    pistolSection(report),
    defaultsSection(report),
    setupsSection(report),
    executesSection(report),
    economySection(report),
    awpSection(report),
    openingsSection(report),
  ];
}

/** The rules the report applied, as one sentence for its footer. */
export function thresholdsText(report: Pick<ScoutReport, 'thresholds'>): string {
  const t = report.thresholds;
  return `Una cifra con menos de ${t.min_reliable_sample} rondas se marca como muestra pequeña. Cada fila guarda hasta ${t.example_limit} rondas de ejemplo, las más recientes primero. Granadas tempranas: primeros ${t.early_utility_seconds} s. Ventana de ejecución: ${t.execute_window_seconds} s. Posiciones de CT: a los ${t.setup_seconds} s.`;
}
