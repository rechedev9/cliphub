'use client';

import { useMemo, type ReactNode } from 'react';
import { ChevronDown, ListOrdered } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import type { CentreRound } from '@/lib/replay/centre-round';
import { docNames } from '@/lib/replay/centre-round';
import { BOMB_SECONDS, aliveCounts, bombState, hudClock, killFeed, teamNamesFor } from '@/lib/replay/map-hud';
import type { BombState } from '@/lib/replay/map-hud';
import { opponentSide, siteLabel } from '@/lib/tactical-labels';
import { frameCursor } from '@/lib/tactical-replay';
import { roundClockSeconds, timelineEvents, timelineTick } from '@/lib/tactical-timeline';
import { TacticalEventList } from '@/components/tactical/tactical-event-list';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { EquipmentIcon, GearGlyph } from './equipment-icon';

const SIDE_TEXT: Record<TacticalSide, string> = { CT: 'text-side-ct', T: 'text-side-t' };
const SIDE_BG: Record<TacticalSide, string> = { CT: 'bg-side-ct', T: 'bg-side-t' };
const HUD_SURFACE = 'bg-[#0b0c0e]/85 text-white shadow-[var(--elev-2)] backdrop-blur-[2px]';
/** The map is dark in both themes, so the HUD over it always takes the dark tokens. */
const HUD_SCOPE = 'dark';

/**
 * The game's HUD over the map: the score bug with the round clock (the bomb
 * timer once it is planted) on the left, and the kill feed on the right, which
 * opens into the round's whole list of events.
 */
export function MapHud({
  focused,
  roundSeconds,
  eventsOpen,
  onToggleEvents,
  onSeek,
}: {
  focused: CentreRound;
  /** The playhead in seconds on the focused round's own bar. */
  roundSeconds: number;
  eventsOpen: boolean;
  onToggleEvents: () => void;
  /** Seeks to a moment given in seconds on the round's own bar. */
  onSeek: (roundSeconds: number) => void;
}): ReactNode {
  const names = useMemo(() => docNames(focused.doc), [focused.doc]);
  const teams = useMemo(() => teamNamesFor(focused.doc, focused.round), [focused.doc, focused.round]);
  const allEvents = useMemo(() => timelineEvents(focused.timeline, focused.round.events), [focused.round.events, focused.timeline]);
  const tick = timelineTick(focused.timeline, roundSeconds);
  const cursor = frameCursor(focused.frames, tick);
  const alive = aliveCounts(cursor === undefined ? [] : (focused.frames[cursor.index]?.samples ?? []));
  const bomb = bombState(allEvents, roundSeconds);
  const feed = killFeed(focused.events, roundSeconds);
  const score: Record<TacticalSide, number> = { CT: focused.round.score_ct_before, T: focused.round.score_t_before };

  return (
    <>
      <div className={cn(HUD_SCOPE, 'pointer-events-none absolute top-2 left-2 flex h-11 items-stretch overflow-hidden rounded-md text-body-sm', HUD_SURFACE)}>
        <TeamCell side={TACTICAL_SIDES.ct} name={teams.CT} score={score.CT} alive={alive.CT} />
        <ClockCell bomb={bomb} clock={hudClock(roundClockSeconds(focused.timeline, roundSeconds))} />
        <TeamCell side={TACTICAL_SIDES.t} name={teams.T} score={score.T} alive={alive.T} mirrored />
      </div>

      <div className={cn(HUD_SCOPE, 'pointer-events-none absolute top-2 right-2 bottom-14 flex w-80 max-w-[45%] flex-col items-end gap-1')}>
        <button
          type="button"
          aria-expanded={eventsOpen}
          onClick={onToggleEvents}
          className={cn(
            'pointer-events-auto flex h-8 items-center gap-1.5 rounded-md px-2.5 text-meta font-semibold transition-colors hover:bg-[#1b1d21]/90',
            HUD_SURFACE,
            FOCUS_RING,
          )}
        >
          <ListOrdered className="size-3.5" aria-hidden />
          Eventos
          <span className="text-white/60 tabular-nums">{focused.events.length}</span>
          <ChevronDown className={cn('size-3.5 transition-transform duration-150', eventsOpen && 'rotate-180')} aria-hidden />
        </button>
        {eventsOpen ? (
          <div className="pointer-events-auto min-h-0 w-full overflow-y-auto overscroll-contain rounded-md border border-border-strong bg-surface-1/95 shadow-[var(--elev-4)] [scrollbar-width:thin] motion-safe:animate-in motion-safe:slide-in-from-top-1">
            <TacticalEventList timeline={focused.timeline} events={focused.events} names={names} onSeek={onSeek} />
          </div>
        ) : (
          <ol aria-label="Últimas bajas" className="flex flex-col items-end gap-1">
            {feed.map((entry) => {
              const { event } = entry;
              const killer = event.side ?? TACTICAL_SIDES.ct;
              return (
                <li
                  key={`${event.tick}-${event.actor_slot ?? 'x'}-${event.target_slot ?? 'x'}`}
                  className={cn(
                    'flex h-7 max-w-full items-center gap-1.5 rounded-sm px-2 text-meta font-semibold motion-safe:animate-in motion-safe:slide-in-from-right-2',
                    HUD_SURFACE,
                    event.opening && 'ring-1 ring-white/30',
                  )}
                >
                  <span className={cn('truncate', SIDE_TEXT[killer])}>{names.get(event.actor_slot ?? -1) ?? 'alguien'}</span>
                  {event.weapon ? <EquipmentIcon name={event.weapon} active className="shrink-0" /> : null}
                  {event.headshot ? <span className="shrink-0 text-white/70" title="Disparo a la cabeza">HS</span> : null}
                  {event.wallbang ? <span className="shrink-0 text-white/70" title="A través de una pared">WB</span> : null}
                  <span className={cn('truncate', SIDE_TEXT[opponentSide(killer)])}>{names.get(event.target_slot ?? -1) ?? 'alguien'}</span>
                </li>
              );
            })}
          </ol>
        )}
      </div>
    </>
  );
}

function TeamCell({ side, name, score, alive, mirrored = false }: { side: TacticalSide; name: string; score: number; alive: number; mirrored?: boolean }): ReactNode {
  return (
    <div className={cn('flex items-center gap-2 px-2.5', mirrored && 'flex-row-reverse')}>
      <span className={cn('h-full w-1 shrink-0', SIDE_BG[side])} aria-hidden />
      <span className={cn('flex min-w-0 flex-col leading-4', mirrored && 'items-end')}>
        <span className="max-w-24 truncate text-meta text-white/75">{name}</span>
        <span className="flex gap-0.5" aria-label={`${alive} vivos`}>
          {[0, 1, 2, 3, 4].map((index) => (
            <span key={index} className={cn('h-1.5 w-2.5 rounded-[1px]', index < alive ? SIDE_BG[side] : 'bg-white/15')} />
          ))}
        </span>
      </span>
      <span className={cn('font-display text-title font-bold tabular-nums', SIDE_TEXT[side])}>{score}</span>
    </div>
  );
}

function ClockCell({ bomb, clock }: { bomb: BombState; clock: string }): ReactNode {
  if (bomb.kind === 'planted') {
    return (
      <div className="relative flex min-w-20 items-center justify-center gap-1.5 bg-destructive/25 px-2.5 text-destructive" aria-label={`Bomba plantada, ${Math.ceil(bomb.remaining)} segundos`}>
        <GearGlyph kind="c4" label="Bomba" className="size-4" />
        <span className="font-display text-title font-bold tabular-nums">{Math.ceil(bomb.remaining)}</span>
        {bomb.site === null ? null : <span className="text-meta font-semibold">{siteLabel(bomb.site)}</span>}
        <span className="absolute inset-x-0 bottom-0 h-0.5 origin-left bg-destructive" style={{ transform: `scaleX(${bomb.remaining / BOMB_SECONDS})` }} aria-hidden />
      </div>
    );
  }
  let note: ReactNode = null;
  if (bomb.kind === 'defused') note = <span className="text-meta font-semibold text-success">Desactivada</span>;
  if (bomb.kind === 'exploded') note = <span className="text-meta font-semibold text-destructive">Explotó</span>;
  return (
    <div className="flex min-w-20 flex-col items-center justify-center bg-white/5 px-2.5 leading-5">
      <span className="font-display text-title font-bold tabular-nums">{clock}</span>
      {note}
    </div>
  );
}
