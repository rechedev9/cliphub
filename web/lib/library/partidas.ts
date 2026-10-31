import type {
  LibraryDemo,
  LibraryMatch,
  LibraryMatchPlayer,
  LibraryMatchTeam,
  LibraryParseState,
  LibraryRole,
  LibrarySource,
} from '../api/library.ts';

/**
 * Pure reads over the library records: how a partida, a team and a player are
 * named, joined and summed for the pages. Every number here is a sum of the
 * match lines the orchestrator derived from the tactical documents.
 */

export const SOURCE_LABEL: Record<LibrarySource, string> = {
  valve: 'Valve',
  faceit: 'FACEIT',
  hltv: 'Torneo',
  scrim: 'Scrim',
  unknown: 'Sin identificar',
};

export const PARSE_STATE_LABEL: Record<LibraryParseState, string> = {
  queued: 'En cola',
  parsing: 'Analizando',
  ready: 'Lista',
  failed: 'Falló',
};

export const ROLE_LABEL: Record<LibraryRole, string> = {
  awp: 'AWPer',
  entry: 'Entry',
  support: 'Soporte',
  rifler: 'Rifler',
  unknown: 'Sin muestra suficiente',
};

const DEMO_ERROR_LABEL: Record<string, string> = {
  demo_unreadable: 'La demo no se puede leer. Puede estar incompleta o dañada.',
  truncated_demo: 'La demo está incompleta: el archivo se corta antes del final. Vuelve a descargarla.',
  demo_incompatible: 'Esta versión de Studio no entiende el formato de la demo.',
  map_uncalibrated: 'El mapa aún no tiene radar calibrado, así que no se puede analizar.',
  write_artifact: 'No se pudo guardar el análisis en disco.',
  write_library: 'No se pudo guardar la partida en la biblioteca.',
  enqueue_failed: 'No se pudo poner la demo en cola.',
};

const IMPORT_ERROR_LABEL: Record<string, string> = {
  csgo_demo: 'Es una demo de CS:GO; solo se admiten demos de CS2.',
  not_a_demo: 'El archivo no es una demo de CS2.',
  unreadable_demo: 'No se pudo leer el archivo.',
  truncated_demo: 'La demo está incompleta: el archivo se corta antes del final. Vuelve a descargarla.',
};

const END_REASON_LABEL: Record<string, string> = {
  bomb_exploded: 'Bomba explotada',
  bomb_defused: 'Bomba desactivada',
  ct_eliminated_t: 'T eliminados',
  t_eliminated_ct: 'CT eliminados',
  time_expired: 'Tiempo agotado',
  t_surrender: 'Rendición T',
  ct_surrender: 'Rendición CT',
  draw: 'Empate',
};

export function demoErrorLabel(code: string | undefined): string {
  return (code && DEMO_ERROR_LABEL[code]) || 'El análisis de la demo falló.';
}

export function importErrorLabel(code: string | undefined): string {
  return (code && IMPORT_ERROR_LABEL[code]) || 'No se pudo importar el archivo.';
}

export function endReasonLabel(reason: string): string {
  return END_REASON_LABEL[reason] ?? 'Sin dato';
}

/** One row of Partidas: an imported demo and, once its scan is done, its match. */
export type PartidaRow = { readonly demo: LibraryDemo; readonly match: LibraryMatch | null };

export function buildPartidaRows(demos: readonly LibraryDemo[], matches: readonly LibraryMatch[]): PartidaRow[] {
  const byDemo = new Map(matches.map((match) => [match.demo_id, match]));
  return demos.map((demo) => ({ demo, match: byDemo.get(demo.id) ?? null }));
}

/** When the partida was played: the match date, else the date given at import, else the import. */
export function partidaDate(row: PartidaRow): string {
  return row.match?.played_at ?? row.demo.played_at ?? row.demo.imported_at;
}

export function teamByKey(match: LibraryMatch, key: LibraryMatchTeam['key']): LibraryMatchTeam | undefined {
  return match.teams.find((team) => team.key === key);
}

/** The team's library name, else its clan name in the demo, else the side it started on. */
export function teamLabel(team: LibraryMatchTeam | undefined): string {
  if (team === undefined) return 'Sin equipo';
  if (team.team_name) return team.team_name;
  if (team.clan_name) return team.clan_name;
  return team.start_side === 'CT' ? 'Equipo CT' : 'Equipo T';
}

/** "Vitality vs MOUZ", the team that started CT first. */
export function matchTitle(match: LibraryMatch): string {
  return `${teamLabel(teamByKey(match, 'ct-start'))} vs ${teamLabel(teamByKey(match, 't-start'))}`;
}

/** "13-7", in the same order as matchTitle. */
export function matchScoreLabel(match: LibraryMatch): string {
  return `${teamByKey(match, 'ct-start')?.score ?? 0}-${teamByKey(match, 't-start')?.score ?? 0}`;
}

export type Outcome = 'win' | 'loss' | 'draw';

/** How a library team fared in a match, or null when it did not play it. */
export function teamOutcome(match: LibraryMatch, teamId: string): Outcome | null {
  const own = match.teams.find((team) => team.team_id === teamId);
  const rival = match.teams.find((team) => team !== own);
  if (own === undefined || rival === undefined) return null;
  if (own.score > rival.score) return 'win';
  if (own.score < rival.score) return 'loss';
  return 'draw';
}

export const OUTCOME_LABEL: Record<Outcome, string> = { win: 'Victoria', loss: 'Derrota', draw: 'Empate' };

/** The other side of a match from a team's point of view. */
export function rivalOf(match: LibraryMatch, teamId: string): LibraryMatchTeam | undefined {
  const own = match.teams.find((team) => team.team_id === teamId);
  return own === undefined ? undefined : match.teams.find((team) => team !== own);
}

/** One filter option per library team that played a listed match, by name. */
export function teamFilterOptions(matches: readonly (LibraryMatch | null)[]): { value: string; label: string }[] {
  const names = new Map<string, string>();
  for (const match of matches) {
    for (const team of match?.teams ?? []) {
      if (team.team_id !== null && !names.has(team.team_id)) names.set(team.team_id, teamLabel(team));
    }
  }
  return [...names]
    .map(([value, label]) => ({ value, label }))
    .sort((a, b) => a.label.localeCompare(b.label, 'es') || a.value.localeCompare(b.value));
}

export const DATE_RANGES = [
  { value: '7d', label: 'Últimos 7 días', days: 7 },
  { value: '30d', label: 'Últimos 30 días', days: 30 },
  { value: '90d', label: 'Últimos 3 meses', days: 90 },
  { value: '365d', label: 'Último año', days: 365 },
] as const;

/** True when a date falls inside a DATE_RANGES window ending at `now`; an unknown range keeps every date. */
export function inDateRange(range: string, iso: string, now: number): boolean {
  const window = DATE_RANGES.find((entry) => entry.value === range);
  if (window === undefined) return true;
  const at = Date.parse(iso);
  if (Number.isNaN(at)) return false;
  return at <= now && now - at <= window.days * 24 * 60 * 60 * 1000;
}

/** A team's record on one map, summed over its matches there. */
export type MapPerformance = {
  readonly map: string;
  readonly matches: number;
  readonly wins: number;
  readonly losses: number;
  readonly draws: number;
  readonly roundsWon: number;
  readonly roundsLost: number;
};

/** Per-map record of a team, most played map first, then by map name. */
export function mapPerformance(matches: readonly LibraryMatch[], teamId: string): MapPerformance[] {
  const byMap = new Map<string, { matches: number; wins: number; losses: number; draws: number; roundsWon: number; roundsLost: number }>();
  for (const match of matches) {
    const outcome = teamOutcome(match, teamId);
    const own = match.teams.find((team) => team.team_id === teamId);
    const rival = rivalOf(match, teamId);
    if (outcome === null || own === undefined || rival === undefined) continue;
    const row = byMap.get(match.map) ?? { matches: 0, wins: 0, losses: 0, draws: 0, roundsWon: 0, roundsLost: 0 };
    row.matches++;
    if (outcome === 'win') row.wins++;
    if (outcome === 'loss') row.losses++;
    if (outcome === 'draw') row.draws++;
    row.roundsWon += own.score;
    row.roundsLost += rival.score;
    byMap.set(match.map, row);
  }
  return [...byMap]
    .map(([map, row]) => ({ map, ...row }))
    .sort((a, b) => b.matches - a.matches || a.map.localeCompare(b.map));
}

/** "62 %" for a count over a total, or an empty string for no total. */
export function percentLabel(count: number, total: number): string {
  if (total <= 0) return '';
  return `${Math.round((count / total) * 100)} %`;
}

/** A player's line in one match. */
export function playerLine(match: LibraryMatch, steamId: string): LibraryMatchPlayer | undefined {
  return match.players?.find((player) => player.steamid64 === steamId);
}

export type PlayerTotals = {
  readonly matches: number;
  readonly rounds: number;
  readonly kills: number;
  readonly deaths: number;
  readonly assists: number;
  readonly damage: number;
};

export function playerTotals(lines: readonly LibraryMatchPlayer[]): PlayerTotals {
  return lines.reduce<PlayerTotals>(
    (sum, line) => ({
      matches: sum.matches + 1,
      rounds: sum.rounds + line.rounds,
      kills: sum.kills + line.kills,
      deaths: sum.deaths + line.deaths,
      assists: sum.assists + line.assists,
      damage: sum.damage + line.damage,
    }),
    { matches: 0, rounds: 0, kills: 0, deaths: 0, assists: 0, damage: 0 },
  );
}

/** Average damage per round, one decimal; empty when no round was played. */
export function adrLabel(damage: number, rounds: number): string {
  if (rounds <= 0) return '';
  return (damage / rounds).toFixed(1);
}

/** Kills over deaths, two decimals; with no deaths the kills stand alone. */
export function kdLabel(kills: number, deaths: number): string {
  if (deaths <= 0) return kills.toFixed(2);
  return (kills / deaths).toFixed(2);
}

/**
 * The player's grenades per round over their teammates' grenades per round,
 * the support rule of `library.InferPlayerRole`; empty without a sample.
 */
export function utilityRatioLabel(utility: { count: number; total: number }, team: { count: number; total: number }): string {
  if (utility.total <= 0 || team.total <= 0 || team.count <= 0) return '';
  return ((utility.count / utility.total) / (team.count / team.total)).toFixed(2);
}
