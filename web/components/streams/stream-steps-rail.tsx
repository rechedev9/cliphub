'use client';
import type { ReactNode } from 'react';
import type { StreamStep, StreamStepEntry } from '@/lib/streams/editor';
import { cn } from '@/lib/utils';
export type StreamAutosaveState = 'saving' | 'saved' | 'failed';
const AUTOSAVE_LABEL = {
  saving: 'Guardando borrador…',
  saved: 'Borrador guardado en este PC',
  failed: 'Borrador local · guardado pendiente',
};
export function StreamStepsRail({
  steps,
  activeStep,
  sourceTitle,
  sourceMeta,
  autosave,
  onSelectStep,
}: {
  steps: StreamStepEntry[];
  activeStep: StreamStep;
  sourceTitle: string;
  sourceMeta: string;
  autosave: StreamAutosaveState;
  onSelectStep: (step: StreamStep) => void;
}): ReactNode {
  return (
    <nav aria-label="Pasos" className="shrink-0 border-b border-border-subtle bg-surface-1 px-4 py-3">
      <div className="mb-3 flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-1">
        <div className="min-w-0">
          <h1 className="truncate font-display text-body font-semibold" title={sourceTitle}>
            {sourceTitle}
          </h1>
          <p className="text-label text-fg-3">{sourceMeta}</p>
        </div>
        <p role="status" className={cn('text-label', autosave === 'failed' ? 'text-destructive' : 'text-fg-3')}>
          {AUTOSAVE_LABEL[autosave]}
        </p>
      </div>
      <div className="flex gap-2 overflow-x-auto pb-1">
        {steps.map((step) => (
          <button
            key={step.key}
            type="button"
            aria-current={step.key === activeStep ? 'step' : undefined}
            onClick={() => onSelectStep(step.key)}
            className={cn(
              'flex min-h-11 flex-1 shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-md border px-3 py-2 text-label focus-visible:outline-2 focus-visible:outline-ring',
              step.key === activeStep
                ? 'border-stream bg-stream/10 text-fg-1'
                : 'border-border-subtle text-fg-2 hover:bg-surface-3',
            )}
          >
            <span className={step.done ? 'text-success' : 'text-fg-3'}>
              {step.done && step.key !== activeStep ? '✓' : step.number}
            </span>
            {step.label}
          </button>
        ))}
      </div>
    </nav>
  );
}
