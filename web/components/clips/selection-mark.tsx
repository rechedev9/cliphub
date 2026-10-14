import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';

export type SelectionMarkProps = {
  selected: boolean;
  className?: string;
};

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
