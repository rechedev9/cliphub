import type { LibrarySource } from '../api/library.ts';
import { MAX_TAG_LENGTH } from '../api/libreta.ts';
import type { LibrarySeries, VetoStep } from '../api/veto.ts';
import { partidaDate, teamLabel, teamOutcome, type Outcome, type PartidaRow } from './partidas.ts';

/** Pure reads for the grouped Partidas list: kind, sort, series with veto, own team and tags. */

export const MATCH_KINDS = ['scrim', 'oficial', 'importada'] as const;
export type MatchKind = (typeof MATCH_KINDS)[number];

export const KIND_LABEL: Record<MatchKind, string> = { scrim: 'Scrims', oficial: 'Oficiales', importada: 'Importadas' };

/**
 * The library stores no match type, so it comes from the demo source and its series:
 * a scrim is a scrim; a tournament (HLTV) demo, or a match in a series with an event,
 * is official; a FACEIT pug with no event and anything else counts as imported.
 */
export function matchKind(input: { source: LibrarySource; inEventSeries: boolean }): MatchKind {
  if (input.source === 'scrim') return 'scrim';
  if (input.source === 'hltv' || input.inEventSeries) return 'oficial';
  return 'importada';
}

export type SeriesById = ReadonlyMap<string, LibrarySeries>;

export function seriesIndex(series: readonly LibrarySeries[]): SeriesById {
  return new Map(series.map((entry) => [entry.id, entry]));
}

/** The event of the series a row's match belongs to, or an empty string. */
export function rowEvent(row: PartidaRow, seriesById: SeriesById): string {
  const id = row.match?.series_id;
  return id === undefined ? '' : (seriesById.get(id)?.event ?? '');
}

export function rowKind(row: PartidaRow, seriesById: SeriesById): MatchKind {
  return matchKind({ source: row.demo.source, inEventSeries: rowEvent(row, seriesById) !== '' });
}

/** Every event name the library's series carry, for the event filter. */
export function eventOptions(series: readonly LibrarySeries[]): { value: string; label: string }[] {
  const events = [...new Set(series.map((entry) => entry.event).filter((event) => event !== ''))];
  return events.sort((a, b) => a.localeCompare(b, 'es')).map((event) => ({ value: event, label: event }));
}

export const SORTS = ['played', 'imported'] as const;
export type PartidaSort = (typeof SORTS)[number];

export const SORT_LABEL: Record<PartidaSort, string> = { played: 'Fecha de la partida', imported: 'Fecha de importación' };

/** Query keys of the Partidas list view, so a reload or a shared link keeps it. */
export const LIBRARY_VIEW_QUERY = { sort: 'orden', kind: 'tipo', pov: 'pov', voice: 'voz', view: 'vista' } as const;

export const LIBRARY_VIEWS = ['series', 'tabla'] as const;
export type LibraryViewMode = (typeof LIBRARY_VIEWS)[number];

const SORT_PARAM: Record<PartidaSort, string> = { played: 'partida', imported: 'importacion' };

export type LibraryListView = {
  readonly sort: PartidaSort;
  readonly kind: MatchKind | null;
  readonly pov: boolean;
  readonly voice: boolean;
  readonly mode: LibraryViewMode;
};

export function parseListView(search: URLSearchParams): LibraryListView {
  const sort = SORTS.find((entry) => SORT_PARAM[entry] === search.get(LIBRARY_VIEW_QUERY.sort)) ?? 'played';
  const kind = MATCH_KINDS.find((entry) => entry === search.get(LIBRARY_VIEW_QUERY.kind)) ?? null;
  const mode = LIBRARY_VIEWS.find((entry) => entry === search.get(LIBRARY_VIEW_QUERY.view)) ?? 'series';
  return { sort, kind, pov: search.get(LIBRARY_VIEW_QUERY.pov) === '1', voice: search.get(LIBRARY_VIEW_QUERY.voice) === '1', mode };
}

/** The query keys of a list view; defaults are left out so a plain /partidas stays plain. */
export function listViewParams(view: LibraryListView): [string, string][] {
  const out: [string, string][] = [];
  if (view.sort !== 'played') out.push([LIBRARY_VIEW_QUERY.sort, SORT_PARAM[view.sort]]);
  if (view.kind !== null) out.push([LIBRARY_VIEW_QUERY.kind, view.kind]);
  if (view.pov) out.push([LIBRARY_VIEW_QUERY.pov, '1']);
  if (view.voice) out.push([LIBRARY_VIEW_QUERY.voice, '1']);
  if (view.mode !== 'series') out.push([LIBRARY_VIEW_QUERY.view, view.mode]);
  return out;
}

/** The search string without the list view keys: what identifies another page's filter link. */
export function withoutListView(search: URLSearchParams): string {
  const rest = new URLSearchParams(search);
  for (const key of Object.values(LIBRARY_VIEW_QUERY)) rest.delete(key);
  return rest.toString();
}

/** The search string with the list view applied over the other keys. */
export function withListView(search: URLSearchParams, view: LibraryListView): string {
  const next = new URLSearchParams(search);
  for (const key of Object.values(LIBRARY_VIEW_QUERY)) next.delete(key);
  for (const [key, value] of listViewParams(view)) next.set(key, value);
  return next.toString();
}

/** A row's date under the chosen sort. */
export function sortDate(row: PartidaRow, sort: PartidaSort): string {
  return sort === 'imported' ? row.demo.imported_at : partidaDate(row);
}

/** True when a recorded video of the partida exists, given the job ids that have videos. */
export function hasPov(row: PartidaRow, jobsWithVideos: ReadonlySet<string>): boolean {
  return row.demo.job_ids.some((jobId) => jobsWithVideos.has(jobId));
}

const OWN_TEAM_MIN_MATCHES = 3;

/**
 * Mi equipo when the user picked it; otherwise a guess from the scrims alone: the
 * library team in the most scrim matches, when it has at least three and no other team ties it.
 */
export function ownTeamId(input: { rows: readonly PartidaRow[]; explicit: string | null }): string | null {
  if (input.explicit !== null) return input.explicit;
  const counts = new Map<string, number>();
  for (const { demo, match } of input.rows) {
    if (match === null || demo.source !== 'scrim') continue;
    for (const team of match.teams) {
      if (team.team_id !== null) counts.set(team.team_id, (counts.get(team.team_id) ?? 0) + 1);
    }
  }
  const ranked = [...counts].sort((a, b) => b[1] - a[1]);
  const [top, runnerUp] = ranked;
  if (top === undefined || top[1] < OWN_TEAM_MIN_MATCHES || (runnerUp !== undefined && runnerUp[1] === top[1])) return null;
  return top[0];
}

/** How the own team fared in a row, or null when it did not play it or none is known. */
export function ownOutcome(row: PartidaRow, ownId: string | null): Outcome | null {
  if (ownId === null || row.match === null) return null;
  return teamOutcome(row.match, ownId);
}

/** The name of a library team as it appears in a row's match, for "Resultado de <equipo>". */
export function ownTeamName(rows: readonly PartidaRow[], teamId: string): string {
  for (const { match } of rows) {
    const team = match?.teams.find((entry) => entry.team_id === teamId);
    if (team !== undefined) return teamLabel(team);
  }
  return '';
}

/** A tag as the library stores it: no commas, trimmed, within the length the library allows. */
export function cleanTag(raw: string): string {
  return raw.replaceAll(',', ' ').split(/\s+/).filter(Boolean).join(' ').slice(0, MAX_TAG_LENGTH).trim();
}

/** A group on the list: one series with its maps, or a partida on its own. */
export type PartidaGroup =
  | { readonly kind: 'series'; readonly series: LibrarySeries; readonly rows: readonly PartidaRow[]; readonly date: string }
  | { readonly kind: 'single'; readonly row: PartidaRow; readonly date: string };

export type DayGroup = { readonly day: string; readonly groups: readonly PartidaGroup[] };

/** The local calendar day of an ISO date, as YYYY-MM-DD; empty for a date that does not parse. */
export function dayOf(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return '';
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
}

/**
 * Groups rows into days, newest first. The maps of a stored series sit together in
 * play order under one entry; the rest stand alone. A series with no row left
 * after filtering disappears, and a row whose series is unknown stands alone.
 */
export function groupPartidas(input: {
  rows: readonly PartidaRow[];
  series: readonly LibrarySeries[];
  sort: PartidaSort;
}): DayGroup[] {
  const seriesById = new Map(input.series.map((entry) => [entry.id, entry]));
  const members = new Map<string, PartidaRow[]>();
  const groups: PartidaGroup[] = [];
  for (const row of input.rows) {
    const known = row.match?.series_id !== undefined ? seriesById.get(row.match.series_id) : undefined;
    if (known === undefined) {
      groups.push({ kind: 'single', row, date: sortDate(row, input.sort) });
      continue;
    }
    members.set(known.id, [...(members.get(known.id) ?? []), row]);
  }
  for (const [id, rows] of members) {
    const series = seriesById.get(id);
    if (series === undefined) continue;
    const played = [...rows].sort((a, b) => Date.parse(partidaDate(a)) - Date.parse(partidaDate(b)));
    const date = rows.map((row) => sortDate(row, input.sort)).sort((a, b) => Date.parse(b) - Date.parse(a))[0] ?? '';
    groups.push({ kind: 'series', series, rows: played, date });
  }
  groups.sort((a, b) => Date.parse(b.date) - Date.parse(a.date));
  const days: DayGroup[] = [];
  for (const group of groups) {
    const day = dayOf(group.date);
    const last = days.at(-1);
    if (last !== undefined && last.day === day) days[days.length - 1] = { day, groups: [...last.groups, group] };
    else days.push({ day, groups: [group] });
  }
  return days;
}

export type VetoMark = { readonly action: 'pick' | 'decider'; readonly team: string };

export type SeriesVeto = {
  /** Banned maps in veto order, each with the team that banned it. */
  readonly bans: readonly { readonly map: string; readonly team: string }[];
  /** How each picked map, or the decider, came to be played. */
  readonly marks: ReadonlyMap<string, VetoMark>;
};

function stepTeam(series: LibrarySeries, step: VetoStep): string {
  return series.teams.find((team) => team.slot === step.team)?.name ?? '';
}

/** Reads the stored veto of a series; empty when none was stored. */
export function seriesVeto(series: LibrarySeries): SeriesVeto {
  const bans: { map: string; team: string }[] = [];
  const marks = new Map<string, VetoMark>();
  for (const step of series.vetoes) {
    if (step.action === 'ban') bans.push({ map: step.map, team: stepTeam(series, step) });
    else marks.set(step.map, { action: step.action, team: stepTeam(series, step) });
  }
  return { bans, marks };
}

function invert(outcome: Outcome | null): Outcome | null {
  if (outcome === 'win') return 'loss';
  if (outcome === 'loss') return 'win';
  return outcome;
}

/** Maps won by each series team, from the finished maps of its rows; null when the series has no two teams. */
export function seriesScore(series: LibrarySeries, rows: readonly PartidaRow[]): readonly [number, number] | null {
  const a = series.teams.find((team) => team.slot === 'a');
  const b = series.teams.find((team) => team.slot === 'b');
  if (a === undefined || b === undefined) return null;
  let wonA = 0;
  let wonB = 0;
  for (const { match } of rows) {
    if (match === null) continue;
    const outcomeA = teamOutcome(match, a.team_id);
    const outcome = outcomeA ?? invert(teamOutcome(match, b.team_id));
    if (outcome === 'win') wonA += 1;
    else if (outcome === 'loss') wonB += 1;
  }
  return [wonA, wonB];
}

const MAX_SUGGESTIONS = 6;

/** Existing tags that start with, then contain, what was typed and are not on the demo yet. */
export function tagSuggestions(input: { all: readonly string[]; typed: string; current: readonly string[] }): string[] {
  const typed = input.typed.trim().toLocaleLowerCase('es');
  const open = [...new Set(input.all)].filter((tag) => !input.current.includes(tag));
  const starts = open.filter((tag) => tag.toLocaleLowerCase('es').startsWith(typed));
  const inside = open.filter((tag) => !starts.includes(tag) && tag.toLocaleLowerCase('es').includes(typed));
  return [...starts, ...inside].slice(0, MAX_SUGGESTIONS);
}

/** Every tag used in the library, most used first. */
export function libraryTags(rows: readonly PartidaRow[]): string[] {
  const counts = new Map<string, number>();
  for (const { demo } of rows) {
    for (const tag of demo.tags) counts.set(tag, (counts.get(tag) ?? 0) + 1);
  }
  return [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'es')).map(([tag]) => tag);
}
