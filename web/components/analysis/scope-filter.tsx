'use client';

import type { ReactNode } from 'react';
import { CalendarDays, Clock, Link2, X } from 'lucide-react';
import { toast } from 'sonner';
import type { ReportSide } from '@/lib/api/scout';
import { scopeCountText, scopeFieldActive, type ScopeField } from '@/lib/analysis/scope';
import { scopeChips } from '@/lib/analysis/scope-chips';
import type { ScopeBarProps } from '@/components/analysis/scope-bar';
import { EntityPicker } from '@/components/analysis/entity-picker';
import { LineupPicker } from '@/components/analysis/lineup-picker';
import { EconomyField } from '@/components/analysis/scope-economy';
import { CAPS_LABEL, FIELD, FIELD_SET } from '@/components/analysis/scope-field-style';
import { FilterMenu, menuActiveFields } from '@/components/analysis/scope-filter-menu';
import { CounterWidget, HeaderWidget, ReferenceWidget } from '@/components/analysis/scope-header-widgets';
import { ScopeMapSelect } from '@/components/analysis/scope-map-select';
import { PeriodField } from '@/components/analysis/scope-period';
import { BuySwapButton } from '@/components/analysis/scope-replay-controls';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** `header` is the sticky bar of the statistics pages; `stack` the compact floating column over a tool canvas. */
export type ScopeFilterForm = 'header' | 'stack';

export type ScopeFilterProps = ScopeBarProps & { form: ScopeFilterForm };

const SIDES: readonly { value: ReportSide; label: string; active: string }[] = [
  { value: '', label: 'Ambos', active: 'bg-surface-5 text-fg-1' },
  { value: 'CT', label: 'CT', active: 'bg-side-ct/20 text-side-ct' },
  { value: 'T', label: 'T', active: 'bg-side-t/20 text-side-t' },
];

/** "JUGANDO": both sides, or CT in blue, or T in orange. */
function SideToggle({ value, onChange }: { value: ReportSide; onChange: (side: ReportSide) => void }): ReactNode {
  return (
    <div role="group" aria-label="Lado" className="inline-flex overflow-hidden rounded-md border border-border-strong bg-surface-1">
      {SIDES.map((side) => (
        <button
          key={side.value}
          type="button"
          aria-pressed={value === side.value}
          onClick={() => onChange(side.value)}
          className={cn(
            'h-6.5 border-l border-border-strong px-2.5 text-body-sm font-semibold whitespace-nowrap text-fg-3 outline-none first:border-l-0',
            'hover:bg-surface-3 hover:text-fg-1 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
            value === side.value && side.active,
          )}
        >
          {side.label}
        </button>
      ))}
    </div>
  );
}

function LimitField({ props }: { props: ScopeBarProps }): ReactNode {
  const { scope, onChange } = props;
  const limits = props.limits ?? { options: [10, 20, 50], fallback: 20 };
  return (
    <Select
      value={String(scope.limit ?? limits.fallback)}
      onValueChange={(limit) => onChange({ limit: Number(limit) === limits.fallback ? null : Number(limit) })}
    >
      <SelectTrigger aria-label="Partidas leídas" className={cn(FIELD, scope.limit !== null && FIELD_SET)}>
        <span className="flex min-w-0 items-center gap-1.5">
          <Clock aria-hidden className="size-3.5 shrink-0 text-fg-3" />
          <SelectValue />
        </span>
      </SelectTrigger>
      <SelectContent>
        {limits.options.map((limit) => (
          <SelectItem key={limit} value={String(limit)}>
            {limit} {limit === 1 ? 'partida' : 'partidas'}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function CopyLinkButton(): ReactNode {
  async function copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(window.location.href);
      toast.success('Enlace copiado');
    } catch {
      toast.error('No se pudo copiar el enlace');
    }
  }
  return (
    <Button type="button" variant="outline" size="icon-sm" className="size-7" aria-label="Copiar enlace con estos filtros" title="Copiar enlace" onClick={() => void copy()}>
      <Link2 aria-hidden className="size-3.5" />
    </Button>
  );
}

function ActiveChips({ props, fields }: { props: ScopeBarProps; fields: readonly ScopeField[] }): ReactNode {
  const names = {
    opponents: new Map((props.opponents ?? []).map((option) => [option.value, option.label])),
    tactics: new Map((props.tactics ?? []).map((option) => [option.value, option.label])),
  };
  const chips = scopeChips({ scope: props.scope, fields, names });
  if (chips.length === 0) return null;
  return (
    <ul aria-label="Filtros activos" className="flex flex-wrap items-center gap-1.5">
      {chips.map((chip) => (
        <li key={chip.field}>
          <Button type="button" variant="outline" size="xs" aria-label={`Quitar el filtro ${chip.label}`} onClick={() => props.onReset([chip.field])}>
            {chip.label}
            <X aria-hidden />
          </Button>
        </li>
      ))}
    </ul>
  );
}

function LabelledRow({ label, children }: { label: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex items-center gap-1.5">
      <span className={cn(CAPS_LABEL, 'w-14 shrink-0')}>{label}</span>
      {children}
    </div>
  );
}

function teamName(props: ScopeBarProps): string | undefined {
  const { entity } = props;
  return entity?.tone === 'team' ? entity.options.find((option) => option.value === entity.id)?.label : undefined;
}

/**
 * The scope controls of an analysis, Skybox EDGE style, wired to the scope the page keeps in its URL.
 * The `header` form sticks under the command strip with the sample widgets at its right; the `stack`
 * form is the same controls as a compact column to float over a tool canvas.
 */
export function ScopeFilter(props: ScopeFilterProps): ReactNode {
  const { scope, fields, counts, results, entity, context, lineups, onChange, onReset, form, className } = props;
  const stack = form === 'stack';
  const filters = fields.filter((field) => field !== 'reference' && field !== 'lineup');
  const present = (field: ScopeField): boolean => fields.includes(field);
  const picked = present('lineup') && lineups ? lineups : null;
  const players = picked !== null && picked.latest.length + picked.others.length > 0 ? picked : null;
  const shown = props.mapRequired === true ? filters.filter((field) => field !== 'map') : filters;
  const active = shown.filter((field) => scopeFieldActive(scope, field));
  const menuActive = menuActiveFields(scope, fields);
  const ownEconomy = present('buy') || present('equip');
  const rivalEconomy = present('opponentBuy') || present('opponentEquip');
  const showBuy = present('buy') || present('opponentBuy');

  const teamRow = (
    <div className="flex items-center gap-1.5">
      {entity ? <EntityPicker entity={entity} /> : null}
      {players ? <LineupPicker lineup={scope.lineup} lineups={players} onChange={(lineup) => onChange({ lineup })} /> : null}
      {context ? <div className="flex items-center gap-2 text-body-sm text-fg-2">{context}</div> : null}
    </div>
  );
  const mapRow = present('map') ? (
    <LabelledRow label="En">
      <ScopeMapSelect
        value={scope.map}
        maps={props.maps ?? []}
        required={props.mapRequired === true}
        counts={props.mapCounts ?? null}
        onChange={(map) => onChange({ map })}
        className="min-w-32"
      />
    </LabelledRow>
  ) : null;
  const sideRow = present('side') ? (
    <LabelledRow label="Jugando">
      <SideToggle value={scope.side} onChange={(side) => onChange({ side })} />
    </LabelledRow>
  ) : null;
  const periodRow =
    present('period') || present('limit') ? (
      <div className="flex flex-wrap items-center gap-1.5">
        {present('period') ? <PeriodField scope={scope} onChange={onChange} icon={<CalendarDays aria-hidden className="size-3.5 shrink-0 text-fg-3" />} /> : null}
        {present('limit') ? <LimitField props={props} /> : null}
      </div>
    ) : null;
  const economyRow =
    ownEconomy || rivalEconomy ? (
      <div className="flex flex-wrap items-center gap-1.5">
        {ownEconomy ? <EconomyField scope={scope} onChange={onChange} rival={false} name={teamName(props)} showBuy={present('buy')} showEquip={present('equip')} /> : null}
        {ownEconomy && rivalEconomy && showBuy ? <BuySwapButton scope={scope} onChange={onChange} /> : null}
        {rivalEconomy ? <EconomyField scope={scope} onChange={onChange} rival name={undefined} showBuy={present('opponentBuy')} showEquip={present('opponentEquip')} /> : null}
      </div>
    ) : null;
  const toolsRow = (
    <div className="flex flex-wrap items-center gap-1.5">
      <FilterMenu
        scope={scope}
        fields={fields}
        onChange={onChange}
        onReset={onReset}
        opponents={props.opponents ?? []}
        tactics={props.tactics ?? []}
        ownTeams={props.ownTeams ?? 0}
      />
      <CopyLinkButton />
      {active.length > 0 ? (
        <Button type="button" variant="ghost" size="xs" className="gap-1 text-fg-3 hover:text-destructive" onClick={() => onReset(active)}>
          <X aria-hidden />
          Limpiar
        </Button>
      ) : null}
    </div>
  );
  const chips = <ActiveChips props={props} fields={menuActive} />;
  const sample = counts === undefined ? null : <CounterWidget counts={counts} results={results} matches={props.matches} />;

  if (stack) {
    return (
      <div role="group" aria-label="Contexto del análisis" className={cn('flex w-72 max-w-full flex-col gap-1.5', className)}>
        {teamRow}
        {mapRow}
        {sideRow}
        {periodRow}
        {economyRow}
        {toolsRow}
        {chips}
        {counts === undefined || counts === null ? null : <p className="text-meta text-fg-3 tabular-nums">{scopeCountText(counts.maps, counts.rounds)}</p>}
      </div>
    );
  }

  return (
    <div
      role="group"
      aria-label="Contexto del análisis"
      className={cn(
        'sticky top-(--shell-strip-height) z-20 -mx-1 flex flex-wrap items-start gap-x-3 gap-y-1.5 border-b border-border bg-background px-1 py-1.5',
        className,
      )}
    >
      <div className="flex min-w-0 flex-1 basis-[30rem] flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          {teamRow}
          {mapRow}
          {sideRow}
        </div>
        <div className="flex flex-wrap items-center gap-x-1.5 gap-y-1.5">
          {periodRow}
          {economyRow}
          {toolsRow}
        </div>
        {chips}
      </div>
      <div className="ml-auto flex flex-wrap items-stretch justify-end gap-2">
        {present('reference') ? <ReferenceWidget scope={scope} onChange={onChange} proReference={props.proReference} /> : null}
        {props.comparison ? <HeaderWidget title="Comparando con">{props.comparison}</HeaderWidget> : null}
        {sample}
      </div>
    </div>
  );
}
