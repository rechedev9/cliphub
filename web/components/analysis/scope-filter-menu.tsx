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
  if (groups.length === 0) return null;
  const active = menuActiveFields(scope, fields);
  const current = groups.find((group) => group.id === picked) ?? groups.find((group) => groupActive(scope, group, fields)) ?? groups[0];
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
        <ul className="w-48 shrink-0 overflow-y-auto border-r border-border p-1">
          {groups.map((group) => (
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
