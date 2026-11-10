'use client';

import { useMemo, type ComponentProps, type ReactNode } from 'react';
import type { CentreRound } from '@/lib/replay/centre-round';
import { docNames } from '@/lib/replay/centre-round';
import type { ReplayLayers } from '@/lib/replay/layers';
import { inFocus, playerInitials } from '@/lib/replay/player-focus';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { FOLLOW_HINT } from './replay-players';
import type { SetposCopy } from '@/lib/replay/setpos';
import type { RoundPovRun } from '@/lib/replay/round-pov';
import { teamNamesFor } from '@/lib/replay/map-hud';
import { playerCards } from '@/lib/replay/player-cards';
import { frameCursor } from '@/lib/tactical-replay';
import { timelineEvents, timelineSeconds } from '@/lib/tactical-timeline';
import { AnnotationPanel } from '@/components/tactical/spatial/annotation-panel';
import { PlayerPanel } from './player-panel';
import { RoundPatterns } from './round-patterns';
import { PovRuns, SetposList } from './round-logs';

/**
 * The right column: both teams at the playhead as two cards, and under them the
 * round's notes, POV runs, copied setpos lines and saved patterns. The events live in the kill feed over the map. It reads the
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
  pov,
  notes,
  patterns,
  folded,
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
  onCopySetpos: (entry: SetposCopy) => void;
  /** Library matches only: the POV runs of this visit and the button that records one. */
  pov: { runs: readonly RoundPovRun[]; onRecord: (slot: number) => void } | null;
  notes: ComponentProps<typeof AnnotationPanel> | null;
  /** Library rounds list the saved patterns they meet. */
  patterns: boolean;
  /** Folded, the panel is a strip with each player's initials. */
  folded: boolean;
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
  if (folded) {
    return (
      <ul aria-label="Jugadores" className="flex flex-col items-center gap-1">
        {[...sides.ct, ...sides.t].map((card) => (
          <li key={card.slot}>
            <button
              type="button"
              aria-pressed={focus.includes(card.slot)}
              aria-label={card.name}
              title={`${card.name}. ${FOLLOW_HINT}`}
              onClick={(event) => onPickSlot(card.slot, event.shiftKey)}
              className={cn(
                'grid size-8 place-items-center rounded-sm border text-meta font-semibold outline-none hover:bg-surface-3',
                card.side === 'CT' ? 'text-side-ct' : 'text-side-t',
                focus.includes(card.slot) ? 'border-fg-2 bg-surface-4' : 'border-border-strong',
                (!card.alive || !inFocus(focus, card.slot)) && 'opacity-45',
                FOCUS_RING,
              )}
            >
              {playerInitials(card.name)}
            </button>
          </li>
        ))}
      </ul>
    );
  }
  return (
    // One scroll for the column: on a short window the players come first and the notes follow.
    <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto overscroll-contain [scrollbar-width:thin]">
      <PlayerPanel
        sides={sides}
        teamNames={teamNames}
        teamScores={{ CT: focused.round.score_ct_before, T: focused.round.score_t_before }}
        hiddenSlots={layers.hiddenSlots}
        focus={focus}
        onPick={onPickSlot}
        onFocus={onFocus}
        onTogglePlayer={onTogglePlayer}
        onShowAll={onShowAll}
        pov={pov}
      />
      {notes === null ? null : (
        <section aria-label="Notas de la ronda" className="flex flex-col gap-1.5 pt-1">
          <h3 className="px-1 text-meta font-semibold text-fg-3">Notas</h3>
          <AnnotationPanel {...notes} tools={false} />
        </section>
      )}
      {pov === null || pov.runs.length === 0 ? null : <PovRuns runs={pov.runs} />}
      {setpos.length === 0 ? null : <SetposList setpos={setpos} onCopy={onCopySetpos} />}
      {patterns ? <RoundPatterns matchId={focused.matchId} round={focused.round.number} /> : null}
    </div>
  );
}
