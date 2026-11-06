'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { equipmentIconForWeapon, equipmentIconUrl } from '@/lib/api/game-assets';
import type { EquipmentIconName } from '@/lib/api/game-assets';
import { equipmentItem } from '@/lib/replay/equipment';
import type { EquipmentCategory } from '@/lib/replay/equipment';
import { cn } from '@/lib/utils';

/** Probed once per session: the game's icons load, or every icon stays a drawn silhouette. */
let iconsProbe: Promise<boolean> | null = null;

function probeIcons(): Promise<boolean> {
  iconsProbe ??= fetch(equipmentIconUrl('ak47'))
    .then((res) => res.ok && (res.headers.get('content-type') ?? '').startsWith('image/'))
    .catch(() => false);
  return iconsProbe;
}

function useGameIcons(): boolean {
  const [available, setAvailable] = useState(false);
  useEffect(() => {
    let active = true;
    void probeIcons().then((ok) => {
      if (active) setAvailable(ok);
    });
    return () => {
      active = false;
    };
  }, []);
  return available;
}

/** Silhouettes per family, drawn on a 24×12 grid for guns and 12×12 for the rest. */
const SHAPES: Record<EquipmentCategory, { width: number; d: string }> = {
  rifle: { width: 24, d: 'M1 4h13l1-1h3v1h5v2h-6l-1 1h-4l-1.5 4h-2.5l1-4H6l-1 2H2l1-2H1z' },
  sniper: { width: 24, d: 'M1 5h15V4h-4V2h7v2h-2v1h6v1.5h-7L15 8h-3l-1.5 3H8l1-3H5l-1 2H2l1-2H1z' },
  smg: { width: 24, d: 'M4 4h12l1-1h2v1h3v2h-5l-1 1h-1l-1 4h-2.5l.5-4H9l-1.5 4H5l1-4H4z' },
  heavy: { width: 24, d: 'M1 4h16l1-1h4v3h-5l-2 1H9l-2 4H4.5l1.5-4H2l-1-1z' },
  pistol: { width: 12, d: 'M1 3h10v2.5H6.5L5.5 10H3l1-4.5H1z' },
  smoke: { width: 12, d: 'M3.5 1h5v1.5h1V11h-7V2.5h1z' },
  flash: { width: 12, d: 'M3 2h4l2-1.5 1 1L8.5 3H9v8H3z' },
  he: { width: 12, d: 'M5 1h2v1.2A4 4 0 1 1 5 2.2z' },
  molotov: { width: 12, d: 'M5 0.5c1 1 1.5 1.8 1 3h1V5l1.5 1.5V11h-5V6.5L5 5V3.5h1C5.6 2.6 4.8 1.6 5 .5z' },
  decoy: { width: 12, d: 'M3.5 1.5h5V11h-5zM5 3v2h2V3z' },
  c4: { width: 12, d: 'M1 3h10v7H1zM2.5 4.5v1.5h3V4.5zm4 0v1.5h3V4.5z' },
  kit: { width: 12, d: 'M4 1.5h4v2h2.5v7h-9v-7H4zm1 1v1h2v-1zM3 6v1.5h6V6z' },
  armor: { width: 12, d: 'M6 .8 10.5 2.5v3.3c0 2.6-1.9 4.5-4.5 5.4-2.6-.9-4.5-2.8-4.5-5.4V2.5z' },
  helmet: { width: 12, d: 'M6 1.5c2.9 0 5 2.1 5 5V8H9v2H3V8H1V6.5c0-2.9 2.1-5 5-5z' },
  taser: { width: 12, d: 'M1 3h10v3H7l-1 5H3.5l1-5H1z' },
  knife: { width: 12, d: 'M1 9 9 1l2 1-6 6 1 1-2 2z' },
  other: { width: 12, d: 'M2 4h8v4H2z' },
};

/** A white game icon used as a mask, so it takes the text colour in either theme. */
function MaskIcon({ icon, label, className }: { icon: EquipmentIconName; label: string; className: string }): ReactNode {
  const url = `url("${equipmentIconUrl(icon)}")`;
  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={cn('inline-block shrink-0 bg-current', className)}
      style={{ maskImage: url, WebkitMaskImage: url, maskSize: 'contain', WebkitMaskSize: 'contain', maskRepeat: 'no-repeat', WebkitMaskRepeat: 'no-repeat', maskPosition: 'center', WebkitMaskPosition: 'center' }}
    />
  );
}

function Silhouette({ category, label, className }: { category: EquipmentCategory; label: string; className: string }): ReactNode {
  const shape = SHAPES[category];
  return (
    <svg viewBox={`0 0 ${shape.width} 12`} className={cn('shrink-0', className)} role="img" aria-label={label}>
      <title>{label}</title>
      <path d={shape.d} fill="currentColor" fillRule="evenodd" />
    </svg>
  );
}

/**
 * One item of a player's equipment: the game's own icon when Studio can read
 * it from CS2, else a drawn silhouette of its family. Tinted with the text colour.
 */
export function EquipmentIcon({ name, active = false, className }: { name: string; active?: boolean; className?: string }): ReactNode {
  const item = equipmentItem(name);
  const gameIcons = useGameIcons();
  const icon = equipmentIconForWeapon(name);
  const wide = SHAPES[item.category].width > 12;
  let tone = active ? 'text-fg-1' : 'text-fg-3';
  if (item.category === 'c4') tone = 'text-destructive';
  if (gameIcons && icon !== null) return <MaskIcon icon={icon} label={item.label} className={cn(wide ? 'h-3.5 w-8' : 'size-3.5', tone, className)} />;
  return <Silhouette category={item.category} label={item.label} className={cn(wide ? 'h-3 w-6' : 'size-3', tone, className)} />;
}

const GEAR_ICONS: Record<'armor' | 'helmet' | 'kit' | 'c4', EquipmentIconName> = { armor: 'kevlar', helmet: 'assaultsuit', kit: 'defuser', c4: 'c4' };

/** The armour, kit and bomb glyphs of a row. */
export function GearGlyph({ kind, label, className }: { kind: 'armor' | 'helmet' | 'kit' | 'c4'; label: string; className?: string }): ReactNode {
  const gameIcons = useGameIcons();
  if (gameIcons) return <MaskIcon icon={GEAR_ICONS[kind]} label={label} className={cn('size-3.5', className)} />;
  return <Silhouette category={kind} label={label} className={cn('size-3', className)} />;
}
