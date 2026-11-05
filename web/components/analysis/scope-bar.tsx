'use client';

import { useState, type ReactNode } from 'react';
import { X } from 'lucide-react';
import type { ReportSide } from '@/lib/api/scout';
import {
  PERIOD_PRESETS,
  SCOPE_BUYS,
  scopeCountText,
  scopeFieldActive,
  type AnalysisScope,
  type ScopeField,
} from '@/lib/analysis/scope';
import { prettyMapName } from '@/lib/format';
import { buyLabel } from '@/lib/tactical-labels';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every value" travels as this. */
const ALL = '__all__';
const RANGE = '__range__';

const SIDES: readonly { value: ReportSide; label: string }[] = [
  { value: '', label: 'Ambos lados' },
  { value: 'CT', label: 'CT' },
  { value: 'T', label: 'T' },
];

export type ScopeCounts = { readonly maps: number; readonly rounds: number };

export type ScopeOption = { readonly value: string; readonly label: string };

export type ScopeBarProps = {
  scope: AnalysisScope;
  onChange: (patch: Partial<AnalysisScope>) => void;
  onReset: (fields: readonly ScopeField[]) => void;
  /** The controls this view reads, in this order; the others stay in the URL untouched. */
  fields: readonly ScopeField[];
  /** Map names for the map control, such as `de_mirage`. */
  maps?: readonly string[];
  /** Rival teams for the opponent control. */
  opponents?: readonly ScopeOption[];
  /** The match limits offered, and the one the view uses while the URL names none. */
  limits?: { readonly options: readonly number[]; readonly fallback: number };
  /** Sample size of the answer on screen; null while it is being counted, absent to hide it. */
  counts?: ScopeCounts | null;
  /** Fixed context the person cannot change here, such as a report's rival and side. */
  context?: ReactNode;
  className?: string;
};

function OptionSelect({
  label,
  allLabel,
  value,
  options,
  onChange,
}: {
  label: string;
  allLabel: string;
  value: string;
  options: readonly ScopeOption[];
  onChange: (value: string) => void;
}): ReactNode {
  return (
    <Select value={value === '' ? ALL : value} onValueChange={(next) => onChange(next === ALL ? '' : next)}>
      <SelectTrigger aria-label={label} className="h-9 min-w-36">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function PeriodControl({ scope, onChange }: { scope: AnalysisScope; onChange: ScopeBarProps['onChange'] }): ReactNode {
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
        <SelectTrigger aria-label="Periodo" className="h-9 min-w-40">
          <SelectValue />
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
        <span className="flex items-center gap-1.5 text-body-sm text-fg-2">
          <Input
            type="date"
            aria-label="Desde"
            value={from}
            max={to || undefined}
            onChange={(event) => onChange({ period: { kind: 'range', from: event.target.value, to } })}
            className="h-9 w-40"
          />
          a
          <Input
            type="date"
            aria-label="Hasta"
            value={to}
            min={from || undefined}
            onChange={(event) => onChange({ period: { kind: 'range', from, to: event.target.value } })}
            className="h-9 w-40"
          />
        </span>
      ) : null}
    </>
  );
}

function buyOptions(): ScopeOption[] {
  return SCOPE_BUYS.map((buy) => ({ value: buy, label: buyLabel(buy) }));
}

function FieldControl({ field, props }: { field: ScopeField; props: ScopeBarProps }): ReactNode {
  const { scope, onChange } = props;
  switch (field) {
    case 'map':
      return (
        <OptionSelect
          label="Mapa"
          allLabel="Todos los mapas"
          value={scope.map}
          options={(props.maps ?? []).map((map) => ({ value: map, label: prettyMapName(map) }))}
          onChange={(map) => onChange({ map })}
        />
      );
    case 'side':
      return <Segmented label="Lado" value={scope.side} options={SIDES} onChange={(side) => onChange({ side })} />;
    case 'period':
      return <PeriodControl scope={scope} onChange={onChange} />;
    case 'limit': {
      const limits = props.limits ?? { options: [10, 20, 50], fallback: 20 };
      return (
        <Segmented
          label="Partidas leídas"
          value={String(scope.limit ?? limits.fallback)}
          options={limits.options.map((limit) => ({ value: String(limit), label: `Últimas ${limit}` }))}
          onChange={(limit) => onChange({ limit: Number(limit) === limits.fallback ? null : Number(limit) })}
        />
      );
    }
    case 'buy':
      return (
        <OptionSelect
          label="Compra"
          allLabel="Todas las compras"
          value={scope.buy}
          options={buyOptions()}
          onChange={(buy) => onChange({ buy: SCOPE_BUYS.find((entry) => entry === buy) ?? '' })}
        />
      );
    case 'opponentBuy':
      return (
        <OptionSelect
          label="Compra del rival"
          allLabel="Cualquier compra rival"
          value={scope.opponentBuy}
          options={buyOptions()}
          onChange={(buy) => onChange({ opponentBuy: SCOPE_BUYS.find((entry) => entry === buy) ?? '' })}
        />
      );
    case 'opponent':
      return (
        <OptionSelect
          label="Rival"
          allLabel="Todos los rivales"
          value={scope.opponent}
          options={props.opponents ?? []}
          onChange={(opponent) => onChange({ opponent })}
        />
      );
    default:
      return null;
  }
}

/**
 * The fixed context of an analysis page: the scope controls the current view
 * reads and, at the right, how many maps and rounds the answer stands on. It
 * sticks under the command strip so the context stays in sight while scrolling.
 */
export function ScopeBar(props: ScopeBarProps): ReactNode {
  const { scope, fields, counts, context, onReset, className } = props;
  const active = fields.filter((field) => scopeFieldActive(scope, field));
  let counter: ReactNode = null;
  if (counts === null) counter = <span className="text-fg-3">Contando</span>;
  else if (counts !== undefined) counter = scopeCountText(counts.maps, counts.rounds);

  return (
    <div
      role="group"
      aria-label="Ámbito del análisis"
      className={cn(
        'sticky top-(--shell-strip-height) z-20 -mx-1 flex flex-wrap items-center gap-2 border-b border-border bg-background px-1 py-2',
        className,
      )}
    >
      {context ? <div className="flex items-center gap-2 text-body-sm text-fg-2">{context}</div> : null}
      {fields.map((field) => (
        <FieldControl key={field} field={field} props={props} />
      ))}
      {active.length > 0 ? (
        <Button type="button" variant="ghost" size="xs" onClick={() => onReset(active)}>
          <X aria-hidden />
          Quitar filtros
        </Button>
      ) : null}
      {counter === null ? null : (
        <p aria-live="polite" className="ml-auto text-body-sm whitespace-nowrap text-fg-2 tabular-nums">
          {counter}
        </p>
      )}
    </div>
  );
}
