'use client';

import { useState, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { SlidersHorizontal, X } from 'lucide-react';
import type { ReportSide } from '@/lib/api/scout';
import type { TacticalBuyType } from '@/lib/api/tactical';
import { BASELINE_SCOPES } from '@/lib/api/baseline';
import { switchEntityHref, resultRecord, type ResultDot } from '@/lib/analysis/context-bar';
import { buyIcon } from '@/lib/analysis/icons';
import { SCOPE_LABEL } from '@/lib/analysis/reference';
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
import { MapThumb } from '@/components/analysis/map-thumb';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every value" travels as this. */
const ALL = '__all__';
const RANGE = '__range__';
const TRIGGER = 'h-8 gap-1.5 px-2.5 text-body-sm';
/** Filters most views rarely change; they open under a "Filtros" button unless one is set. */
const SECONDARY: ReadonlySet<ScopeField> = new Set(['buy', 'opponentBuy', 'opponent']);
const MAX_DOTS = 10;
const MAX_INLINE = 4;

const SIDES: readonly { value: ReportSide; label: string }[] = [
  { value: '', label: 'Ambos' },
  { value: 'CT', label: 'CT' },
  { value: 'T', label: 'T' },
];

export type ScopeCounts = { readonly maps: number; readonly rounds: number };

export type ScopeOption = { readonly value: string; readonly label: string };

/** The team or player the page is about, switchable without leaving the tab. */
export type ScopeEntity = {
  /** What the picker chooses, such as "Equipo". */
  readonly noun: string;
  readonly id: string;
  readonly options: readonly ScopeOption[];
  readonly hrefFor: (id: string) => string;
};

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
  /** Latest results in scope, oldest first, drawn as dots under the sample. */
  results?: readonly ResultDot[];
  entity?: ScopeEntity;
  /** Why Pro cannot be chosen, from the pro baseline; null or absent leaves it enabled. */
  proUnavailable?: string | null;
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
  icon,
}: {
  label: string;
  allLabel: string;
  value: string;
  options: readonly ScopeOption[];
  onChange: (value: string) => void;
  icon?: (value: string) => ReactNode;
}): ReactNode {
  return (
    <Select value={value === '' ? ALL : value} onValueChange={(next) => onChange(next === ALL ? '' : next)}>
      <SelectTrigger aria-label={label} className={cn(TRIGGER, value !== '' && 'border-fg-3 text-fg-1')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            <span className="flex items-center gap-2">
              {icon?.(option.value)}
              {option.label}
            </span>
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
        <SelectTrigger aria-label="Periodo" className={cn(TRIGGER, value !== ALL && 'border-fg-3 text-fg-1')}>
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
        <span className="flex items-center gap-1 text-body-sm text-fg-2">
          <Input
            type="date"
            aria-label="Desde"
            value={from}
            max={to || undefined}
            onChange={(event) => onChange({ period: { kind: 'range', from: event.target.value, to } })}
            className="h-8 w-36 px-2"
          />
          a
          <Input
            type="date"
            aria-label="Hasta"
            value={to}
            min={from || undefined}
            onChange={(event) => onChange({ period: { kind: 'range', from, to: event.target.value } })}
            className="h-8 w-36 px-2"
          />
        </span>
      ) : null}
    </>
  );
}

function buyOptions(): ScopeOption[] {
  return SCOPE_BUYS.map((buy) => ({ value: buy, label: buyLabel(buy) }));
}

function BuyGlyph({ value }: { value: string }): ReactNode {
  const buy = SCOPE_BUYS.find((entry) => entry === value);
  const icon = buy === undefined ? null : buyIcon(buy);
  return icon === null ? null : <EquipmentIcon name={icon} height={12} className="text-fg-2" />;
}

function parseBuy(value: string): TacticalBuyType | '' {
  return SCOPE_BUYS.find((entry) => entry === value) ?? '';
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
          icon={(map) => <MapThumb map={map} className="h-4 w-6" crop={1.6} />}
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
        <Select
          value={String(scope.limit ?? limits.fallback)}
          onValueChange={(limit) => onChange({ limit: Number(limit) === limits.fallback ? null : Number(limit) })}
        >
          <SelectTrigger aria-label="Partidas leídas" className={TRIGGER}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {limits.options.map((limit) => (
              <SelectItem key={limit} value={String(limit)}>
                Últimas {limit} partidas
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );
    }
    case 'buy':
      return (
        <OptionSelect
          label="Compra propia"
          allLabel="Compra propia: todas"
          value={scope.buy}
          options={buyOptions()}
          icon={(value) => <BuyGlyph value={value} />}
          onChange={(buy) => onChange({ buy: parseBuy(buy) })}
        />
      );
    case 'opponentBuy':
      return (
        <OptionSelect
          label="Compra del rival"
          allLabel="Compra rival: todas"
          value={scope.opponentBuy}
          options={buyOptions()}
          icon={(value) => <BuyGlyph value={value} />}
          onChange={(buy) => onChange({ opponentBuy: parseBuy(buy) })}
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

function EntityPicker({ entity }: { entity: ScopeEntity }): ReactNode {
  const router = useRouter();
  const query = useSearchParams().toString();
  if (entity.options.length < 2) {
    const current = entity.options.find((option) => option.value === entity.id);
    return current ? <span className="px-1 text-body-sm font-semibold text-fg-1">{current.label}</span> : null;
  }
  return (
    <Select value={entity.id} onValueChange={(id) => router.push(switchEntityHref(entity.hrefFor(id), query))}>
      <SelectTrigger aria-label={entity.noun} className={cn(TRIGGER, 'max-w-56 font-semibold text-fg-1')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {entity.options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

const DOT: Record<ResultDot, string> = { win: 'bg-outcome-win', loss: 'bg-outcome-loss', draw: 'bg-fg-3' };
const DOT_WORD: Record<ResultDot, string> = { win: 'Victoria', loss: 'Derrota', draw: 'Empate' };

function SampleBox({ counts, results }: { counts: ScopeCounts | null | undefined; results: readonly ResultDot[] | undefined }): ReactNode {
  if (counts === undefined) return null;
  const dots = (results ?? []).slice(-MAX_DOTS);
  return (
    <div aria-live="polite" className="flex h-8 flex-col justify-center rounded-md border border-border px-2.5">
      <p className="text-meta leading-tight font-medium whitespace-nowrap text-fg-1 tabular-nums">
        {counts === null ? <span className="text-fg-3">Contando</span> : scopeCountText(counts.maps, counts.rounds)}
      </p>
      {dots.length > 0 ? (
        <p className="flex items-center gap-0.5 pt-0.5" title={`Últimos resultados: ${resultRecord(dots)}`}>
          <span className="sr-only">Últimos resultados: {resultRecord(dots)}</span>
          {dots.map((dot, index) => (
            <span key={index} aria-hidden title={DOT_WORD[dot]} className={cn('size-1.5 rounded-full', DOT[dot])} />
          ))}
        </p>
      ) : null}
    </div>
  );
}

/**
 * The context of an analysis page, Skybox style: whose page it is, the scope
 * controls the current view reads and, at the right, what the numbers are
 * compared with and how many maps and rounds they stand on. It sticks under
 * the command strip so the context stays in sight while scrolling.
 */
export function ScopeBar(props: ScopeBarProps): ReactNode {
  const { scope, fields, counts, results, entity, context, onReset, onChange, className } = props;
  const [more, setMore] = useState(false);
  const filters = fields.filter((field) => field !== 'reference');
  const active = filters.filter((field) => scopeFieldActive(scope, field));
  // Only a long bar folds its rarely used filters away; a short one keeps them in sight.
  const folds = filters.length > MAX_INLINE;
  const primary = filters.filter((field) => !folds || !SECONDARY.has(field));
  const secondary = filters.filter((field) => folds && SECONDARY.has(field));
  const secondaryActive = secondary.filter((field) => active.includes(field)).length;
  const showSecondary = secondary.length > 0 && (more || secondaryActive > 0);

  return (
    <div
      role="group"
      aria-label="Contexto del análisis"
      className={cn(
        'sticky top-(--shell-strip-height) z-20 -mx-1 flex flex-col gap-1.5 border-b border-border bg-background px-1 py-1.5',
        className,
      )}
    >
      <div className="flex flex-wrap items-center gap-1.5">
        {entity ? <EntityPicker entity={entity} /> : null}
        {context ? <div className="flex items-center gap-2 text-body-sm text-fg-2">{context}</div> : null}
        {primary.map((field) => (
          <FieldControl key={field} field={field} props={props} />
        ))}
        {secondary.length > 0 && secondaryActive === 0 ? (
          <Button type="button" variant="ghost" size="xs" aria-expanded={showSecondary} onClick={() => setMore((open) => !open)}>
            <SlidersHorizontal aria-hidden />
            Más filtros
          </Button>
        ) : null}
        {active.length > 0 ? (
          <Button type="button" variant="ghost" size="xs" onClick={() => onReset(active)}>
            <X aria-hidden />
            Quitar filtros
          </Button>
        ) : null}
        <div className="ml-auto flex items-center gap-2">
          {fields.includes('reference') ? (
            <div className="flex items-center gap-1.5">
              <span className="text-meta whitespace-nowrap text-fg-3">Comparando con</span>
              <Segmented
                label="Comparando con"
                value={props.proUnavailable ? 'library' : scope.reference}
                options={BASELINE_SCOPES.map((value) => ({
                  value,
                  label: SCOPE_LABEL[value],
                  disabled: value === 'pro' && Boolean(props.proUnavailable),
                  reason: value === 'pro' ? (props.proUnavailable ?? undefined) : undefined,
                }))}
                onChange={(reference) => onChange({ reference })}
              />
            </div>
          ) : null}
          <SampleBox counts={counts} results={results} />
        </div>
      </div>
      {showSecondary ? (
        <div className="flex flex-wrap items-center gap-1.5">
          {secondary.map((field) => (
            <FieldControl key={field} field={field} props={props} />
          ))}
        </div>
      ) : null}
    </div>
  );
}
