import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { matchResult, type MatchResult } from '@/lib/library/partidas';
import { endedBySurrender } from '@/lib/library/rounds';
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
  const rows = rounds.kind === 'ready' ? rounds.rows : null;
  const result = matchResult({
    scores: { 'ct-start': leftScore, 't-start': rightScore },
    maxRounds: rounds.kind === 'ready' ? rounds.doc.demo.max_rounds : 0,
    surrendered: rows !== null && endedBySurrender(rows),
  });
  const winner = leaderOf(result);

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
    <section aria-label="Marcador e historial de rondas" className="studio-panel flex flex-col gap-6 p-4 @[40rem]/content:p-6">
      <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3 @[40rem]/content:gap-6">
        <TeamSide team={left} teamKey="ct-start" name={names['ct-start']} won={winner === 'ct-start'} record={rows && left ? sideRecord(rows, left.key) : null} />
        <div className="flex flex-col items-center gap-1.5">
          <p className="text-display font-bold whitespace-nowrap tabular-nums">
            <span className="sr-only">{`${names['ct-start']} ${leftScore}, ${names['t-start']} ${rightScore}`}</span>
            <span aria-hidden className={winner === 'ct-start' ? 'text-fg-1' : 'text-fg-3'}>{leftScore}</span>
            <span aria-hidden className="px-3.5 font-normal text-fg-4">:</span>
            <span aria-hidden className={winner === 't-start' ? 'text-fg-1' : 'text-fg-3'}>{rightScore}</span>
          </p>
          <ResultLine result={result} names={names} />
        </div>
        <TeamSide team={right} teamKey="t-start" name={names['t-start']} won={winner === 't-start'} record={rows && right ? sideRecord(rows, right.key) : null} align="end" />
      </div>
      <Separator tone="subtle" />
      {history}
    </section>
  );
}

function leaderOf(result: MatchResult): LibraryTeamKey | null {
  if (result.kind === 'won') return result.winner;
  if (result.kind === 'unfinished') return result.leader;
  return null;
}

function ResultLine({ result, names }: { result: MatchResult; names: TeamNames }): ReactNode {
  if (result.kind === 'draw') return <Badge variant="secondary">Empate</Badge>;
  if (result.kind === 'won') {
    return (
      <Badge variant="success">
        <Check aria-hidden />
        Gana {names[result.winner]}
      </Badge>
    );
  }
  return (
    <span className="flex flex-col items-center gap-1 text-center">
      <Badge variant="secondary" title="La demo termina antes de que un equipo gane la partida.">
        Demo incompleta
      </Badge>
      {result.leader === null ? null : <span className="text-meta text-fg-2">Va ganando {names[result.leader]}</span>}
    </span>
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
