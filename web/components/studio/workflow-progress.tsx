import type { ReactNode } from 'react';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';

export function WorkflowProgress({
  steps,
  current,
  variant = 'compact',
}: {
  steps: readonly string[];
  current: number;
  variant?: 'compact' | 'connected';
}): ReactNode {
  const connected = variant === 'connected';

  return (
    <ol
      aria-label="Pasos de creación"
      className={connected
        ? 'grid grid-cols-2 gap-x-4 gap-y-5 @[56rem]/content:flex @[56rem]/content:items-center'
        : 'flex flex-wrap gap-x-6 gap-y-3 border-b border-border pb-4'}
    >
      {steps.map((step, index) => (
        <li
          key={step}
          aria-current={index === current ? 'step' : undefined}
          className={cn(
            'flex min-w-0 items-center text-body-sm',
            connected ? 'gap-3 @[56rem]/content:flex-1 @[56rem]/content:last:flex-none' : 'gap-2',
            index === current ? 'text-primary' : 'text-fg-3',
          )}
        >
          <span className={cn(
            'flex shrink-0 items-center justify-center border border-current font-mono',
            connected ? 'size-10 rounded-full text-body' : 'size-6 text-meta',
          )}>
            {index < current ? <Check aria-label="Completado" className="size-3.5" /> : index + 1}
          </span>
          <span className={connected ? 'font-medium @[56rem]/content:whitespace-nowrap' : undefined}>{step}</span>
          {connected && index < steps.length - 1 ? (
            <span aria-hidden className="ml-2 hidden h-px min-w-4 flex-1 bg-border @[56rem]/content:block" />
          ) : null}
        </li>
      ))}
    </ol>
  );
}
