'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Check, ChevronDown, Shield, User } from 'lucide-react';
import { switchEntityHref } from '@/lib/analysis/context-bar';
import { searchOptions } from '@/lib/analysis/entity-search';
import { recentTeamOptions, rememberRecentTeam } from '@/lib/analysis/recent-teams';
import type { ScopeEntity, ScopeOption } from '@/components/analysis/scope-bar';
import { TEAM_RED } from '@/components/analysis/scope-field-style';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

const ROW = 'flex min-h-9 w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-body-sm text-popover-foreground outline-hidden transition-colors duration-(--dur-instant) ease-standard hover:bg-surface-5 focus-visible:bg-surface-5';

type RowsProps = { title: string; options: readonly ScopeOption[]; openId: string; onPick: (id: string) => void };

function Rows({ title, options, openId, onPick }: RowsProps): ReactNode {
  return (
    <div role="listbox" aria-label={title} className="flex flex-col gap-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="option"
          tabIndex={-1}
          aria-selected={option.value === openId}
          className={cn(ROW, option.value === openId && 'text-fg-1')}
          onClick={() => onPick(option.value)}
        >
          <span className="truncate">{option.label}</span>
          {option.value === openId ? <Check aria-hidden className="size-4 shrink-0 text-fg-1" /> : null}
        </button>
      ))}
    </div>
  );
}

/**
 * The team or player the page is about, switchable without leaving the tab:
 * for teams a "Recientes" group with the last four others opened, then a search box.
 */
export function EntityPicker({ entity }: { entity: ScopeEntity }): ReactNode {
  const router = useRouter();
  const query = useSearchParams().toString();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState('');
  const [recent, setRecent] = useState<string[]>([]);
  const content = useRef<HTMLDivElement>(null);
  const remembers = entity.recents === true;
  const known = entity.options.map((option) => option.value).join(',');

  useEffect(() => {
    // Until the other teams load the list holds only this one, which would prune every recent.
    if (remembers && known.includes(',')) setRecent(rememberRecentTeam({ id: entity.id, known: known.split(',') }));
  }, [entity.id, remembers, known]);

  if (entity.options.length < 2) {
    const only = entity.options.find((option) => option.value === entity.id);
    return only ? <span className="px-1 text-body-sm font-semibold text-fg-1">{only.label}</span> : null;
  }

  const current = entity.options.find((option) => option.value === entity.id);
  const found = searchOptions(entity.options, text);
  const recents = remembers ? recentTeamOptions(recent, entity.options, entity.id) : [];
  const noun = entity.noun.toLowerCase();
  const team = entity.tone === 'team';

  function pick(id: string): void {
    setOpen(false);
    if (id === entity.id) return;
    if (entity.onSelect) entity.onSelect(id);
    else router.push(switchEntityHref(entity.hrefFor(id), query));
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    const stops = Array.from(content.current?.querySelectorAll<HTMLElement>('input, [role="option"]') ?? []);
    const at = stops.findIndex((stop) => stop === document.activeElement);
    const next = event.key === 'ArrowDown' ? Math.min(at + 1, stops.length - 1) : Math.max(at - 1, 0);
    stops[next]?.focus();
    event.preventDefault();
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) setText('');
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={entity.noun}
          className={cn('h-7 max-w-56 min-w-36 justify-between gap-1.5 px-2 text-body-sm normal-case tracking-normal', team && TEAM_RED)}
        >
          {team ? <Shield aria-hidden className="size-3.5 shrink-0 text-fg-3" /> : <User aria-hidden className="size-3.5 shrink-0 text-fg-3" />}
          <span className="flex-1 truncate text-left">{current?.label}</span>
          <ChevronDown aria-hidden className="size-4 shrink-0 text-fg-3" />
        </Button>
      </PopoverTrigger>
      <PopoverContent ref={content} onKeyDown={onKeyDown} className="flex max-h-80 flex-col gap-1 overflow-y-auto">
        {recents.length > 0 ? (
          <section className="flex flex-col">
            <h3 className="px-2 py-1.5 text-label font-medium text-fg-3">Recientes</h3>
            <Rows title="Recientes" options={recents} openId={entity.id} onPick={pick} />
          </section>
        ) : null}
        <Input
          autoFocus
          value={text}
          placeholder={`Buscar ${noun}`}
          aria-label={`Buscar ${noun}`}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && found[0]) pick(found[0].value);
          }}
          className="h-8 px-2.5 text-body-sm md:text-body-sm"
        />
        <Rows title={entity.noun} options={found} openId={entity.id} onPick={pick} />
        <p role="status" className={cn('px-2 py-3 text-body-sm text-fg-2', found.length > 0 && 'sr-only')}>
          {found.length === 0 ? `Ningún ${noun} coincide con la búsqueda.` : ''}
        </p>
      </PopoverContent>
    </Popover>
  );
}
