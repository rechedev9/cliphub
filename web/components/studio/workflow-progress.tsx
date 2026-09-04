import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';

export function WorkflowProgress({ steps, current }: { steps: readonly string[]; current: number }): ReactNode {
  return (
    <ol aria-label="Pasos de creación" className="flex flex-wrap gap-x-6 gap-y-3 border-b border-border pb-4">
      {steps.map((step, index) => (
        <li key={step} aria-current={index === current ? 'step' : undefined}
          className={cn('flex items-center gap-2 text-body-sm', index === current ? 'text-primary' : 'text-fg-3')}>
          <span className="flex size-6 items-center justify-center border border-current font-mono text-meta">
            {index < current ? <Check aria-label="Completado" className="size-3.5" /> : index + 1}
          </span>
          {step}
        </li>
      ))}
    </ol>
  );
}
