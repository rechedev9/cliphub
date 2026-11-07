import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import type { LibraryMatchTeam, LibraryTeamKey } from '@/lib/api/library';
import type { TacticalSide } from '@/lib/api/tactical';
import { sideRecord, type SideRecord, type TeamNames } from '@/lib/library/match-story';
import { MatchRoundHistory } from '@/components/matches/match-round-history';
import type { MatchRoundRows } from '@/components/matches/use-match-round-rows';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

/** The final score with the side each team started on and won from, over the round history. */
export function MatchScoreboard({
  teams,
  names,
  rounds,
  pathname,
  onOpenRound,
}: {
  teams: readonly [LibraryMatchTeam | undefined, LibraryMatchTeam | undefined];
  names: TeamNames;
  rounds: MatchRoundRows;
  pathname: string;
  onOpenRound: () => void;
}): ReactNode {
  const [left, right] = teams;
  const leftScore = left?.score ?? 0;
  const rightScore = right?.score ?? 0;
  let winner: LibraryTeamKey | null = null;
  if (leftScore > rightScore) winner = 'ct-start';
  if (rightScore > leftScore) winner = 't-start';
  const rows = rounds.kind === 'ready' ? rounds.rows : null;

  let history: ReactNode;
  if (rounds.kind === 'error') {
    history = (
      <div className="flex flex-wrap items-center gap-3">
        <p className="text-body-sm text-destructive">{rounds.message}</p>
        <Button type="button" size="sm" variant="outline" onClick={rounds.retry}>
          Reintentar
        </Button>
      </div>
    );
  } else if (rounds.kind === 'pending') history = <p className="text-body-sm text-fg-3">La partida aún se está analizando.</p>;
  else if (rows === null) history = <Skeleton role="status" aria-label="Cargando rondas" className="h-32 w-full" />;
  else if (rows.length === 0) history = <p className="text-body-sm text-fg-3">Esta partida no tiene rondas analizadas.</p>;
  else history = <MatchRoundHistory rows={rows} names={names} winner={winner} pathname={pathname} onOpenRound={onOpenRound} />;

  return (
    <section aria-label="Marcador e historial de rondas" className="studio-panel flex flex-col gap-6 p-6">
      <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-6">
        <TeamSide team={left} teamKey="ct-start" name={names['ct-start']} won={winner === 'ct-start'} record={rows && left ? sideRecord(rows, left.key) : null} />
        <div className="flex flex-col items-center gap-1.5">
          <p className="text-display font-bold whitespace-nowrap tabular-nums">
            <span className="sr-only">{`${names['ct-start']} ${leftScore}, ${names['t-start']} ${rightScore}`}</span>
            <span aria-hidden className={winner === 'ct-start' ? 'text-fg-1' : 'text-fg-3'}>{leftScore}</span>
            <span aria-hidden className="px-3.5 font-normal text-fg-4">:</span>
            <span aria-hidden className={winner === 't-start' ? 'text-fg-1' : 'text-fg-3'}>{rightScore}</span>
          </p>
          {winner === null ? (
            <Badge variant="secondary">Empate</Badge>
          ) : (
            <Badge variant="success">
              <Check aria-hidden />
              Gana {names[winner]}
            </Badge>
          )}
        </div>
        <TeamSide team={right} teamKey="t-start" name={names['t-start']} won={winner === 't-start'} record={rows && right ? sideRecord(rows, right.key) : null} align="end" />
      </div>
      <Separator tone="subtle" />
      {history}
    </section>
  );
}

function TeamSide({
  team,
  teamKey,
  name,
  won,
  record,
  align = 'start',
}: {
  team: LibraryMatchTeam | undefined;
  teamKey: LibraryTeamKey;
  name: string;
  won: boolean;
  record: SideRecord | null;
  align?: 'start' | 'end';
}): ReactNode {
  const start: TacticalSide = team?.start_side ?? (teamKey === 'ct-start' ? 'CT' : 'T');
  const other: TacticalSide = start === 'CT' ? 'T' : 'CT';
  const sides = record === null ? [] : [start, other].filter((side) => record[side].played > 0);
  return (
    <div className={cn('flex min-w-0 flex-col gap-1', align === 'end' && 'items-end text-right')}>
      <span className={cn('max-w-full truncate text-section font-semibold', won ? 'text-fg-1' : 'text-fg-2')}>{name}</span>
      <span className="text-body-sm text-fg-2">
        <span className={cn('font-semibold', start === 'CT' ? 'text-side-ct' : 'text-side-t')}>Empezó {start}</span>
        {record === null || sides.length === 0
          ? null
          : ` · ${sides.map((side) => `${record[side].won} de ${record[side].played} como ${side}`).join(', ')}`}
      </span>
    </div>
  );
}
