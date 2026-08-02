'use client';

import { useId, useState, type ReactNode } from 'react';
import { FilterX, SlidersHorizontal } from 'lucide-react';
import {
  TACTICAL_BUY_TYPE_ORDER,
  TACTICAL_CT_PATTERN_ORDER,
  TACTICAL_OUTCOMES,
  TACTICAL_PHASES,
  TACTICAL_ROUND_TAGS,
  TACTICAL_SIDES,
  TACTICAL_SITE_ORDER,
  TACTICAL_T_PATTERN_ORDER,
} from '@/lib/api/tactical';
import type {
  TacticalBuyType,
  TacticalCTPattern,
  TacticalDocument,
  TacticalFilter,
  TacticalOutcome,
  TacticalPhase,
  TacticalSide,
  TacticalSite,
  TacticalTPattern,
} from '@/lib/api/tactical';
import { tacticalFilterCount } from '@/lib/tactical-filter';
import {
  buyLabel,
  ctPatternLabel,
  outcomeLabel,
  phaseLabel,
  roundTagLabel,
  siteLabel,
  tPatternLabel,
} from '@/lib/tactical-labels';
import { STUDIO_FILTER_CHIP_CLASS } from '@/components/studio/filter-chip';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** Sentinel for the "no constraint" chip of a single-choice group. */
const ANY_VALUE = 'any';

type Option<T extends string> = { value: T; label: string };

function optionsFor<T extends string>(values: readonly T[], label: (value: T) => string): Option<T>[] {
  return values.map((value) => ({ value, label: label(value) }));
}

function GroupLabel({ children, id }: { children: ReactNode; id: string }): ReactNode {
  return (
    <span id={id} className="font-mono text-meta uppercase tracking-widest text-fg-3">
      {children}
    </span>
  );
}

/** A multi-value field: every chip ORs into the same filter field. */
function MultiChips<T extends string>({
  label,
  options,
  selected,
  onChange,
}: {
  label: string;
  options: readonly Option<T>[];
  selected: readonly T[] | undefined;
  onChange: (next: T[] | undefined) => void;
}): ReactNode {
  const labelId = useId();
  const active = selected ?? [];
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <GroupLabel id={labelId}>{label}</GroupLabel>
      <ToggleGroup
        type="multiple"
        value={[...active]}
        onValueChange={(next: string[]) => {
          const kept = options.filter((option) => next.includes(option.value)).map((option) => option.value);
          onChange(kept.length > 0 ? kept : undefined);
        }}
        aria-labelledby={labelId}
        className="flex w-full flex-wrap gap-2"
      >
        {options.map((option) => (
          <ToggleGroupItem
            key={option.value}
            value={option.value}
            aria-label={`${label}: ${option.label}`}
            className={STUDIO_FILTER_CHIP_CLASS}
          >
            {option.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}

/** A single-choice field, with an explicit "any" chip so clearing is discoverable. */
function SingleChips<T extends string>({
  label,
  anyLabel,
  options,
  selected,
  onChange,
}: {
  label: string;
  anyLabel: string;
  options: readonly Option<T>[];
  selected: T | undefined;
  onChange: (next: T | undefined) => void;
}): ReactNode {
  const labelId = useId();
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <GroupLabel id={labelId}>{label}</GroupLabel>
      <ToggleGroup
        type="single"
        value={selected ?? ANY_VALUE}
        onValueChange={(next: string) => {
          const match = options.find((option) => option.value === next);
          onChange(match?.value);
        }}
        aria-labelledby={labelId}
        className="flex w-full flex-wrap gap-2"
      >
        <ToggleGroupItem value={ANY_VALUE} aria-label={`${label}: ${anyLabel}`} className={STUDIO_FILTER_CHIP_CLASS}>
          {anyLabel}
        </ToggleGroupItem>
        {options.map((option) => (
          <ToggleGroupItem
            key={option.value}
            value={option.value}
            aria-label={`${label}: ${option.label}`}
            className={STUDIO_FILTER_CHIP_CLASS}
          >
            {option.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}

/** A round bound; an empty or non-positive value simply removes the bound. */
function BoundInput({
  label,
  value,
  max,
  placeholder,
  edgeClassName,
  onChange,
}: {
  label: string;
  value: number | undefined;
  max: number;
  placeholder: string;
  /** Which outer corners this cell keeps, so the two bounds read as one plate. */
  edgeClassName: string;
  onChange: (next: number | undefined) => void;
}): ReactNode {
  const inputId = useId();
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-2">
      <label htmlFor={inputId} className="font-mono text-meta uppercase tracking-widest text-fg-3">
        {label}
      </label>
      <Input
        id={inputId}
        type="number"
        inputMode="numeric"
        min={1}
        max={max}
        placeholder={placeholder}
        value={value === undefined ? '' : String(value)}
        onChange={(event) => {
          const parsed = Number(event.target.value);
          onChange(Number.isInteger(parsed) && parsed >= 1 ? parsed : undefined);
        }}
        // The fill stays the primitive's --surface-3, which the ramp annotates
        // as "field-on-panel": these are the only <Input>s in the zone, and a
        // local override made them the only two fields in the product that do
        // not match the field recipe. It also contradicted itself — --elev-0 is
        // the RAISED bevel (light top, shade bottom), so a step-down fill under
        // a step-up bevel read as two depths at once. Only the joined corners
        // and the mono/tabular type are local. z-10 on focus is
        // toggle-group.tsx's fix, so one cell's 2px focus outline is not painted
        // over by its neighbour's background.
        className={cn('font-mono text-body-sm tabular-nums focus-visible:z-10', edgeClassName)}
      />
    </div>
  );
}

const BUY_OPTIONS = optionsFor<TacticalBuyType>(TACTICAL_BUY_TYPE_ORDER, buyLabel);
const SITE_OPTIONS = optionsFor<TacticalSite>(TACTICAL_SITE_ORDER, siteLabel);
const T_PATTERN_OPTIONS = optionsFor<TacticalTPattern>(TACTICAL_T_PATTERN_ORDER, tPatternLabel);
const CT_PATTERN_OPTIONS = optionsFor<TacticalCTPattern>(TACTICAL_CT_PATTERN_ORDER, ctPatternLabel);
const TAG_OPTIONS: Option<string>[] = Object.values(TACTICAL_ROUND_TAGS).map((tag) => ({
  value: tag,
  label: roundTagLabel(tag),
}));
const SIDE_OPTIONS: Option<TacticalSide>[] = [
  { value: TACTICAL_SIDES.ct, label: 'CT' },
  { value: TACTICAL_SIDES.t, label: 'T' },
];
const OUTCOME_OPTIONS = optionsFor<TacticalOutcome>(
  [TACTICAL_OUTCOMES.win, TACTICAL_OUTCOMES.loss],
  outcomeLabel,
);
const PHASE_OPTIONS = optionsFor<TacticalPhase>(
  [TACTICAL_PHASES.regulation, TACTICAL_PHASES.overtime],
  phaseLabel,
);

/**
 * The filter, which is also the URL. Every control writes the whole filter back
 * through `onChange`, the workspace puts it in the query string, and the round
 * list and the aggregate endpoint both read that one filter — so a filtered view
 * is linkable, survives a reload, and can never show rounds the tendencies were
 * not computed from.
 */
export function TacticalFilterBar({
  doc,
  filter,
  onChange,
  matched,
  total,
}: {
  doc: TacticalDocument;
  filter: TacticalFilter;
  onChange: (next: TacticalFilter) => void;
  matched: number;
  total: number;
}): ReactNode {
  const teamOptions: Option<string>[] = doc.teams.map((team) => ({
    value: team.key,
    label: team.name || team.key,
  }));
  const playerOptions: Option<string>[] = doc.players.map((player) => ({
    value: String(player.slot),
    label: player.name,
  }));
  const selectedSlots = (filter.slots ?? []).map(String);

  const advancedCount = [
    filter.buys?.length,
    filter.opponent_buys?.length,
    filter.sites?.length,
    filter.t_patterns?.length,
    filter.ct_patterns?.length,
    filter.tags?.length,
    filter.slots?.length,
  ].filter((count) => count !== undefined && count > 0).length;
  // Opened by a pasted URL that already carries advanced constraints, and left
  // under the user's control from then on.
  const [advancedOpen, setAdvancedOpen] = useState(advancedCount > 0);

  // An undefined value means "no constraint": the serializer skips it, so the
  // URL stays as short as the filter is.
  const patch = (next: Partial<TacticalFilter>): void => {
    onChange({ ...filter, ...next });
  };

  return (
    // shadow-md maps onto --elev-2, so the control rail sits one step above the
    // --elev-1 content panels it filters. .studio-panel owns the radius.
    <section
      className="studio-panel px-5 py-5 shadow-md sm:px-6 sm:py-6"
      aria-label="Filtros de rondas"
    >
      <div className="flex flex-col gap-5">
        {/* One wrapping rail, not two nested flex containers: two levels can
            never share a line, so the counter was always pushed onto a third,
            otherwise empty band. Flat, every group is a sibling and the rail
            wraps once. */}
        <div className="flex flex-wrap items-end gap-x-8 gap-y-4">
          {teamOptions.length > 0 ? (
            <SingleChips
              label="Equipo"
              anyLabel="Ambos"
              options={teamOptions}
              selected={filter.team_key}
              onChange={(team_key) => patch({ team_key })}
            />
          ) : null}
          <SingleChips
            label="Lado"
            anyLabel="Ambos"
            options={SIDE_OPTIONS}
            selected={filter.side}
            onChange={(side) => patch({ side })}
          />
          <SingleChips
            label="Resultado"
            anyLabel="Todas"
            options={OUTCOME_OPTIONS}
            selected={filter.outcome}
            onChange={(outcome) => patch({ outcome })}
          />
          <SingleChips
            label="Fase"
            anyLabel="Todas"
            options={PHASE_OPTIONS}
            selected={filter.phase}
            onChange={(phase) => patch({ phase })}
          />

          {/* Range, readout and reset are one scope group: how much of the
              match, and what that left. They stay in ONE flex item so the
              counter can never be pushed onto a line of its own, and that item
              GROWS instead of taking an ml-auto: an auto margin would pin the
              whole group to the right end of its line, leaving the rail's left
              two thirds empty. Growing plus justify-between spends the same
              free space between the fields and the actions, so the plate keeps
              the rail's left edge (under EQUIPO) and the counter stays flush
              right. items-end bottom-aligns the labelled plate with the counter
              cluster, which keeps its own items-center so the readout stays
              centred on LIMPIAR. */}
          <div className="flex grow flex-wrap items-end justify-between gap-x-8 gap-y-4">
            {/* One plate, not two boxes: 160px of two 80px cells sharing a
                single hairline (the joined recipe from toggle-group.tsx), each
                showing the bound it already declares in min/max as a
                placeholder, so an unset window reads "1 | 19" instead of as two
                empty voids. */}
            <div className="flex w-40 shrink-0">
              <BoundInput
                label="Desde"
                value={filter.round_from}
                max={total}
                placeholder="1"
                edgeClassName="rounded-r-none"
                onChange={(round_from) => patch({ round_from })}
              />
              <BoundInput
                label="Hasta"
                value={filter.round_to}
                max={total}
                placeholder={String(total)}
                edgeClassName="rounded-l-none border-l-0"
                onChange={(round_to) => patch({ round_to })}
              />
            </div>

            <div className="flex shrink-0 items-center gap-3">
              <span className="font-mono text-meta uppercase text-fg-2 tabular-nums">
                {matched} / {total} rondas
              </span>
              <Button
                variant="outline"
                onClick={() => onChange({})}
                disabled={tacticalFilterCount(filter) === 0}
                // The outline variant already ships a measured disabled recipe
                // (--surface-2 + --fg-3, the AA floor); the base opacity-50 was
                // compositing that floor down to ~2:1.
                className="font-mono text-meta tracking-wider disabled:opacity-100"
              >
                <FilterX aria-hidden />
                LIMPIAR
              </Button>
            </div>
          </div>
        </div>

        <details
          open={advancedOpen}
          onToggle={(event) => setAdvancedOpen(event.currentTarget.open)}
          className="group border-t border-border-subtle pt-4"
        >
          <summary
            className={cn(
              'flex cursor-pointer list-none items-center gap-2 font-mono text-meta uppercase tracking-widest text-fg-3 outline-none hover:text-foreground',
              FOCUS_RING,
            )}
          >
            <SlidersHorizontal className="size-3.5" aria-hidden />
            Filtros avanzados
            {advancedCount > 0 ? (
              <Badge variant="outline" className="border-primary/45 text-primary tabular-nums">
                {advancedCount}
              </Badge>
            ) : null}
          </summary>

          <div className="mt-5 grid gap-x-8 gap-y-5 lg:grid-cols-2">
            <MultiChips
              label="Economía propia"
              options={BUY_OPTIONS}
              selected={filter.buys}
              onChange={(buys) => patch({ buys })}
            />
            <MultiChips
              label="Economía rival"
              options={BUY_OPTIONS}
              selected={filter.opponent_buys}
              onChange={(opponent_buys) => patch({ opponent_buys })}
            />
            <MultiChips
              label="Sitio"
              options={SITE_OPTIONS}
              selected={filter.sites}
              onChange={(sites) => patch({ sites })}
            />
            <MultiChips
              label="Etiquetas"
              options={TAG_OPTIONS}
              selected={filter.tags}
              onChange={(tags) => patch({ tags })}
            />
            <MultiChips
              label="Forma T"
              options={T_PATTERN_OPTIONS}
              selected={filter.t_patterns}
              onChange={(t_patterns) => patch({ t_patterns })}
            />
            <MultiChips
              label="Forma CT"
              options={CT_PATTERN_OPTIONS}
              selected={filter.ct_patterns}
              onChange={(ct_patterns) => patch({ ct_patterns })}
            />
            <div className="lg:col-span-2">
              <MultiChips
                label="Jugador"
                options={playerOptions}
                selected={selectedSlots}
                onChange={(slots) =>
                  patch({ slots: slots === undefined ? undefined : slots.map(Number) })
                }
              />
            </div>
          </div>
        </details>
      </div>
    </section>
  );
}
