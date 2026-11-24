import type { ReactNode } from 'react';
import { OUTCOME_LABEL, type Outcome } from '@/lib/library/partidas';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

const VARIANT = { win: 'success', loss: 'destructive', draw: 'secondary' } as const;

const LETTER: Record<Outcome, string> = { win: 'V', loss: 'D', draw: 'E' };

export function ResultBadge({ outcome, compact = false, className }: { outcome: Outcome; compact?: boolean; className?: string }): ReactNode {
  const label = OUTCOME_LABEL[outcome];
  return (
    <Badge
      variant={VARIANT[outcome]}
      shape="square"
      title={compact ? label : undefined}
      className={cn('font-bold', compact && 'w-6 justify-center px-0', className)}
    >
      {compact ? (
        <>
          <span aria-hidden>{LETTER[outcome]}</span>
          <span className="sr-only">{label}</span>
        </>
      ) : (
        label
      )}
    </Badge>
  );
}
