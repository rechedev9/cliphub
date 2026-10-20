'use client';

import type { ReactNode } from 'react';
import { Check, Cloud, Monitor } from 'lucide-react';
import {
  CAPTURE_TARGET,
  CAPTURE_TARGET_HINT,
  CAPTURE_TARGET_LABEL,
  type CaptureTarget,
  type CaptureTargetTone,
  type CaptureTargetView,
} from '@/lib/cloud/capture-target';
import { cn } from '@/lib/utils';
import { FOCUS_RING } from '@/components/ui/button';

const OPTIONS = [
  { value: CAPTURE_TARGET.local, icon: Monitor },
  { value: CAPTURE_TARGET.cloud, icon: Cloud },
] as const;

const OPTION_CLASS = {
  selected: 'border-primary bg-primary/10',
  idle: 'border-border-subtle bg-transparent hover:border-border-strong hover:bg-surface-3',
} as const;

const TONE_CLASS = {
  neutral: 'text-fg-2',
  ok: 'text-fg-2',
  warning: 'text-warning',
  danger: 'text-destructive',
} as const satisfies Record<CaptureTargetTone, string>;

const LABEL_CLASS = 'font-mono text-meta uppercase tracking-widest text-fg-3';

export type CaptureTargetControlProps = {
  view: CaptureTargetView;
  onPick: (target: CaptureTarget) => void;
  disabled?: boolean;
};

/** "Dónde se graba": this PC or the ClipHub cloud, each with what it means, and one status line for the selected one. */
export function CaptureTargetControl({ view, onPick, disabled = false }: CaptureTargetControlProps): ReactNode {
  return (
    <div data-slot="capture-target" className="flex min-w-0 flex-col gap-2">
      <span id="capture-target-label" className={LABEL_CLASS}>
        Dónde se graba
      </span>
      <div role="group" aria-labelledby="capture-target-label" className="grid gap-2 @[40rem]/content:grid-cols-2">
        {OPTIONS.map(({ value, icon: Icon }) => {
          const selected = view.target === value;
          const unavailable = value === CAPTURE_TARGET.cloud && !view.cloudSelectable;
          return (
            <button
              key={value}
              type="button"
              aria-pressed={selected}
              disabled={disabled || unavailable}
              title={unavailable ? 'Esta instalación no puede grabar en la nube' : undefined}
              className={cn(
                'flex min-w-0 items-start gap-3 rounded-md border px-3 py-2 text-left transition-colors duration-(--dur-fast) disabled:cursor-not-allowed disabled:opacity-50',
                FOCUS_RING,
                selected ? OPTION_CLASS.selected : OPTION_CLASS.idle,
              )}
              onClick={() => onPick(value)}
            >
              <Icon aria-hidden className={cn('mt-0.5 size-5 shrink-0', selected ? 'text-primary' : 'text-fg-3')} />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className={cn('text-body-sm font-semibold', selected ? 'text-primary' : 'text-fg-1')}>
                  {CAPTURE_TARGET_LABEL[value]}
                </span>
                <span className="text-body-sm text-fg-2">{CAPTURE_TARGET_HINT[value]}</span>
              </span>
              {selected ? <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-primary" /> : null}
            </button>
          );
        })}
      </div>
      <p role="status" className={cn('min-w-0 text-body-sm', TONE_CLASS[view.status.tone])}>
        <span className="text-fg-3">{CAPTURE_TARGET_LABEL[view.target]}: </span>
        {view.status.text}
      </p>
    </div>
  );
}
