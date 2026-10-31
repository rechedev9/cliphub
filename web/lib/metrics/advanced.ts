import type { DuelBucket, MetricCount, MetricsReport, PlayerMetrics, RoundTimeline, WinPoint } from '../api/metrics.ts';
import type { ScoutEvidence, ScoutMatchSummary } from '../api/scout.ts';
import type { TacticalSide } from '../api/tactical.ts';
import { ROLE_LABEL } from '../library/partidas.ts';
import { placeLabel } from '../tactical-labels.ts';
import { prettyMapName } from '../format.ts';
import { dateText, type ReportCell, type ReportSection, type ReportTable } from '../reports/scout-sections.ts';

/**
 * The advanced metrics as report sections, the model the match page, the
 * player page, the report page and the exported report all render. Every
 * count sits next to the rounds it came from.
 */

const decimal1 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** Win probability added, in percentage points with its sign: 0.123 reads "+12,3 pp". */
export function ppText(value: number): string {
  const points = Math.round(value * 1000) / 10;
  let sign = '';
  if (points > 0) sign = '+';
  else if (points < 0) sign = '-';
  return `${sign}${decimal1.format(Math.abs(points))} pp`;
}

/** A probability as a whole percentage: 0.714 reads "71 %". */
export function percentText(probability: number): string {
  return `${Math.round(probability * 100)} %`;
}

/** An equipment-weighted count with two decimals. */
export function ecoText(value: number): string {
  return decimal2.format(value);
}

/** "3-2 (60 %)", or "Sin duelos" when there were none. */
export function duelText(bucket: DuelBucket): string {
  const total = bucket.won.count + bucket.lost.count;
  if (total === 0) return 'Sin duelos';
  return `${bucket.won.count}-${bucket.lost.count} (${Math.round((bucket.won.count / total) * 100)} %)`;
}

const ROLE_TEXT: Record<string, string> = ROLE_LABEL;

export function roleText(role: string): string {
  return ROLE_TEXT[role] ?? role;
}

const SWING_LABEL: Record<string, string> = {
  kill: 'Baja',
  death: 'Muerte',
  team_kill: 'Baja a un compañero',
  plant: 'Plantada',
  defuse: 'Desactivación',
};

export function swingKindText(kind: string): string {
  return SWING_LABEL[kind] ?? kind;
}

/**
 * The matches the evidence chips name. Metrics carry each player's match lines,
 * which is enough to label a chip with its map and date.
 */
export function metricsMatchIndex(report: MetricsReport, known?: ReadonlyMap<string, ScoutMatchSummary>): ReadonlyMap<string, ScoutMatchSummary> {
  const index = new Map(known ?? []);
  for (const player of report.players) {
    for (const line of player.match_lines) {
      if (index.has(line.match_id)) continue;
      index.set(line.match_id, {
        match_id: line.match_id, map: line.map, played_at: line.played_at, team_key: line.team_key,
        opponent: '', score: 0, opponent_score: 0, rounds: line.rounds, wins: 0,
      });
    }
  }
  return index;
}

export type MetricsSectionOptions = {
  /** Links a player's name to their record. */
  readonly playerHref?: (steamId: string) => string;
  /** Adds the per match table, for one player's page. */
  readonly byMatch?: boolean;
  readonly matchHref?: (matchId: string) => string;
};

const text = (value: string): ReportCell => ({ text: value });
const num = (value: string | number): ReportCell => ({ text: String(value), numeric: true });
const chips = (value: readonly ScoutEvidence[]): ReportCell => ({ text: value.length === 0 ? 'Sin ejemplos' : '', evidence: value });
const sideCell = (value: TacticalSide): ReportCell => ({ text: value, tone: value === 'CT' ? 'ct' : 't' });

function signedTone(value: number): ReportCell['tone'] {
  if (value > 0.0005) return 'win';
  if (value < -0.0005) return 'loss';
  return undefined;
}

function playerCell(player: PlayerMetrics, options: MetricsSectionOptions): ReportCell {
  const name = player.name || player.steamid64;
  return options.playerHref ? { text: name, href: options.playerHref(player.steamid64) } : text(name);
}

function countWithChips(metric: MetricCount): ReportCell[] {
  return [num(metric.count), chips(metric.examples)];
}

function impactSection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  const m = report.model;
  const table: ReportTable = {
    caption: 'Impacto en la probabilidad de victoria',
    columns: ['Jugador', 'Rol', 'Rondas', 'WPA', 'Por ronda', 'Mejores momentos', 'Peores momentos'],
    rows: report.players.map((p) => [
      playerCell(p, options),
      text(roleText(p.role)),
      num(p.rounds),
      { ...num(ppText(p.swing.wpa)), tone: signedTone(p.swing.wpa) },
      { ...num(ppText(p.swing.per_round)), tone: signedTone(p.swing.per_round) },
      chips(p.swing.best),
      chips(p.swing.worst),
    ]),
    empty: 'Ningún jugador de la selección jugó estas rondas.',
  };
  const swings: ReportTable = {
    caption: 'Momentos que más movieron la ronda',
    columns: ['Jugador', 'Qué pasó', 'Antes', 'Después', 'Cambio', 'Ronda'],
    rows: report.players.flatMap((p) =>
      [...p.swing.best.slice(0, 2), ...p.swing.worst.slice(0, 1)].map((s) => [
        playerCell(p, options),
        text(swingKindText(s.kind)),
        num(percentText(s.before)),
        num(percentText(s.after)),
        { ...num(ppText(s.delta)), tone: signedTone(s.delta) },
        chips([s]),
      ]),
    ),
    empty: 'Sin momentos que movieran la probabilidad.',
  };
  return {
    id: 'metricas-impacto',
    title: 'Impacto',
    intro:
      'WPA suma cuánto movió cada jugador la probabilidad de victoria de su lado. Una baja abona al que mata lo que gana su lado y resta a la víctima lo que pierde el suyo. La plantada abona al que planta y la desactivación al que desactiva. Se mide en puntos porcentuales (pp).',
    facts: [
      { label: 'Partidas', value: String(report.matches) },
      { label: 'Rondas', value: String(report.rounds) },
      { label: 'Modelo', value: `${m.version}, regresión logística con coeficientes fijos` },
    ],
    tables: [table, swings],
  };
}

function economySection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  return {
    id: 'metricas-economia',
    title: 'Economía',
    intro:
      'Cada duelo vale 2·Ev/(Ek+Ev), con Ek y Ev el equipamiento de inicio de ronda del que mata y de la víctima. Entre iguales vale 1, contra un rival más pobre menos y contra uno más rico más. El mismo valor cuenta como baja para uno y como muerte para el otro.',
    facts: [{ label: 'Equipamiento mínimo por jugador', value: `${report.model.min_equip} $` }],
    tables: [
      {
        caption: 'Bajas y muertes ajustadas por equipamiento',
        columns: ['Jugador', 'Bajas', 'Ajustadas', 'Muertes', 'Ajustadas', 'Balance ajustado', 'Bajas de más valor'],
        rows: report.players.map((p) => {
          const balance = p.eco.kills - p.eco.deaths;
          return [
            playerCell(p, options),
            num(p.kills),
            num(ecoText(p.eco.kills)),
            num(p.deaths),
            num(ecoText(p.eco.deaths)),
            { ...num(`${balance > 0 ? '+' : ''}${ecoText(balance)}`), tone: signedTone(balance) },
            chips(p.eco.examples),
          ];
        }),
        empty: 'Sin duelos en la selección.',
      },
    ],
  };
}

function duelSection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  const ratio = decimal1.format(report.thresholds.duel_even_ratio);
  const buckets = [
    ['even', 'Justos'],
    ['advantage', 'Con ventaja'],
    ['disadvantage', 'En desventaja'],
  ] as const;
  return {
    id: 'metricas-duelos',
    title: 'Duelos',
    intro: `Un duelo es justo cuando ninguno de los dos llevaba más de ${ratio} veces el equipamiento del otro al empezar la ronda. Con ventaja, el jugador llevaba más; en desventaja, menos.`,
    facts: [],
    tables: [
      {
        caption: 'Duelos por equipamiento',
        columns: ['Jugador', 'Tipo', 'Ganados y perdidos', 'Ganados', 'Perdidos'],
        rows: report.players.flatMap((p) =>
          buckets.map(([key, label]) => {
            const bucket = p.duels[key];
            return [playerCell(p, options), text(label), num(duelText(bucket)), chips(bucket.won.examples), chips(bucket.lost.examples)];
          }),
        ),
        empty: 'Sin duelos en la selección.',
      },
    ],
  };
}

function tradeSection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  return {
    id: 'metricas-trades',
    title: 'Trades',
    intro: `Un trade es matar al rival que acaba de matar a un compañero, como mucho ${report.thresholds.trade_seconds} s después. Una muerte tradeada es una que un compañero vengó dentro de esa ventana.`,
    facts: [],
    tables: [
      {
        caption: 'Trades por jugador',
        columns: ['Jugador', 'Trades hechos', 'Rondas', 'Muertes tradeadas', 'Rondas', 'Muertes sin trade', '% tradeadas'],
        rows: report.players.map((p) => {
          const deaths = p.trades.traded_deaths.count + p.trades.untraded_deaths.count;
          return [
            playerCell(p, options),
            ...countWithChips(p.trades.trade_kills),
            ...countWithChips(p.trades.traded_deaths),
            num(p.trades.untraded_deaths.count),
            num(deaths === 0 ? 'Sin muertes' : percentText(p.trades.traded_deaths.count / deaths)),
          ];
        }),
        empty: 'Sin bajas en la selección.',
      },
    ],
  };
}

function utilitySection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  const seconds = decimal1.format(report.thresholds.flash_min_seconds);
  const missing = report.matches - report.utility_matches;
  const intro = [
    `Un jugador cuenta como cegado cuando la flash lo deja ciego al menos ${seconds} s. Una asistencia de flash es una baja de un compañero sobre un rival mientras seguía ciego por tu flash. El daño es la vida que quitan HE y molotovs.`,
    missing > 0
      ? `${missing} de ${report.matches} partidas se analizaron antes de medir la utilidad: vuelve a analizarlas desde su ficha para incluirlas.`
      : '',
  ].filter((part) => part !== '').join(' ');
  const measured = (p: PlayerMetrics, cells: ReportCell[]): ReportCell[] =>
    p.utility.measured ? cells : cells.map((_, i) => text(i === 0 ? 'Sin medir' : ''));
  return {
    id: 'metricas-utilidad',
    title: 'Utilidad',
    intro,
    facts: [{ label: 'Partidas con utilidad medida', value: `${report.utility_matches} de ${report.matches}` }],
    tables: [
      {
        caption: 'Flashes',
        columns: ['Jugador', 'Lanzadas', 'Rivales cegados', 'Rondas', 'Compañeros cegados', 'Rondas', 'Segundos de ceguera', 'Asistencias', 'Rondas'],
        rows: report.players.map((p) => {
          const f = p.utility.flash;
          return [
            playerCell(p, options),
            ...measured(p, [
              num(f.thrown),
              ...countWithChips(f.enemies_blinded),
              ...countWithChips(f.team_blinded),
              num(decimal1.format(f.enemy_blind_seconds)),
              ...countWithChips(f.assists),
            ]),
          ];
        }),
        empty: 'Sin flashes en la selección.',
      },
      {
        caption: 'HE y molotovs',
        columns: ['Jugador', 'HE', 'Daño a rivales', 'Daño propio', 'Mejores HE', 'Molotovs', 'Daño a rivales', 'Daño propio', 'Mejores molotovs'],
        rows: report.players.map((p) => {
          const { he, molotov } = p.utility;
          return [
            playerCell(p, options),
            ...measured(p, [
              num(he.thrown), num(he.enemy_damage), num(he.team_damage), chips(he.examples),
              num(molotov.thrown), num(molotov.enemy_damage), num(molotov.team_damage), chips(molotov.examples),
            ]),
          ];
        }),
        empty: 'Sin granadas en la selección.',
      },
    ],
  };
}

function positionSection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  return {
    id: 'metricas-posiciones',
    title: 'Posiciones',
    intro: `La posición de cada ronda es la zona donde estaba el jugador a los ${report.thresholds.setup_seconds} s de acabar el tiempo de compra. Las rondas en las que ya había muerto, o sin posiciones grabadas, no cuentan.`,
    facts: [],
    tables: [
      {
        caption: 'Rendimiento por posición',
        columns: ['Jugador', 'Lado', 'Posición', 'Rondas', '% ganadas', 'Bajas', 'Muertes', 'WPA', 'Rondas de ejemplo'],
        rows: report.players.flatMap((p) =>
          p.positions.map((line) => [
            playerCell(p, options),
            sideCell(line.side),
            text(placeLabel(line.callout)),
            num(line.rounds),
            num(line.rounds === 0 ? '' : percentText(line.won / line.rounds)),
            num(line.kills),
            num(line.deaths),
            { ...num(ppText(line.wpa)), tone: signedTone(line.wpa) },
            chips(line.examples),
          ]),
        ),
        empty: 'Sin posiciones: las partidas no tienen posiciones grabadas o los jugadores murieron antes.',
      },
    ],
  };
}

function roleSection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  const names = new Map(report.players.map((p) => [p.steamid64, p.name || p.steamid64]));
  const perRound = (value: number, rounds: number): string => (rounds === 0 ? '' : ecoText(value / rounds));
  return {
    id: 'metricas-roles',
    title: 'Por rol',
    intro:
      'El rol de cada jugador se infiere de las mismas partidas que miden estas cifras (AWP, entry, soporte o rifler), con un mínimo de 20 rondas. Cada fila suma a los jugadores de ese rol; sus rondas de ejemplo están en las otras pestañas.',
    facts: [],
    tables: [
      {
        caption: 'Rendimiento por rol',
        columns: ['Rol', 'Jugadores', 'Rondas', 'Bajas ajustadas por ronda', 'WPA por ronda', 'Asistencias de flash', 'Daño de utilidad'],
        rows: report.roles.map((line) => [
          text(roleText(line.role)),
          options.playerHref && line.players.length === 1
            ? { text: names.get(line.players[0] ?? '') ?? '', href: options.playerHref(line.players[0] ?? '') }
            : text(line.players.map((id) => names.get(id) ?? id).join(', ')),
          num(line.rounds),
          num(perRound(line.eco_kills, line.rounds)),
          { ...num(line.rounds === 0 ? '' : ppText(line.wpa / line.rounds)), tone: signedTone(line.wpa) },
          num(line.flash_assists),
          num(line.utility_damage),
        ]),
        empty: 'Sin jugadores en la selección.',
      },
    ],
  };
}

function byMatchSection(report: MetricsReport, options: MetricsSectionOptions): ReportSection {
  const lines = report.players.flatMap((p) => p.match_lines);
  return {
    id: 'metricas-partidas',
    title: 'Por partida',
    intro: 'Las mismas cifras partida a partida, la más reciente primero. Cada partida abre su ficha.',
    facts: [],
    tables: [
      {
        caption: 'Partidas leídas',
        columns: ['Partida', 'Rondas', 'Bajas', 'Muertes', 'Bajas ajustadas', 'WPA'],
        rows: lines.map((line) => {
          const label = `${prettyMapName(line.map)}, ${dateText(line.played_at)}`;
          return [
          options.matchHref ? { text: label, href: options.matchHref(line.match_id) } : text(label),
          num(line.rounds),
          num(line.kills),
          num(line.deaths),
          num(ecoText(line.eco_kills)),
          { ...num(ppText(line.wpa)), tone: signedTone(line.wpa) },
          ];
        }),
        empty: 'Sin partidas leídas.',
      },
    ],
  };
}

/** Every metrics section, in the order an analyst reads a player. */
export function metricsSections(report: MetricsReport, options: MetricsSectionOptions = {}): ReportSection[] {
  const sections = [
    impactSection(report, options),
    economySection(report, options),
    duelSection(report, options),
    tradeSection(report, options),
    utilitySection(report, options),
    positionSection(report, options),
    roleSection(report, options),
  ];
  if (options.byMatch) sections.push(byMatchSection(report, options));
  return sections;
}

/** The model and its coefficients as one sentence, for a footer. */
export function modelText(report: Pick<MetricsReport, 'model'>): string {
  const m = report.model;
  const n = (value: number): string => decimal1.format(value);
  return `Probabilidad de victoria ${m.version}: regresión logística desde el lado CT con coeficientes fijos, no ajustados con datos. Diferencia de vivos ×${n(m.alive_coef)}, logaritmo del equipamiento medio CT entre T ×${n(m.equip_coef)}, reloj sin plantar ×${n(m.clock_coef)} sobre ${m.round_seconds} s, plantada ${n(m.plant_coef)} y temporizador de la bomba ${n(m.bomb_coef)} sobre ${m.bomb_seconds} s, más un sesgo por mapa.`;
}

export type WinChartPoint = { x: number; y: number; point: WinPoint };

/**
 * The chart coordinates of a round's win probability: seconds from freeze end
 * across, CT probability up. The step path holds each value until the next event.
 */
export function winChart(timeline: RoundTimeline, width: number, height: number): { points: WinChartPoint[]; path: string; maxSeconds: number } {
  const maxSeconds = Math.max(10, ...timeline.points.map((p) => p.seconds));
  const points = timeline.points.map((point) => ({
    x: Math.round((point.seconds / maxSeconds) * width * 10) / 10,
    y: Math.round((1 - point.probability_ct) * height * 10) / 10,
    point,
  }));
  let path = '';
  points.forEach((p, i) => {
    if (i === 0) {
      path = `M${p.x} ${p.y}`;
      return;
    }
    const prev = points[i - 1];
    path += ` H${p.x}`;
    if (prev && prev.y !== p.y) path += ` V${p.y}`;
  });
  return { points, path, maxSeconds };
}
