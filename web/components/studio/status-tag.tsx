import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type StatusTagTone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger' | 'stream';
type StatusTagSize = 'sm' | 'md';

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
      className={cn(
        'inline-flex shrink-0 items-center border font-mono uppercase',
        SIZE_TYPE_CLASS[size],
        SIZE_CLASS[size],
        TONE_CLASS[tone],
        className,
      )}
    >
      {dot ? <span aria-hidden className="size-1.5 shrink-0 bg-current shadow-[0_0_6px_currentColor]" /> : null}
      {Icon ? <Icon aria-hidden className={cn('shrink-0', ICON_CLASS[size])} /> : null}
      {children}
    </span>
  );
}
