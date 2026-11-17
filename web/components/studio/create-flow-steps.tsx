import Link from 'next/link';
import type { ReactNode } from 'react';
import { createFlowSteps, type CreateFlow, type CreateStep } from '@/lib/produce/create-flow';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { cn } from '@/lib/utils';

/** Where a finished step takes the user back to: a route, or an in-page action. */
export type CreateStepTarget = { readonly href: string } | { readonly onSelect: () => void; readonly busy?: boolean };

export type CreateFlowStepsProps = {
  flow: CreateFlow;
  step: number;
  /** Indexed by step; only a finished step with a target is clickable. */
  targets?: readonly (CreateStepTarget | null)[];
  className?: string;
};

const STEP_LINK_CLASS =
  'flex min-w-0 flex-col gap-2 rounded-sm text-left hover:[&>span:last-child]:text-fg-1 hover:[&>span:last-child]:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-progress';

/** The progress track of a Crear flow in the flow's colour: one segment per step, its name under it. */
export function CreateFlowSteps({ flow, step, targets, className }: CreateFlowStepsProps): ReactNode {
  const steps = createFlowSteps(flow, step);
  return (
    <ol
      aria-label="Pasos"
      className={cn('grid w-120 max-w-full gap-x-1.5 gap-y-2', className)}
      style={{ gridTemplateColumns: `repeat(${steps.length}, minmax(0, 1fr))` }}
    >
      {steps.map((entry, index) => (
        <li key={entry.label} aria-current={entry.state === 'current' ? 'step' : undefined} className="flex min-w-0 flex-col">
          <StepBody flow={flow} entry={entry} index={index} target={entry.state === 'done' ? (targets?.[index] ?? null) : null} />
        </li>
      ))}
    </ol>
  );
}

function StepBody({ flow, entry, index, target }: { flow: CreateFlow; entry: CreateStep; index: number; target: CreateStepTarget | null }): ReactNode {
  const content = (
    <>
      <span
        aria-hidden
        className={cn('h-1 rounded-full', entry.state === 'pending' ? 'bg-surface-4' : FORMAT_TONE[flow].bg, entry.state === 'done' && 'opacity-50')}
      />
      <span className={cn('truncate font-mono text-meta uppercase tracking-wide', entry.state === 'current' ? 'font-semibold text-fg-1' : 'text-fg-2')}>
        {index + 1} · {entry.label}
        {entry.state === 'done' ? <span className="sr-only"> (hecho)</span> : null}
      </span>
    </>
  );
  const title = `Volver al paso ${index + 1}: ${entry.label}`;
  if (target === null) return <div className="flex min-w-0 flex-col gap-2">{content}</div>;
  if ('href' in target) {
    return <Link href={target.href} title={title} className={STEP_LINK_CLASS}>{content}</Link>;
  }
  return (
    <button type="button" title={title} disabled={target.busy === true} aria-busy={target.busy === true} onClick={target.onSelect} className={STEP_LINK_CLASS}>
      {content}
    </button>
  );
}
