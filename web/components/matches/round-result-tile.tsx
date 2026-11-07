import type { ReactNode } from 'react';
import { Bomb, Clock, Crosshair, Flag, Shield, type LucideIcon } from 'lucide-react';
import { endReasonLabel } from '@/lib/library/partidas';
import { cn } from '@/lib/utils';

/** The icon of each way a round ends; the round history legend reads the same map. */
export const END_ICON: Readonly<Record<string, LucideIcon>> = {
  bomb_exploded: Bomb,
  bomb_defused: Shield,
  ct_eliminated_t: Crosshair,
  t_eliminated_ct: Crosshair,
  time_expired: Clock,
  t_surrender: Flag,
  ct_surrender: Flag,
};

/** The square of a won round: filled with the side that won it, with an icon for how it ended. */
export function RoundResultTile({
  side,
  endReason,
  size = 'md',
  className,
}: {
  side: 'CT' | 'T';
  endReason: string;
  size?: 'sm' | 'md';
  className?: string;
}): ReactNode {
  const Icon = END_ICON[endReason] ?? Crosshair;
  return (
    <span
      title={endReasonLabel(endReason)}
      className={cn(
        'grid shrink-0 place-items-center rounded-sm text-primary-foreground',
        side === 'CT' ? 'bg-side-ct' : 'bg-side-t',
        size === 'md' ? 'size-7 [&_svg]:size-3.5' : 'size-[22px] [&_svg]:size-3',
        className,
      )}
    >
      <Icon aria-hidden />
    </span>
  );
}
