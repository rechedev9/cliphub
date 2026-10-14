import { CircleDashed, CircleSlash, Loader2, Radar, TriangleAlert } from 'lucide-react';
import type { ReactNode } from 'react';
import { TACTICAL_STATES } from '@/lib/api/tactical';
import type { TacticalState } from '@/lib/api/tactical';
import { stateLabel } from '@/lib/tactical-labels';
import { cn } from '@/lib/utils';

/** Icon and colour per lifecycle state; state is never communicated by colour alone. */
const STATE_STYLE: Record<TacticalState, { icon: typeof Radar; className: string; spin?: boolean }> = {
  [TACTICAL_STATES.none]: {
    icon: CircleDashed,
    className: 'border-border-strong bg-surface-3 text-fg-2',
  },
  [TACTICAL_STATES.queued]: {
    icon: CircleSlash,
    className: 'border-primary/45 bg-primary/10 text-primary',
  },
  [TACTICAL_STATES.running]: {
    icon: Loader2,
    className: 'border-primary/45 bg-primary/10 text-primary',
    spin: true,
  },
  [TACTICAL_STATES.ready]: {
    icon: Radar,
    className: 'border-success/45 bg-success/10 text-success',
  },
  [TACTICAL_STATES.failed]: {
    icon: TriangleAlert,
    className: 'border-destructive/45 bg-destructive/10 text-destructive',
  },
};

export function TacticalStateBadge({
  state,
  className,
}: {
  state: TacticalState;
  className?: string;
}): ReactNode {
  const style = STATE_STYLE[state];
  const Icon = style.icon;
  return (
    <span
      className={cn(
        'inline-flex h-7 items-center gap-1.5 border px-2.5 font-mono text-meta uppercase tracking-wider',
        style.className,
        className,
      )}
    >
      <Icon className={cn('size-3.5', style.spin && 'animate-spin motion-reduce:animate-none')} aria-hidden />
      {stateLabel(state)}
    </span>
  );
}
