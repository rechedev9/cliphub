'use client';

import { useState, type ReactNode } from 'react';
import { SlidersHorizontal, X } from 'lucide-react';
import type { ReportSide } from '@/lib/api/scout';
import { BASELINE_SCOPES, type Baseline } from '@/lib/api/baseline';
import type { SampleNotes, TeamLineups } from '@/lib/api/team-insights';
import { resultRecord, type ResultDot } from '@/lib/analysis/context-bar';
import { buyIcon } from '@/lib/analysis/icons';
import { SCOPE_LABEL, proReferenceShort, proSourceText } from '@/lib/analysis/reference';
import {
  HALF_BUY,
  PERIOD_PRESETS,
  SCOPE_BUYS,
  SCOPE_LIMIT_MAX,
  parseScopeBuy,
  scopeCountText,
  scopeFieldActive,
  type AnalysisScope,
  type ScopeField,
} from '@/lib/analysis/scope';
import type { MapCounts } from '@/lib/analysis/map-counts';
import { buyLabel } from '@/lib/tactical-labels';
import { EntityPicker } from '@/components/analysis/entity-picker';
import { LineupPicker } from '@/components/analysis/lineup-picker';
import { ScopeMapSelect } from '@/components/analysis/scope-map-select';
import {
  AdvantageSelect,
  AwpSelect,
  BuyMatchupSelect,
  OpponentTopSelect,
  RoundRangeControl,
  WhoseSelect,
} from '@/components/analysis/scope-round-controls';
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
const SECONDARY: ReadonlySet<ScopeField> = new Set([
  'buy', 'opponentBuy', 'opponent', 'awp', 'opponentAwp', 'advantage', 'rounds', 'opponentTop', 'whose',
]);
const MAX_DOTS = 10;
const MAX_INLINE = 4;

const SIDES: readonly { value: ReportSide; label: string }[] = [
  { value: '', label: 'Ambos' },
  { value: 'CT', label: 'CT' },
  { value: 'T', label: 'T' },
];

/** `notes` says what the count leaves out or estimates: unrated rivals, the match cap, AWP from kills. */
export type ScopeCounts = { readonly maps: number; readonly rounds: number; readonly notes?: SampleNotes };

export type ScopeOption = { readonly value: string; readonly label: string };

/** The team or player the page is about, switchable without leaving the tab. */
export type ScopeEntity = {
  /** What the picker chooses, such as "Equipo". */
  readonly noun: string;
  readonly id: string;
  readonly options: readonly ScopeOption[];
  /** Keep the last four opened in the picker's "Recientes" group. */
  readonly recents?: boolean;
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
  /** The view reads one map at a time, so the empty choice asks for one instead of offering all. */
  mapRequired?: boolean;
  /** Matches per map in the scope, shown as "N partidas"; a map with 0 cannot be chosen. */
  mapCounts?: MapCounts | null;
  /** Rival teams for the opponent control. */
  opponents?: readonly ScopeOption[];
  /** The match limits offered, and the one the view uses while the URL names none. */
  limits?: { readonly options: readonly number[]; readonly fallback: number };
  /** Sample size of the answer on screen; null while it is being counted, absent to hide it. */
  counts?: ScopeCounts | null;
  /** Latest results in scope, oldest first, drawn as dots under the sample. */
  results?: readonly ResultDot[];
  entity?: ScopeEntity;
  /** The team's players for the lineup control; it shows only while the view reads `lineup`. */
  lineups?: TeamLineups | null;
  /** The pro reference the view reads, so the bar can say where Pro comes from; null while loading. */
  proReference?: Baseline | null;
  /** Teams marked as Mi equipo; with none, the whose-matches control stays off. */
  ownTeams?: number;
  /** Fixed context the person cannot change here, such as a report's rival and side. */
  context?: ReactNode;
  /** What the view's figures are compared with, such as the "Comparar con" teams; sits by the sample. */
  comparison?: ReactNode;
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
  return [...SCOPE_BUYS.map((buy) => ({ value: buy, label: buyLabel(buy) })), { value: HALF_BUY, label: 'Media (semi o force)' }];
}

function BuyGlyph({ value }: { value: string }): ReactNode {
  const buy = SCOPE_BUYS.find((entry) => entry === value);
  const icon = buy === undefined ? null : buyIcon(buy);
  return icon === null ? null : <EquipmentIcon name={icon} height={12} className="text-fg-2" />;
}

function FieldControl({ field, props }: { field: ScopeField; props: ScopeBarProps }): ReactNode {
  const { scope, onChange } = props;
  switch (field) {
    case 'map':
      return (
        <ScopeMapSelect
          value={scope.map}
          maps={props.maps ?? []}
          required={props.mapRequired === true}
          counts={props.mapCounts ?? null}
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
        <>
          {props.fields.includes('opponentBuy') ? <BuyMatchupSelect scope={scope} onChange={onChange} /> : null}
          <OptionSelect
          label="Compra propia"
          allLabel="Compra propia: todas"
          value={scope.buy}
          options={buyOptions()}
          icon={(value) => <BuyGlyph value={value} />}
          onChange={(buy) => onChange({ buy: parseScopeBuy(buy) })}
          />
        </>
      );
    case 'opponentBuy':
      return (
        <OptionSelect
          label="Compra del rival"
          allLabel="Compra rival: todas"
          value={scope.opponentBuy}
          options={buyOptions()}
          icon={(value) => <BuyGlyph value={value} />}
          onChange={(buy) => onChange({ opponentBuy: parseScopeBuy(buy) })}
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
    case 'awp':
      return <AwpSelect rival={false} scope={scope} onChange={onChange} />;
    case 'opponentAwp':
      return <AwpSelect rival scope={scope} onChange={onChange} />;
    case 'advantage':
      return <AdvantageSelect scope={scope} onChange={onChange} />;
    case 'rounds':
      return <RoundRangeControl scope={scope} onChange={onChange} />;
    case 'opponentTop':
      return <OpponentTopSelect scope={scope} onChange={onChange} />;
    case 'whose':
      return <WhoseSelect scope={scope} ownTeams={props.ownTeams ?? 0} onChange={onChange} />;
    default:
      return null;
  }
}

const DOT: Record<ResultDot, string> = { win: 'bg-outcome-win', loss: 'bg-outcome-loss', draw: 'bg-fg-3' };
const DOT_WORD: Record<ResultDot, string> = { win: 'Victoria', loss: 'Derrota', draw: 'Empate' };

function SampleNoteList({ notes }: { notes: SampleNotes }): ReactNode {
  const items = [
    { count: notes.unrated, text: `${notes.unrated} sin ranking`, title: 'Partidas fuera porque el rival no tiene ranking HLTV' },
    { count: notes.capped, text: `${notes.capped} fuera por el tope de ${SCOPE_LIMIT_MAX}`, title: `Se leen como mucho las ${SCOPE_LIMIT_MAX} partidas más recientes` },
    {
      count: notes.awpEstimated,
      text: `AWP estimado en ${notes.awpEstimated} ${notes.awpEstimated === 1 ? 'partida' : 'partidas'}`,
      title: 'Su análisis es anterior al registro de AWP: se deduce de las bajas y los daños con AWP. Vuelve a analizarlas para el dato exacto.',
    },
  ].filter((item) => item.count > 0);
  return items.map((item) => (
    <p key={item.text} className="text-meta whitespace-nowrap text-fg-3 tabular-nums" title={item.title}>
      {item.text}
    </p>
  ));
}

function SampleBox({ counts, results }: { counts: ScopeCounts | null | undefined; results: readonly ResultDot[] | undefined }): ReactNode {
  if (counts === undefined) return null;
  const dots = (results ?? []).slice(-MAX_DOTS);
  return (
    <div aria-live="polite" className="flex h-8 items-center gap-2.5 pl-1">
      <p className="text-meta whitespace-nowrap text-fg-2 tabular-nums">
        {counts === null ? <span className="text-fg-3">Contando</span> : scopeCountText(counts.maps, counts.rounds)}
      </p>
      {counts !== null && counts.notes !== undefined ? <SampleNoteList notes={counts.notes} /> : null}
      {dots.length > 0 ? (
        <p className="flex items-center gap-1" title={`Últimos resultados: ${resultRecord(dots)}`}>
          <span className="sr-only">Últimos resultados: {resultRecord(dots)}</span>
          {dots.map((dot, index) => (
            <span key={index} aria-hidden title={DOT_WORD[dot]} className={cn('size-2 rounded-full', DOT[dot])} />
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
  const { scope, fields, counts, results, entity, context, lineups, onReset, onChange, className } = props;
  const [more, setMore] = useState(false);
  // The lineup sits by the entity it belongs to and keeps its own reset, like the reference.
  const filters = fields.filter((field) => field !== 'reference' && field !== 'lineup');
  const picked = fields.includes('lineup') && lineups ? lineups : null;
  const players = picked !== null && picked.latest.length + picked.others.length > 0 ? picked : null;
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
        {players ? <LineupPicker lineup={scope.lineup} lineups={players} onChange={(lineup) => onChange({ lineup })} /> : null}
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
          {props.comparison}
          {fields.includes('reference') ? (
            <div className="flex items-center gap-1.5">
              <span className="text-meta whitespace-nowrap text-fg-3">Comparando con</span>
              <Segmented
                label="Comparando con"
                value={scope.reference}
                options={BASELINE_SCOPES.map((value) => ({ value, label: SCOPE_LABEL[value] }))}
                onChange={(reference) => onChange({ reference })}
              />
              {scope.reference === 'pro' && props.proReference ? (
                <span className="text-meta whitespace-nowrap text-fg-3" title={proSourceText(props.proReference)}>
                  {proReferenceShort(props.proReference)}
                </span>
              ) : null}
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
