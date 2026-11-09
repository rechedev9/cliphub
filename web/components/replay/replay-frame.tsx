'use client';

import { useState, type ReactNode } from 'react';
import { FilterX, SlidersHorizontal, X } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalSide } from '@/lib/api/tactical';
import { REPLAY_FORM_LABEL, filterChips } from '@/lib/replay/forms';
import type { ReplayForm } from '@/lib/replay/forms';
import { tacticalFilterCount, withoutOrphanOutcome } from '@/lib/tactical-filter';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { cn } from '@/lib/utils';
import { ReplayFilters } from './replay-filters';

const ANY = 'any';
const SIDE_TONE: Record<TacticalSide, string> = { CT: 'text-side-ct', T: 'text-side-t' };

function roundsLabel(matched: number, total: number): string {
  if (matched === total) return `${total} ${total === 1 ? 'ronda' : 'rondas'}`;
  return `${matched} de ${total} rondas`;
}

/**
 * The Replay's scope: who and which side as controls, every other active
 * filter as a chip with its own cross, the full list behind "Más filtros" and
 * how many rounds are left. It writes the whole filter back; the page keeps it
 * in the URL, so the four forms read the same rounds.
 */
export function ReplayScopeBar({
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
  const [open, setOpen] = useState(false);
  const patch = (next: Partial<TacticalFilter>): void => onChange(withoutOrphanOutcome({ ...filter, ...next }));
  // Team and side sit in the bar as controls, so their chips would say the same thing twice.
  const chips = filterChips(filter, { teams: doc.teams, players: doc.players }).filter((chip) => chip.key !== 'team' && chip.key !== 'side');
  const active = tacticalFilterCount(filter);
  const sideOptions = [
    { value: ANY, label: 'Ambos' },
    ...[TACTICAL_SIDES.ct, TACTICAL_SIDES.t].map((side) => ({
      value: side,
      label: <span className={filter.side === side ? SIDE_TONE[side] : undefined}>{side}</span>,
    })),
  ];

  return (
    <div role="region" aria-label="Alcance del Replay" className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border-subtle pb-2">
      {doc.teams.length > 0 ? (
        <Segmented
          label="Equipo"
          value={filter.team_key ?? ANY}
          options={[{ value: ANY, label: 'Ambos equipos' }, ...doc.teams.map((team) => ({ value: team.key, label: team.name || team.key }))]}
          onChange={(value) => patch({ team_key: value === ANY ? undefined : value })}
        />
      ) : null}
      <Segmented
        label="Lado"
        value={filter.side ?? ANY}
        options={sideOptions}
        onChange={(value) => patch({ side: value === TACTICAL_SIDES.ct || value === TACTICAL_SIDES.t ? value : undefined })}
      />
      {chips.length === 0 ? null : (
        <ul className="flex min-w-0 flex-wrap items-center gap-1" aria-label="Filtros activos">
          {chips.map((chip) => (
            <li key={chip.key} className="inline-flex h-7 items-center rounded-sm border border-border-strong bg-surface-3 pl-2 text-meta text-fg-1">
              <span className="max-w-48 truncate">{chip.label}</span>
              <button
                type="button"
                aria-label={`Quitar el filtro ${chip.label}`}
                onClick={() => onChange(chip.without)}
                className={cn('grid size-7 place-items-center rounded-sm text-fg-3 outline-none hover:text-fg-1', FOCUS_RING)}
              >
                <X className="size-3.5" aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}
      <Button type="button" size="xs" variant="outline" onClick={() => setOpen(true)}>
        <SlidersHorizontal aria-hidden />
        Más filtros
        {chips.length > 0 ? <span className="tabular-nums text-fg-2">· {chips.length}</span> : null}
      </Button>
      <span className="ml-auto flex items-center gap-2">
        {active > 0 ? (
          <Button type="button" size="xs" variant="ghost" onClick={() => onChange({})}>
            <FilterX aria-hidden />
            Quitar filtros
          </Button>
        ) : null}
        <span className="text-body-sm tabular-nums text-fg-2" aria-live="polite">
          {roundsLabel(matched, total)}
        </span>
      </span>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="w-80 gap-0 overflow-y-auto sm:max-w-80">
          <SheetHeader>
            <SheetTitle>Filtros</SheetTitle>
            <SheetDescription>{roundsLabel(matched, total)}</SheetDescription>
          </SheetHeader>
          <div className="px-4 pb-4">
            <ReplayFilters doc={doc} filter={filter} onChange={onChange} matched={matched} total={total} />
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}

/** The four forms as tabs under the scope; the open one carries the accent underline. */
export function ReplayFormTabs({ forms, value, onChange }: { forms: readonly ReplayForm[]; value: ReplayForm; onChange: (form: ReplayForm) => void }): ReactNode {
  return (
    <div role="tablist" aria-label="Cómo leer las rondas" className="flex items-end gap-1 border-b border-border-subtle">
      {forms.map((form) => (
        <button
          key={form}
          type="button"
          role="tab"
          aria-selected={value === form}
          onClick={() => onChange(form)}
          className={cn(
            '-mb-px h-8 border-b-2 px-3 text-body-sm outline-none transition-colors duration-(--dur-fast)',
            FOCUS_RING,
            value === form ? 'border-primary font-medium text-fg-1' : 'border-transparent text-fg-2 hover:text-fg-1',
          )}
        >
          {REPLAY_FORM_LABEL[form]}
        </button>
      ))}
    </div>
  );
}
