'use client';

import { useState, type ReactNode } from 'react';
import { PERIOD_PRESETS, type AnalysisScope } from '@/lib/analysis/scope';
import { ALL, FIELD, FIELD_SET } from '@/components/analysis/scope-field-style';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

const RANGE = '__range__';

export type PeriodFieldProps = {
  scope: AnalysisScope;
  onChange: (patch: Partial<AnalysisScope>) => void;
  /** Drawn at the left inside the trigger, such as a calendar. */
  icon?: ReactNode;
  className?: string;
};

/** The period of the scope: a preset window, every date or two exact days. */
export function PeriodField({ scope, onChange, icon, className }: PeriodFieldProps): ReactNode {
  // Exact days with both ends empty are "every date" in the URL, so the open date fields live here.
  const [picking, setPicking] = useState(false);
  const { period } = scope;
  let value: string = ALL;
  if (period.kind === 'preset') value = period.preset;
  else if (period.kind === 'range' || picking) value = RANGE;
  const from = period.kind === 'range' ? period.from : '';
  const to = period.kind === 'range' ? period.to : '';

  function choose(next: string): void {
    setPicking(next === RANGE);
    const preset = PERIOD_PRESETS.find((entry) => entry.value === next);
    if (preset !== undefined) onChange({ period: { kind: 'preset', preset: preset.value } });
    else if (next === ALL) onChange({ period: { kind: 'all' } });
  }

  return (
    <>
      <Select value={value} onValueChange={choose}>
        <SelectTrigger aria-label="Periodo" className={cn(FIELD, value !== ALL && FIELD_SET, className)}>
          <span className="flex min-w-0 items-center gap-1.5">
            {icon}
            <SelectValue />
          </span>
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>Todas las fechas</SelectItem>
          {PERIOD_PRESETS.map((preset) => (
            <SelectItem key={preset.value} value={preset.value}>
              {preset.label}
            </SelectItem>
          ))}
          <SelectItem value={RANGE}>Entre dos fechas</SelectItem>
        </SelectContent>
      </Select>
      {value === RANGE ? (
        <span className="flex items-center gap-1 text-body-sm text-fg-2">
          <Input
            type="date"
            aria-label="Desde"
            value={from}
            max={to || undefined}
            onChange={(event) => onChange({ period: { kind: 'range', from: event.target.value, to } })}
            className="h-7 w-34 px-2"
          />
          a
          <Input
            type="date"
            aria-label="Hasta"
            value={to}
            min={from || undefined}
            onChange={(event) => onChange({ period: { kind: 'range', from, to: event.target.value } })}
            className="h-7 w-34 px-2"
          />
        </span>
      ) : null}
    </>
  );
}
