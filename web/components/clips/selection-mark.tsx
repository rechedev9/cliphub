import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';

export type SelectionMarkProps = {
  selected: boolean;
  className?: string;
};

/**
 * The one selection glyph on the highlight screen.
 *
 * The screen used to speak three checkbox languages at once: a 20px square in
 * the play row, a differently-bordered 20px square in the preset card, and a
 * native 13px checkbox in the approval gate. All three are this 24px square
 * plate now — square because HUD chrome is square, `--border-strong` because a
 * control boundary has to clear 3:1, and filled cyan when on so selection reads
 * without relying on the border alone.
 *
 * Decorative: it mirrors its host control's `aria-pressed`/`checked` state and
 * is never the accessible name.
 */
export function SelectionMark({ selected, className }: SelectionMarkProps): ReactNode {
  return (
    <span
      aria-hidden
      className={cn(
        'grid size-6 shrink-0 place-items-center border transition-colors duration-(--dur-fast) ease-standard',
        selected
          ? 'border-primary bg-primary text-primary-foreground shadow-[var(--glow-primary-sm)]'
          : 'border-border-strong bg-surface-0 text-transparent',
        className,
      )}
    >
      <Check className="size-4" strokeWidth={3} />
    </span>
  );
}
