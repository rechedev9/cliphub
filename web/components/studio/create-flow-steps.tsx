import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { createFlowPhases, type CreateFlow } from '@/lib/produce/create-flow';
import { cn } from '@/lib/utils';

/**
 * The shared progress rail of the Crear flows: source, settings, result. Each
 * flow passes the index of its own step; the rail names the phase and the step.
 */
export function CreateFlowSteps({ flow, step }: { flow: CreateFlow; step: number }): ReactNode {
  const phases = createFlowPhases(flow, step);
  return (
    <ol aria-label="Pasos de creación" className="grid grid-cols-3 gap-2">
      {phases.map((phase, index) => (
        <li
          key={phase.label}
          aria-current={phase.state === 'current' ? 'step' : undefined}
          className={cn(
            'flex min-w-0 flex-col gap-0.5 border-t-2 pt-2',
            phase.state === 'pending' ? 'border-border' : 'border-primary',
          )}
        >
          <span
            className={cn(
              'flex items-center gap-1.5 text-meta font-medium',
              phase.state === 'pending' ? 'text-fg-3' : 'text-fg-1',
            )}
          >
            {phase.state === 'done' ? (
              <Check aria-label="Completado" className="size-3.5 text-fg-1" />
            ) : (
              <span aria-hidden>{index + 1}</span>
            )}
            {phase.label}
          </span>
          <span className={cn('truncate text-body-sm', phase.state === 'current' ? 'text-fg-1' : 'text-fg-3')}>
            {phase.detail}
          </span>
        </li>
      ))}
    </ol>
  );
}
