import type { LibraryDemo, LibraryMatch } from '../api/library.ts';
import type { HubMatch } from '../clips/hub.ts';
import { matchTitle, teamByKey } from '../library/partidas.ts';

/** An analysed library demo that no Crear job covers yet: its card hands it to the flow. */
export type LibraryDemoRow = { readonly demo: LibraryDemo; readonly match: LibraryMatch };

/** Ready library demos that the job rows miss, newest played first. A demo counts as covered
 * when a row shares its checksum or is one of its jobs. */
export function libraryOnlyRows({ demos, matches, rows }: { demos: readonly LibraryDemo[]; matches: readonly LibraryMatch[]; rows: readonly HubMatch[] }): LibraryDemoRow[] {
  const covered = new Set<string>();
  for (const row of rows) {
    covered.add(row.match.id);
    if (row.match.demoSha256 !== undefined) covered.add(row.match.demoSha256);
  }
  const matchById = new Map(matches.map((match) => [match.id, match]));
  const out: LibraryDemoRow[] = [];
  for (const demo of demos) {
    if (demo.state !== 'ready' || demo.match_id === undefined) continue;
    if (covered.has(demo.sha256) || demo.job_ids.some((jobId) => covered.has(jobId))) continue;
    const match = matchById.get(demo.match_id);
    if (match !== undefined) out.push({ demo, match });
  }
  return out.sort((a, b) => b.match.played_at.localeCompare(a.match.played_at) || b.demo.imported_at.localeCompare(a.demo.imported_at));
}

/** "Los Galacticos vs Mokas" and "13-5", the team that started CT first; a side without a name reads "Equipo CT". */
export function libraryRowTeams(match: LibraryMatch): { title: string; score: string } {
  const ct = teamByKey(match, 'ct-start');
  const t = teamByKey(match, 't-start');
  if (ct === undefined || t === undefined) return { title: '', score: '' };
  return { title: matchTitle(match), score: `${ct.score}-${t.score}` };
}
