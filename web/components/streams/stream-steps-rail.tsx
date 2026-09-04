'use client';

import type { ReactNode } from 'react';
import type { StreamStep, StreamStepEntry } from '@/lib/streams/editor';
import { cn } from '@/lib/utils';

export type StreamAutosaveState = 'saving' | 'saved' | 'failed';

type StepTone = 'active' | 'done' | 'idle';

const RAIL_CLASS = {
  active: 'border-stream bg-stream/8',
  done: 'border-success',
  idle: 'border-transparent',
} as const satisfies Record<StepTone, string>;

const NUMBER_CLASS = {
  active: 'text-stream-text',
  done: 'text-success',
  idle: 'text-fg-3',
} as const satisfies Record<StepTone, string>;

const LABEL_CLASS = {
  active: 'text-fg-1',
  done: 'text-fg-2',
  idle: 'text-fg-3',
} as const satisfies Record<StepTone, string>;

const OPTIONAL_LABEL = 'opcional';

const AUTOSAVE_LABEL = {
  saving: 'Guardando borrador…',
  saved: 'Borrador guardado en este PC',
  failed: 'Borrador local · guardado pendiente',
} as const satisfies Record<StreamAutosaveState, string>;

const AUTOSAVE_TONE = {
  saving: 'text-stream-text',
  saved: 'text-fg-3',
  failed: 'text-destructive',
} as const satisfies Record<StreamAutosaveState, string>;

/** Left rail: numbered steps (magenta = active, green = done) plus the source and autosave readout. */
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
    <nav
      aria-label="Pasos"
      className="flex min-h-0 flex-col overflow-y-auto bg-surface-1 py-4 shadow-[inset_-1px_0_0_0_var(--border-subtle)]"
    >
      <p className="px-4 pb-2.5 font-mono text-meta uppercase tracking-widest text-fg-3">Pasos</p>
      {steps.map((step) => {
        const active = step.key === activeStep;
        let tone: StepTone = 'idle';
        if (active) tone = 'active';
        else if (step.done) tone = 'done';
        return (
          <button
            key={step.key}
            type="button"
            aria-current={active ? 'step' : undefined}
            onClick={() => onSelectStep(step.key)}
            className={cn(
              'flex min-h-11 items-center gap-3 border-l-[3px] px-3.5 py-3 text-left transition-colors duration-(--dur-fast) ease-standard hover:bg-stream/5 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
              RAIL_CLASS[tone],
            )}
          >
            <span
              className={cn(
                'w-[18px] shrink-0 font-mono text-meta tabular-nums',
                NUMBER_CLASS[tone],
              )}
            >
              {step.done && !active ? '✓' : step.number}
            </span>
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="flex min-w-0 items-center gap-2">
                <span
                  className={cn(
                    'truncate font-display text-label font-semibold uppercase tracking-wide',
                    LABEL_CLASS[tone],
                  )}
                >
                  {step.label}
                </span>
                {step.optional && !step.done ? (
                  <span className="shrink-0 border border-border-strong px-1 font-mono text-meta uppercase tracking-wider text-fg-3">
                    {OPTIONAL_LABEL}
                  </span>
                ) : null}
              </span>
              <span
                className="line-clamp-2 font-mono text-meta uppercase tracking-wider text-fg-3"
                title={step.detail}
              >
                {step.detail}
              </span>
            </span>
          </button>
        );
      })}

      <div className="mt-auto flex flex-col gap-2 px-4 pt-4">
        <p className="font-mono text-meta uppercase tracking-widest text-fg-3">Fuente</p>
        <h1 className="truncate font-display text-label font-semibold uppercase text-fg-1" title={sourceTitle}>
          {sourceTitle}
        </h1>
        <p className="line-clamp-2 font-mono text-meta uppercase tracking-wider text-fg-3" title={sourceMeta}>
          {sourceMeta}
        </p>
        <p
          role="status"
          aria-live="polite"
          className={cn(
            'font-mono text-meta uppercase tracking-wider transition-colors duration-(--dur-base) ease-standard',
            AUTOSAVE_TONE[autosave],
          )}
        >
          {AUTOSAVE_LABEL[autosave]}
        </p>
      </div>
    </nav>
  );
}
