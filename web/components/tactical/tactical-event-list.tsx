'use client';

import type { ReactNode } from 'react';
import { prettifyWeapon } from '@/lib/api/map';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalEvent } from '@/lib/api/tactical';
import { eventKindLabel, opponentSide, placeLabel, siteLabel } from '@/lib/tactical-labels';
import { roundClockLabelFor } from '@/lib/tactical-timeline';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { EquipmentIcon } from '@/components/replay/equipment-icon';

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

/** The grenade a utility event names, as an equipment id for its icon. */
const UTILITY_ITEM: Partial<Record<TacticalEvent['kind'], string>> = {
  [TACTICAL_EVENT_KINDS.smoke]: 'smokegrenade',
  [TACTICAL_EVENT_KINDS.flash]: 'flashbang',
  [TACTICAL_EVENT_KINDS.he]: 'hegrenade',
  [TACTICAL_EVENT_KINDS.molotov]: 'molotov',
  [TACTICAL_EVENT_KINDS.decoy]: 'decoy',
};

function victimClass(event: TacticalEvent): string {
  if (event.side === undefined) return 'text-fg-2';
  return opponentSide(event.side) === TACTICAL_SIDES.ct ? 'text-side-ct' : 'text-side-t';
}

function sideClass(event: TacticalEvent): string {
  if (event.side === TACTICAL_SIDES.ct) return 'text-side-ct';
  if (event.side === TACTICAL_SIDES.t) return 'text-side-t';
  return 'text-fg-3';
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
      <p className="px-3 py-6 text-center text-body-sm leading-5 text-fg-2">
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
            className="border-b border-border-subtle last:border-b-0"
          >
            <button
              type="button"
              onClick={() => onSeek(entry.seconds)}
              className={cn(
                'flex min-h-9 w-full items-baseline gap-2 px-2 py-1.5 text-left outline-none transition-colors hover:bg-surface-3',
                FOCUS_RING,
                'focus-visible:outline-offset-[-2px]',
              )}
            >
              <span className="w-11 shrink-0 text-meta tracking-normal tabular-nums text-fg-3">
                {roundClockLabelFor(timeline, entry.seconds)}
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                {event.kind === TACTICAL_EVENT_KINDS.kill ? (
                  <span className="flex min-w-0 items-center gap-1.5 text-body-sm leading-5">
                    <span className={cn('truncate', sideClass(event))}>{nameOf(names, event.actor_slot)}</span>
                    {event.weapon ? <EquipmentIcon name={event.weapon} active className="shrink-0" /> : <span className="text-fg-3">mata a</span>}
                    {event.headshot ? <span className="text-meta font-semibold text-fg-2" title="Disparo a la cabeza">HS</span> : null}
                    <span className={cn('truncate', victimClass(event))}>
                      {nameOf(names, event.target_slot)}
                    </span>
                  </span>
                ) : (
                  <span className={cn('flex min-w-0 items-center gap-1.5 text-body-sm leading-5', sideClass(event))}>
                    {UTILITY_ITEM[event.kind] === undefined ? null : <EquipmentIcon name={UTILITY_ITEM[event.kind] ?? ''} active />}
                    <span className="truncate">{describe(event, names)}</span>
                  </span>
                )}
                {event.weapon || notes.length > 0 || event.place ? (
                  <span className="text-meta text-fg-3">
                    {[event.kind === TACTICAL_EVENT_KINDS.kill && event.weapon ? prettifyWeapon(event.weapon) : '', event.place && placeLabel(event.place), ...notes.filter((note) => note !== 'HS')].filter(Boolean).join(' · ')}
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
