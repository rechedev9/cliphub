import type { LibraryMatch, LibraryMatchPlayer } from '../api/library.ts';
import { loadErrorKind } from '../load-error.ts';
import { playerLine, teamByKey, teamLabel } from './partidas.ts';

export type PlayerMatchOutcome = 'win' | 'loss' | 'draw';

export type PlayerMatchRow = {
  match: LibraryMatch;
  teamName: string;
  rivalName: string;
  outcome: PlayerMatchOutcome | null;
};

export type PlayerMatchRows = {
  rows: PlayerMatchRow[];
  lines: LibraryMatchPlayer[];
  gone: number;
  failed: number;
};

export type PlayerMatchRowsInput = {
  steamId: string;
  results: readonly PromiseSettledResult<LibraryMatch>[];
};

function outcomeOf(own: number, rival: number): PlayerMatchOutcome {
  if (own > rival) return 'win';
  if (own < rival) return 'loss';
  return 'draw';
}

export function playerMatchRows({ steamId, results }: PlayerMatchRowsInput): PlayerMatchRows {
  const rows: PlayerMatchRow[] = [];
  const lines: LibraryMatchPlayer[] = [];
  let gone = 0;
  let failed = 0;
  for (const result of results) {
    if (result.status === 'rejected') {
      if (loadErrorKind(result.reason) === 'not_found') gone += 1;
      else failed += 1;
      continue;
    }
    const match = result.value;
    const line = playerLine(match, steamId);
    if (line === undefined) continue;
    lines.push(line);
    const own = teamByKey(match, line.team_key);
    const rival = match.teams.find((team) => team.key !== line.team_key);
    const outcome = own !== undefined && rival !== undefined ? outcomeOf(own.score, rival.score) : null;
    rows.push({ match, teamName: teamLabel(own), rivalName: teamLabel(rival), outcome });
  }
  return { rows, lines, gone, failed };
}

export function unreadMatchesText({ gone, failed }: Pick<PlayerMatchRows, 'gone' | 'failed'>): string {
  const parts: string[] = [];
  if (gone > 0) parts.push(gone === 1 ? '1 partida ya no está en la biblioteca' : `${gone} partidas ya no están en la biblioteca`);
  if (failed > 0) parts.push(failed === 1 ? 'no se pudo leer 1 partida' : `no se pudieron leer ${failed} partidas`);
  if (parts.length === 0) return '';
  const text = parts.join(' y ');
  return `${text.charAt(0).toUpperCase()}${text.slice(1)}. Las cifras cuentan solo las demás.`;
}
