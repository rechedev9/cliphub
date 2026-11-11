'use client';

import type { MouseEvent, ReactNode } from 'react';
import { Crosshair } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import { GRENADE_CATEGORIES, equipmentItem, rowInventory } from '@/lib/replay/equipment';
import type { PlayerCard } from '@/lib/replay/player-cards';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { formatDecimal } from '@/lib/format';
import { inFocus } from '@/lib/replay/player-focus';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import { EquipmentIcon, GearGlyph } from './equipment-icon';
import { FOLLOW_HINT } from './replay-players';

const money = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 0 });

const CARD_EDGE: Record<TacticalSide, string> = { CT: 'border-side-ct', T: 'border-side-t' };
const HEALTH_FILL: Record<TacticalSide, string> = { CT: 'bg-side-ct', T: 'bg-side-t' };

/** The gun in hand, else the primary then the pistol, else the last kill's weapon: the one weapon line 1 shows. */
function WeaponIcon({ card }: { card: PlayerCard }): ReactNode {
  if (!card.alive) return null;
  const carried = card.inventory === null ? [] : rowInventory(card.inventory);
  const held = card.activeWeapon === null ? undefined : equipmentItem(card.activeWeapon);
  const gun = held !== undefined && !GRENADE_CATEGORIES.has(held.category) ? held.id : carried.find((item) => !GRENADE_CATEGORIES.has(item.category) && item.category !== 'c4')?.id;
  const shown = gun ?? card.lastWeapon;
  return shown === null ? null : <EquipmentIcon name={shown} active />;
}

/** Line 3, right: the grenades carried, the bomb, the kit and a flash blind. */
function Utility({ card }: { card: PlayerCard }): ReactNode {
  if (!card.alive) return null;
  const grenades = card.inventory === null ? [] : rowInventory(card.inventory).filter((item) => GRENADE_CATEGORIES.has(item.category));
  const bomb = card.hasBomb || (card.inventory?.some((entry) => entry.item === 'c4') ?? false);
  return (
    <span className="flex min-w-0 items-center gap-1 text-fg-2">
      {grenades.map((item, index) => (
        <EquipmentIcon key={`${item.id}-${index}`} name={item.id} />
      ))}
      {bomb ? <GearGlyph kind="c4" label="Lleva la bomba" className="text-destructive" /> : null}
      {card.defuser === true ? <GearGlyph kind="kit" label="Kit de desactivación" className="text-side-ct" /> : null}
      {card.flashed > 0 ? (
        <span className="rounded-sm bg-fg-1 px-1 text-meta leading-4 font-semibold text-surface-0" title={`Cegado ${formatDecimal(card.flashed, 1)} s`}>
          Cegado
        </span>
      ) : null}
    </span>
  );
}

function PlayerRow({
  card,
  selected,
  dimmed,
  onSelect,
  onHover,
}: {
  card: PlayerCard;
  selected: boolean;
  /** Others are in the foreground and this player is not. */
  dimmed: boolean;
  onSelect: (event: MouseEvent) => void;
  onHover: (element: HTMLElement | null) => void;
}): ReactNode {
  const health = card.health ?? 0;
  const armorLabel = card.helmet === true ? 'Chaleco y casco' : 'Chaleco';
  return (
    <li
      onMouseEnter={(event) => onHover(event.currentTarget)}
      onMouseLeave={() => onHover(null)}
      onFocus={(event) => onHover(event.currentTarget)}
      onBlur={() => onHover(null)}
      className={cn(
        'relative flex min-w-0 flex-col gap-0.5 rounded-sm px-2 py-0.5 transition-[background-color,opacity] duration-(--dur-fast)',
        selected ? 'bg-surface-4' : 'hover:bg-surface-3',
        (!card.alive || dimmed) && 'opacity-45',
      )}
    >
      <button
        type="button"
        onClick={onSelect}
        aria-pressed={selected}
        aria-label={`${selected ? 'Quitar del primer plano' : 'Poner en primer plano'} a ${card.name} y copiar su setpos`}
        title={FOLLOW_HINT}
        className={cn('absolute inset-0 rounded-sm outline-none', FOCUS_RING)}
      />
      <span className="pointer-events-none relative flex min-w-0 items-center gap-1.5">
        <span className={cn('min-w-0 flex-1 truncate text-body-sm leading-4 font-semibold tracking-wide text-fg-1 uppercase', !card.alive && 'line-through decoration-fg-3')}>{card.name}</span>
        <span
          className="flex items-center gap-0.5 text-meta tabular-nums text-fg-3"
          title={
            card.assists === null
              ? `${card.kills} ${card.kills === 1 ? 'baja' : 'bajas'}; las asistencias se cuentan al acabar la ronda`
              : `${card.kills} ${card.kills === 1 ? 'baja' : 'bajas'} y ${card.assists} ${card.assists === 1 ? 'asistencia' : 'asistencias'} en la ronda`
          }
        >
          <span aria-hidden className="flex items-center gap-0.5">
            <Crosshair className="size-3" />
            <span className={cn(card.kills > 0 && 'text-fg-1')}>{card.kills}</span>/<span>{card.assists ?? '·'}</span>
          </span>
          <span className="sr-only">
            {card.kills} {card.kills === 1 ? 'baja' : 'bajas'}
            {card.assists === null ? '' : `, ${card.assists} ${card.assists === 1 ? 'asistencia' : 'asistencias'}`}
          </span>
        </span>
        <WeaponIcon card={card} />
      </span>
      <span className="pointer-events-none relative block h-[3px] overflow-hidden rounded-full bg-surface-0" aria-hidden>
        <span className={cn('block h-full rounded-full', HEALTH_FILL[card.side])} style={{ width: `${health}%` }} />
      </span>
      <span className="pointer-events-none relative flex min-w-0 items-center gap-2">
        <span className="w-6 text-meta font-semibold tabular-nums text-fg-1" aria-label={card.alive ? `${health} de vida` : 'Muerto'}>
          {card.alive ? (card.health ?? '·') : '0'}
        </span>
        <span className="flex shrink-0 items-center gap-1 text-meta tabular-nums text-fg-2">
          {card.alive && card.armor !== null && card.armor > 0 ? <GearGlyph kind={card.helmet === true ? 'helmet' : 'armor'} label={`${armorLabel} (${card.armor})`} /> : null}
          ${money.format(card.money)}
        </span>
        <span className="ml-auto min-w-0 overflow-hidden">
          <Utility card={card} />
        </span>
      </span>
    </li>
  );
}

/**
 * Both teams at the playhead as two cards, each outlined in its side's colour,
 * with the team and its score on top and three lines per player. A click leaves
 * that player in the foreground and copies their setpos, Shift and click adds another.
 */
export function PlayerPanel({
  sides,
  teamNames,
  teamScores,
  focus,
  onPick,
  onHover,
}: {
  sides: { ct: readonly PlayerCard[]; t: readonly PlayerCard[] };
  teamNames: Readonly<Record<TacticalSide, string>>;
  /** Rounds each side's team has won before the round shown. */
  teamScores: Readonly<Record<TacticalSide, number>>;
  focus: PlayerFocus;
  onPick: (slot: number, additive: boolean) => void;
  /** The row under the pointer or the keyboard, null when it leaves. */
  onHover: (slot: number, element: HTMLElement | null) => void;
}): ReactNode {
  const groups = [
    { side: TACTICAL_SIDES.ct, cards: sides.ct },
    { side: TACTICAL_SIDES.t, cards: sides.t },
  ];
  return (
    <div className="flex min-w-0 flex-col gap-2">
      {groups.map(({ side, cards }) => {
        const alive = cards.filter((card) => card.alive).length;
        return (
          <section key={side} aria-label={`${teamNames[side]}, lado ${side}`} className={cn('flex min-w-0 flex-col overflow-hidden rounded-sm border', CARD_EDGE[side])}>
            <h3 className="flex items-center gap-2 px-2 pt-1 pb-0.5">
              <span className="min-w-0 flex-1 truncate text-body-sm font-bold text-fg-1">{teamNames[side]}</span>
              <span className="text-meta tabular-nums text-fg-3" title="Jugadores vivos">
                <span className="text-fg-1">{alive}</span>/{cards.length}
              </span>
              <span className="min-w-5 text-right text-body-sm font-bold tabular-nums text-fg-1" title="Rondas ganadas antes de esta">
                <span aria-hidden>{teamScores[side]}</span>
                <span className="sr-only">{teamScores[side]} rondas ganadas antes de esta</span>
              </span>
            </h3>
            <ul className="flex min-w-0 flex-col px-0.5 pb-0.5">
              {cards.map((card) => (
                <PlayerRow
                  key={card.slot}
                  card={card}
                  selected={focus.includes(card.slot)}
                  dimmed={!inFocus(focus, card.slot)}
                  onSelect={(event) => onPick(card.slot, event.shiftKey)}
                  onHover={(element) => onHover(card.slot, element)}
                />
              ))}
            </ul>
          </section>
        );
      })}
    </div>
  );
}
