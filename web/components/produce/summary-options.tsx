'use client';

import type { ReactNode } from 'react';
import { PRE_ROLL_CHOICES, type CaptureHud, type PreRollSeconds } from '@/lib/api/types';
import { SUMMARY_STYLES } from '@/lib/produce/summary-style';
import { cn } from '@/lib/utils';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';

type PreRollPickerProps = { value: PreRollSeconds; disabled: boolean; tone: FormatTone; onChange: (seconds: PreRollSeconds) => void };

/** How much play comes before each play's first kill; changing it needs no new analysis. */
export function PreRollPicker({ value, disabled, tone, onChange }: PreRollPickerProps): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      <div role="group" aria-labelledby="short-pre-roll-title" className="flex w-fit gap-1 rounded-lg border border-border-subtle bg-surface-1 p-1">
        {PRE_ROLL_CHOICES.map((seconds) => (
          <button key={seconds} type="button" disabled={disabled} aria-pressed={value === seconds} onClick={() => onChange(seconds)}
            className={cn('min-h-9 rounded-md px-4 font-display text-body-sm font-semibold tabular-nums transition-colors duration-(--dur-fast) focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50',
              value === seconds ? FORMAT_TONE[tone].fill : 'text-fg-2 hover:bg-surface-3')}>
            {seconds} s
          </button>
        ))}
      </div>
      <p className="text-body-sm text-fg-2">Segundos de juego antes de la primera kill de cada jugada. Las kills seguidas van en la misma jugada.</p>
    </div>
  );
}

type SummaryStylePickerProps = { value: CaptureHud; disabled: boolean; onChange: (hud: CaptureHud) => void };

/** The HUD the summary records: CS2's full HUD, its kill feed alone, or none. */
export function SummaryStylePicker({ value, disabled, onChange }: SummaryStylePickerProps): ReactNode {
  const chosen = SUMMARY_STYLES.find((style) => style.hud === value) ?? SUMMARY_STYLES[0];
  return (
    <div className="flex flex-col gap-2">
      <Select value={chosen.hud} onValueChange={(hud) => onChange(SUMMARY_STYLES.find((style) => style.hud === hud)?.hud ?? 'gameplay')} disabled={disabled}>
        <SelectTrigger id="short-preset" className="h-10 font-display font-semibold">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SUMMARY_STYLES.map((style) => (
            <SelectItem key={style.hud} value={style.hud}>{style.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-body-sm text-fg-2">{chosen.description}</p>
    </div>
  );
}
