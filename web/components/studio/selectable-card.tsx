import type { ReactNode } from 'react';
import { TiltSurface } from '@/components/studio/tilt-surface';
import { cn } from '@/lib/utils';

type SelectableCardTone = 'primary' | 'stream';

const TONE_SELECTED_CLASS = {
  primary: 'border-primary',
  stream: 'border-stream',
} as const satisfies Record<SelectableCardTone, string>;

const TONE_EDGE_CLASS = {
  primary: 'bg-primary',
  stream: 'bg-stream',
} as const satisfies Record<SelectableCardTone, string>;

export type SelectableCardProps = {
  selected: boolean;
  onSelect: () => void;
  children: ReactNode;
  /** Accessible name when the visible content does not read as a label. */
  label?: string;
  tone?: SelectableCardTone;
  disabled?: boolean;
  /** Off for dense text surfaces: rotating 12px metadata is a reading regression. */
  tilt?: boolean;
  className?: string;
};

/**
 * A selectable surface that is a real control: `aria-pressed`, a visible focus
 * ring, and a selected state carried by an accent edge as well as colour. The
 * picker this replaces (`components/clips/preset-cards.tsx`) had no
 * `focus-visible` rule at all, so tabbing through the reel-style choices showed
 * nothing — WCAG 2.4.7. The ring is written explicitly rather than inherited
 * from the base `*:focus-visible` rule so a later `outline-none` sweep in the
 * primitive layer cannot silently reopen the hole.
 *
 * `group/selectable` is exposed so children can react to hover without each
 * card inventing its own group name.
 */
export function SelectableCard({
  selected,
  onSelect,
  children,
  label,
  tone = 'primary',
  disabled = false,
  tilt = true,
  className,
}: SelectableCardProps): ReactNode {
  const card = (
    <button
      type="button"
      onClick={onSelect}
      disabled={disabled}
      aria-pressed={selected}
      aria-label={label}
      className={cn(
        'studio-panel studio-panel-interactive group/selectable relative flex h-full w-full flex-col items-start gap-3 overflow-hidden p-5 text-left',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
        'disabled:pointer-events-none disabled:opacity-50',
        selected ? cn('studio-panel-raised', TONE_SELECTED_CLASS[tone]) : 'hover:border-border-strong',
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'absolute inset-x-0 top-0 h-0.5 transition-opacity duration-(--dur-fast) ease-standard',
          TONE_EDGE_CLASS[tone],
          selected ? 'opacity-100' : 'opacity-0',
        )}
      />
      {children}
    </button>
  );

  if (!tilt) return card;

  return (
    <TiltSurface className="h-full" planeClassName="h-full">
      {card}
    </TiltSurface>
  );
}
