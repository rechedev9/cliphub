import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type StatusTagTone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger' | 'stream';
type StatusTagSize = 'sm' | 'md';

/**
 * One alpha per role instead of one per call site. The audit found six tags with
 * two heights, two paddings, three trackings and two background strategies; here
 * the tinted fill is always 10% and the edge always 45%, so six tones read as the
 * same object in six colours. `neutral` uses the surface ramp rather than a tint
 * because it has no hue to tint with.
 */
const TONE_CLASS = {
  neutral: 'border-border-strong bg-surface-3 text-fg-2',
  primary: 'border-primary/45 bg-primary/10 text-primary',
  success: 'border-success/45 bg-success/10 text-success',
  warning: 'border-warning/45 bg-warning/10 text-warning',
  danger: 'border-destructive/45 bg-destructive/10 text-destructive',
  stream: 'border-stream/45 bg-stream/10 text-stream-text',
} as const satisfies Record<StatusTagTone, string>;

const SIZE_CLASS = {
  sm: 'min-h-7 gap-1.5 px-2',
  md: 'min-h-8 gap-2 px-2.5',
} as const satisfies Record<StatusTagSize, string>;

/*
 * Kept OUT of the cn() merge on purpose. tailwind-merge classifies every custom
 * `--text-*` theme key as a text COLOUR, so `cn('text-meta', 'text-fg-2')`
 * resolves to `text-fg-2` alone and the tag silently renders at inherited body
 * size. Concatenating the step is correct both before and after `cn` teaches
 * tailwind-merge the v4 font-size group.
 */
const SIZE_TYPE_CLASS = {
  sm: 'text-meta',
  md: 'text-label',
} as const satisfies Record<StatusTagSize, string>;

const ICON_CLASS = {
  sm: 'size-3.5',
  md: 'size-4',
} as const satisfies Record<StatusTagSize, string>;

export type StatusTagProps = {
  children: ReactNode;
  tone?: StatusTagTone;
  size?: StatusTagSize;
  /** Square status LED before the label. Decorative: the label carries the state. */
  dot?: boolean;
  icon?: LucideIcon;
  className?: string;
};

/**
 * Square HUD status tag — queued / recording / ready / failed / expiry and the
 * rest of the pipeline vocabulary. Geometry rule for the kit: panels are
 * rounded, HUD chrome is square, so a tag never picks up a radius.
 */
export function StatusTag({
  children,
  tone = 'neutral',
  size = 'sm',
  dot = false,
  icon: Icon,
  className,
}: StatusTagProps): ReactNode {
  return (
    <span
      className={`${SIZE_TYPE_CLASS[size]} ${cn(
        'inline-flex shrink-0 items-center border font-mono uppercase',
        SIZE_CLASS[size],
        TONE_CLASS[tone],
        className,
      )}`}
    >
      {dot ? <span aria-hidden className="size-1.5 shrink-0 bg-current shadow-[0_0_6px_currentColor]" /> : null}
      {Icon ? <Icon aria-hidden className={cn('shrink-0', ICON_CLASS[size])} /> : null}
      {children}
    </span>
  );
}
