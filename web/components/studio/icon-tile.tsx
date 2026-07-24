import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type IconTileSize = 'sm' | 'md' | 'lg';
export type IconTileTone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger' | 'stream';
export type IconTileDepth = 'raised' | 'inset';

const SIZE_CLASS = {
  sm: 'size-9',
  md: 'size-11',
  lg: 'size-14',
} as const satisfies Record<IconTileSize, string>;

const ICON_CLASS = {
  sm: 'size-4',
  md: 'size-5',
  lg: 'size-6',
} as const satisfies Record<IconTileSize, string>;

/** The tone colours the edge and the glyph only; depth owns the fill. */
const TONE_CLASS = {
  neutral: 'border-border-strong text-fg-2',
  primary: 'border-primary/45 text-primary',
  success: 'border-success/45 text-success',
  warning: 'border-warning/45 text-warning',
  danger: 'border-destructive/45 text-destructive',
  stream: 'border-stream/45 text-stream-text',
} as const satisfies Record<IconTileTone, string>;

/**
 * Depth comes from the surface ramp, not from a shadow. `shadow-inner` on a
 * near-black tile produced no pixels at all (audit B3); a step *down* the ramp
 * to the well surface reads as recessed on its own, and survives the efficiency
 * profile because it is a colour, not a blur.
 */
const DEPTH_CLASS = {
  raised: 'bg-surface-3 shadow-sm',
  inset: 'bg-surface-0',
} as const satisfies Record<IconTileDepth, string>;

export type IconTileProps = {
  icon: LucideIcon;
  size?: IconTileSize;
  tone?: IconTileTone;
  depth?: IconTileDepth;
  /** Accessible name. Without it the tile is decoration and is hidden from AT. */
  label?: string;
  className?: string;
};

/** Square glyph plate: the shared shape behind every "this is what this is" icon. */
export function IconTile({
  icon: Icon,
  size = 'md',
  tone = 'primary',
  depth = 'raised',
  label,
  className,
}: IconTileProps): ReactNode {
  const described = label !== undefined;

  return (
    <span
      role={described ? 'img' : undefined}
      aria-label={label}
      aria-hidden={described ? undefined : true}
      className={cn(
        'grid shrink-0 place-items-center border',
        SIZE_CLASS[size],
        TONE_CLASS[tone],
        DEPTH_CLASS[depth],
        className,
      )}
    >
      <Icon aria-hidden className={ICON_CLASS[size]} />
    </span>
  );
}
