import type { CSSProperties, ReactNode } from 'react';
import { equipmentIconUrl, type EquipmentIconName } from '@/lib/api/game-assets';
import { iconAspect } from '@/lib/analysis/icons';
import { cn } from '@/lib/utils';

/**
 * A CS2 equipment silhouette tinted with the text colour, so it reads in both
 * themes. The icon is a mask: when CS2 is missing the URL fails and the box
 * stays empty, never a broken image.
 */
export function EquipmentIcon({
  name,
  height = 14,
  label,
  className,
}: {
  name: EquipmentIconName;
  height?: number;
  /** Accessible name; decorative when absent because a word sits beside it. */
  label?: string;
  className?: string;
}): ReactNode {
  const url = `url("${equipmentIconUrl(name)}")`;
  const style: CSSProperties = {
    width: Math.round(height * iconAspect(name)),
    height,
    maskImage: url,
    WebkitMaskImage: url,
    maskSize: 'contain',
    WebkitMaskSize: 'contain',
    maskRepeat: 'no-repeat',
    WebkitMaskRepeat: 'no-repeat',
    maskPosition: 'center',
    WebkitMaskPosition: 'center',
  };
  return (
    <span
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      className={cn('inline-block shrink-0 bg-current align-middle', className)}
      style={style}
    />
  );
}
