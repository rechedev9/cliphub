'use client';
import type { ReactNode } from 'react';
import { STREAM_WORKFLOW_STEPS, type StreamStep, type StreamStepEntry } from '@/lib/streams/editor';
import { FORMAT_TONE } from '@/components/studio/format-tone';
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
      {/* Wrap instead of scrolling sideways: a clipped "3 R…" at 390px gives
          no hint that a third step exists. */}
      <div className="flex flex-wrap gap-2 pb-1">
        {/* The video is already imported here, so step 1 is done and not a destination. */}
        <div className="flex min-h-11 flex-1 basis-[10rem] items-center justify-center gap-2 whitespace-nowrap rounded-md border border-border-subtle px-3 py-2 text-label text-fg-2">
          <span className="grid size-5.5 shrink-0 place-items-center rounded-full bg-success/15 text-meta font-semibold text-success">
            ✓
          </span>
          {STREAM_WORKFLOW_STEPS[0]}
          <span className="sr-only"> (hecho)</span>
        </div>
        {steps.map((step) => (
          <button
            key={step.key}
            type="button"
            aria-current={step.key === activeStep ? 'step' : undefined}
            disabled={step.disabled}
            title={step.disabled ? step.detail : undefined}
            onClick={() => onSelectStep(step.key)}
            className={cn(
              'flex min-h-11 flex-1 basis-[10rem] items-center justify-center gap-2 whitespace-nowrap rounded-md border px-3 py-2 text-label focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-default disabled:opacity-60',
              step.key === activeStep
                ? cn('border-format-stream font-semibold text-fg-1', FORMAT_TONE.stream.tint)
                : 'border-border-subtle text-fg-2 enabled:hover:bg-surface-3',
            )}
          >
            <span
              className={cn(
                'grid size-5.5 shrink-0 place-items-center rounded-full text-meta font-semibold tabular-nums',
                step.key === activeStep && cn(FORMAT_TONE.stream.bg, 'text-format-stream-foreground'),
                step.key !== activeStep && (step.done ? 'bg-success/15 text-success' : 'bg-surface-3 text-fg-2'),
              )}
            >
              {step.done && step.key !== activeStep ? '✓' : step.number}
            </span>
            {step.label}
          </button>
        ))}
      </div>
    </nav>
  );
}
