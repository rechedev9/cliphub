'use client';

import type { MouseEvent, ReactNode } from 'react';
import { Crosshair, Eye, EyeOff, Video } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import { GRENADE_CATEGORIES, equipmentItem, rowInventory } from '@/lib/replay/equipment';
import type { PlayerCard } from '@/lib/replay/player-cards';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { formatDecimal } from '@/lib/format';
import { inFocus } from '@/lib/replay/player-focus';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import { EquipmentIcon, GearGlyph } from './equipment-icon';
import { AllTeamRow, FOLLOW_HINT } from './replay-players';
import { RowAction } from './round-logs';

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
    <span className="flex shrink-0 items-center gap-1 text-fg-2">
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
  hidden,
  selected,
  dimmed,
  onSelect,
  onToggle,
  onRecordPov,
}: {
  card: PlayerCard;
  hidden: boolean;
  selected: boolean;
  /** Others are in the foreground and this player is not. */
  dimmed: boolean;
  onSelect: (event: MouseEvent) => void;
  onToggle: () => void;
  onRecordPov: (() => void) | null;
}): ReactNode {
  const health = card.health ?? 0;
  const armorLabel = card.helmet === true ? 'Chaleco y casco' : 'Chaleco';
  return (
    <li
      className={cn(
        'group relative flex flex-col gap-1 rounded-sm px-2 py-1.5 transition-[background-color,opacity] duration-(--dur-fast)',
        selected ? 'bg-surface-4' : 'hover:bg-surface-3',
        (!card.alive || dimmed) && 'opacity-45',
        hidden && 'opacity-35',
      )}
    >
      <button
        type="button"
        onClick={onSelect}
        aria-pressed={selected}
        aria-label={`${selected ? 'Quitar del primer plano' : 'Poner en primer plano'} a ${card.name}`}
        title={FOLLOW_HINT}
        className={cn('absolute inset-0 rounded-sm outline-none', FOCUS_RING)}
      />
      <span className="pointer-events-none relative flex min-w-0 items-center gap-1.5">
        <span className={cn('min-w-0 flex-1 truncate text-body-sm font-semibold tracking-wide text-fg-1 uppercase', !card.alive && 'line-through decoration-fg-3')}>{card.name}</span>
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
      <span className="pointer-events-none relative flex min-w-0 items-center gap-2">
        <span className="h-1 flex-1 overflow-hidden rounded-full bg-surface-0" aria-hidden>
          <span className={cn('block h-full rounded-full', HEALTH_FILL[card.side])} style={{ width: `${health}%` }} />
        </span>
        <span className="w-7 text-right text-meta font-semibold tabular-nums text-fg-1" aria-label={card.alive ? `${health} de vida` : 'Muerto'}>
          {card.alive ? (card.health ?? '·') : '0'}
        </span>
      </span>
      <span className="pointer-events-none relative flex min-w-0 items-center gap-2">
        <span className="flex items-center gap-1 text-meta tabular-nums text-fg-2">
          {card.alive && card.armor !== null && card.armor > 0 ? <GearGlyph kind={card.helmet === true ? 'helmet' : 'armor'} label={`${armorLabel} (${card.armor})`} /> : null}
          ${money.format(card.money)}
        </span>
        <span className="ml-auto">
          <Utility card={card} />
        </span>
      </span>
      <span className="absolute top-1 right-1 hidden items-center gap-0.5 rounded-sm bg-surface-3 group-focus-within:flex group-hover:flex">
        {onRecordPov === null ? null : (
          <RowAction label={`Grabar su POV de esta ronda (${card.name})`} pressed={false} onClick={onRecordPov}>
            <Video aria-hidden />
          </RowAction>
        )}
        <RowAction label={hidden ? `Devolver a ${card.name} al mapa` : `Atenuar a ${card.name} en el mapa`} pressed={!hidden} onClick={onToggle}>
          {hidden ? <EyeOff aria-hidden /> : <Eye aria-hidden />}
        </RowAction>
      </span>
    </li>
  );
}

/**
 * Both teams at the playhead as two cards, each outlined in its side's colour,
 * with the team and its score on top and three lines per player. A click leaves
 * that player in the foreground, Shift and click adds another, and the first
 * row gives the whole team back.
 */
export function PlayerPanel({
  sides,
  teamNames,
  teamScores,
  hiddenSlots,
  focus,
  onPick,
  onFocus,
  onTogglePlayer,
  onShowAll,
  pov,
}: {
  sides: { ct: readonly PlayerCard[]; t: readonly PlayerCard[] };
  teamNames: Readonly<Record<TacticalSide, string>>;
  /** Rounds each side's team has won before the round shown. */
  teamScores: Readonly<Record<TacticalSide, number>>;
  hiddenSlots: readonly number[];
  focus: PlayerFocus;
  onPick: (slot: number, additive: boolean) => void;
  onFocus: (focus: number[]) => void;
  onTogglePlayer: (slot: number) => void;
  onShowAll: () => void;
  /** Library matches only: a row can record that player's POV of the round shown. */
  pov: { onRecord: (slot: number) => void } | null;
}): ReactNode {
  const groups = [
    { side: TACTICAL_SIDES.ct, cards: sides.ct },
    { side: TACTICAL_SIDES.t, cards: sides.t },
  ];
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <AllTeamRow focus={focus} onChange={onFocus} />
        {hiddenSlots.length === 0 ? null : (
          <Button type="button" size="xs" variant="ghost" onClick={onShowAll} className="h-6">
            Mostrar {hiddenSlots.length === 1 ? 'el atenuado' : `los ${hiddenSlots.length} atenuados`}
          </Button>
        )}
      </div>
      {groups.map(({ side, cards }) => {
        const alive = cards.filter((card) => card.alive).length;
        return (
          <section key={side} aria-label={`${teamNames[side]}, lado ${side}`} className={cn('flex flex-col rounded-sm border', CARD_EDGE[side])}>
            <h3 className="flex items-center gap-2 px-2 pt-1.5 pb-1">
              <span className="min-w-0 flex-1 truncate text-body-sm font-bold text-fg-1">{teamNames[side]}</span>
              <span className="text-meta tabular-nums text-fg-3" title="Jugadores vivos">
                <span className="text-fg-1">{alive}</span>/{cards.length}
              </span>
              <span className="min-w-5 text-right text-body-sm font-bold tabular-nums text-fg-1" title="Rondas ganadas antes de esta">
                <span aria-hidden>{teamScores[side]}</span>
                <span className="sr-only">{teamScores[side]} rondas ganadas antes de esta</span>
              </span>
            </h3>
            <ul className="flex flex-col px-0.5 pb-0.5">
              {cards.map((card) => (
                <PlayerRow
                  key={card.slot}
                  card={card}
                  hidden={hiddenSlots.includes(card.slot)}
                  selected={focus.includes(card.slot)}
                  dimmed={!inFocus(focus, card.slot)}
                  onSelect={(event) => onPick(card.slot, event.shiftKey)}
                  onToggle={() => onTogglePlayer(card.slot)}
                  onRecordPov={pov === null ? null : () => pov.onRecord(card.slot)}
                />
              ))}
            </ul>
          </section>
        );
      })}
    </div>
  );
}
