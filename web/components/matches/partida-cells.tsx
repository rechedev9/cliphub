'use client';

import type { ReactNode } from 'react';
import { Clapperboard, RefreshCw } from 'lucide-react';
import { deleteLibraryDemo, isLibraryNotFound, type LibraryMatch } from '@/lib/api/library';
import { PRODUCE_FORMAT } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { PARSE_STATE_LABEL, demoErrorLabel, matchTitle, scoreline, withoutDemo, type Outcome, type PartidaRow } from '@/lib/library/partidas';
import { cn } from '@/lib/utils';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { OpenInCrearButton } from '@/components/matches/open-in-crear-button';
import { TeamLogo } from '@/components/matches/team-logo';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

const OUTCOME_CLASS: Record<Outcome, string> = {
  win: 'font-semibold text-fg-1',
  loss: 'text-fg-3',
  draw: 'text-fg-2',
};

const STACK_SCORE: Record<Outcome, string> = {
  win: 'text-success',
  loss: 'text-destructive',
  draw: 'text-fg-2',
};

const STACK_NAME: Record<Outcome, string> = {
  win: 'font-bold text-fg-1',
  loss: 'text-fg-3',
  draw: 'text-fg-2',
};

/** The two scores stacked beside their team names, the winner in green and bold, the loser in red and dim. */
export function ScoreStack({ match }: { match: LibraryMatch }): ReactNode {
  return (
    <span title={matchTitle(match)} className="flex min-w-40 flex-col gap-0.5 text-body-sm">
      {scoreline(match).map((side, index) => (
        <span key={index} className="grid grid-cols-[1.75rem_1rem_minmax(0,1fr)] items-center gap-2">
          <span className={cn('text-right font-bold tabular-nums', STACK_SCORE[side.outcome])}>{side.score}</span>
          <TeamLogo name={side.name} className="size-4" />
          <span className={cn('truncate', STACK_NAME[side.outcome])}>{side.name}</span>
        </span>
      ))}
    </span>
  );
}

export function isPending(row: PartidaRow): boolean {
  return row.demo.state === 'queued' || row.demo.state === 'parsing';
}

/** "Vitality 13 : 7 MOUZ", the winner in full ink and the loser dimmed. */
export function Scoreline({ match }: { match: LibraryMatch }): ReactNode {
  const [left, right] = scoreline(match);
  return (
    <span title={matchTitle(match)} className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3">
      <span className={cn('truncate text-right', OUTCOME_CLASS[left.outcome])}>{left.name}</span>
      <span className="text-body whitespace-nowrap tabular-nums">
        <span className={OUTCOME_CLASS[left.outcome]}>{left.score}</span>
        <span aria-hidden className="px-1.5 text-fg-3">:</span>
        <span className="sr-only"> a </span>
        <span className={OUTCOME_CLASS[right.outcome]}>{right.score}</span>
      </span>
      <span className={cn('truncate', OUTCOME_CLASS[right.outcome])}>{right.name}</span>
    </span>
  );
}

/** Why a row has no score yet: still in the queue, being analysed, or the reason the scan failed. */
export function DemoStatus({ row, className }: { row: PartidaRow; className?: string }): ReactNode {
  const { demo } = row;
  if (demo.state !== 'failed') return <span className="text-fg-3">{PARSE_STATE_LABEL[demo.state]}</span>;
  const reason = demoErrorLabel(demo.error_code);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {/* Focusable so the full reason also opens from the keyboard. */}
        <span tabIndex={0} className={cn('block truncate rounded-sm text-destructive', FOCUS_RING, className)}>
          {reason}
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="start" className="max-w-sm">
        {reason}
      </TooltipContent>
    </Tooltip>
  );
}

/** onRetry is absent in the web edition: the .dem stays in the browser that read it. */
export type RowHandlers = { onRetry?: (demoId: string) => void; onRemoved: () => void };

/** A demo already gone counts as removed: the row only has to leave the list. */
export async function removeDemo(demoId: string): Promise<void> {
  try {
    await deleteLibraryDemo(demoId);
  } catch (error) {
    if (!isLibraryNotFound(error)) throw error;
  }
}

/** A ready row offers its Short; a failed one, a new try or its removal; a match still re-analysing says so. */
export function RowActions({ row, handlers }: { row: PartidaRow; handlers: RowHandlers }): ReactNode {
  const { demo, match } = row;
  if (demo.state === 'failed') {
    return (
      <span className="flex items-center justify-end gap-2">
        {handlers.onRetry === undefined ? null : (
          <Button type="button" variant="outline" size="xs" onClick={() => handlers.onRetry?.(demo.id)}>
            <RefreshCw aria-hidden />
            Reintentar
          </Button>
        )}
        <DeleteMatchButton label={match ? prettyMapName(match.map) : demo.file_name} onConfirm={() => removeDemo(demo.id)} onDeleted={handlers.onRemoved} />
      </span>
    );
  }
  if (match === null || withoutDemo(demo.source)) return null;
  if (demo.state !== 'ready') return <span className="block text-right whitespace-nowrap text-fg-3">{PARSE_STATE_LABEL[demo.state]}</span>;
  return (
    <span className="flex justify-end">
      <OpenInCrearButton request={{ demo, format: PRODUCE_FORMAT.short }} variant="ghost" size="xs">
        <Clapperboard aria-hidden />
        Crear Short
      </OpenInCrearButton>
    </span>
  );
}
