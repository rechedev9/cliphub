'use client';

import type { ComponentProps, ReactNode } from 'react';
import { Layers } from 'lucide-react';
import { MULTI_MAX_ROUNDS } from '@/lib/replay/playlist';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ReplayTimeline } from './replay-timeline';
import { RoundStrip } from './round-strip';

/** What the Superponer button says it superposes, and its tooltip, for the rounds and players in play. */
export function superposeCopy({
  multi,
  everyRound,
  anyFocus,
  focusNames,
  shown,
  total,
}: {
  multi: boolean;
  /** Every round of the list is superposed: players are in focus or the list opened superposed. */
  everyRound: boolean;
  anyFocus: boolean;
  /** The players in the foreground joined with "y". */
  focusNames: string;
  /** The rounds on the map now. */
  shown: number;
  /** The rounds of the list. */
  total: number;
}): { note: string; hint: string } {
  let note = `hasta ${MULTI_MAX_ROUNDS} rondas`;
  if (everyRound && !anyFocus) note = multi ? `${shown} de ${total}` : 'todas las de la lista';
  else if (everyRound) note = multi ? `${shown} de ${focusNames}` : `todas las de ${focusNames}`;
  else if (multi) note = `${shown} de ${MULTI_MAX_ROUNDS} como máximo`;
  let hint = `Superpone todas las rondas del alcance con ${focusNames} en primer plano, desde el final del tiempo de compra`;
  if (everyRound && !anyFocus) hint = 'Superpone todas las rondas de la lista desde el final del tiempo de compra';
  else if (!everyRound) hint = `Superpone hasta ${MULTI_MAX_ROUNDS} rondas desde el final del tiempo de compra: pulsa una ronda para sumarla. Elige un jugador para ver todas las suyas`;
  return { note, hint };
}

/**
 * The transport under the map and the side panels: Superponer and the strip of
 * rounds, the timeline with its marks, and the round's team voice.
 */
export function ReplayTransport({
  fullscreen,
  superpose,
  strip,
  timeline,
  voice,
}: {
  fullscreen: boolean;
  superpose: { on: boolean; disabled: boolean; note: string; hint: string; onToggle: () => void };
  strip: ComponentProps<typeof RoundStrip>;
  timeline: ComponentProps<typeof ReplayTimeline>;
  voice: ReactNode;
}): ReactNode {
  return (
    <div
      className={cn(
        'flex min-w-0 flex-col gap-1.5 rounded-lg border border-border bg-surface-1 px-3 py-2',
        fullscreen ? 'col-span-2 row-start-2' : 'min-[1180px]:col-span-3 min-[1180px]:row-start-2',
      )}
    >
      <div className="flex min-w-0 items-end gap-3">
        <Button
          type="button"
          size="xs"
          variant={superpose.on ? 'outline-primary' : 'outline'}
          aria-pressed={superpose.on}
          disabled={superpose.disabled}
          onClick={superpose.onToggle}
          className="h-11 shrink-0"
          title={`${superpose.hint} (M)`}
        >
          <Layers aria-hidden />
          <span className="flex flex-col items-start leading-tight">
            Superponer
            <span className="text-meta font-normal text-fg-3">{superpose.note}</span>
          </span>
        </Button>
        <RoundStrip {...strip} />
      </div>
      <ReplayTimeline {...timeline} />
      {voice}
    </div>
  );
}
