'use client';

import type { ReactNode } from 'react';
import { Cloud, Link2, Monitor } from 'lucide-react';
import {
  CAPTURE_TARGET,
  CAPTURE_TARGET_LABEL,
  type CaptureTarget,
  type CaptureTargetTone,
  type CaptureTargetView,
} from '@/lib/cloud/capture-target';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

const OPTIONS = [
  { value: CAPTURE_TARGET.local, icon: Monitor },
  { value: CAPTURE_TARGET.cloud, icon: Cloud },
] as const;

/** Same selected and idle recipe as the format bar, so both choices read as one family. */
const ITEM_CLASS = {
  selected: 'border-primary bg-primary/15 text-primary hover:bg-primary/20 hover:text-primary',
  idle: 'border-border-subtle bg-transparent font-medium text-fg-3 hover:border-border-strong hover:bg-surface-3 hover:text-fg-1',
} as const;

const TONE_CLASS = {
  neutral: 'text-fg-2',
  ok: 'text-fg-2',
  warning: 'text-warning',
  danger: 'text-destructive',
} as const satisfies Record<CaptureTargetTone, string>;

export type CaptureTargetControlProps = {
  view: CaptureTargetView;
  onPick: (target: CaptureTarget) => void;
  onLink: () => void;
  disabled?: boolean;
};

/** "Dónde grabar": this PC or the ClipHub cloud, with one status line for the selected option. */
export function CaptureTargetControl({ view, onPick, onLink, disabled = false }: CaptureTargetControlProps): ReactNode {
  return (
    <div data-slot="capture-target" className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
      <span id="capture-target-label" className="font-mono text-meta uppercase tracking-widest text-fg-3">
        Dónde grabar
      </span>
      <div role="group" aria-labelledby="capture-target-label" className="grid shrink-0 grid-cols-2 gap-2">
        {OPTIONS.map(({ value, icon: Icon }) => {
          const selected = view.target === value;
          const unavailable = value === CAPTURE_TARGET.cloud && !view.cloudSelectable;
          return (
            <Button
              key={value}
              type="button"
              size="xs"
              variant="ghost"
              aria-pressed={selected}
              disabled={disabled || unavailable}
              title={unavailable ? 'Esta instalación no puede grabar en la nube' : undefined}
              className={cn('border', selected ? ITEM_CLASS.selected : ITEM_CLASS.idle)}
              onClick={() => onPick(value)}
            >
              <Icon aria-hidden />
              {CAPTURE_TARGET_LABEL[value]}
            </Button>
          );
        })}
      </div>
      <p role="status" className={cn('min-w-0 flex-1 basis-48 text-body-sm', TONE_CLASS[view.status.tone])}>
        {view.status.text}
      </p>
      {view.offerLink ? (
        <Button type="button" size="xs" variant="outline-primary" disabled={disabled} onClick={onLink}>
          <Link2 aria-hidden />
          Conectar cuenta
        </Button>
      ) : null}
    </div>
  );
}
