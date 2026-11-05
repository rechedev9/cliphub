'use client';

import { useMemo, useState, type ComponentProps, type ReactNode } from 'react';
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

type Tab = 'players' | 'events' | 'notes';

/**
 * The right panel: who is alive with what, the round as text, and the notes.
 * It reads the playhead a few times a second, never per frame.
 */
export function ReplaySidePanel({
  focused,
  nowSeconds,
  tickOf,
  clockSecondsOf,
  layers,
  onTogglePlayer,
  onSeek,
  notes,
}: {
  focused: CentreRound | null;
  nowSeconds: number;
  tickOf: (round: CentreRound, seconds: number) => number;
  clockSecondsOf: (timeline: RoundTimeline, tick: number) => number;
  layers: ReplayLayers;
  onTogglePlayer: (slot: number) => void;
  onSeek: (clockSeconds: number) => void;
  notes: ComponentProps<typeof AnnotationPanel> | null;
}): ReactNode {
  const [tab, setTab] = useState<Tab>('players');
  const names = useMemo(() => (focused === null ? new Map<number, string>() : docNames(focused.doc)), [focused]);
  const rawEvents = useMemo(
    () => (focused === null ? [] : timelineEvents(focused.timeline, focused.round.events)),
    [focused],
  );

  const tabs: { value: Tab; label: string }[] = [
    { value: 'players', label: 'Jugadores' },
    { value: 'events', label: 'Eventos' },
  ];
  if (notes !== null) tabs.push({ value: 'notes', label: 'Notas' });
  const shown = notes === null && tab === 'notes' ? 'players' : tab;

  let body: ReactNode = null;
  if (focused === null) {
    body = <p className="text-body-sm text-fg-3">Los jugadores aparecen cuando carga la ronda.</p>;
  } else if (shown === 'players') {
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
    body = <PlayerPanel sides={sides} hiddenSlots={layers.hiddenSlots} onTogglePlayer={onTogglePlayer} />;
  } else if (shown === 'events') {
    const timeline = focused.timeline;
    body = (
      <div className="-mx-1 max-h-[60vh] overflow-y-auto overscroll-contain">
        <TacticalEventList
          timeline={timeline}
          events={focused.events}
          names={names}
          onSeek={(seconds) => onSeek(clockSecondsOf(timeline, timelineTick(timeline, seconds)))}
        />
      </div>
    );
  } else if (notes !== null) {
    body = <AnnotationPanel {...notes} />;
  }

  return (
    <aside aria-label="Jugadores y eventos de la ronda" className="flex min-w-0 flex-col gap-3">
      <Segmented label="Panel" value={shown} options={tabs} onChange={setTab} />
      {body}
    </aside>
  );
}
