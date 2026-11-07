import { Fragment, type ReactNode } from 'react';
import Link from 'next/link';
import { Crosshair } from 'lucide-react';
import { LIBRARY_TEAM_KEYS, type LibraryTeamKey } from '@/lib/api/library';
import { endReasonLabel } from '@/lib/library/partidas';
import { roundBlocks, type TeamNames } from '@/lib/library/match-story';
import { roundEvidenceHref, type RoundRow } from '@/lib/library/rounds';
import { END_ICON, RoundResultTile } from '@/components/matches/round-result-tile';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** The end reasons the legend explains; each draws the icon RoundResultTile gives its first reason. */
const LEGEND: readonly { reasons: readonly string[]; label: string }[] = [
  { reasons: ['ct_eliminated_t', 't_eliminated_ct'], label: 'Eliminados' },
  { reasons: ['bomb_defused'], label: 'Bomba desactivada' },
  { reasons: ['bomb_exploded'], label: 'Bomba explotada' },
  { reasons: ['time_expired'], label: 'Tiempo agotado' },
  { reasons: ['t_surrender', 'ct_surrender'], label: 'Rendición' },
];

/**
 * Every round as a tile in the row of the team that won it, coloured by the side
 * it won on, cut at the half and before each overtime. A tile opens the round in Táctica.
 */
export function MatchRoundHistory({
  rows,
  names,
  winner,
  pathname,
  onOpenRound,
}: {
  rows: readonly RoundRow[];
  names: TeamNames;
  winner: LibraryTeamKey | null;
  pathname: string;
  onOpenRound: () => void;
}): ReactNode {
  const blocks = roundBlocks(rows);
  const reasons = new Set(rows.map((row) => row.round.end_reason));
  return (
    <div className="flex flex-col gap-4">
      <div className="overflow-x-auto pb-1">
        <div className="flex min-w-max gap-1">
          <div className="flex w-28 flex-none flex-col gap-2.5">
            <span aria-hidden className="h-4" />
            {LIBRARY_TEAM_KEYS.map((key) => (
              <span
                key={key}
                className={cn('flex h-7 items-center truncate text-body-sm', key === winner ? 'font-semibold text-fg-1' : 'text-fg-2')}
              >
                {names[key]}
              </span>
            ))}
            <span className="flex h-4 items-center text-meta text-fg-3">Ronda</span>
          </div>
          {blocks.map((block, index) => (
            <Fragment key={block.key}>
              {index > 0 ? (
                <span aria-hidden className="my-6.5 flex w-5 flex-none justify-center">
                  <span className="w-px bg-border" />
                </span>
              ) : null}
              <div role="group" aria-label={`${block.label}, ${block.wins.join('-')}`} className="flex flex-col gap-2.5">
                <span className="h-4 text-meta font-medium whitespace-nowrap text-fg-3">
                  {block.label} · {block.wins.join('-')}
                </span>
                {LIBRARY_TEAM_KEYS.map((key) => (
                  <div key={key} className="flex gap-1">
                    {block.rounds.map((row) => (
                      <RoundCell key={row.number} row={row} team={key} name={names[key]} pathname={pathname} onOpenRound={onOpenRound} />
                    ))}
                  </div>
                ))}
                <div aria-hidden className="flex gap-1">
                  {block.rounds.map((row) => (
                    <span key={row.number} className="h-4 w-7 text-center text-meta text-fg-3">
                      {row.number}
                    </span>
                  ))}
                </div>
              </div>
            </Fragment>
          ))}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-x-4.5 gap-y-2 text-meta text-fg-2">
        <LegendSwatch className="bg-side-ct" label="Ganada como CT" />
        <LegendSwatch className="bg-side-t" label="Ganada como T" />
        {LEGEND.filter((entry) => entry.reasons.some((reason) => reasons.has(reason))).map(({ reasons: [first = ''], label }) => {
          const Icon = END_ICON[first] ?? Crosshair;
          return (
            <span key={label} className="inline-flex items-center gap-1.5">
              <Icon aria-hidden className="size-3.5" />
              {label}
            </span>
          );
        })}
        <span className="ml-auto text-fg-3">Pulsa una ronda para abrirla en Táctica</span>
      </div>
    </div>
  );
}

function RoundCell({
  row,
  team,
  name,
  pathname,
  onOpenRound,
}: {
  row: RoundRow;
  team: LibraryTeamKey;
  name: string;
  pathname: string;
  onOpenRound: () => void;
}): ReactNode {
  if (row.winnerTeamKey !== team || row.winnerSide === '') {
    return (
      <span aria-hidden className="grid size-7 shrink-0 place-items-center">
        <span className="size-1 rounded-full bg-fg-4" />
      </span>
    );
  }
  return (
    <Link
      href={roundEvidenceHref(pathname, row.number, row.tick)}
      onClick={onOpenRound}
      aria-label={`Ronda ${row.number}, gana ${name} como ${row.winnerSide}: ${endReasonLabel(row.round.end_reason)}. Abrir en Táctica`}
      className={cn('grid rounded-sm transition-opacity hover:opacity-80', FOCUS_RING)}
    >
      <RoundResultTile side={row.winnerSide} endReason={row.round.end_reason} />
    </Link>
  );
}

function LegendSwatch({ className, label }: { className: string; label: string }): ReactNode {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span aria-hidden className={cn('size-3 rounded-sm', className)} />
      {label}
    </span>
  );
}
