'use client';

import type { ReactNode } from 'react';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalEvent } from '@/lib/api/tactical';
import { eventKindLabel, siteLabel } from '@/lib/tactical-labels';
import { roundClockLabelFor } from '@/lib/tactical-timeline';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { cn } from '@/lib/utils';

/** A slot's display name, or the slot itself when the identity table has none. */
function nameOf(names: ReadonlyMap<number, string>, slot: number | undefined): string {
  if (slot === undefined) return 'alguien';
  return names.get(slot) ?? `slot ${slot}`;
}

/** The qualifiers the demo recorded for a kill, in a fixed order. */
function killNotes(event: TacticalEvent): string[] {
  const notes: string[] = [];
  if (event.opening) notes.push('apertura');
  if (event.traded) notes.push('tradeada');
  if (event.headshot) notes.push('HS');
  if (event.wallbang) notes.push('wallbang');
  if (event.through_smoke) notes.push('por humo');
  if (event.attacker_blind) notes.push('cegado');
  if (event.no_scope) notes.push('no scope');
  return notes;
}

function describe(event: TacticalEvent, names: ReadonlyMap<number, string>): string {
  switch (event.kind) {
    case TACTICAL_EVENT_KINDS.kill:
      return `${nameOf(names, event.actor_slot)} mata a ${nameOf(names, event.target_slot)}`;
    case TACTICAL_EVENT_KINDS.plant:
      return `${nameOf(names, event.actor_slot)} planta${event.site ? ` en ${siteLabel(event.site)}` : ''}`;
    case TACTICAL_EVENT_KINDS.defuse:
      return `${nameOf(names, event.actor_slot)} desactiva`;
    case TACTICAL_EVENT_KINDS.explode:
      return 'La bomba explota';
    case TACTICAL_EVENT_KINDS.bombDrop:
      return `${nameOf(names, event.actor_slot)} suelta la bomba`;
    default:
      return `${nameOf(names, event.actor_slot)} lanza ${eventKindLabel(event.kind).toLowerCase()}`;
  }
}

function sideClass(event: TacticalEvent): string {
  if (event.side === TACTICAL_SIDES.ct) return 'text-primary';
  if (event.side === TACTICAL_SIDES.t) return 'text-warning';
  return 'text-muted-foreground';
}

/**
 * The round as text. The canvas is `aria-hidden` by design — a moving picture is
 * not readable — so this list is the accessible rendering of the same facts, and
 * every entry seeks the replay to the moment it describes.
 */
export function TacticalEventList({
  timeline,
  events,
  names,
  onSeek,
}: {
  timeline: RoundTimeline;
  events: readonly TimelineEvent[];
  names: ReadonlyMap<number, string>;
  onSeek: (seconds: number) => void;
}): ReactNode {
  if (events.length === 0) {
    return (
      <p className="px-3 py-6 text-center text-[13px] leading-5 text-muted-foreground">
        Esta ronda no registró ningún evento.
      </p>
    );
  }

  return (
    <ol className="flex flex-col">
      {events.map((entry, index) => {
        const { event } = entry;
        const notes = event.kind === TACTICAL_EVENT_KINDS.kill ? killNotes(event) : [];
        return (
          <li
            // The index is part of the key because kind/tick/slots do not
            // identify an event: one player can throw two utilities on the same
            // tick, and neither carries a target. The list is a fixed ordered
            // slice of one round, so the index is stable for its lifetime.
            key={`${event.kind}-${event.tick}-${event.actor_slot ?? 'x'}-${event.target_slot ?? 'x'}-${index}`}
            className="border-b border-border/50 last:border-b-0"
          >
            <button
              type="button"
              onClick={() => onSeek(entry.seconds)}
              className="flex min-h-11 w-full items-baseline gap-3 px-3 py-2 text-left outline-none transition-colors hover:bg-primary/8 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
            >
              <span className="w-14 shrink-0 font-[family-name:var(--font-mono)] text-[11px] tabular-nums text-muted-foreground">
                {roundClockLabelFor(timeline, entry.seconds)}
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className={cn('text-[13px] leading-5', sideClass(event))}>
                  {describe(event, names)}
                </span>
                {event.weapon || notes.length > 0 || event.place ? (
                  <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.1em] text-muted-foreground">
                    {[event.weapon, event.place, ...notes].filter(Boolean).join(' · ')}
                  </span>
                ) : null}
              </span>
            </button>
          </li>
        );
      })}
    </ol>
  );
}
