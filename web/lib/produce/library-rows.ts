import type { LibraryDemo, LibraryMatch } from '../api/library.ts';
import type { HubMatch } from '../clips/hub.ts';
import { prettyMapName } from '../format.ts';
import { matchTitle, teamByKey, withoutDemo } from '../library/partidas.ts';

/** An analysed library demo that no Crear job covers yet: its card hands it to the flow. */
export type LibraryDemoRow = { readonly demo: LibraryDemo; readonly match: LibraryMatch };

type LibraryRowsInput = { demos: readonly LibraryDemo[]; matches: readonly LibraryMatch[]; rows: readonly HubMatch[] };

/** Ready library demos that the job rows miss, newest played first. A demo counts as covered
 * when a row shares its checksum or is one of its jobs. A shared or pro match has no .dem to record. */
export function libraryOnlyRows({ demos, matches, rows }: LibraryRowsInput): LibraryDemoRow[] {
  const covered = new Set<string>();
  for (const row of rows) {
    covered.add(row.match.id);
    if (row.match.demoSha256 !== undefined) covered.add(row.match.demoSha256);
  }
  const matchById = new Map(matches.map((match) => [match.id, match]));
  const out: LibraryDemoRow[] = [];
  for (const demo of demos) {
    if (demo.state !== 'ready' || demo.match_id === undefined || withoutDemo(demo.source)) continue;
    if (covered.has(demo.sha256) || demo.job_ids.some((jobId) => covered.has(jobId))) continue;
    const match = matchById.get(demo.match_id);
    if (match !== undefined) out.push({ demo, match });
  }
  return out.sort((a, b) => b.match.played_at.localeCompare(a.match.played_at) || b.demo.imported_at.localeCompare(a.demo.imported_at));
}

/** The analysed library demo behind each job row, keyed by job id: same checksum or one of its jobs. */
export function linkedLibraryRows({ demos, matches, rows }: LibraryRowsInput): Map<string, LibraryDemoRow> {
  const matchById = new Map(matches.map((match) => [match.id, match]));
  const bySha = new Map<string, LibraryDemoRow>();
  const byJob = new Map<string, LibraryDemoRow>();
  for (const demo of demos) {
    const match = demo.match_id === undefined ? undefined : matchById.get(demo.match_id);
    if (demo.state !== 'ready' || match === undefined) continue;
    bySha.set(demo.sha256, { demo, match });
    for (const jobId of demo.job_ids) byJob.set(jobId, { demo, match });
  }
  const out = new Map<string, LibraryDemoRow>();
  for (const row of rows) {
    const linked = byJob.get(row.match.id) ?? (row.match.demoSha256 === undefined ? undefined : bySha.get(row.match.demoSha256));
    if (linked !== undefined) out.set(row.match.id, linked);
  }
  return out;
}

/** "Los Galacticos vs Mokas" and "13-5", the team that started CT first; a side without a name reads "Equipo CT". */
export function libraryRowTeams(match: LibraryMatch): { title: string; score: string } {
  const ct = teamByKey(match, 'ct-start');
  const t = teamByKey(match, 't-start');
  if (ct === undefined || t === undefined) return { title: '', score: '' };
  return { title: matchTitle(match), score: `${ct.score}-${t.score}` };
}

/** The words a demo card answers to in the list filter: map, teams and every player name. */
export function demoSearchText({ map, player, library }: { map: string; player?: string | undefined; library?: LibraryMatch | undefined }): string {
  const parts = [map, prettyMapName(map), player ?? ''];
  if (library !== undefined) {
    parts.push(libraryRowTeams(library).title);
    for (const p of library.players ?? []) parts.push(p.name);
  }
  return foldSearch(parts.join(' '));
}

/** True when every word of the query appears in the card's text, ignoring case and accents. */
export function matchesDemoQuery(text: string, query: string): boolean {
  const words = foldSearch(query).split(/\s+/).filter((word) => word !== '');
  return words.every((word) => text.includes(word));
}

function foldSearch(value: string): string {
  return value.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}
