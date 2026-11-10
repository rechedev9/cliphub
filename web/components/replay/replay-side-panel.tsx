'use client';

import { useMemo, type ReactNode } from 'react';
import type { CentreRound } from '@/lib/replay/centre-round';
import { docNames } from '@/lib/replay/centre-round';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import { teamNamesFor } from '@/lib/replay/map-hud';
import { playerCards } from '@/lib/replay/player-cards';
import { frameCursor } from '@/lib/tactical-replay';
import { timelineEvents, timelineSeconds } from '@/lib/tactical-timeline';
import { PlayerPanel } from './player-panel';

/**
 * The right column: both teams at the playhead as two cards. The events live in
 * the kill feed over the map. It reads the playhead a few times a second, never per frame.
 */
export function ReplaySidePanel({
  focused,
  nowSeconds,
  tickOf,
  focus,
  onPickSlot,
}: {
  focused: CentreRound | null;
  nowSeconds: number;
  tickOf: (round: CentreRound, seconds: number) => number;
  focus: PlayerFocus;
  onPickSlot: (slot: number, additive: boolean) => void;
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
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2 overflow-x-hidden overflow-y-auto overscroll-contain [scrollbar-width:thin]">
      <PlayerPanel
        sides={sides}
        teamNames={teamNames}
        teamScores={{ CT: focused.round.score_ct_before, T: focused.round.score_t_before }}
        focus={focus}
        onPick={onPickSlot}
      />
    </div>
  );
}
