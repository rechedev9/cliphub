'use client';

import { useState, type ReactNode } from 'react';
import { Filter } from 'lucide-react';
import { groupFields, visibleGroups, type FilterGroup } from '@/lib/analysis/scope-filter-groups';
import { scopeFieldActive, type AnalysisScope, type ScopeField } from '@/lib/analysis/scope';
import type { TacticOption } from '@/lib/analysis/scope-tactics';
import type { ScopeOption } from '@/components/analysis/scope-bar';
import { FIELD, FIELD_SET } from '@/components/analysis/scope-field-style';
import { MenuField } from '@/components/analysis/scope-menu-field';
import { BuyMatchupSelect } from '@/components/analysis/scope-round-controls';
import { BuySwapButton } from '@/components/analysis/scope-replay-controls';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

export type FilterMenuProps = {
  scope: AnalysisScope;
  fields: readonly ScopeField[];
  onChange: (patch: Partial<AnalysisScope>) => void;
  onReset: (fields: readonly ScopeField[]) => void;
  opponents: readonly ScopeOption[];
  tactics: readonly TacticOption[];
  ownTeams: number;
};

/** The fields of the menu that hold something other than their default. */
export function menuActiveFields(scope: AnalysisScope, fields: readonly ScopeField[]): ScopeField[] {
  return visibleGroups(fields).flatMap((group) => groupFields(group, fields).filter((field) => scopeFieldActive(scope, field)));
}

function groupActive(scope: AnalysisScope, group: FilterGroup, fields: readonly ScopeField[]): boolean {
  if (group.matchup === true) return scopeFieldActive(scope, 'buy') || scopeFieldActive(scope, 'opponentBuy');
  return groupFields(group, fields).some((field) => scopeFieldActive(scope, field));
}

/** Lower case without accents, so "prorroga" finds "Prórroga". */
function folded(text: string): string {
  return text.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLocaleLowerCase('es');
}

function triggerLabel(count: number): string {
  if (count === 0) return 'Filtros';
  return count === 1 ? '1 filtro' : `${count} filtros`;
}

/**
 * The "Filtros" popover in two columns, like the reference's filter menu: the groups at the left, the
 * controls of the highlighted group at the right. Every scope field the view reads that has no control
 * in the header row lives here.
 */
export function FilterMenu(props: FilterMenuProps): ReactNode {
  const { scope, fields, onChange, onReset } = props;
  const groups = visibleGroups(fields);
  const [picked, setPicked] = useState('');
  const [term, setTerm] = useState('');
  if (groups.length === 0) return null;
  const active = menuActiveFields(scope, fields);
  const shown = term.trim() === '' ? groups : groups.filter((group) => folded(group.label).includes(folded(term.trim())));
  const current = shown.find((group) => group.id === picked) ?? shown.find((group) => groupActive(scope, group, fields)) ?? shown[0];
  const body = current === undefined ? [] : groupFields(current, fields);
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" size="sm" className={cn(FIELD, 'font-medium normal-case tracking-normal', active.length > 0 && FIELD_SET)}>
          <Filter aria-hidden className={cn('size-3.5', active.length > 0 ? 'text-primary' : 'text-fg-3')} />
          {triggerLabel(active.length)}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" aria-label="Filtros" className="flex max-h-[28rem] w-[min(36rem,92vw)] p-0">
        <div className="flex w-48 shrink-0 flex-col border-r border-border">
          <div className="p-1">
            <Input type="search" aria-label="Buscar un filtro" placeholder="Buscar un filtro" value={term} onChange={(event) => setTerm(event.target.value)} className="h-7 text-body-sm" />
          </div>
          <ul className="min-h-0 flex-1 overflow-y-auto p-1 pt-0">
            {shown.length === 0 ? <li className="px-2 py-1 text-meta text-fg-3">Ningún filtro con ese nombre</li> : null}
            {shown.map((group) => (
            <li key={group.id}>
              <button
                type="button"
                aria-pressed={current?.id === group.id}
                onClick={() => setPicked(group.id)}
                className={cn(
                  'flex min-h-8 w-full items-center gap-2 rounded-sm px-2 text-left text-body-sm text-fg-2 hover:bg-surface-5 hover:text-fg-1',
                  current?.id === group.id && 'bg-surface-3 text-fg-1',
                )}
              >
                <span className="flex-1 truncate">{group.label}</span>
                {groupActive(scope, group, fields) ? <span role="img" aria-label="Con filtro" className="size-1.5 shrink-0 rounded-full bg-primary" /> : null}
              </button>
            </li>
          ))}
          </ul>
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2 overflow-y-auto p-2">
          {current === undefined ? null : (
            <>
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-meta font-medium tracking-wider text-fg-3 uppercase">{current.label}</h3>
                {groupActive(scope, current, fields) ? (
                  <Button type="button" variant="ghost" size="xs" onClick={() => onReset(current.matchup === true ? ['buy', 'opponentBuy'] : body)}>
                    Restablecer
                  </Button>
                ) : null}
              </div>
              {current.matchup === true ? (
                <div className="flex items-center gap-1.5">
                  <BuyMatchupSelect scope={scope} onChange={onChange} />
                  <BuySwapButton scope={scope} onChange={onChange} />
                </div>
              ) : null}
              {body.map((field) => (
                <div key={field} className="flex flex-wrap items-center gap-1.5">
                  <MenuField field={field} scope={scope} onChange={onChange} opponents={props.opponents} tactics={props.tactics} ownTeams={props.ownTeams} />
                </div>
              ))}
            </>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
