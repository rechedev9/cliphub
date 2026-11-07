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
  onOpenTactical,
}: {
  match: LibraryMatch;
  swings: readonly KillSwing[];
  ctTeams: ReadonlyMap<number, LibraryTeamKey>;
  names: ReadonlyMap<number, string>;
  onOpenTactical?: () => void;
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
            <li key={`${swing.round}:${swing.tick}`} className="flex min-w-0 flex-col gap-1.5 rounded-md border border-border-subtle bg-surface-1 p-3">
              <span className={cn('text-stat font-bold', side === 'CT' ? 'text-side-ct' : 'text-side-t')}>{ppText(Math.abs(swing.delta))}</span>
              <span className="truncate text-body-sm font-semibold text-fg-1" title={kill}>
                {kill}
              </span>
              <span className="text-meta text-fg-3">
                Ronda {swing.round} · {secondsText(swing.seconds)} · a favor de {teamLabel(teamByKey(match, favouredTeam(ctTeams, swing)))} ({side})
              </span>
              <Button asChild variant="ghost" size="xs" className="-ml-2 self-start">
                <Link href={roundEvidenceHref(matchHref(match.id), swing.round, swing.tick)} onClick={onOpenTactical}>
                  Ver en Táctica
                </Link>
              </Button>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
