'use client';

import { useCallback } from 'react';
import { SearchX } from 'lucide-react';
import type { Match } from '@/lib/api/types';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { MatchRow } from './match-row';
import { attachMatchRowParallax } from './row-parallax';

export type MatchListProps = {
  matches: Match[];
  /** Deletes a match by job id; when set, each row shows a trash button. */
  onDelete?: (jobId: string) => Promise<void>;
  /** Called after a successful delete so the page can re-fetch its lists. */
  onDeleted?: () => void;
};

/**
 * The scoreboard: one MatchRow per match (the first one featured), or an empty
 * state when filtered out.
 *
 * The list — not the row — owns the pointer tracking that drives every row's
 * lift and specular sweep, so a forty-match inbox still installs exactly one
 * `pointermove` listener and one rAF loop.
 */
export function MatchList({ matches, onDelete, onDeleted }: MatchListProps) {
  // React 19 runs the returned function as the ref cleanup, so the listener is
  // detached on unmount without a separate effect.
  const trackPointer = useCallback((node: HTMLElement | null): (() => void) | undefined => {
    if (node === null) return undefined;
    return attachMatchRowParallax(node);
  }, []);

  if (matches.length === 0) {
    return (
      <StudioEmptyState
        icon={SearchX}
        title="Sin resultados"
        description="Prueba otro mapa u otro filtro."
        compact
      />
    );
  }

  return (
    <section ref={trackPointer} className="flex flex-col gap-3" aria-label="Partidas disponibles">
      {matches.map((match, index) => (
        <MatchRow key={match.id} match={match} featured={index === 0} onDelete={onDelete} onDeleted={onDeleted} />
      ))}
    </section>
  );
}
