'use client';

import { useMemo, type ComponentProps, type ReactNode } from 'react';
import type { CentreRound } from '@/lib/replay/centre-round';
import { docNames } from '@/lib/replay/centre-round';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import type { SetposCopy } from '@/lib/replay/setpos';
import { teamNamesFor } from '@/lib/replay/map-hud';
import { playerCards } from '@/lib/replay/player-cards';
import { frameCursor } from '@/lib/tactical-replay';
import { timelineEvents, timelineSeconds } from '@/lib/tactical-timeline';
import { AnnotationPanel } from '@/components/tactical/spatial/annotation-panel';
import { PlayerPanel } from './player-panel';

/**
 * The right column: both teams at the playhead, whole, and under them the
 * round's notes. The events live in the kill feed over the map. It reads the
 * playhead a few times a second, never per frame.
 */
export function ReplaySidePanel({
  focused,
  nowSeconds,
  tickOf,
  layers,
  focus,
  onPickSlot,
  onFocus,
  onTogglePlayer,
  onShowAll,
  setpos,
  onCopySetpos,
  notes,
}: {
  focused: CentreRound | null;
  nowSeconds: number;
  tickOf: (round: CentreRound, seconds: number) => number;
  layers: ReplayLayers;
  focus: PlayerFocus;
  onPickSlot: (slot: number, additive: boolean) => void;
  onFocus: (focus: number[]) => void;
  onTogglePlayer: (slot: number) => void;
  onShowAll: () => void;
  setpos: readonly SetposCopy[];
  onCopySetpos: (command: string) => void;
  notes: ComponentProps<typeof AnnotationPanel> | null;
}): ReactNode {
  const names = useMemo(() => (focused === null ? new Map<number, string>() : docNames(focused.doc)), [focused]);
  const rawEvents = useMemo(() => (focused === null ? [] : timelineEvents(focused.timeline, focused.round.events)), [focused]);
  const teamNames = useMemo(() => (focused === null ? null : teamNamesFor(focused.doc, focused.round)), [focused]);

  if (focused === null || teamNames === null) {
    return <p className="p-2 text-body-sm text-fg-3">Los jugadores aparecen cuando carga la ronda.</p>;
  }

  const nowTick = tickOf(focused, nowSeconds);
  const cursor = frameCursor(focused.frames, nowTick);
  const sides = playerCards({
    round: focused.round,
    samples: cursor === undefined ? [] : (focused.frames[cursor.index]?.samples ?? []),
    events: rawEvents,
    nowSeconds: timelineSeconds(focused.timeline, nowTick),
    nowTick,
    names,
  });
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <div className="min-h-0 shrink overflow-y-auto overscroll-contain [scrollbar-width:thin]">
      <PlayerPanel
        sides={sides}
        teamNames={teamNames}
        hiddenSlots={layers.hiddenSlots}
        focus={focus}
        onPick={onPickSlot}
        onFocus={onFocus}
        onTogglePlayer={onTogglePlayer}
        onShowAll={onShowAll}
        setpos={setpos}
        onCopySetpos={onCopySetpos}
      />
      </div>
      {notes === null ? null : (
        <section aria-label="Notas de la ronda" className="flex min-h-40 flex-1 flex-col gap-1.5 border-t border-border-subtle pt-2">
          <h3 className="px-1 text-meta font-semibold text-fg-3">Notas</h3>
          <div className="-mx-1 min-h-0 flex-1 overflow-y-auto overscroll-contain px-1 [scrollbar-width:thin]">
            <AnnotationPanel {...notes} tools={false} />
          </div>
        </section>
      )}
    </div>
  );
}
