'use client';

import type { MouseEvent, ReactNode } from 'react';
import Link from 'next/link';
import { Copy, Crosshair, Eye, EyeOff, Video } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import { equipmentId, rowInventory } from '@/lib/replay/equipment';
import type { PlayerCard } from '@/lib/replay/player-cards';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { EquipmentIcon, GearGlyph } from './equipment-icon';
import { formatDecimal } from '@/lib/format';
import { inFocus } from '@/lib/replay/player-focus';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import type { SetposCopy } from '@/lib/replay/setpos';
import { roundPovLabel } from '@/lib/replay/round-pov';
import type { RoundPovRun } from '@/lib/replay/round-pov';
import { RESULTS_HREF } from '@/lib/clips/routes';
import { Button } from '@/components/ui/button';
import { AllTeamRow, FOCUS_HINT } from './replay-players';

const money = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 0 });


function hpTone(health: number): string {
  if (health > 50) return 'bg-fg-1';
  if (health > 20) return 'bg-fg-3';
  return 'bg-destructive';
}

function RowAction({ label, pressed, onClick, children }: { label: string; pressed: boolean; onClick: () => void; children: ReactNode }): ReactNode {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={cn(
        'grid size-6 place-items-center rounded-sm text-fg-3 outline-none hover:bg-surface-4 hover:text-fg-1 [&_svg]:size-3.5',
        FOCUS_RING,
      )}
    >
      {children}
    </button>
  );
}

function Equipment({ card }: { card: PlayerCard }): ReactNode {
  if (!card.alive) return null;
  if (card.inventory !== null) {
    const items = rowInventory(card.inventory);
    const active = card.activeWeapon === null ? null : equipmentId(card.activeWeapon);
    return (
      <span className="flex min-w-0 items-center gap-1.5 overflow-hidden">
        {items.map((item, index) => (
          <EquipmentIcon key={`${item.id}-${index}`} name={item.id} active={item.id === active} />
        ))}
      </span>
    );
  }
  // Without an inventory the last kill's weapon is the only gun the analysis knows.
  return card.lastWeapon === null ? null : (
    <span className="flex min-w-0 items-center gap-1.5" title="Arma de su última baja">
      <EquipmentIcon name={card.lastWeapon} />
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
  const ct = card.side === TACTICAL_SIDES.ct;
  const health = card.health ?? 0;
  const armorLabel = card.helmet === true ? 'Chaleco y casco' : 'Chaleco';
  return (
    <li
      className={cn(
        'group relative flex flex-col gap-1 rounded-sm py-1.5 pr-1 pl-2.5 transition-[background-color,opacity] duration-(--dur-fast)',
        selected ? 'bg-surface-4' : 'hover:bg-surface-3',
        (!card.alive || dimmed) && 'opacity-45',
        hidden && 'opacity-35',
      )}
    >
      <span className={cn('absolute top-1.5 bottom-1.5 left-0 w-0.5 rounded-full', ct ? 'bg-side-ct' : 'bg-side-t')} aria-hidden />
      <button
        type="button"
        onClick={onSelect}
        aria-pressed={selected}
        aria-label={`${selected ? 'Quitar del primer plano' : 'Poner en primer plano'} a ${card.name}`}
        title={FOCUS_HINT}
        className={cn('absolute inset-0 rounded-sm outline-none', FOCUS_RING)}
      />
      <span className="pointer-events-none relative flex min-w-0 items-center gap-1.5">
        <span className={cn('min-w-0 flex-1 truncate text-body-sm font-medium text-fg-1', !card.alive && 'line-through decoration-fg-3')}>{card.name}</span>
        {card.kills > 0 ? (
          <span className="flex items-center gap-0.5 text-meta tabular-nums text-fg-2" title={`${card.kills} ${card.kills === 1 ? 'baja' : 'bajas'} en la ronda`}>
            <Crosshair className="size-3" aria-hidden />
            {card.kills}
          </span>
        ) : null}
        <span className="text-meta tabular-nums text-success">${money.format(card.money)}</span>
      </span>
      <span className="pointer-events-none relative flex min-w-0 items-center gap-2">
        <span className="flex w-16 shrink-0 items-center gap-1.5">
          <span className="h-1 flex-1 overflow-hidden rounded-full bg-surface-0" aria-hidden>
            <span className={cn('block h-full rounded-full', hpTone(health))} style={{ width: `${health}%` }} />
          </span>
          <span className="w-6 text-right text-meta tabular-nums text-fg-1" aria-label={card.alive ? `${health} de vida` : 'Muerto'}>
            {card.alive ? (card.health ?? '·') : '0'}
          </span>
        </span>
        <Equipment card={card} />
        <span className="ml-auto flex shrink-0 items-center gap-1 text-fg-2">
          {card.alive && card.armor !== null && card.armor > 0 ? <GearGlyph kind={card.helmet === true ? 'helmet' : 'armor'} label={`${armorLabel} (${card.armor})`} /> : null}
          {card.alive && card.defuser === true ? <GearGlyph kind="kit" label="Kit de desactivación" className="text-side-ct" /> : null}
          {card.hasBomb && (card.inventory === null || !card.inventory.some((entry) => entry.item === 'c4')) ? (
            <GearGlyph kind="c4" label="Lleva la bomba" className="text-destructive" />
          ) : null}
          {card.flashed > 0 ? (
            <span className="rounded-sm bg-fg-1 px-1 text-meta leading-4 font-semibold text-surface-0" title={`Cegado ${formatDecimal(card.flashed, 1)} s`}>
              Cegado
            </span>
          ) : null}
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

function PovRuns({ runs }: { runs: readonly RoundPovRun[] }): ReactNode {
  return (
    <section aria-label="POV grabados" className="flex flex-col gap-1 border-t border-border-subtle pt-2">
      <h3 className="px-1 text-meta font-semibold text-fg-3">POV grabados</h3>
      <ul className="flex flex-col gap-px">
        {runs.map((run) => (
          <li key={run.key} className="flex flex-col gap-0.5 rounded-sm px-1 py-0.5 hover:bg-surface-3">
            <span className="flex items-center gap-1.5 text-body-sm text-fg-1">
              <span className="min-w-0 flex-1 truncate">
                {run.name} <span className="text-meta tabular-nums text-fg-3">ronda {run.round}</span>
              </span>
              <span role="status" className={cn('text-meta', run.phase === 'failed' ? 'text-destructive' : 'text-fg-2')}>
                {roundPovLabel(run)}
              </span>
            </span>
            {run.phase === 'ready' ? (
              <Link href={RESULTS_HREF} className="text-meta text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
                Ver en Mis vídeos
              </Link>
            ) : null}
            {run.error === undefined ? null : <span className="text-meta text-fg-3">{run.error}</span>}
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * Both teams at the playhead, one dense row per player. A click leaves that
 * player in the foreground, Shift and click adds another, and the first row
 * gives the whole team back. Under them, the setpos lines copied with S or a right-click.
 */
export function PlayerPanel({
  sides,
  teamNames,
  hiddenSlots,
  focus,
  onPick,
  onFocus,
  onTogglePlayer,
  onShowAll,
  setpos,
  onCopySetpos,
  pov,
}: {
  sides: { ct: readonly PlayerCard[]; t: readonly PlayerCard[] };
  teamNames: Readonly<Record<TacticalSide, string>>;
  hiddenSlots: readonly number[];
  focus: PlayerFocus;
  onPick: (slot: number, additive: boolean) => void;
  onFocus: (focus: number[]) => void;
  onTogglePlayer: (slot: number) => void;
  onShowAll: () => void;
  /** The setpos lines copied in this visit, newest first. */
  setpos: readonly SetposCopy[];
  onCopySetpos: (entry: SetposCopy) => void;
  /** Library matches only: records a player's POV of the round shown; the runs of this visit follow. */
  pov: { runs: readonly RoundPovRun[]; onRecord: (slot: number) => void } | null;
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
          <section key={side} aria-label={`${teamNames[side]}, lado ${side}`} className="flex flex-col">
            <h3 className="flex items-center gap-2 px-1 pb-1">
              <span className={cn('rounded-sm px-1 text-meta font-semibold', side === TACTICAL_SIDES.ct ? 'bg-side-ct/15 text-side-ct' : 'bg-side-t/15 text-side-t')}>{side}</span>
              <span className="min-w-0 flex-1 truncate text-body-sm font-semibold text-fg-1">{teamNames[side]}</span>
              <span className="text-meta tabular-nums text-fg-3">
                <span className="text-fg-1">{alive}</span> de {cards.length} vivos
              </span>
            </h3>
            <ul className="flex flex-col gap-px">
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
      {pov === null || pov.runs.length === 0 ? null : <PovRuns runs={pov.runs} />}
      {setpos.length === 0 ? null : (
        <section aria-label="Setpos copiados" className="flex flex-col gap-1 border-t border-border-subtle pt-2">
          <h3 className="px-1 text-meta font-semibold text-fg-3">Setpos copiados</h3>
          <ul className="flex flex-col gap-px">
            {setpos.map((entry) => (
              <li key={entry.id} className="flex items-center gap-1.5 rounded-sm px-1 py-0.5 hover:bg-surface-3">
                <span className="min-w-0 flex-1 truncate text-body-sm text-fg-1" title={entry.command}>
                  {entry.name} <span className="text-meta tabular-nums text-fg-3">{entry.at}</span>
                </span>
                <RowAction label={`Copiar otra vez el setpos de ${entry.name}`} pressed={false} onClick={() => onCopySetpos(entry)}>
                  <Copy aria-hidden />
                </RowAction>
              </li>
            ))}
          </ul>
          <p className="px-1 text-meta text-fg-3">Pégalo en la consola de CS2, con sv_cheats 1.</p>
        </section>
      )}
    </div>
  );
}
