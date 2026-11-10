import type { ReactNode } from 'react';
import type { LibraryMatch } from '@/lib/api/library';
import type { RoundRow } from '@/lib/library/rounds';
import { adrLabel } from '@/lib/library/partidas';
import { EdgeTile, EdgeTileGrid } from '@/components/records/edge-tile';

/** The headline numbers of a partida as EDGE tiles: rounds, minutes, rounds won on each side and the best ADR. */
export function MatchTiles({
  match,
  rows,
  best,
}: {
  match: LibraryMatch;
  /** The rounds once read; null while they load, so the side tiles wait for them. */
  rows: readonly RoundRow[] | null;
  best: { steamid64: string; adr: number } | null;
}): ReactNode {
  const minutes = match.tickrate > 0 ? Math.round(match.duration_ticks / match.tickrate / 60) : null;
  const bestPlayer = best === null ? undefined : (match.players ?? []).find((player) => player.steamid64 === best.steamid64);
  const ctWins = rows?.filter((row) => row.winnerSide === 'CT').length ?? null;
  const tWins = rows?.filter((row) => row.winnerSide === 'T').length ?? null;
  return (
    <EdgeTileGrid label="Cifras de la partida">
      <EdgeTile label="Rondas" value={match.round_count} />
      {minutes === null ? null : <EdgeTile label="Duración" value={`${minutes} min`} />}
      {ctWins === null ? null : <EdgeTile label="Rondas ganadas como CT" value={ctWins} tone="ct" />}
      {tWins === null ? null : <EdgeTile label="Rondas ganadas como T" value={tWins} tone="t" />}
      {bestPlayer === undefined ? null : (
        <EdgeTile label="Mejor ADR" value={adrLabel(bestPlayer.damage, bestPlayer.rounds)} note={bestPlayer.name || bestPlayer.steamid64} tone="good" />
      )}
    </EdgeTileGrid>
  );
}
