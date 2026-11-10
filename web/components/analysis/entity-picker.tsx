'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Check, ChevronDown } from 'lucide-react';
import { switchEntityHref } from '@/lib/analysis/context-bar';
import { searchOptions } from '@/lib/analysis/entity-search';
import { recentTeamOptions, rememberRecentTeam } from '@/lib/analysis/recent-teams';
import type { ScopeEntity, ScopeOption } from '@/components/analysis/scope-bar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

const ROW = 'flex min-h-9 w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-body-sm text-popover-foreground outline-hidden transition-colors duration-(--dur-instant) ease-standard hover:bg-surface-5 focus-visible:bg-surface-5';

function Group({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <div role="group" aria-label={title} className="flex flex-col">
      <p className="px-2 py-1.5 text-label font-medium text-fg-3">{title}</p>
      {children}
    </div>
  );
}

function Row({ option, current, onPick }: { option: ScopeOption; current: boolean; onPick: (id: string) => void }): ReactNode {
  return (
    <button type="button" role="option" aria-selected={current} className={cn(ROW, current && 'text-fg-1')} onClick={() => onPick(option.value)}>
      <span className="truncate">{option.label}</span>
      {current ? <Check aria-hidden className="size-4 shrink-0 text-fg-1" /> : null}
    </button>
  );
}

/**
 * The team or player the page is about, switchable without leaving the tab:
 * a search box and, for teams, a "Recientes" group with the last four opened.
 */
export function EntityPicker({ entity }: { entity: ScopeEntity }): ReactNode {
  const router = useRouter();
  const query = useSearchParams().toString();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState('');
  const [recent, setRecent] = useState<string[]>([]);
  const list = useRef<HTMLDivElement>(null);
  const remembers = entity.recents === true;

  useEffect(() => {
    if (remembers) setRecent(rememberRecentTeam(entity.id));
  }, [entity.id, remembers]);

  if (entity.options.length < 2) {
    const only = entity.options.find((option) => option.value === entity.id);
    return only ? <span className="px-1 text-body-sm font-semibold text-fg-1">{only.label}</span> : null;
  }

  const current = entity.options.find((option) => option.value === entity.id);
  const searching = text.trim() !== '';
  const found = searchOptions(entity.options, text);
  const recents = remembers && !searching ? recentTeamOptions(recent, entity.options) : [];
  const noun = entity.noun.toLowerCase();
  const everyRow = found.map((option) => <Row key={option.value} option={option} current={option.value === entity.id} onPick={pick} />);

  function pick(id: string): void {
    setOpen(false);
    if (id !== entity.id) router.push(switchEntityHref(entity.hrefFor(id), query));
  }

  function rows(): HTMLButtonElement[] {
    return Array.from(list.current?.querySelectorAll('button') ?? []);
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
    const buttons = rows();
    const at = buttons.findIndex((button) => button === document.activeElement);
    if (event.key === 'ArrowDown') buttons[Math.min(at + 1, buttons.length - 1)]?.focus();
    else if (event.key === 'ArrowUp' && at > 0) buttons[at - 1]?.focus();
    else if (event.key === 'ArrowUp') event.currentTarget.querySelector('input')?.focus();
    else return;
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
        <Button type="button" variant="outline" size="xs" aria-label={entity.noun} aria-haspopup="listbox" className="max-w-56 justify-between gap-1.5 text-body-sm">
          <span className="truncate">{current?.label}</span>
          <ChevronDown aria-hidden className="size-4 shrink-0 text-fg-3" />
        </Button>
      </PopoverTrigger>
      <PopoverContent onKeyDown={onKeyDown} className="flex max-h-80 flex-col gap-1">
        <Input
          autoFocus
          value={text}
          placeholder={`Buscar ${noun}`}
          aria-label={`Buscar ${noun}`}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== 'Enter') return;
            const first = recents[0] ?? found[0];
            if (first) pick(first.value);
          }}
          className="h-8 px-2.5 text-body-sm md:text-body-sm"
        />
        <div ref={list} role="listbox" aria-label={entity.noun} className="flex min-h-0 flex-col gap-0.5 overflow-y-auto">
          {recents.length > 0 ? (
            <Group title="Recientes">
              {recents.map((option) => (
                <Row key={option.value} option={option} current={option.value === entity.id} onPick={pick} />
              ))}
            </Group>
          ) : null}
          {found.length === 0 ? <p className="px-2 py-3 text-body-sm text-fg-2">Ningún {noun} coincide con la búsqueda.</p> : null}
          {recents.length > 0 ? <Group title="Todos">{everyRow}</Group> : everyRow}
        </div>
      </PopoverContent>
    </Popover>
  );
}
