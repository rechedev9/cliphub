'use client';

import { useId, type ReactNode } from 'react';
import Link from 'next/link';
import type { LibraryMatch, LibraryTeamKey } from '@/lib/api/library';
import { matchHref } from '@/lib/clips/routes';
import { teamByKey, teamLabel } from '@/lib/library/partidas';
import { roundEvidenceHref } from '@/lib/library/rounds';
import { ppText } from '@/lib/metrics/advanced';
import { favouredTeam, winPointText, type KillSwing } from '@/lib/metrics/match-view';
import { secondsText } from '@/lib/reports/scout-sections';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** The kills that moved the match's win chance most, each one a door into the replay at its tick. */
export function MatchSwings({
  match,
  swings,
  ctTeams,
  names,
  onOpenReplay,
}: {
  match: LibraryMatch;
  swings: readonly KillSwing[];
  ctTeams: ReadonlyMap<number, LibraryTeamKey>;
  names: ReadonlyMap<number, string>;
  onOpenReplay?: () => void;
}): ReactNode {
  const headingId = useId();
  if (swings.length === 0) return null;
  return (
    <section aria-labelledby={headingId} className="studio-panel @container flex flex-col gap-3 p-5">
      <h2 id={headingId} className="text-title font-semibold text-fg-1">
        Las bajas que más pesaron
      </h2>
      <ol className="grid grid-cols-2 gap-2.5 @min-[34rem]:grid-cols-3 @min-[50rem]:grid-cols-5">
        {swings.map((swing) => {
          const side = swing.delta > 0 ? 'CT' : 'T';
          const kill = winPointText(swing.point, names);
          return (
            <li
              key={`${swing.round}:${swing.tick}`}
              className={cn('flex min-w-0 flex-col gap-1.5 border border-l-2 border-border bg-surface-2 px-3 py-2', side === 'CT' ? 'border-l-side-ct' : 'border-l-side-t')}
            >
              <span className="text-meta font-medium tracking-wider text-fg-3 uppercase">Ronda {swing.round}</span>
              <span className={cn('text-stat font-bold', side === 'CT' ? 'text-side-ct' : 'text-side-t')}>{ppText(Math.abs(swing.delta))}</span>
              <span className="line-clamp-2 text-body-sm font-semibold [overflow-wrap:anywhere] text-fg-1" title={kill}>
                {kill}
              </span>
              <span className="text-meta [overflow-wrap:anywhere] text-fg-3">
                {secondsText(swing.seconds)} · a favor de {teamLabel(teamByKey(match, favouredTeam(ctTeams, swing)))} ({side})
              </span>
              <Button asChild variant="ghost" size="xs" className="-ml-2 mt-auto self-start">
                <Link href={roundEvidenceHref(matchHref(match.id), swing.round, swing.tick)} onClick={onOpenReplay}>
                  Ver en el Replay
                </Link>
              </Button>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
