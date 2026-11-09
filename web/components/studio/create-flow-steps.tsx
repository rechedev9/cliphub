import type { ReactNode } from 'react';
import { createFlowSteps, type CreateFlow } from '@/lib/produce/create-flow';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { cn } from '@/lib/utils';

/** The progress track of a Crear flow in the flow's colour: one segment per step, its name under it. */
export function CreateFlowSteps({ flow, step, className }: { flow: CreateFlow; step: number; className?: string }): ReactNode {
  const steps = createFlowSteps(flow, step);
  const tone = FORMAT_TONE[flow];
  return (
    <ol
      aria-label="Pasos"
      className={cn('grid max-w-120 gap-x-1.5 gap-y-2', className)}
      style={{ gridTemplateColumns: `repeat(${steps.length}, minmax(0, 1fr))` }}
    >
      {steps.map((entry, index) => (
        <li key={entry.label} aria-current={entry.state === 'current' ? 'step' : undefined} className="flex min-w-0 flex-col gap-2">
          <span
            aria-hidden
            className={cn('h-1 rounded-full', entry.state === 'pending' ? 'bg-surface-4' : tone.bg, entry.state === 'done' && 'opacity-50')}
          />
          <span className={cn('truncate font-mono text-meta uppercase tracking-wide', entry.state === 'current' ? 'font-semibold text-fg-1' : 'text-fg-2')}>
            {index + 1} · {entry.label}
            {entry.state === 'done' ? <span className="sr-only"> (hecho)</span> : null}
          </span>
        </li>
      ))}
    </ol>
  );
}
