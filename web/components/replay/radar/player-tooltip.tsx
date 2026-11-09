'use client';

import type { ReactNode, Ref } from 'react';
import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '@/lib/api/tactical';
import type { TacticalSample } from '@/lib/api/tactical';
import { equipmentItem, rowInventory } from '@/lib/replay/equipment';
import { cn } from '@/lib/utils';
import { EquipmentIcon } from '../equipment-icon';
import { formatDecimal } from '@/lib/format';

const money = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 0 });
const WIDTH = 208;

/** Places the card beside a player at (x, y) inside the map box; the map calls it every frame. */
export function placeTooltip(element: HTMLElement, x: number, y: number, box: { width: number; height: number }): void {
  const left = Math.min(box.width - WIDTH - 8, Math.max(8, x + 16));
  const below = y < 140;
  element.style.transform = `translate(${left}px, ${below ? y + 16 : y - 16}px)${below ? '' : ' translateY(-100%)'}`;
}

/**
 * The card a hovered player shows on the map: name, health, armour, the gun in
 * hand, the rest of the gear and how to copy their setpos. The map moves it with `placeTooltip`.
 */
export function PlayerTooltip({ sample, name, ref }: { sample: TacticalSample; name: string; ref: Ref<HTMLDivElement> }): ReactNode {
  const ct = !hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT);
  const active = sample.activeWeapon === undefined || sample.activeWeapon === null ? null : equipmentItem(sample.activeWeapon);
  const rest = rowInventory(sample.inventory ?? []).filter((item) => active === null || item.id !== active.id);
  const facts = [
    `${Math.max(0, sample.health)} de vida`,
    sample.armor === undefined || sample.armor <= 0 ? null : `${sample.helmet === true ? 'Chaleco y casco' : 'Chaleco'} ${sample.armor}`,
    sample.defuser === true ? 'Kit' : null,
  ].filter((fact) => fact !== null);
  return (
    <div
      role="tooltip"
      ref={ref}
      className="pointer-events-none absolute top-0 left-0 z-10 flex flex-col gap-1.5 rounded-md border border-border-strong bg-surface-2 p-2.5 shadow-[var(--elev-4)] will-change-transform"
      style={{ width: WIDTH }}
    >
      <p className="flex items-center gap-1.5">
        <span className={cn('size-2 shrink-0 rounded-full', ct ? 'bg-side-ct' : 'bg-side-t')} aria-hidden />
        <span className="min-w-0 flex-1 truncate text-body-sm font-semibold text-fg-1">{name}</span>
        {sample.money === undefined ? null : <span className="text-meta tabular-nums text-success">${money.format(sample.money)}</span>}
      </p>
      <p className="text-meta text-fg-2">{facts.join(' · ')}</p>
      {active === null ? null : (
        <p className="flex items-center gap-2 text-body-sm text-fg-1">
          <EquipmentIcon name={active.id} active />
          {active.label}
        </p>
      )}
      {rest.length === 0 ? null : (
        <p className="flex flex-wrap items-center gap-1.5">
          {rest.map((item, index) => (
            <EquipmentIcon key={`${item.id}-${index}`} name={item.id} />
          ))}
        </p>
      )}
      <p className="text-meta text-fg-3">Pulsa S o clic derecho para copiar su setpos</p>
      {(sample.flashed ?? 0) > 0 ? <p className="text-meta font-medium text-fg-1">Cegado {formatDecimal(sample.flashed ?? 0, 1)} s</p> : null}
    </div>
  );
}
