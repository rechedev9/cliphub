'use client';

import { useId, useState, type ReactNode } from 'react';
import { ChevronRight, FilterX } from 'lucide-react';
import {
  TACTICAL_BUY_TYPE_ORDER,
  TACTICAL_OUTCOMES,
  TACTICAL_PHASES,
  TACTICAL_ROUND_TAGS,
  TACTICAL_SIDES,
  TACTICAL_SITE_ORDER,
} from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter } from '@/lib/api/tactical';
import { roundBoundCommit, tacticalFilterCount, withoutOrphanOutcome } from '@/lib/tactical-filter';
import { buyLabel, ctPatternLabel, outcomeLabel, phaseLabel, roundTagLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { roundTypeCounts, toggleValue } from '@/lib/replay/round-types';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ReplayChip, ReplayGroup } from './replay-chip';

type Choice<T extends string> = { value: T | undefined; label: string };

/** `disabledHint` leaves only the first option ("any") pickable and says why. */
function ChoiceRow<T extends string>({ label, options, value, onChange, disabledHint }: { label: string; options: readonly Choice<T>[]; value: T | undefined; onChange: (next: T | undefined) => void; disabledHint?: string }): ReactNode {
  return (
    <div className="flex flex-col gap-1" role="group" aria-label={label}>
      <span className="text-meta text-fg-3">{label}</span>
      <div className="flex min-w-0 flex-wrap gap-1">
        {options.map((option, index) => (
          <ReplayChip
            key={option.label}
            pressed={value === option.value}
            disabled={disabledHint !== undefined && index > 0}
            onClick={() => onChange(option.value)}
            className="max-w-full"
          >
            <span className="truncate">{option.label}</span>
          </ReplayChip>
        ))}
      </div>
      {disabledHint === undefined ? null : <span className="text-meta text-fg-3">{disabledHint}</span>}
    </div>
  );
}

function MultiRow<T extends string>({ label, values, selected, labelOf, onChange }: { label: string; values: readonly T[]; selected: readonly T[] | undefined; labelOf: (value: T) => string; onChange: (next: T[] | undefined) => void }): ReactNode {
  return (
    <ReplayGroup title={label}>
      <div className="flex flex-wrap gap-1">
        {values.map((value) => (
          <ReplayChip key={value} pressed={selected?.includes(value) ?? false} onClick={() => onChange(toggleValue(selected, value))}>
            {labelOf(value)}
          </ReplayChip>
        ))}
      </div>
    </ReplayGroup>
  );
}

function TypeRow({ label, count, selected, onClick }: { label: string; count: number; selected: boolean; onClick: () => void }): ReactNode {
  return (
    <li>
      <button
        type="button"
        aria-pressed={selected}
        onClick={onClick}
        className={cn(
          'flex h-7 w-full items-center gap-2 rounded-sm px-2 text-left text-body-sm outline-none transition-[background-color,color] duration-(--dur-fast)',
          FOCUS_RING,
          selected ? 'bg-surface-4 font-medium text-fg-1 shadow-[inset_2px_0_0_0_var(--primary)]' : 'text-fg-2 hover:bg-surface-3 hover:text-fg-1',
        )}
      >
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <span className="text-meta tabular-nums text-fg-3">{count}</span>
      </button>
    </li>
  );
}

type RoundBoundProps = { label: string; value: number | undefined; other: number | undefined; edge: 'from' | 'to'; placeholder: string; min: number; max: number; onChange: (next: number | undefined) => void };

/** A value that would invert the range stays a draft in the field until it fits, instead of clearing both bounds. */
function RoundBound({ label, value, other, edge, placeholder, min, max, onChange }: RoundBoundProps): ReactNode {
  const [draft, setDraft] = useState<string | null>(null);
  const inverted = draft !== null && roundBoundCommit({ text: draft, other, edge }) === null;
  const invertedTitle = edge === 'from' ? 'Debe ser menor o igual que el final' : 'Debe ser mayor o igual que el inicio';
  return (
    <input
      type="number"
      inputMode="numeric"
      aria-label={label}
      aria-invalid={inverted || undefined}
      title={inverted ? invertedTitle : undefined}
      min={min}
      max={max}
      placeholder={placeholder}
      value={draft ?? (value === undefined ? '' : String(value))}
      onChange={(event) => {
        const text = event.target.value;
        setDraft(text);
        const commit = roundBoundCommit({ text, other, edge });
        if (commit !== null) onChange(commit.value);
      }}
      onBlur={() => setDraft(null)}
      className={cn(
        'h-7 w-14 rounded-sm border border-border-strong bg-surface-1 px-2 text-meta tabular-nums text-fg-1 outline-none placeholder:text-fg-3 aria-invalid:border-destructive',
        FOCUS_RING,
      )}
    />
  );
}

/**
 * The replay's filter column: who and which side, then the round types with
 * how many rounds each holds, then the rest behind "Más filtros". Every control
 * writes the whole filter back, which the page keeps in the URL.
 */
export function ReplayFilters({
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
  const moreId = useId();
  const patch = (next: Partial<TacticalFilter>): void => onChange(withoutOrphanOutcome({ ...filter, ...next }));
  const outcomeNeedsPerspective = filter.team_key === undefined && filter.side === undefined;
  const types = roundTypeCounts(doc.teams, filter, doc.rounds);
  const moreCount = [filter.buys, filter.opponent_buys, filter.sites, filter.tags, filter.slots, filter.phase === undefined ? undefined : [filter.phase]].filter(
    (values) => values !== undefined && values.length > 0,
  ).length;
  const [moreOpen, setMoreOpen] = useState(moreCount > 0);
  const firstRound = doc.rounds[0]?.number ?? 1;
  const lastRound = doc.rounds.at(-1)?.number ?? total;
  const players = doc.players.map((player) => String(player.slot));
  const nameOf = new Map(doc.players.map((player) => [String(player.slot), player.name]));

  return (
    <div className="flex flex-col gap-3" aria-label="Filtros de rondas" role="region">
      <div className="flex flex-col gap-2.5">
        {doc.teams.length > 0 ? (
          <ChoiceRow
            label="Equipo"
            value={filter.team_key}
            options={[{ value: undefined, label: 'Ambos' }, ...doc.teams.map((team) => ({ value: team.key, label: team.name || team.key }))]}
            onChange={(team_key) => patch({ team_key })}
          />
        ) : null}
        <ChoiceRow
          label="Lado"
          value={filter.side}
          options={[{ value: undefined, label: 'Ambos' }, { value: TACTICAL_SIDES.ct, label: 'CT' }, { value: TACTICAL_SIDES.t, label: 'T' }]}
          onChange={(side) => patch({ side })}
        />
        <ChoiceRow
          label="Resultado"
          value={filter.outcome}
          options={[{ value: undefined, label: 'Todas' }, ...[TACTICAL_OUTCOMES.win, TACTICAL_OUTCOMES.loss].map((value) => ({ value, label: outcomeLabel(value) }))]}
          onChange={(outcome) => patch({ outcome })}
          disabledHint={outcomeNeedsPerspective ? 'Elige un equipo o un lado' : undefined}
        />
        <div className="flex flex-col gap-1" role="group" aria-label="Rondas">
          <span className="text-meta text-fg-3">Rondas</span>
          <div className="flex items-center gap-1.5 text-meta text-fg-3">
            <RoundBound label="Desde la ronda" value={filter.round_from} other={filter.round_to} edge="from" placeholder={String(firstRound)} min={firstRound} max={lastRound} onChange={(round_from) => patch({ round_from })} />
            a
            <RoundBound label="Hasta la ronda" value={filter.round_to} other={filter.round_from} edge="to" placeholder={String(lastRound)} min={firstRound} max={lastRound} onChange={(round_to) => patch({ round_to })} />
          </div>
        </div>
      </div>

      {types.t.length === 0 && types.ct.length === 0 ? null : (
        <ReplayGroup title="Tipos de ronda">
          <div className="flex flex-col gap-2">
            {types.t.length === 0 ? null : (
              <div className="flex flex-col">
                <span className="px-2 pb-0.5 text-meta text-side-t">Ataque</span>
                <ul className="flex flex-col">
                  {types.t.map((entry) => (
                    <TypeRow key={entry.pattern} label={tPatternLabel(entry.pattern)} count={entry.count} selected={entry.selected} onClick={() => patch({ t_patterns: toggleValue(filter.t_patterns, entry.pattern) })} />
                  ))}
                </ul>
              </div>
            )}
            {types.ct.length === 0 ? null : (
              <div className="flex flex-col">
                <span className="px-2 pb-0.5 text-meta text-side-ct">Defensa</span>
                <ul className="flex flex-col">
                  {types.ct.map((entry) => (
                    <TypeRow key={entry.pattern} label={ctPatternLabel(entry.pattern)} count={entry.count} selected={entry.selected} onClick={() => patch({ ct_patterns: toggleValue(filter.ct_patterns, entry.pattern) })} />
                  ))}
                </ul>
              </div>
            )}
          </div>
        </ReplayGroup>
      )}

      <div className="flex flex-col gap-2">
        <button
          type="button"
          aria-expanded={moreOpen}
          aria-controls={moreId}
          onClick={() => setMoreOpen((open) => !open)}
          className={cn('flex h-7 items-center gap-1.5 rounded-sm text-meta text-fg-3 outline-none hover:text-fg-1', FOCUS_RING)}
        >
          <ChevronRight className={cn('size-3.5 transition-transform duration-(--dur-fast)', moreOpen && 'rotate-90')} aria-hidden />
          Más filtros
          {moreCount > 0 ? <span className="rounded-sm bg-surface-4 px-1 tabular-nums text-fg-1">{moreCount}</span> : null}
        </button>
        {moreOpen ? (
          <div id={moreId} className="flex flex-col gap-3">
            <ChoiceRow
              label="Fase"
              value={filter.phase}
              options={[{ value: undefined, label: 'Todas' }, ...[TACTICAL_PHASES.regulation, TACTICAL_PHASES.overtime].map((value) => ({ value, label: phaseLabel(value) }))]}
              onChange={(phase) => patch({ phase })}
            />
            <MultiRow label="Economía propia" values={TACTICAL_BUY_TYPE_ORDER} selected={filter.buys} labelOf={buyLabel} onChange={(buys) => patch({ buys })} />
            <MultiRow label="Economía rival" values={TACTICAL_BUY_TYPE_ORDER} selected={filter.opponent_buys} labelOf={buyLabel} onChange={(opponent_buys) => patch({ opponent_buys })} />
            <MultiRow label="Sitio" values={TACTICAL_SITE_ORDER} selected={filter.sites} labelOf={siteLabel} onChange={(sites) => patch({ sites })} />
            <MultiRow label="Etiquetas" values={Object.values(TACTICAL_ROUND_TAGS)} selected={filter.tags} labelOf={roundTagLabel} onChange={(tags) => patch({ tags })} />
            <MultiRow
              label="Jugador"
              values={players}
              selected={filter.slots?.map(String)}
              labelOf={(slot) => nameOf.get(slot) ?? slot}
              onChange={(slots) => patch({ slots: slots?.map(Number) })}
            />
          </div>
        ) : null}
      </div>

      <div className="flex items-center justify-between gap-2 border-t border-border-subtle pt-2">
        <span className="text-meta tabular-nums text-fg-2">
          {matched} de {total} rondas
        </span>
        <Button type="button" size="xs" variant="ghost" disabled={tacticalFilterCount(filter) === 0} onClick={() => onChange({})}>
          <FilterX aria-hidden />
          Limpiar
        </Button>
      </div>
    </div>
  );
}
