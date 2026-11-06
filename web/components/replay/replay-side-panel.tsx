'use client';

import { useMemo, useState, type ComponentProps, type ReactNode } from 'react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalRound, TacticalSide } from '@/lib/api/tactical';
import type { CentreRound } from '@/lib/replay/centre-round';
import { docNames } from '@/lib/replay/centre-round';
import type { ReplayLayers } from '@/lib/replay/layers';
import { playerCards } from '@/lib/replay/player-cards';
import { frameCursor } from '@/lib/tactical-replay';
import { timelineEvents, timelineSeconds, timelineTick } from '@/lib/tactical-timeline';
import type { RoundTimeline } from '@/lib/tactical-timeline';
import { TacticalEventList } from '@/components/tactical/tactical-event-list';
import { AnnotationPanel } from '@/components/tactical/spatial/annotation-panel';
import { Segmented } from '@/components/ui/segmented';
import { PlayerPanel } from './player-panel';

type Tab = 'events' | 'notes';

/** Each side's team name in this round: sides swap at the half, the players do not. */
function teamNamesFor(doc: TacticalDocument, round: TacticalRound): Record<TacticalSide, string> {
  const teamOf = new Map(doc.players.map((player) => [player.slot, player.team_key]));
  const nameOf = new Map(doc.teams.map((team) => [team.key, team.name || team.key]));
  const out: Record<TacticalSide, string> = { CT: 'Antiterroristas', T: 'Terroristas' };
  for (const side of [TACTICAL_SIDES.ct, TACTICAL_SIDES.t]) {
    const player = round.players.find((entry) => entry.side === side);
    const key = player === undefined ? undefined : teamOf.get(player.slot);
    const name = key === undefined ? undefined : nameOf.get(key);
    if (name !== undefined) out[side] = name;
  }
  return out;
}

/**
 * The right column: both teams at the playhead on top, then the round as a
 * list of events or its notes, which scroll inside the column. It reads the
 * playhead a few times a second, never per frame.
 */
export function ReplaySidePanel({
  focused,
  nowSeconds,
  tickOf,
  clockSecondsOf,
  layers,
  selectedSlot,
  onSelectSlot,
  onTogglePlayer,
  follow,
  onSeek,
  notes,
}: {
  focused: CentreRound | null;
  nowSeconds: number;
  tickOf: (round: CentreRound, seconds: number) => number;
  clockSecondsOf: (timeline: RoundTimeline, tick: number) => number;
  layers: ReplayLayers;
  selectedSlot: number | null;
  onSelectSlot: (slot: number | null) => void;
  onTogglePlayer: (slot: number) => void;
  /** The first-person button on each row, while the centre view can follow a player. */
  follow: { slot: number | null; onFollow: (slot: number | null) => void } | null;
  onSeek: (clockSeconds: number) => void;
  notes: ComponentProps<typeof AnnotationPanel> | null;
}): ReactNode {
  const [tab, setTab] = useState<Tab>('events');
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
  const tabs: { value: Tab; label: string }[] = [{ value: 'events', label: 'Eventos' }];
  if (notes !== null) tabs.push({ value: 'notes', label: 'Notas' });
  const shown = notes === null ? 'events' : tab;
  const timeline = focused.timeline;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <PlayerPanel
        sides={sides}
        teamNames={teamNames}
        hiddenSlots={layers.hiddenSlots}
        selectedSlot={selectedSlot}
        onSelect={onSelectSlot}
        onTogglePlayer={onTogglePlayer}
        follow={follow}
      />
      <div className="flex min-h-0 flex-1 flex-col gap-2 border-t border-border-subtle pt-2">
        <Segmented label="Panel" value={shown} options={tabs} onChange={setTab} />
        <div className="-mx-1 min-h-0 flex-1 overflow-y-auto overscroll-contain px-1">
          {shown === 'events' || notes === null ? (
            <TacticalEventList
              timeline={timeline}
              events={focused.events}
              names={names}
              onSeek={(seconds) => onSeek(clockSecondsOf(timeline, timelineTick(timeline, seconds)))}
            />
          ) : (
            <AnnotationPanel {...notes} />
          )}
        </div>
      </div>
    </div>
  );
}
